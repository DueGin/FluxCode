//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type schedulableRepoSpy struct {
	*accountRepoStub

	calls      int
	enabledIDs []int64
}

func (s *schedulableRepoSpy) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	s.calls++
	if schedulable {
		s.enabledIDs = append(s.enabledIDs, id)
	}
	return nil
}

func TestRateLimitReactivateWorker_reactivateCandidatesOnlyPicks429DueAccounts(t *testing.T) {
	now := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	future := now.Add(time.Second)
	expiredAt := now.Add(-time.Hour)

	stub := &schedulableRepoSpy{accountRepoStub: &accountRepoStub{}}
	w := &RateLimitReactivateWorker{
		accountRepo: stub,
		now:         func() time.Time { return now },
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

	w.reactivateCandidates(context.Background(), accounts)

	require.Equal(t, 1, stub.calls)
	require.Equal(t, []int64{1}, stub.enabledIDs)
}
