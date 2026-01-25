//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSettingService_GetUsageWindowCooldownSeconds_Default(t *testing.T) {
	var svc *SettingService
	require.Equal(t, 300, svc.GetUsageWindowCooldownSeconds(context.Background()))
}

func TestAccountUsageService_EnforceUsageWindows_5hExceed_SetsTempUnschedulableOnly(t *testing.T) {
	repo := &accountRepoForUnschedTest{}
	settingSvc := &SettingService{
		settingCache: &fakeSettingCache{
			values: map[string]string{
				SettingKeyUsageWindowDisablePercent:   "80",
				SettingKeyUsageWindowCooldownSeconds: "300",
			},
		},
	}
	svc := &AccountUsageService{
		accountRepo:    repo,
		settingService: settingSvc,
	}

	now := time.Now()
	resetAt := now.Add(4 * time.Hour)
	usage := &UsageInfo{
		FiveHour: &UsageProgress{
			Utilization: 99,
			ResetsAt:    &resetAt,
		},
	}
	account := &Account{
		ID:          1,
		Platform:    PlatformAnthropic,
		Schedulable: true,
	}

	start := time.Now()
	svc.enforceUsageWindows(context.Background(), account, usage)

	require.Equal(t, 0, repo.unschedCalls, "不应关闭 schedulable")
	require.Equal(t, 1, repo.tempUnschedCalls, "应设置临时不可调度")
	require.True(t, account.Schedulable, "账号调度开关不应被关闭")
	require.True(t, repo.tempUntil.After(start.Add(295*time.Second)))
	require.True(t, repo.tempUntil.Before(start.Add(305*time.Second)))
}

