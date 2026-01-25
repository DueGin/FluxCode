package service

import (
	"context"
	"database/sql"
	"time"

	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
)

// SubscriptionExpirationWorker 定期将已过期但仍为 active 的订阅标记为 expired。
// 为支持多机部署，使用 PostgreSQL advisory lock 保证同一时刻只有一个实例执行。
type SubscriptionExpirationWorker struct {
	db          *sql.DB
	timingWheel *TimingWheelService
	interval    time.Duration
}

const subscriptionExpirationAdvisoryLockID int64 = 74298347002
const subscriptionExpirationWorkerName = "worker:subscription_expiration"

func NewSubscriptionExpirationWorker(db *sql.DB, timingWheel *TimingWheelService, interval time.Duration) *SubscriptionExpirationWorker {
	if interval <= 0 {
		interval = time.Minute
	}
	return &SubscriptionExpirationWorker{
		db:          db,
		timingWheel: timingWheel,
		interval:    interval,
	}
}

func (w *SubscriptionExpirationWorker) Start() {
	if w == nil || w.timingWheel == nil {
		return
	}
	w.timingWheel.ScheduleRecurring(subscriptionExpirationWorkerName, w.interval, w.runOnce)
	applog.Printf("[SubscriptionExpirationWorker] Started (interval: %v)", w.interval)
}

func (w *SubscriptionExpirationWorker) Stop() {
	if w == nil || w.timingWheel == nil {
		return
	}
	w.timingWheel.Cancel(subscriptionExpirationWorkerName)
	applog.Printf("[SubscriptionExpirationWorker] Stopped")
}

func (w *SubscriptionExpirationWorker) runOnce() {
	if w == nil || w.db == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := w.db.Conn(ctx)
	if err != nil {
		applog.Printf("[SubscriptionExpirationWorker] db conn failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", subscriptionExpirationAdvisoryLockID).Scan(&locked); err != nil {
		applog.Printf("[SubscriptionExpirationWorker] acquire lock failed: %v", err)
		return
	}
	if !locked {
		return
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", subscriptionExpirationAdvisoryLockID)
	}()

	res, err := conn.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET status = $1,
			updated_at = NOW()
		WHERE deleted_at IS NULL
			AND status = $2
			AND expires_at <= NOW()
	`, SubscriptionStatusExpired, SubscriptionStatusActive)
	if err != nil {
		applog.Printf("[SubscriptionExpirationWorker] mark expired failed: %v", err)
		return
	}

	affected, _ := res.RowsAffected()
	if affected > 0 {
		applog.Printf("[SubscriptionExpirationWorker] Marked %d subscriptions expired", affected)
	}
}
