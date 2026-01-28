package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const userPrefKeyDashboardQQGroupPopupDisabledFmt = "user:%d:dashboard_qq_group_popup_disabled"

type UserUIPreferencesService struct {
	settingRepo SettingRepository
}

func NewUserUIPreferencesService(settingRepo SettingRepository) *UserUIPreferencesService {
	return &UserUIPreferencesService{settingRepo: settingRepo}
}

func (s *UserUIPreferencesService) GetDashboardQQGroupPopupDisabled(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf(userPrefKeyDashboardQQGroupPopupDisabledFmt, userID)
	raw, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return false, nil
		}
		return false, err
	}
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true, nil
	default:
		return false, nil
	}
}

func (s *UserUIPreferencesService) SetDashboardQQGroupPopupDisabled(ctx context.Context, userID int64, disabled bool) error {
	key := fmt.Sprintf(userPrefKeyDashboardQQGroupPopupDisabledFmt, userID)
	return s.settingRepo.Set(ctx, key, strconv.FormatBool(disabled))
}
