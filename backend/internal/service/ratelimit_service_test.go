//go:build unit

package service

import (
	"context"
	"fmt"
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

func TestRateLimitService_HandleUpstreamError_429QuotaExceededDoesNotDisableScheduling(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          202,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	resetAt := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	headers := make(http.Header)
	headers.Set("x-ratelimit-reset", strconv.FormatInt(resetAt.Unix(), 10))
	body := []byte(`{"error":{"code":"insufficient_quota","message":"You exceeded your current quota"}}`)

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.True(t, account.Schedulable)
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, account.ID, repo.setRateLimitedAccountID)
	require.WithinDuration(t, resetAt, repo.setRateLimitedResetAt, time.Second)
	require.Equal(t, 0, repo.setUnschedulableCalls)

	require.NotNil(t, account.RateLimitResetAt)
	require.WithinDuration(t, resetAt, *account.RateLimitResetAt, time.Second)
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

func TestRateLimitService_HandleUpstreamError_429QuotaExceeded_UsesRetryAfterSecondsAsResetAt(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          404,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	start := time.Now()
	headers := make(http.Header)
	headers.Set("Retry-After", "17")
	body := []byte(`{"error":{"code":"insufficient_quota","message":"The usage limit has been reached"}}`)

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.True(t, account.Schedulable)
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, 0, repo.setUnschedulableCalls)
	require.WithinDuration(t, start.Add(17*time.Second), repo.setRateLimitedResetAt, 2*time.Second)
}

func TestRateLimitService_HandleUpstreamError_429QuotaExceeded_UsesRetryAfterHTTPDateAsResetAt(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          505,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	expectedResetAt := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	headers := make(http.Header)
	headers.Set("Retry-After", expectedResetAt.Format(http.TimeFormat))
	body := []byte(`{"error":{"code":"insufficient_quota","message":"The usage limit has been reached"}}`)

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.True(t, account.Schedulable)
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, 0, repo.setUnschedulableCalls)
	require.WithinDuration(t, expectedResetAt, repo.setRateLimitedResetAt, time.Second)
}

func TestRateLimitService_HandleUpstreamError_429QuotaExceeded_UsesResetsAtFromBodyAsResetAt(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          707,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Schedulable: true,
	}

	expectedResetAt := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	var headers http.Header
	body := []byte(fmt.Sprintf(`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","plan_type":"plus","resets_at":%d,"resets_in_seconds":95505}}`, expectedResetAt.Unix()))

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.True(t, account.Schedulable)
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, 0, repo.setUnschedulableCalls)
	require.WithinDuration(t, expectedResetAt, repo.setRateLimitedResetAt, time.Second)
}

func TestRateLimitService_HandleUpstreamError_429QuotaExceeded_UsesFallbackResetAt(t *testing.T) {
	ctx := context.Background()
	repo := newRateLimitAccountRepoSpy()
	svc := &RateLimitService{accountRepo: repo}

	account := &Account{
		ID:          606,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	start := time.Now()
	var headers http.Header
	body := []byte(`{"error":{"code":"insufficient_quota","message":"The usage limit has been reached"}}`)

	shouldDisable := svc.HandleUpstreamError(ctx, account, 429, headers, body)

	require.True(t, shouldDisable)
	require.True(t, account.Schedulable)
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Equal(t, 0, repo.setUnschedulableCalls)
	require.WithinDuration(t, start.Add(30*time.Minute), repo.setRateLimitedResetAt, 3*time.Second)
}

// NOTE: quota exceeded 的 resetAt 解析逻辑由 parseRateLimitReset/parseRateLimitResetFromBody 覆盖，
// 单测通过 repo.SetRateLimited 的参数断言验证解析结果。
