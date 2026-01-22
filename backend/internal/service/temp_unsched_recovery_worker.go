package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
)

// TempUnschedRecoveryWorker re-checks usage windows after cooldown and clears temp unsched when quotas recover.
// It only targets usage-window-based temp unsched records.
type TempUnschedRecoveryWorker struct {
	db               *sql.DB
	timingWheel      *TimingWheelService
	refreshInterval  time.Duration
	pollInterval     time.Duration
	refreshWindow    time.Duration
	retryDelay       time.Duration
	leaseDuration    time.Duration
	batchSize        int
	accountRepo      AccountRepository
	usageService     *AccountUsageService
	rateLimitService *RateLimitService
	cache            TempUnschedRecoveryCache
}

const tempUnschedRecoveryBatchSize = 200

func NewTempUnschedRecoveryWorker(
	db *sql.DB,
	timingWheel *TimingWheelService,
	accountRepo AccountRepository,
	usageService *AccountUsageService,
	rateLimitService *RateLimitService,
	cache TempUnschedRecoveryCache,
	refreshInterval time.Duration,
	pollInterval time.Duration,
) *TempUnschedRecoveryWorker {
	if refreshInterval <= 0 {
		refreshInterval = time.Hour
	}
	if pollInterval <= 0 {
		pollInterval = 5 * time.Minute
	}
	lease := pollInterval * 2
	if lease <= 0 {
		lease = 10 * time.Minute
	}
	if lease < time.Minute {
		lease = time.Minute
	}
	return &TempUnschedRecoveryWorker{
		db:               db,
		timingWheel:      timingWheel,
		refreshInterval:  refreshInterval,
		pollInterval:     pollInterval,
		refreshWindow:    time.Hour,
		retryDelay:       5 * time.Minute,
		leaseDuration:    lease,
		batchSize:        tempUnschedRecoveryBatchSize,
		accountRepo:      accountRepo,
		usageService:     usageService,
		rateLimitService: rateLimitService,
		cache:            cache,
	}
}

func (w *TempUnschedRecoveryWorker) Start() {
	if w == nil || w.timingWheel == nil {
		return
	}
	if w.cache == nil {
		applog.Printf("[TempUnschedRecoveryWorker] disabled: cache is nil")
		return
	}
	w.refreshCandidates()
	w.timingWheel.ScheduleRecurring("worker:temp_unsched_recovery_refresh", w.refreshInterval, w.refreshCandidates)
	w.timingWheel.ScheduleRecurring("worker:temp_unsched_recovery_poll", w.pollInterval, w.pollCandidates)
	applog.Printf("[TempUnschedRecoveryWorker] Started (refresh: %v, poll: %v)", w.refreshInterval, w.pollInterval)
}

func (w *TempUnschedRecoveryWorker) Stop() {
	if w == nil || w.timingWheel == nil {
		return
	}
	w.timingWheel.Cancel("worker:temp_unsched_recovery_refresh")
	w.timingWheel.Cancel("worker:temp_unsched_recovery_poll")
	applog.Printf("[TempUnschedRecoveryWorker] Stopped")
}

