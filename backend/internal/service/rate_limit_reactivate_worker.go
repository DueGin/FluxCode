package service

import (
	"context"
	"database/sql"
	"strings"
	"time"

	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
)

// UsageRefresher is implemented by workers that can refresh usage windows for accounts.
// DailyUsageRefreshWorker satisfies this interface.
type UsageRefresher interface {
	RefreshAccounts(ctx context.Context, accounts []*Account) []UsageRefreshResult
}

// RateLimitReactivateWorker re-checks accounts disabled by 429 and re-enables scheduling after reset.
// For multi-host deployments, it uses PostgreSQL advisory lock to ensure only one instance runs.
type RateLimitReactivateWorker struct {
	db         *sql.DB
	timingWheel *TimingWheelService
	interval   time.Duration
	batchSize  int

	accountRepo AccountRepository
	refresher   UsageRefresher

	now func() time.Time
}

const rateLimitReactivateAdvisoryLockID int64 = 74298347004

func NewRateLimitReactivateWorker(
	db *sql.DB,
	timingWheel *TimingWheelService,
	accountRepo AccountRepository,
	refresher UsageRefresher,
	interval time.Duration,
) *RateLimitReactivateWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &RateLimitReactivateWorker{
		db:          db,
		timingWheel: timingWheel,
		interval:    interval,
		batchSize:   200,
		accountRepo: accountRepo,
		refresher:   refresher,
		now:         time.Now,
	}
}

func (w *RateLimitReactivateWorker) Start() {
	if w == nil || w.timingWheel == nil {
		return
	}
	w.timingWheel.ScheduleRecurring("worker:rate_limit_reactivate", w.interval, w.runOnce)
	applog.Printf("[RateLimitReactivateWorker] Started (interval: %v)", w.interval)
}

func (w *RateLimitReactivateWorker) Stop() {
	if w == nil || w.timingWheel == nil {
		return
	}
	w.timingWheel.Cancel("worker:rate_limit_reactivate")
	applog.Printf("[RateLimitReactivateWorker] Stopped")
}

func (w *RateLimitReactivateWorker) runOnce() {
	if w == nil || w.db == nil || w.accountRepo == nil || w.refresher == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := w.db.Conn(ctx)
	if err != nil {
		applog.Printf("[RateLimitReactivateWorker] db conn failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", rateLimitReactivateAdvisoryLockID).Scan(&locked); err != nil {
		applog.Printf("[RateLimitReactivateWorker] acquire lock failed: %v", err)
		return
	}
	if !locked {
		return
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", rateLimitReactivateAdvisoryLockID)
	}()

	ids, err := w.listCandidateIDs(ctx, conn)
	if err != nil {
		applog.Printf("[RateLimitReactivateWorker] list candidates failed: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	accounts, err := w.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		applog.Printf("[RateLimitReactivateWorker] load accounts failed: %v", err)
		return
	}
	w.refreshCandidates(ctx, accounts)
}

func (w *RateLimitReactivateWorker) listCandidateIDs(ctx context.Context, conn *sql.Conn) ([]int64, error) {
	if w == nil || conn == nil {
		return nil, nil
	}
	limit := w.batchSize
	if limit <= 0 {
		limit = 200
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT id
		FROM accounts
		WHERE deleted_at IS NULL
			AND status = 'active'
			AND schedulable = FALSE
			AND rate_limit_reset_at IS NOT NULL
			AND rate_limit_reset_at <= NOW()
			AND COALESCE(temp_unschedulable_reason, '') ILIKE '%429%'
		ORDER BY rate_limit_reset_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (w *RateLimitReactivateWorker) refreshCandidates(ctx context.Context, accounts []*Account) {
	if w == nil || w.refresher == nil || len(accounts) == 0 {
		return
	}
	now := time.Now()
	if w.now != nil {
		now = w.now()
	}

	candidates := make([]*Account, 0, len(accounts))
	for _, account := range accounts {
		if is429ReactivateCandidate(account, now) {
			candidates = append(candidates, account)
		}
	}
	if len(candidates) == 0 {
		return
	}

	results := w.refresher.RefreshAccounts(ctx, candidates)
	applog.Printf("[RateLimitReactivateWorker] refreshed=%d results=%d", len(candidates), len(results))
}

func is429ReactivateCandidate(account *Account, now time.Time) bool {
	if account == nil {
		return false
	}
	if account.Status != StatusActive {
		return false
	}
	if account.Schedulable {
		return false
	}
	if account.ExpiresAt != nil && !account.ExpiresAt.After(now) {
		return false
	}
	if account.RateLimitResetAt == nil || account.RateLimitResetAt.After(now) {
		return false
	}
	if !strings.Contains(account.TempUnschedulableReason, "429") {
		return false
	}
	return true
}
