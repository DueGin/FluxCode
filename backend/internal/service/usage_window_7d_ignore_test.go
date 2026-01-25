//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/DueGin/FluxCode/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type fakeSettingCache struct {
	values map[string]string
}

func (c *fakeSettingCache) GetValue(ctx context.Context, key string) (string, error) {
	if c.values == nil {
		return "", ErrSettingNotFound
	}
	if v, ok := c.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (c *fakeSettingCache) Set(ctx context.Context, key, value string) error {
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[key] = value
	return nil
}

func (c *fakeSettingCache) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, err := c.GetValue(ctx, k); err == nil {
			out[k] = v
		}
	}
	return out, nil
}

func (c *fakeSettingCache) SetMultiple(ctx context.Context, settings map[string]string) error {
	if c.values == nil {
		c.values = make(map[string]string)
	}
	for k, v := range settings {
		c.values[k] = v
	}
	return nil
}

func (c *fakeSettingCache) AcquireUpdateLock(ctx context.Context, ttl time.Duration) (token string, ok bool, err error) {
	return "test", true, nil
}

func (c *fakeSettingCache) ReleaseUpdateLock(ctx context.Context, token string) error {
	return nil
}

type accountRepoForUnschedTest struct {
	unschedCalls int
	lastReason   string
}

func (r *accountRepoForUnschedTest) SetUnschedulableWithReason(ctx context.Context, id int64, reason string) error {
	r.unschedCalls++
	r.lastReason = reason
	return nil
}

func (r *accountRepoForUnschedTest) Create(ctx context.Context, account *Account) error {
	panic("unexpected Create call")
}

func (r *accountRepoForUnschedTest) GetByID(ctx context.Context, id int64) (*Account, error) {
	panic("unexpected GetByID call")
}

func (r *accountRepoForUnschedTest) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	panic("unexpected GetByIDs call")
}

func (r *accountRepoForUnschedTest) ExistsByID(ctx context.Context, id int64) (bool, error) {
	panic("unexpected ExistsByID call")
}

func (r *accountRepoForUnschedTest) GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error) {
	panic("unexpected GetByCRSAccountID call")
}

func (r *accountRepoForUnschedTest) Update(ctx context.Context, account *Account) error {
	panic("unexpected Update call")
}

func (r *accountRepoForUnschedTest) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (r *accountRepoForUnschedTest) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *accountRepoForUnschedTest) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *accountRepoForUnschedTest) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListByGroup call")
}

func (r *accountRepoForUnschedTest) ListActive(ctx context.Context) ([]Account, error) {
	panic("unexpected ListActive call")
}

func (r *accountRepoForUnschedTest) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListByPlatform call")
}

func (r *accountRepoForUnschedTest) UpdateLastUsed(ctx context.Context, id int64) error {
	panic("unexpected UpdateLastUsed call")
}

func (r *accountRepoForUnschedTest) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	panic("unexpected BatchUpdateLastUsed call")
}

func (r *accountRepoForUnschedTest) SetError(ctx context.Context, id int64, errorMsg string) error {
	panic("unexpected SetError call")
}

func (r *accountRepoForUnschedTest) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	panic("unexpected SetSchedulable call")
}

func (r *accountRepoForUnschedTest) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	panic("unexpected BindGroups call")
}

func (r *accountRepoForUnschedTest) ListSchedulable(ctx context.Context) ([]Account, error) {
	panic("unexpected ListSchedulable call")
}

func (r *accountRepoForUnschedTest) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupID call")
}

func (r *accountRepoForUnschedTest) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatform call")
}

func (r *accountRepoForUnschedTest) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatform call")
}

func (r *accountRepoForUnschedTest) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatforms call")
}

func (r *accountRepoForUnschedTest) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatforms call")
}

func (r *accountRepoForUnschedTest) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	panic("unexpected SetRateLimited call")
}

func (r *accountRepoForUnschedTest) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	panic("unexpected SetOverloaded call")
}

func (r *accountRepoForUnschedTest) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	panic("unexpected SetTempUnschedulable call")
}

func (r *accountRepoForUnschedTest) ClearTempUnschedulable(ctx context.Context, id int64) error {
	panic("unexpected ClearTempUnschedulable call")
}

func (r *accountRepoForUnschedTest) ClearRateLimit(ctx context.Context, id int64) error {
	panic("unexpected ClearRateLimit call")
}

func (r *accountRepoForUnschedTest) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	panic("unexpected UpdateSessionWindow call")
}

func (r *accountRepoForUnschedTest) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	panic("unexpected UpdateExtra call")
}

func (r *accountRepoForUnschedTest) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	panic("unexpected BulkUpdate call")
}

func TestExceededWindows_Ignores7d(t *testing.T) {
	exceeded := exceededWindows([]usageWindow{
		{name: "7d", used: 99},
	}, 80)
	require.Len(t, exceeded, 0)
}

func TestCodexUsageWindows_Ignores7dForExceeded(t *testing.T) {
	now := time.Now()
	account := &Account{
		Extra: map[string]any{
			"codex_7d_used_percent":          99.0,
			"codex_7d_reset_after_seconds":   3600,
			"codex_usage_updated_at":         now.Format(time.RFC3339),
			"codex_5h_used_percent":          10.0,
			"codex_5h_reset_after_seconds":   3600,
			"codex_5h_window_minutes":        300,
			"codex_7d_window_minutes":        10080,
			"codex_primary_window_minutes":   10080,
			"codex_secondary_window_minutes": 300,
		},
	}

	_, exceeded := codexUsageWindows(account, now, 80)
	require.Len(t, exceeded, 0)
}

func TestAccountUsageService_EnforceUsageWindows_Ignores7d(t *testing.T) {
	repo := &accountRepoForUnschedTest{}
	settingSvc := &SettingService{
		settingCache: &fakeSettingCache{
			values: map[string]string{
				SettingKeyUsageWindowDisablePercent: "80",
			},
		},
	}
	svc := &AccountUsageService{
		accountRepo:    repo,
		settingService: settingSvc,
	}

	now := time.Now()
	resetAt := now.Add(1 * time.Hour)
	usage := &UsageInfo{
		SevenDay: &UsageProgress{
			Utilization: 99,
			ResetsAt:    &resetAt,
		},
	}
	account := &Account{
		ID:       1,
		Platform: PlatformAnthropic,
	}

	svc.enforceUsageWindows(context.Background(), account, usage)
	require.Equal(t, 0, repo.unschedCalls)
}