func (w *TempUnschedRecoveryWorker) refreshCandidates() {
	if w == nil || w.db == nil || w.cache == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	refreshSeconds := int64(w.refreshWindow.Seconds())
	if refreshSeconds <= 0 {
		refreshSeconds = int64(time.Hour.Seconds())
	}
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, temp_unschedulable_until
		FROM accounts
		WHERE deleted_at IS NULL
			AND temp_unschedulable_until IS NOT NULL
			AND temp_unschedulable_until <= NOW() + ($1 * INTERVAL '1 second')
			AND temp_unschedulable_reason LIKE '%额度已超限%'
	`, refreshSeconds)
	if err != nil {
		applog.Printf("[TempUnschedRecoveryWorker] query failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	entries := make([]TempUnschedRecoveryEntry, 0, 128)
	for rows.Next() {
		var id int64
		var until time.Time
		if err := rows.Scan(&id, &until); err != nil {
			applog.Printf("[TempUnschedRecoveryWorker] scan failed: %v", err)
			return
		}
		if id > 0 && !until.IsZero() {
			entries = append(entries, TempUnschedRecoveryEntry{AccountID: id, Until: until})
		}
	}
	if err := rows.Err(); err != nil {
		applog.Printf("[TempUnschedRecoveryWorker] rows error: %v", err)
		return
	}

	if len(entries) == 0 {
		return
	}
	if err := w.cache.AddCandidates(ctx, entries); err != nil {
		applog.Printf("[TempUnschedRecoveryWorker] cache add failed: %v", err)
		return
	}
	applog.Printf("[TempUnschedRecoveryWorker] refreshed candidates: %d", len(entries))
}

func (w *TempUnschedRecoveryWorker) pollCandidates() {
	if w == nil || w.accountRepo == nil || w.rateLimitService == nil || w.cache == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	now := time.Now()
	if _, err := w.cache.RequeueExpired(ctx, now, w.retryDelay, w.batchSize); err != nil {
		applog.Printf("[TempUnschedRecoveryWorker] requeue expired failed: %v", err)
	}

	for {
		now = time.Now()
		ids, err := w.cache.ClaimDue(ctx, now, w.batchSize, w.leaseDuration)
		if err != nil {
			applog.Printf("[TempUnschedRecoveryWorker] claim failed: %v", err)
			return
		}
		if len(ids) == 0 {
			return
		}

		for _, id := range ids {
			nextUntil := w.checkAccount(ctx, id, now)
			if nextUntil == nil {
				if err := w.cache.Ack(ctx, id); err != nil {
					applog.Printf("[TempUnschedRecoveryWorker] ack failed for account %d: %v", id, err)
				}
				continue
			}
			if err := w.cache.Requeue(ctx, id, *nextUntil); err != nil {
				applog.Printf("[TempUnschedRecoveryWorker] requeue failed for account %d: %v", id, err)
			}
		}

		if len(ids) < w.batchSize {
			return
		}
	}
}

func (w *TempUnschedRecoveryWorker) checkAccount(ctx context.Context, accountID int64, now time.Time) *time.Time {
	account, err := w.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil
		}
		applog.Printf("[TempUnschedRecoveryWorker] load account %d failed: %v", accountID, err)
		until := now.Add(w.retryDelay)
		return w.trackUntil(&until, now)
	}
	if account.TempUnschedulableUntil == nil {
		return nil
	}
	if !strings.Contains(account.TempUnschedulableReason, "额度已超限") {
		return nil
	}

	if account.TempUnschedulableUntil.After(now) {
		return w.trackUntil(account.TempUnschedulableUntil, now)
	}
	windows, exceeded, err := w.collectUsageWindows(ctx, account, now)
	if err != nil {
		applog.Printf("[TempUnschedRecoveryWorker] account %d usage check failed: %v", account.ID, err)
		until := now.Add(w.retryDelay)
		w.extendTempUnsched(ctx, account.ID, until, "额度已超限（用量检查失败，稍后重试）")
		return w.trackUntil(&until, now)
	}
	if len(windows) == 0 {
		applog.Printf("[TempUnschedRecoveryWorker] account %d usage windows empty, retry later", account.ID)
		until := now.Add(w.retryDelay)
		w.extendTempUnsched(ctx, account.ID, until, "额度已超限（用量窗口不可用，稍后重试）")
		return w.trackUntil(&until, now)
	}
	if len(exceeded) == 0 {
		if err := w.rateLimitService.ClearTempUnschedulable(ctx, account.ID); err != nil {
			applog.Printf("[TempUnschedRecoveryWorker] clear temp unsched failed for account %d: %v", account.ID, err)
		}
		return nil
	}

	until := selectLatestReset(exceeded, now, w.retryDelay)
	reason := buildUsageExceededReason(account.Platform, exceeded)
	if err := w.rateLimitService.SetTempUnschedulableWithReason(ctx, account.ID, until, reason); err != nil {
		applog.Printf("[TempUnschedRecoveryWorker] set temp unsched failed for account %d: %v", account.ID, err)
	}
	return w.trackUntil(&until, now)
}

func (w *TempUnschedRecoveryWorker) trackUntil(until *time.Time, now time.Time) *time.Time {
	if !w.shouldTrack(until, now) {
		return nil
	}
	return until
}

func (w *TempUnschedRecoveryWorker) shouldTrack(until *time.Time, now time.Time) bool {
	if until == nil {
		return false
	}
	if w.refreshWindow <= 0 {
		return true
	}
	limit := now.Add(w.refreshWindow)
	return !until.After(limit)
}

type usageWindow struct {
	name  string
	used  float64
	reset *time.Time
}

func (w *TempUnschedRecoveryWorker) collectUsageWindows(ctx context.Context, account *Account, now time.Time) ([]usageWindow, []usageWindow, error) {
	if account.Platform == PlatformOpenAI {
		windows, exceeded := codexUsageWindows(account, now)
		return windows, exceeded, nil
	}
	if w.usageService == nil {
		return nil, nil, nil
	}
	usage, err := w.usageService.GetUsage(ctx, account.ID)
	if err != nil {
		return nil, nil, err
	}
	windows := usageWindowsFromUsageInfo(usage)
	exceeded := exceededWindows(windows)
	return windows, exceeded, nil
}

func usageWindowsFromUsageInfo(usage *UsageInfo) []usageWindow {
	if usage == nil {
		return nil
	}
	windows := make([]usageWindow, 0, 3)
	if usage.FiveHour != nil {
		windows = append(windows, usageWindow{
			name:  "5h",
			used:  usage.FiveHour.Utilization,
			reset: usage.FiveHour.ResetsAt,
		})
	}
	if usage.SevenDay != nil {
		windows = append(windows, usageWindow{
			name:  "7d",
			used:  usage.SevenDay.Utilization,
			reset: usage.SevenDay.ResetsAt,
		})
	}
	if usage.SevenDaySonnet != nil {
		windows = append(windows, usageWindow{
			name:  "7d_sonnet",
			used:  usage.SevenDaySonnet.Utilization,
			reset: usage.SevenDaySonnet.ResetsAt,
		})
	}
	return windows
}

func exceededWindows(windows []usageWindow) []usageWindow {
	if len(windows) == 0 {
		return nil
	}
	var exceeded []usageWindow
	for _, w := range windows {
		if w.used >= 100 {
			exceeded = append(exceeded, w)
		}
	}
	return exceeded
}

func selectLatestReset(exceeded []usageWindow, now time.Time, fallback time.Duration) time.Time {
	var until *time.Time
	for _, w := range exceeded {
		if w.reset == nil || !w.reset.After(now) {
			continue
		}
		if until == nil || w.reset.After(*until) {
			u := *w.reset
			until = &u
		}
	}
	if until == nil {
		u := now.Add(fallback)
		return u
	}
	return *until
}

func buildUsageExceededReason(platform string, exceeded []usageWindow) string {
	platformLabel := "账号"
	switch platform {
	case PlatformAnthropic:
		platformLabel = "Anthropic"
	case PlatformOpenAI:
		platformLabel = "OpenAI"
	case PlatformGemini:
		platformLabel = "Gemini"
	case PlatformAntigravity:
		platformLabel = "Antigravity"
	}

	var b strings.Builder
	b.WriteString(platformLabel)
	b.WriteString(" 额度已超限：")
	for i, w := range exceeded {
		if i > 0 {
			b.WriteString("；")
		}
		b.WriteString(w.name)
		b.WriteString(" 已用 ")
		b.WriteString(strconv.FormatFloat(w.used, 'f', 1, 64))
		b.WriteString("%")
		if w.reset != nil {
			b.WriteString("，预计 ")
			b.WriteString(w.reset.Format(time.RFC3339))
			b.WriteString(" 恢复")
		} else {
			b.WriteString("（重置时间未知）")
		}
	}
	return strings.TrimSpace(b.String())
}

func codexUsageWindows(account *Account, now time.Time) ([]usageWindow, []usageWindow) {
	if account == nil || account.Extra == nil {
		return nil, nil
	}
	baseTime := now
	if t, ok := getExtraTime(account.Extra, "codex_usage_updated_at"); ok {
		baseTime = t
	}

	windows := make([]usageWindow, 0, 2)
	exceeded := make([]usageWindow, 0, 2)

	if used, ok := getExtraFloat(account.Extra, "codex_5h_used_percent"); ok {
		if resetSeconds, okReset := getExtraInt(account.Extra, "codex_5h_reset_after_seconds"); okReset && resetSeconds > 0 {
			resetAt := baseTime.Add(time.Duration(resetSeconds) * time.Second)
			w := usageWindow{name: "5h", used: used, reset: &resetAt}
			windows = append(windows, w)
			if used >= 100 && resetAt.After(now) {
				exceeded = append(exceeded, w)
			}
		}
	}

	if used, ok := getExtraFloat(account.Extra, "codex_7d_used_percent"); ok {
		if resetSeconds, okReset := getExtraInt(account.Extra, "codex_7d_reset_after_seconds"); okReset && resetSeconds > 0 {
			resetAt := baseTime.Add(time.Duration(resetSeconds) * time.Second)
			w := usageWindow{name: "7d", used: used, reset: &resetAt}
			windows = append(windows, w)
			if used >= 100 && resetAt.After(now) {
				exceeded = append(exceeded, w)
			}
		}
	}

	return windows, exceeded
}

func getExtraFloat(extra map[string]any, key string) (float64, bool) {
	if extra == nil {
		return 0, false
	}
	v, ok := extra[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func getExtraInt(extra map[string]any, key string) (int, bool) {
	if extra == nil {
		return 0, false
	}
	v, ok := extra[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return int(i), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return i, true
		}
	}
	return 0, false
}

func getExtraTime(extra map[string]any, key string) (time.Time, bool) {
	if extra == nil {
		return time.Time{}, false
	}
	v, ok := extra[key]
	if !ok || v == nil {
		return time.Time{}, false
	}
	switch val := v.(type) {
	case time.Time:
		return val, true
	case string:
		s := strings.TrimSpace(val)
		if s == "" {
			return time.Time{}, false
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, true
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (w *TempUnschedRecoveryWorker) extendTempUnsched(ctx context.Context, accountID int64, until time.Time, reason string) {
	if w.rateLimitService == nil {
		return
	}
	if err := w.rateLimitService.SetTempUnschedulableWithReason(ctx, accountID, until, reason); err != nil {
		applog.Printf("[TempUnschedRecoveryWorker] extend temp unsched failed for account %d: %v", accountID, err)
	}
}
