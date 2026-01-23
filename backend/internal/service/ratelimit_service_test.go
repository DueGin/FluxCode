//go:build unit

package service

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type rateLimitAccountRepoSpy struct {
	*accountRepoStub

	setRateLimitedCalls       int
	setRateLimitedAccountID   int64
	setRateLimitedResetAt     time.Time
	setUnschedulableCalls     int
	setUnschedulableAccountID int64
	setUnschedulableReason    string

	updateSessionWindowCalls     int
	updateSessionWindowAccountID int64
	updateSessionWindowStart     *time.Time
	updateSessionWindowEnd       *time.Time
	updateSessionWindowStatus    string
}

func newRateLimitAccountRepoSpy() *rateLimitAccountRepoSpy {
	return &rateLimitAccountRepoSpy{accountRepoStub: &accountRepoStub{}}
}

func (s *rateLimitAccountRepoSpy) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	s.setRateLimitedCalls++
	s.setRateLimitedAccountID = id
	s.setRateLimitedResetAt = resetAt
	return nil
}

func (s *rateLimitAccountRepoSpy) SetUnschedulableWithReason(ctx context.Context, id int64, reason string) error {
	s.setUnschedulableCalls++
	s.setUnschedulableAccountID = id
	s.setUnschedulableReason = reason
	return nil
}

func (s *rateLimitAccountRepoSpy) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	s.updateSessionWindowCalls++
	s.updateSessionWindowAccountID = id
	s.updateSessionWindowStart = start
	s.updateSessionWindowEnd = end
	s.updateSessionWindowStatus = status
	return nil
}

func TestRateLimitService_HandleUpstreamError_429RateLimitDoesNotDisableScheduling(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          101,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	resetAt := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	headers := make(http.Header)
	headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(resetAt.Unix(), 10))
	body := []byte(`{"error":{"message":"rate limit"}}`)

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.True(t, account.Schedulable)
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, account.ID, repo.setRateLimitedAccountID)
	require.WithinDuration(t, resetAt, repo.setRateLimitedResetAt, time.Second)
	require.Equal(t, 0, repo.setUnschedulableCalls)

	require.Equal(t, 1, repo.updateSessionWindowCalls)
	require.Equal(t, account.ID, repo.updateSessionWindowAccountID)
	require.NotNil(t, repo.updateSessionWindowStart)
	require.NotNil(t, repo.updateSessionWindowEnd)
	require.Equal(t, "rejected", repo.updateSessionWindowStatus)
	require.WithinDuration(t, resetAt.Add(-5*time.Hour), *repo.updateSessionWindowStart, time.Second)
	require.WithinDuration(t, resetAt, *repo.updateSessionWindowEnd, time.Second)

	require.NotNil(t, account.RateLimitResetAt)
	require.WithinDuration(t, resetAt, *account.RateLimitResetAt, time.Second)
}

func TestRateLimitService_HandleUpstreamError_429QuotaExceededDisablesScheduling(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          202,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	headers := make(http.Header)
	headers.Set("x-ratelimit-reset", strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10))
	body := []byte(`{"error":{"code":"insufficient_quota","message":"You exceeded your current quota"}}`)

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.False(t, account.Schedulable)
	require.Equal(t, 0, repo.setRateLimitedCalls)
	require.Equal(t, 1, repo.setUnschedulableCalls)
	require.Equal(t, account.ID, repo.setUnschedulableAccountID)
	require.Contains(t, repo.setUnschedulableReason, "Upstream quota exceeded")
}

func TestRateLimitService_HandleUpstreamError_429UsesGenericResetHeader(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          303,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	start := time.Now()
	headers := make(http.Header)
	headers.Set("x-ratelimit-reset-requests", "17")
	body := []byte(`{"error":{"message":"rate limit"}}`)

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, 0, repo.updateSessionWindowCalls)
	require.WithinDuration(t, start.Add(17*time.Second), repo.setRateLimitedResetAt, 2*time.Second)
}
