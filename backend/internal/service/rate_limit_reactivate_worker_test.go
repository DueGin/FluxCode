//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type usageRefresherStub struct {
	calls    int
	accounts []*Account
}

func (s *usageRefresherStub) RefreshAccounts(ctx context.Context, accounts []*Account) []UsageRefreshResult {
	s.calls++
	s.accounts = accounts
	return nil
}

func TestRateLimitReactivateWorker_refreshCandidatesOnlyPicks429DueAccounts(t *testing.T) {
	now := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	future := now.Add(time.Second)
	expiredAt := now.Add(-time.Hour)

	stub := &usageRefresherStub{}
	w := &RateLimitReactivateWorker{
		refresher: stub,
		now:       func() time.Time { return now },
	}

	accounts := []*Account{
		{
			ID:                    1,
			Status:                StatusActive,
			Schedulable:           false,
			RateLimitResetAt:      &past,
			TempUnschedulableReason: "Gemini 429 限流，已取消调度",
		},
		{
			ID:                    2,
			Status:                StatusActive,
			Schedulable:           false,
			RateLimitResetAt:      &future,
			TempUnschedulableReason: "Gemini 429 限流，已取消调度",
		},
		{
			ID:                    3,
			Status:                StatusActive,
			Schedulable:           false,
			RateLimitResetAt:      &past,
			TempUnschedulableReason: "已手动关闭调度开关",
		},
		{
			ID:                    4,
			Status:                StatusError,
			Schedulable:           false,
			RateLimitResetAt:      &past,
			TempUnschedulableReason: "Gemini 429 限流，已取消调度",
		},
		{
			ID:                    5,
			Status:                StatusActive,
			Schedulable:           false,
			RateLimitResetAt:      &past,
			TempUnschedulableReason: "Gemini 429 限流，已取消调度",
			ExpiresAt:             &expiredAt,
		},
	}

	w.refreshCandidates(context.Background(), accounts)

	require.Equal(t, 1, stub.calls)
	require.Len(t, stub.accounts, 1)
	require.Equal(t, int64(1), stub.accounts[0].ID)
}

