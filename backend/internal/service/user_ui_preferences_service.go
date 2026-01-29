package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	userPrefKeyDashboardAttractPopupDisabledFmt       = "user:%d:dashboard_attract_popup_disabled"
	userPrefKeyDashboardAttractPopupDisabledLegacyFmt = "user:%d:dashboard_qq_group_popup_disabled"
)

type UserUIPreferencesService struct {
	settingRepo SettingRepository
}

func NewUserUIPreferencesService(settingRepo SettingRepository) *UserUIPreferencesService {
	return &UserUIPreferencesService{settingRepo: settingRepo}
}

func (s *UserUIPreferencesService) GetDashboardAttractPopupDisabled(ctx context.Context, userID int64) (bool, error) {
	// 先读新 key；若不存在再读旧 key（历史兼容）。
	raw, err := s.settingRepo.GetValue(ctx, fmt.Sprintf(userPrefKeyDashboardAttractPopupDisabledFmt, userID))
	if err != nil {
		if !errors.Is(err, ErrSettingNotFound) {
			return false, err
		}
		raw, err = s.settingRepo.GetValue(ctx, fmt.Sprintf(userPrefKeyDashboardAttractPopupDisabledLegacyFmt, userID))
		if err != nil {
			if errors.Is(err, ErrSettingNotFound) {
				return false, nil
			}
			return false, err
		}
	}

	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true, nil
	default:
		return false, nil
	}
}

func (s *UserUIPreferencesService) SetDashboardAttractPopupDisabled(ctx context.Context, userID int64, disabled bool) error {
	// 以新 key 为准；旧 key 同步写入仅用于兼容回滚/灰度（失败不影响主流程）。
	if err := s.settingRepo.Set(ctx, fmt.Sprintf(userPrefKeyDashboardAttractPopupDisabledFmt, userID), strconv.FormatBool(disabled)); err != nil {
		return err
	}
	_ = s.settingRepo.Set(ctx, fmt.Sprintf(userPrefKeyDashboardAttractPopupDisabledLegacyFmt, userID), strconv.FormatBool(disabled))
	return nil
}
