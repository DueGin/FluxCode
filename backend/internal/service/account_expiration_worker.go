package service

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// AccountExpirationWorker 定期将已过期账号的 schedulable 开关自动关闭。
// 为支持多机部署，使用 PostgreSQL advisory lock 保证同一时刻只有一个实例执行。
type AccountExpirationWorker struct {
	db          *sql.DB
	timingWheel *TimingWheelService
	interval    time.Duration
}

const accountExpirationAdvisoryLockID int64 = 74298347001

func NewAccountExpirationWorker(db *sql.DB, timingWheel *TimingWheelService, interval time.Duration) *AccountExpirationWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &AccountExpirationWorker{
		db:          db,
		timingWheel: timingWheel,
		interval:    interval,
	}
}

func (w *AccountExpirationWorker) Start() {
	if w == nil || w.timingWheel == nil {
		return
	}
	w.timingWheel.ScheduleRecurring("worker:account_expiration", w.interval, w.runOnce)
	log.Printf("[AccountExpirationWorker] Started (interval: %v)", w.interval)
}

func (w *AccountExpirationWorker) Stop() {
	if w == nil || w.timingWheel == nil {
		return
	}
	w.timingWheel.Cancel("worker:account_expiration")
	log.Printf("[AccountExpirationWorker] Stopped")
}

func (w *AccountExpirationWorker) runOnce() {
	if w == nil || w.db == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := w.db.Conn(ctx)
	if err != nil {
		log.Printf("[AccountExpirationWorker] db conn failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", accountExpirationAdvisoryLockID).Scan(&locked); err != nil {
		log.Printf("[AccountExpirationWorker] acquire lock failed: %v", err)
		return
	}
	if !locked {
		return
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", accountExpirationAdvisoryLockID)
	}()

	res, err := conn.ExecContext(ctx, `
		UPDATE accounts
		SET schedulable = FALSE,
			updated_at = NOW()
		WHERE deleted_at IS NULL
			AND schedulable = TRUE
			AND expires_at IS NOT NULL
			AND expires_at <= NOW()
	`)
	if err != nil {
		log.Printf("[AccountExpirationWorker] disable expired schedulable failed: %v", err)
		return
	}
	affected, _ := res.RowsAffected()
	if affected > 0 {
		log.Printf("[AccountExpirationWorker] Disabled schedulable for %d expired accounts", affected)
	}
}
