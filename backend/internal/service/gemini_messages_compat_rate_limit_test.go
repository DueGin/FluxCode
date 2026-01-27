//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type gemini429AccountRepoSpy struct {
	*accountRepoStub

	setRateLimitedCalls     int
	setRateLimitedAccountID int64
	setRateLimitedResetAt   time.Time

	setUnschedulableCalls     int
	setUnschedulableAccountID int64
	setUnschedulableReason    string
}

func newGemini429AccountRepoSpy() *gemini429AccountRepoSpy {
	return &gemini429AccountRepoSpy{accountRepoStub: &accountRepoStub{}}
}

func (s *gemini429AccountRepoSpy) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	s.setRateLimitedCalls++
	s.setRateLimitedAccountID = id
	s.setRateLimitedResetAt = resetAt
	return nil
}

func (s *gemini429AccountRepoSpy) SetUnschedulableWithReason(ctx context.Context, id int64, reason string) error {
	s.setUnschedulableCalls++
	s.setUnschedulableAccountID = id
	s.setUnschedulableReason = reason
	return nil
}

func TestGeminiMessagesCompatService_handleGeminiUpstreamError_429DoesNotDisableScheduling(t *testing.T) {
	ctx := context.Background()
	repo := newGemini429AccountRepoSpy()
	svc := &GeminiMessagesCompatService{accountRepo: repo}

	account := &Account{
		ID:          42,
		Platform:    PlatformGemini,
		Type:        AccountTypeOAuth,
		Schedulable: true,
	}

	start := time.Now()
	body := []byte(`{"error":{"message":"rate limit","details":[{"metadata":{"quotaResetDelay":"17s"}}]}}`)

	svc.handleGeminiUpstreamError(ctx, account, 429, nil, body)

	require.True(t, account.Schedulable)
	require.NotNil(t, account.RateLimitResetAt)
	require.WithinDuration(t, start.Add(17*time.Second), *account.RateLimitResetAt, 2*time.Second)

	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, account.ID, repo.setRateLimitedAccountID)
	require.WithinDuration(t, start.Add(17*time.Second), repo.setRateLimitedResetAt, 2*time.Second)

	require.Equal(t, 0, repo.setUnschedulableCalls)
}

