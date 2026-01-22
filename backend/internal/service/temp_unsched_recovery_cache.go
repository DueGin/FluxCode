package service

import (
	"context"
	"time"
)

// TempUnschedRecoveryEntry represents a candidate account for temp unsched recovery.
type TempUnschedRecoveryEntry struct {
	AccountID int64
	Until     time.Time
}

// TempUnschedRecoveryCache coordinates recovery candidates across instances.
type TempUnschedRecoveryCache interface {
	AddCandidates(ctx context.Context, entries []TempUnschedRecoveryEntry) error
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]int64, error)
	RequeueExpired(ctx context.Context, now time.Time, retryDelay time.Duration, limit int) (int, error)
	Requeue(ctx context.Context, accountID int64, until time.Time) error
	Ack(ctx context.Context, accountID int64) error
}
