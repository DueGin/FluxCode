package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
	"github.com/DueGin/FluxCode/internal/pkg/openai"
)

// DailyUsageRefreshWorker refreshes usage windows once per day at a fixed time.
type DailyUsageRefreshWorker struct {
	db                *sql.DB
	settingService    *SettingService
	accountRepo       AccountRepository
	usageService      *AccountUsageService
	rateLimitService  *RateLimitService
	httpUpstream      HTTPUpstream
	perAccountTimeout time.Duration
	concurrency       int

	stopCh   chan struct{}
	resetCh  chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

const dailyUsageRefreshAdvisoryLockID int64 = 74298347003

func NewDailyUsageRefreshWorker(
	db *sql.DB,
	settingService *SettingService,
	accountRepo AccountRepository,
	usageService *AccountUsageService,
	rateLimitService *RateLimitService,
	httpUpstream HTTPUpstream,
) *DailyUsageRefreshWorker {
	return &DailyUsageRefreshWorker{
		db:                db,
		settingService:    settingService,
		accountRepo:       accountRepo,
		usageService:      usageService,
		rateLimitService:  rateLimitService,
		httpUpstream:      httpUpstream,
		perAccountTimeout: 30 * time.Second,
		concurrency:       5,
		resetCh:           make(chan struct{}, 1),
	}
}

func (w *DailyUsageRefreshWorker) Start() {
	if w == nil || w.db == nil || w.accountRepo == nil || w.usageService == nil {
		return
	}
	if w.stopCh != nil {
		return
	}
	if w.resetCh == nil {
		w.resetCh = make(chan struct{}, 1)
	}
	w.stopCh = make(chan struct{})
	w.wg.Add(1)
	go w.loop()
	applog.Printf("[DailyUsageRefreshWorker] Started")
}

func (w *DailyUsageRefreshWorker) Stop() {
	if w == nil || w.stopCh == nil {
		return
	}
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	applog.Printf("[DailyUsageRefreshWorker] Stopped")
}

func (w *DailyUsageRefreshWorker) ResetSchedule() {
	if w == nil || w.resetCh == nil {
		return
	}
	select {
	case w.resetCh <- struct{}{}:
	default:
	}
}

func (w *DailyUsageRefreshWorker) loop() {
	defer w.wg.Done()
	for {
		nextRun := w.nextRun(time.Now())
		delay := time.Until(nextRun)
		if delay < 0 {
			delay = 0
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			applog.Printf("[DailyUsageRefreshWorker] Tick next_run=%s", nextRun.Format(time.RFC3339))
			w.runOnce()
		case <-w.resetCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			applog.Printf("[DailyUsageRefreshWorker] Schedule reset")
		case <-w.stopCh:
			timer.Stop()
			return
		}
	}
}

func (w *DailyUsageRefreshWorker) nextRun(now time.Time) time.Time {
	schedule := defaultDailyUsageRefreshTime
	if w.settingService != nil {
		schedule = w.settingService.GetDailyUsageRefreshTime(context.Background())
	}
	hour, minute, ok := parseDailyTime(schedule)
	if !ok {
		hour, minute, _ = parseDailyTime(defaultDailyUsageRefreshTime)
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func (w *DailyUsageRefreshWorker) runOnce() {
	ctx := context.Background()
	start := time.Now()
	conn, err := w.db.Conn(ctx)
	if err != nil {
		applog.Printf("[DailyUsageRefreshWorker] db conn failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var locked bool
	if err := conn.QueryRowContext(lockCtx, "SELECT pg_try_advisory_lock($1)", dailyUsageRefreshAdvisoryLockID).Scan(&locked); err != nil {
		applog.Printf("[DailyUsageRefreshWorker] acquire lock failed: %v", err)
		return
	}
	if !locked {
		applog.Printf("[DailyUsageRefreshWorker] RunOnce skipped: lock held")
		return
	}
	applog.Printf("[DailyUsageRefreshWorker] RunOnce started")
	defer func() {
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", dailyUsageRefreshAdvisoryLockID)
	}()

	accounts, err := w.accountRepo.ListActive(ctx)
	if err != nil {
		applog.Printf("[DailyUsageRefreshWorker] list active accounts failed: %v", err)
		return
	}
	if len(accounts) == 0 {
		applog.Printf("[DailyUsageRefreshWorker] RunOnce completed: accounts=0 took=%s", time.Since(start))
		return
	}
	applog.Printf("[DailyUsageRefreshWorker] RunOnce accounts=%d", len(accounts))

	sem := make(chan struct{}, w.concurrency)
	var wg sync.WaitGroup
	for i := range accounts {
		acc := accounts[i]
		wg.Add(1)
		sem <- struct{}{}
		go func(account Account) {
			defer wg.Done()
			defer func() { <-sem }()
			w.refreshAccount(ctx, &account)
		}(acc)
	}
	wg.Wait()
	applog.Printf("[DailyUsageRefreshWorker] RunOnce completed: accounts=%d took=%s", len(accounts), time.Since(start))
}

func (w *DailyUsageRefreshWorker) refreshAccount(ctx context.Context, account *Account) {
	if account == nil {
		return
	}
	start := time.Now()
	action, outcome, detail := w.refreshAccountInternal(ctx, account)
	applog.Printf(
		"[DailyUsageRefreshWorker] account_id=%d name=%q platform=%s type=%s action=%s outcome=%s detail=%s took=%s",
		account.ID,
		account.Name,
		account.Platform,
		account.Type,
		action,
		outcome,
		detail,
		time.Since(start),
	)
}

func (w *DailyUsageRefreshWorker) refreshAccountInternal(ctx context.Context, account *Account) (action string, outcome string, detail string) {
	if account == nil {
		return "refresh", "skipped", "nil_account"
	}

	if account.Platform == PlatformOpenAI {
		return w.refreshCodexUsage(ctx, account)
	}

	action = "usage_refresh"
	if !shouldFetchUsageFromUpstream(account) {
		return action, "skipped", "no_upstream_fetch"
	}

	reqCtx, cancel := context.WithTimeout(ctx, w.perAccountTimeout)
	defer cancel()

	usage, err := w.usageService.GetUsageFresh(reqCtx, account.ID)
	if err != nil {
		return action, "error", "fetch_failed"
	}
	if usage == nil {
		return action, "noop", "no_usage"
	}

	windows := formatUsageInfo(usage)
	if usageExceeded(usage) {
		return action, "noop", "usage_exceeded " + windows
	}

	if account.TempUnschedulableUntil == nil {
		return action, "ok", "usage_ok " + windows
	}
	if !shouldEnableSchedulingByUsage(account.TempUnschedulableReason) {
		return action, "ok", "usage_ok temp_unsched_other_reason " + windows
	}
	if w.rateLimitService == nil {
		return action, "noop", "rate_limit_service_nil " + windows
	}

	if err := w.rateLimitService.ClearTempUnschedulable(reqCtx, account.ID); err != nil {
		return action, "error", "clear_temp_unsched_failed " + windows
	}
	return action, "ok", "cleared_temp_unsched " + windows
}

func (w *DailyUsageRefreshWorker) refreshCodexUsage(ctx context.Context, account *Account) (action string, outcome string, detail string) {
	action = "codex_probe"
	probeCtx, cancel := context.WithTimeout(ctx, w.perAccountTimeout)
	updates, attempted, statusCode, modelID, err := w.probeCodexUsage(probeCtx, account)
	cancel()

	if err != nil {
		if attempted {
			return action, "error", fmt.Sprintf("probe_failed status=%d model=%s err=%s", statusCode, modelID, sanitizeSensitiveText(err.Error()))
		}
		return action, "error", fmt.Sprintf("probe_failed model=%s err=%s", modelID, sanitizeSensitiveText(err.Error()))
	}
	if !attempted {
		action = "codex_refresh"
		return action, "skipped", "probe_not_applicable"
	}

	extraUpdated := false
	if len(updates) > 0 {
		extraUpdated = true
		if account.Extra == nil {
			account.Extra = make(map[string]any)
		}
		for k, v := range updates {
			account.Extra[k] = v
		}
		if w.accountRepo != nil {
			reqCtx, cancel := context.WithTimeout(ctx, w.perAccountTimeout)
			if err := w.accountRepo.UpdateExtra(reqCtx, account.ID, updates); err != nil {
				cancel()
				return action, "error", "update_extra_failed"
			}
			cancel()
		}
	}

	if w.rateLimitService == nil {
		return action, "ok", fmt.Sprintf("probe_ok status=%d model=%s extra_updated=%t", statusCode, modelID, extraUpdated)
	}

	now := time.Now()
	windows, exceeded := codexUsageWindows(account, now)
	windowSummary := formatUsageWindows(windows)
	if len(windows) == 0 {
		return action, "noop", fmt.Sprintf("probe_ok status=%d model=%s no_windows", statusCode, modelID)
	}

	if len(exceeded) == 0 {
		if account.TempUnschedulableUntil == nil {
			return action, "ok", fmt.Sprintf("probe_ok status=%d model=%s extra_updated=%t %s", statusCode, modelID, extraUpdated, windowSummary)
		}
		if !shouldEnableSchedulingByUsage(account.TempUnschedulableReason) {
			return action, "ok", fmt.Sprintf("probe_ok status=%d model=%s temp_unsched_other_reason %s", statusCode, modelID, windowSummary)
		}
		reqCtx, cancel := context.WithTimeout(ctx, w.perAccountTimeout)
		defer cancel()
		if err := w.rateLimitService.ClearTempUnschedulable(reqCtx, account.ID); err != nil {
			return action, "error", "clear_temp_unsched_failed " + windowSummary
		}
		return action, "ok", "cleared_temp_unsched " + windowSummary
	}

	until := selectLatestReset(exceeded, now, 5*time.Minute)
	reason := buildUsageExceededReason(account.Platform, exceeded)
	reqCtx, cancel := context.WithTimeout(ctx, w.perAccountTimeout)
	defer cancel()
	if err := w.rateLimitService.SetTempUnschedulableWithReason(reqCtx, account.ID, until, reason); err != nil {
		return action, "error", "set_temp_unsched_failed " + windowSummary
	}
	return action, "ok", "set_temp_unsched " + windowSummary
}

func (w *DailyUsageRefreshWorker) probeCodexUsage(ctx context.Context, account *Account) (updates map[string]any, attempted bool, statusCode int, modelID string, err error) {
	if account == nil || w.httpUpstream == nil {
		return nil, false, 0, "", nil
	}
	if !account.IsOpenAIOAuth() {
		return nil, false, 0, "", nil
	}

	accessToken := account.GetOpenAIAccessToken()
	if accessToken == "" {
		return nil, false, 0, "", errors.New("missing openai access token")
	}
	modelID = openai.DefaultTestModel
	if len(openai.DefaultModels) > 0 && strings.TrimSpace(openai.DefaultModels[0].ID) != "" {
		modelID = openai.DefaultModels[0].ID
	}
	payload := createOpenAIProbePayload(modelID)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, true, 0, modelID, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", chatgptCodexURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, true, 0, modelID, err
	}
	req.Host = "chatgpt.com"
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+accessToken)
	req.Header.Set("accept", "text/event-stream")
	if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
		req.Header.Set("chatgpt-account-id", chatgptAccountID)
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		req.Header.Set("user-agent", customUA)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := w.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, true, 0, modelID, err
	}
	defer func() { _ = resp.Body.Close() }()
	statusCode = resp.StatusCode

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if w.rateLimitService != nil && shouldHandleCodexProbeStatus(resp.StatusCode) {
			_ = w.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
		}
		msg := strings.TrimSpace(extractUpstreamErrorMessage(body))
		msg = sanitizeSensitiveText(msg)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		if msg != "" {
			return nil, true, resp.StatusCode, modelID, fmt.Errorf("codex probe returned status %d: %s", resp.StatusCode, msg)
		}
		return nil, true, resp.StatusCode, modelID, fmt.Errorf("codex probe returned status %d", resp.StatusCode)
	}

	snapshot := extractCodexUsageHeaders(resp.Header)
	if snapshot == nil {
		return nil, true, resp.StatusCode, modelID, nil
	}
	return buildCodexUsageUpdates(snapshot), true, resp.StatusCode, modelID, nil
}

func createOpenAIProbePayload(modelID string) map[string]any {
	return map[string]any{
		"model": modelID,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": ".",
					},
				},
			},
		},
		"instructions": "You are a helpful assistant.",
		"stream":       true,
		"store":        false,
	}
}

func shouldHandleCodexProbeStatus(statusCode int) bool {
	switch statusCode {
	case 401, 402, 403, 429, 529:
		return true
	default:
		return statusCode >= 500
	}
}

func shouldEnableSchedulingByUsage(reason string) bool {
	r := strings.TrimSpace(reason)
	if r == "" {
		return false
	}
	var state TempUnschedState
	if err := json.Unmarshal([]byte(r), &state); err == nil {
		return state.RuleIndex == -2
	}
	if strings.Contains(r, "额度已超限") {
		return true
	}
	lower := strings.ToLower(r)
	switch {
	case strings.Contains(lower, "upstream quota exceeded"),
		strings.Contains(lower, "insufficient_quota"),
		strings.Contains(lower, "quota_exceeded"),
		strings.Contains(lower, "billing_hard_limit"),
		strings.Contains(lower, "hard_limit_reached"),
		strings.Contains(lower, "account_limit_reached"):
		return true
	default:
		return false
	}
}

func formatUsageInfo(usage *UsageInfo) string {
	if usage == nil {
		return ""
	}
	windows := usageWindowsFromUsageInfo(usage)
	return formatUsageWindows(windows)
}

func formatUsageWindows(windows []usageWindow) string {
	if len(windows) == 0 {
		return "windows=none"
	}
	parts := make([]string, 0, len(windows))
	for _, w := range windows {
		part := w.name + ":" + strconv.FormatFloat(w.used, 'f', 1, 64) + "%"
		if w.reset != nil {
			part += "@" + w.reset.Format(time.RFC3339)
		}
		parts = append(parts, part)
	}
	return "windows=" + strings.Join(parts, ",")
}

func parseDailyTime(value string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

func shouldFetchUsageFromUpstream(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Platform == PlatformGemini || account.Platform == PlatformAntigravity {
		return true
	}
	if account.Type == AccountTypeSetupToken {
		return true
	}
	return account.Platform == PlatformAnthropic && account.Type == AccountTypeOAuth
}

func usageExceeded(usage *UsageInfo) bool {
	if usage == nil {
		return false
	}
	if usageValue(usage.FiveHour) >= 100 {
		return true
	}
	if usageValue(usage.SevenDay) >= 100 {
		return true
	}
	if usageValue(usage.SevenDaySonnet) >= 100 {
		return true
	}
	return false
}
