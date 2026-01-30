package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DueGin/FluxCode/internal/config"
	infraerrors "github.com/DueGin/FluxCode/internal/pkg/errors"
)

var (
	ErrRegistrationDisabled = infraerrors.Forbidden("REGISTRATION_DISABLED", "registration is currently disabled")
	ErrSettingNotFound      = infraerrors.NotFound("SETTING_NOT_FOUND", "setting not found")
)

const (
	defaultGatewayRetrySwitchAfter           = 2
	defaultAuth401CooldownSeconds            = 300
	defaultUsageWindowDisablePercent         = 100
	defaultUsageWindowCooldownSeconds        = 300
	defaultUserConcurrencyWaitTimeoutSeconds = 30
)

type SettingRepository interface {
	Get(ctx context.Context, key string) (*Setting, error)
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	SetMultiple(ctx context.Context, settings map[string]string) error
	GetAll(ctx context.Context) (map[string]string, error)
	Delete(ctx context.Context, key string) error
}

// SettingService 系统设置服务
type SettingService struct {
	settingRepo  SettingRepository
	settingCache SettingCache
	cfg          *config.Config
	listenersMu  sync.Mutex
}

func (s *SettingService) webTitleDefault() string {
	if title := strings.TrimSpace(os.Getenv("WEB_TITLE")); title != "" {
		return title
	}
	return "FluxCode"
}

// NewSettingService 创建系统设置服务实例
func NewSettingService(settingRepo SettingRepository, settingCache SettingCache, cfg *config.Config) *SettingService {
	return &SettingService{
		settingRepo:  settingRepo,
		settingCache: settingCache,
		cfg:          cfg,
	}
}

// GetAllSettings 获取所有系统设置
func (s *SettingService) GetAllSettings(ctx context.Context) (*SystemSettings, error) {
	settings, err := s.settingRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}

	return s.parseSettings(settings), nil
}

// GetPublicSettings 获取公开设置（无需登录）
func (s *SettingService) GetPublicSettings(ctx context.Context) (*PublicSettings, error) {
	keys := []string{
		SettingKeyRegistrationEnabled,
		SettingKeyEmailVerifyEnabled,
		SettingKeyTurnstileEnabled,
		SettingKeyTurnstileSiteKey,
		SettingKeySiteName,
		SettingKeySiteLogo,
		SettingKeySiteSubtitle,
		SettingKeyAPIBaseURL,
		SettingKeyContactInfo,
		SettingKeyAfterSaleContact,
		SettingKeyDocURL,
		SettingKeyAttractPopupTitle,
		SettingKeyAttractPopupMarkdown,
		SettingKeyLegacyAttractPopupTitle,
		SettingKeyLegacyAttractPopupMarkdown,
	}

	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get public settings: %w", err)
	}

	siteName := s.getStringOrDefault(settings, SettingKeySiteName, s.webTitleDefault())
	if title := strings.TrimSpace(os.Getenv("WEB_TITLE")); title != "" {
		siteName = title
	}

	return &PublicSettings{
		RegistrationEnabled: settings[SettingKeyRegistrationEnabled] == "true",
		EmailVerifyEnabled:  settings[SettingKeyEmailVerifyEnabled] == "true",
		TurnstileEnabled:    settings[SettingKeyTurnstileEnabled] == "true",
		TurnstileSiteKey:    settings[SettingKeyTurnstileSiteKey],
		SiteName:            siteName,
		SiteLogo:            settings[SettingKeySiteLogo],
		SiteSubtitle:        s.getStringOrDefault(settings, SettingKeySiteSubtitle, "Subscription to API Conversion Platform"),
		APIBaseURL:          settings[SettingKeyAPIBaseURL],
		ContactInfo:         settings[SettingKeyContactInfo],
		AfterSaleContact:    s.parseKVItems(settings[SettingKeyAfterSaleContact]),
		DocURL:              settings[SettingKeyDocURL],
		AttractPopupTitle: s.getStringWithFallback(
			settings,
			SettingKeyAttractPopupTitle,
			SettingKeyLegacyAttractPopupTitle,
			"加入社群领取测试卡",
		),
		AttractPopupMarkdown: s.getStringWithFallback(
			settings,
			SettingKeyAttractPopupMarkdown,
			SettingKeyLegacyAttractPopupMarkdown,
			`加入我们的社群即可领取 **$5 测试卡**。

请在管理后台「系统设置」中配置此弹窗文案。`),
	}, nil
}

// UpdateSettings 更新系统设置
func (s *SettingService) UpdateSettings(ctx context.Context, settings *SystemSettings) error {
	if s == nil || s.settingRepo == nil {
		return infraerrors.InternalServer("SETTING_SERVICE_NOT_READY", "setting service not initialized")
	}

	if s.settingCache != nil {
		token, ok, err := s.settingCache.AcquireUpdateLock(ctx, 30*time.Second)
		if err != nil {
			return infraerrors.ServiceUnavailable("SETTING_CACHE_UNAVAILABLE", "failed to acquire settings update lock").WithCause(err)
		}
		if !ok {
			return infraerrors.TooManyRequests("SETTINGS_UPDATE_BUSY", "settings are being updated, please retry later")
		}
		defer func(token string) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.settingCache.ReleaseUpdateLock(bgCtx, token)
		}(token)
	}

	updates := make(map[string]string)

	// 注册设置
	updates[SettingKeyRegistrationEnabled] = strconv.FormatBool(settings.RegistrationEnabled)
	updates[SettingKeyEmailVerifyEnabled] = strconv.FormatBool(settings.EmailVerifyEnabled)

	// 告警设置
	updates[SettingKeyAlertEmails] = s.marshalStringList(settings.AlertEmails)
	if settings.AlertCooldownMinutes < 0 {
		settings.AlertCooldownMinutes = 0
	}
	updates[SettingKeyAlertCooldownMinutes] = strconv.Itoa(settings.AlertCooldownMinutes)

	// 邮件服务设置（只有非空才更新密码）
	updates[SettingKeySMTPHost] = settings.SMTPHost
	updates[SettingKeySMTPPort] = strconv.Itoa(settings.SMTPPort)
	updates[SettingKeySMTPUsername] = settings.SMTPUsername
	if settings.SMTPPassword != "" {
		updates[SettingKeySMTPPassword] = settings.SMTPPassword
	}
	updates[SettingKeySMTPFrom] = settings.SMTPFrom
	updates[SettingKeySMTPFromName] = settings.SMTPFromName
	updates[SettingKeySMTPUseTLS] = strconv.FormatBool(settings.SMTPUseTLS)

	// Cloudflare Turnstile 设置（只有非空才更新密钥）
	updates[SettingKeyTurnstileEnabled] = strconv.FormatBool(settings.TurnstileEnabled)
	updates[SettingKeyTurnstileSiteKey] = settings.TurnstileSiteKey
	if settings.TurnstileSecretKey != "" {
		updates[SettingKeyTurnstileSecretKey] = settings.TurnstileSecretKey
	}

	// OEM设置
	updates[SettingKeySiteName] = settings.SiteName
	updates[SettingKeySiteLogo] = settings.SiteLogo
	updates[SettingKeySiteSubtitle] = settings.SiteSubtitle
	updates[SettingKeyAPIBaseURL] = settings.APIBaseURL
	updates[SettingKeyContactInfo] = settings.ContactInfo
	updates[SettingKeyAfterSaleContact] = s.marshalKVItems(settings.AfterSaleContact)
	updates[SettingKeyDocURL] = settings.DocURL
	updates[SettingKeyRedeemDeliveryText] = settings.RedeemDeliveryText
	updates[SettingKeyAttractPopupTitle] = settings.AttractPopupTitle
	updates[SettingKeyAttractPopupMarkdown] = settings.AttractPopupMarkdown
	// 历史兼容：保持旧 key 与新 key 同步，避免灰度/回滚期间出现配置丢失。
	updates[SettingKeyLegacyAttractPopupTitle] = settings.AttractPopupTitle
	updates[SettingKeyLegacyAttractPopupMarkdown] = settings.AttractPopupMarkdown

	// 默认配置
	updates[SettingKeyDefaultConcurrency] = strconv.Itoa(settings.DefaultConcurrency)
	updates[SettingKeyDefaultBalance] = strconv.FormatFloat(settings.DefaultBalance, 'f', 8, 64)
	if settings.GatewayRetrySwitchAfter <= 0 {
		settings.GatewayRetrySwitchAfter = defaultGatewayRetrySwitchAfter
	}
	updates[SettingKeyGatewayRetrySwitchAfter] = strconv.Itoa(settings.GatewayRetrySwitchAfter)
	if settings.Auth401CooldownSeconds <= 0 {
		settings.Auth401CooldownSeconds = defaultAuth401CooldownSeconds
	}
	updates[SettingKeyAuth401CooldownSeconds] = strconv.Itoa(settings.Auth401CooldownSeconds)
	settings.UsageWindowDisablePercent = normalizeUsageWindowDisablePercent(settings.UsageWindowDisablePercent)
	updates[SettingKeyUsageWindowDisablePercent] = strconv.Itoa(settings.UsageWindowDisablePercent)
	if settings.UsageWindowCooldownSeconds <= 0 {
		settings.UsageWindowCooldownSeconds = defaultUsageWindowCooldownSeconds
	}
	updates[SettingKeyUsageWindowCooldownSeconds] = strconv.Itoa(settings.UsageWindowCooldownSeconds)
	if settings.UserConcurrencyWaitTimeoutSeconds <= 0 {
		settings.UserConcurrencyWaitTimeoutSeconds = defaultUserConcurrencyWaitTimeoutSeconds
	}
	updates[SettingKeyUserConcurrencyWaitTimeoutSeconds] = strconv.Itoa(settings.UserConcurrencyWaitTimeoutSeconds)

	// Model fallback configuration
	updates[SettingKeyEnableModelFallback] = strconv.FormatBool(settings.EnableModelFallback)
	updates[SettingKeyFallbackModelAnthropic] = settings.FallbackModelAnthropic
	updates[SettingKeyFallbackModelOpenAI] = settings.FallbackModelOpenAI
	updates[SettingKeyFallbackModelGemini] = settings.FallbackModelGemini
	updates[SettingKeyFallbackModelAntigravity] = settings.FallbackModelAntigravity

	updateKeys := make([]string, 0, len(updates))
	for key := range updates {
		updateKeys = append(updateKeys, key)
	}
	previousValues, err := s.settingRepo.GetMultiple(ctx, updateKeys)
	if err != nil {
		return fmt.Errorf("snapshot settings: %w", err)
	}
	previouslyMissing := make(map[string]struct{}, len(updateKeys))
	for _, key := range updateKeys {
		if _, ok := previousValues[key]; !ok {
			previouslyMissing[key] = struct{}{}
		}
	}

	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}

	if s.settingCache != nil {
		cacheUpdates := map[string]string{
			SettingKeyGatewayRetrySwitchAfter:           updates[SettingKeyGatewayRetrySwitchAfter],
			SettingKeyAuth401CooldownSeconds:            updates[SettingKeyAuth401CooldownSeconds],
			SettingKeyUsageWindowDisablePercent:         updates[SettingKeyUsageWindowDisablePercent],
			SettingKeyUsageWindowCooldownSeconds:        updates[SettingKeyUsageWindowCooldownSeconds],
			SettingKeyUserConcurrencyWaitTimeoutSeconds: updates[SettingKeyUserConcurrencyWaitTimeoutSeconds],
		}
		if err := s.settingCache.SetMultiple(ctx, cacheUpdates); err != nil {
			rollbackErr := s.rollbackSettings(ctx, previousValues, previouslyMissing)
			if rollbackErr != nil {
				return infraerrors.ServiceUnavailable("SETTING_UPDATE_ROLLBACK_FAILED", "settings cache update failed; rollback failed").WithCause(fmt.Errorf("cache=%v rollback=%w", err, rollbackErr))
			}
			_ = s.bestEffortRestoreSettingsCache(ctx, previousValues)
			return infraerrors.ServiceUnavailable("SETTING_CACHE_UPDATE_FAILED", "failed to update settings cache").WithCause(err)
		}
	}

	return nil
}

func (s *SettingService) rollbackSettings(ctx context.Context, previousValues map[string]string, previouslyMissing map[string]struct{}) error {
	if s == nil || s.settingRepo == nil {
		return errors.New("nil setting repo")
	}

	if len(previousValues) > 0 {
		if err := s.settingRepo.SetMultiple(ctx, previousValues); err != nil {
			return err
		}
	}

	for key := range previouslyMissing {
		// Best-effort: if deletion fails, continue so we at least restore existing keys.
		_ = s.settingRepo.Delete(ctx, key)
	}

	return nil
}

func (s *SettingService) bestEffortRestoreSettingsCache(ctx context.Context, previousValues map[string]string) error {
	if s == nil || s.settingCache == nil {
		return nil
	}
	if previousValues == nil {
		return nil
	}
	restore := map[string]string{}
	if v, ok := previousValues[SettingKeyGatewayRetrySwitchAfter]; ok {
		restore[SettingKeyGatewayRetrySwitchAfter] = v
	}
	if v, ok := previousValues[SettingKeyAuth401CooldownSeconds]; ok {
		restore[SettingKeyAuth401CooldownSeconds] = v
	}
	if v, ok := previousValues[SettingKeyUsageWindowDisablePercent]; ok {
		restore[SettingKeyUsageWindowDisablePercent] = v
	}
	if v, ok := previousValues[SettingKeyUsageWindowCooldownSeconds]; ok {
		restore[SettingKeyUsageWindowCooldownSeconds] = v
	}
	if v, ok := previousValues[SettingKeyUserConcurrencyWaitTimeoutSeconds]; ok {
		restore[SettingKeyUserConcurrencyWaitTimeoutSeconds] = v
	}
	if len(restore) == 0 {
		return nil
	}
	return s.settingCache.SetMultiple(ctx, restore)
}

// IsRegistrationEnabled 检查是否开放注册
func (s *SettingService) IsRegistrationEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEnabled)
	if err != nil {
		// 默认开放注册
		return true
	}
	return value == "true"
}

// IsEmailVerifyEnabled 检查是否开启邮件验证
func (s *SettingService) IsEmailVerifyEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEmailVerifyEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// GetSiteName 获取网站名称
func (s *SettingService) GetSiteName(ctx context.Context) string {
	if title := strings.TrimSpace(os.Getenv("WEB_TITLE")); title != "" {
		return title
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeySiteName)
	if err != nil || value == "" {
		return s.webTitleDefault()
	}
	return value
}

// GetDefaultConcurrency 获取默认并发量
func (s *SettingService) GetDefaultConcurrency(ctx context.Context) int {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDefaultConcurrency)
	if err != nil {
		return s.cfg.Default.UserConcurrency
	}
	if v, err := strconv.Atoi(value); err == nil && v > 0 {
		return v
	}
	return s.cfg.Default.UserConcurrency
}

// GetDefaultBalance 获取默认余额
func (s *SettingService) GetDefaultBalance(ctx context.Context) float64 {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDefaultBalance)
	if err != nil {
		return s.cfg.Default.UserBalance
	}
	if v, err := strconv.ParseFloat(value, 64); err == nil && v >= 0 {
		return v
	}
	return s.cfg.Default.UserBalance
}

// GetGatewayRetrySwitchAfter returns how many client retries trigger account switching.
func (s *SettingService) GetGatewayRetrySwitchAfter(ctx context.Context) int {
	if s == nil {
		return defaultGatewayRetrySwitchAfter
	}
	if s.settingCache != nil {
		if value, err := s.settingCache.GetValue(ctx, SettingKeyGatewayRetrySwitchAfter); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && v > 0 {
				return v
			}
		}
	}

	if s.settingRepo == nil {
		return defaultGatewayRetrySwitchAfter
	}

	value, err := s.settingRepo.GetValue(ctx, SettingKeyGatewayRetrySwitchAfter)
	if err != nil {
		return defaultGatewayRetrySwitchAfter
	}
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || v <= 0 {
		v = defaultGatewayRetrySwitchAfter
	}
	if s.settingCache != nil {
		_ = s.settingCache.Set(ctx, SettingKeyGatewayRetrySwitchAfter, strconv.Itoa(v))
	}
	return v
}

func (s *SettingService) GetAuth401CooldownSeconds(ctx context.Context) int {
	if s == nil {
		return defaultAuth401CooldownSeconds
	}
	if s.settingCache != nil {
		if value, err := s.settingCache.GetValue(ctx, SettingKeyAuth401CooldownSeconds); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && v > 0 {
				return v
			}
		}
	}

	if s.settingRepo == nil {
		return defaultAuth401CooldownSeconds
	}

	value, err := s.settingRepo.GetValue(ctx, SettingKeyAuth401CooldownSeconds)
	if err != nil {
		return defaultAuth401CooldownSeconds
	}
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || v <= 0 {
		v = defaultAuth401CooldownSeconds
	}
	if s.settingCache != nil {
		_ = s.settingCache.Set(ctx, SettingKeyAuth401CooldownSeconds, strconv.Itoa(v))
	}
	return v
}

func (s *SettingService) GetUsageWindowDisablePercent(ctx context.Context) int {
	if s == nil {
		return defaultUsageWindowDisablePercent
	}
	if s.settingCache != nil {
		if value, err := s.settingCache.GetValue(ctx, SettingKeyUsageWindowDisablePercent); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				return normalizeUsageWindowDisablePercent(v)
			}
		}
	}

	if s.settingRepo == nil {
		return defaultUsageWindowDisablePercent
	}

	value, err := s.settingRepo.GetValue(ctx, SettingKeyUsageWindowDisablePercent)
	if err != nil {
		return defaultUsageWindowDisablePercent
	}
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		v = defaultUsageWindowDisablePercent
	}
	v = normalizeUsageWindowDisablePercent(v)
	if s.settingCache != nil {
		_ = s.settingCache.Set(ctx, SettingKeyUsageWindowDisablePercent, strconv.Itoa(v))
	}
	return v
}

func (s *SettingService) GetUsageWindowCooldownSeconds(ctx context.Context) int {
	if s == nil {
		return defaultUsageWindowCooldownSeconds
	}
	if s.settingCache != nil {
		if value, err := s.settingCache.GetValue(ctx, SettingKeyUsageWindowCooldownSeconds); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && v > 0 {
				return v
			}
		}
	}

	if s.settingRepo == nil {
		return defaultUsageWindowCooldownSeconds
	}

	value, err := s.settingRepo.GetValue(ctx, SettingKeyUsageWindowCooldownSeconds)
	if err != nil {
		return defaultUsageWindowCooldownSeconds
	}
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || v <= 0 {
		v = defaultUsageWindowCooldownSeconds
	}
	if s.settingCache != nil {
		_ = s.settingCache.Set(ctx, SettingKeyUsageWindowCooldownSeconds, strconv.Itoa(v))
	}
	return v
}

func (s *SettingService) GetUsageWindowCooldown(ctx context.Context) time.Duration {
	return time.Duration(s.GetUsageWindowCooldownSeconds(ctx)) * time.Second
}

func (s *SettingService) GetUserConcurrencyWaitTimeoutSeconds(ctx context.Context) int {
	if s == nil {
		return defaultUserConcurrencyWaitTimeoutSeconds
	}
	if s.settingCache != nil {
		if value, err := s.settingCache.GetValue(ctx, SettingKeyUserConcurrencyWaitTimeoutSeconds); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && v > 0 {
				return v
			}
		}
	}

	if s.settingRepo == nil {
		return defaultUserConcurrencyWaitTimeoutSeconds
	}

	value, err := s.settingRepo.GetValue(ctx, SettingKeyUserConcurrencyWaitTimeoutSeconds)
	if err != nil {
		return defaultUserConcurrencyWaitTimeoutSeconds
	}
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || v <= 0 {
		v = defaultUserConcurrencyWaitTimeoutSeconds
	}
	if s.settingCache != nil {
		_ = s.settingCache.Set(ctx, SettingKeyUserConcurrencyWaitTimeoutSeconds, strconv.Itoa(v))
	}
	return v
}

func (s *SettingService) GetUserConcurrencyWaitTimeout(ctx context.Context) time.Duration {
	return time.Duration(s.GetUserConcurrencyWaitTimeoutSeconds(ctx)) * time.Second
}

// InitializeDefaultSettings 初始化默认设置
func (s *SettingService) InitializeDefaultSettings(ctx context.Context) error {
	// 检查是否已有设置
	_, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEnabled)
	if err == nil {
		// 已有设置，不需要初始化
		return nil
	}
	if !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("check existing settings: %w", err)
	}

	// 初始化默认设置
	defaults := map[string]string{
		SettingKeyRegistrationEnabled:               "true",
		SettingKeyEmailVerifyEnabled:                "false",
		SettingKeyAlertEmails:                       "[]",
		SettingKeyAlertCooldownMinutes:              strconv.Itoa(defaultAlertCooldownMinutes),
		SettingKeySiteName:                          s.webTitleDefault(),
		SettingKeySiteLogo:                          "",
		SettingKeyAfterSaleContact:                  "[]",
		SettingKeyRedeemDeliveryText:                "${redeemCodes}",
		SettingKeyDefaultConcurrency:                strconv.Itoa(s.cfg.Default.UserConcurrency),
		SettingKeyDefaultBalance:                    strconv.FormatFloat(s.cfg.Default.UserBalance, 'f', 8, 64),
		SettingKeyGatewayRetrySwitchAfter:           strconv.Itoa(defaultGatewayRetrySwitchAfter),
		SettingKeyAuth401CooldownSeconds:            strconv.Itoa(defaultAuth401CooldownSeconds),
		SettingKeyUsageWindowDisablePercent:         strconv.Itoa(defaultUsageWindowDisablePercent),
		SettingKeyUsageWindowCooldownSeconds:        strconv.Itoa(defaultUsageWindowCooldownSeconds),
		SettingKeyUserConcurrencyWaitTimeoutSeconds: strconv.Itoa(defaultUserConcurrencyWaitTimeoutSeconds),
		SettingKeySMTPPort:                          "587",
		SettingKeySMTPUseTLS:                        "false",
		SettingKeyAttractPopupTitle:                 "加入社群领取测试卡",
		SettingKeyAttractPopupMarkdown: `加入我们的社群即可领取 **$5 测试卡**。

请在管理后台「系统设置」中配置此弹窗文案。`,
		SettingKeyLegacyAttractPopupTitle: "加入社群领取测试卡",
		SettingKeyLegacyAttractPopupMarkdown: `加入我们的社群即可领取 **$5 测试卡**。

请在管理后台「系统设置」中配置此弹窗文案。`,
		// Model fallback defaults
		SettingKeyEnableModelFallback:      "false",
		SettingKeyFallbackModelAnthropic:   "claude-3-5-sonnet-20241022",
		SettingKeyFallbackModelOpenAI:      "gpt-4o",
		SettingKeyFallbackModelGemini:      "gemini-2.5-pro",
		SettingKeyFallbackModelAntigravity: "gemini-2.5-pro",
	}

	if err := s.settingRepo.SetMultiple(ctx, defaults); err != nil {
		return err
	}
	if s.settingCache != nil {
		_ = s.settingCache.SetMultiple(ctx, map[string]string{
			SettingKeyGatewayRetrySwitchAfter:           defaults[SettingKeyGatewayRetrySwitchAfter],
			SettingKeyAuth401CooldownSeconds:            defaults[SettingKeyAuth401CooldownSeconds],
			SettingKeyUsageWindowDisablePercent:         defaults[SettingKeyUsageWindowDisablePercent],
			SettingKeyUsageWindowCooldownSeconds:        defaults[SettingKeyUsageWindowCooldownSeconds],
			SettingKeyUserConcurrencyWaitTimeoutSeconds: defaults[SettingKeyUserConcurrencyWaitTimeoutSeconds],
		})
	}
	return nil
}

// parseSettings 解析设置到结构体
func (s *SettingService) parseSettings(settings map[string]string) *SystemSettings {
	siteName := s.getStringOrDefault(settings, SettingKeySiteName, s.webTitleDefault())
	if title := strings.TrimSpace(os.Getenv("WEB_TITLE")); title != "" {
		siteName = title
	}

	result := &SystemSettings{
		RegistrationEnabled: settings[SettingKeyRegistrationEnabled] == "true",
		EmailVerifyEnabled:  settings[SettingKeyEmailVerifyEnabled] == "true",
		AlertEmails:         s.parseStringList(settings[SettingKeyAlertEmails]),
		SMTPHost:            settings[SettingKeySMTPHost],
		SMTPUsername:        settings[SettingKeySMTPUsername],
		SMTPFrom:            settings[SettingKeySMTPFrom],
		SMTPFromName:        settings[SettingKeySMTPFromName],
		SMTPUseTLS:          settings[SettingKeySMTPUseTLS] == "true",
		TurnstileEnabled:    settings[SettingKeyTurnstileEnabled] == "true",
		TurnstileSiteKey:    settings[SettingKeyTurnstileSiteKey],
		SiteName:            siteName,
		SiteLogo:            settings[SettingKeySiteLogo],
		SiteSubtitle:        s.getStringOrDefault(settings, SettingKeySiteSubtitle, "Subscription to API Conversion Platform"),
		APIBaseURL:          settings[SettingKeyAPIBaseURL],
		ContactInfo:         settings[SettingKeyContactInfo],
		AfterSaleContact:    s.parseKVItems(settings[SettingKeyAfterSaleContact]),
		DocURL:              settings[SettingKeyDocURL],
		RedeemDeliveryText:  s.getStringOrDefault(settings, SettingKeyRedeemDeliveryText, "${redeemCodes}"),
		AttractPopupTitle: s.getStringWithFallback(
			settings,
			SettingKeyAttractPopupTitle,
			SettingKeyLegacyAttractPopupTitle,
			"加入社群领取测试卡",
		),
		AttractPopupMarkdown: s.getStringWithFallback(
			settings,
			SettingKeyAttractPopupMarkdown,
			SettingKeyLegacyAttractPopupMarkdown,
			`加入我们的社群即可领取 **$5 测试卡**。

请在管理后台「系统设置」中配置此弹窗文案。`),
	}

	// 告警冷却时间（分钟）：默认 5 分钟；允许 0 表示不限制
	result.AlertCooldownMinutes = defaultAlertCooldownMinutes
	if raw := strings.TrimSpace(settings[SettingKeyAlertCooldownMinutes]); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			if v < 0 {
				v = 0
			}
			result.AlertCooldownMinutes = v
		}
	}

	// 解析整数类型
	if port, err := strconv.Atoi(settings[SettingKeySMTPPort]); err == nil {
		result.SMTPPort = port
	} else {
		result.SMTPPort = 587
	}

	if concurrency, err := strconv.Atoi(settings[SettingKeyDefaultConcurrency]); err == nil {
		result.DefaultConcurrency = concurrency
	} else {
		result.DefaultConcurrency = s.cfg.Default.UserConcurrency
	}

	if switchAfter, err := strconv.Atoi(settings[SettingKeyGatewayRetrySwitchAfter]); err == nil {
		result.GatewayRetrySwitchAfter = switchAfter
	} else {
		result.GatewayRetrySwitchAfter = defaultGatewayRetrySwitchAfter
	}
	if v, err := strconv.Atoi(settings[SettingKeyAuth401CooldownSeconds]); err == nil && v > 0 {
		result.Auth401CooldownSeconds = v
	} else {
		result.Auth401CooldownSeconds = defaultAuth401CooldownSeconds
	}
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyUsageWindowDisablePercent])); err == nil {
		result.UsageWindowDisablePercent = normalizeUsageWindowDisablePercent(v)
	} else {
		result.UsageWindowDisablePercent = defaultUsageWindowDisablePercent
	}
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyUsageWindowCooldownSeconds])); err == nil && v > 0 {
		result.UsageWindowCooldownSeconds = v
	} else {
		result.UsageWindowCooldownSeconds = defaultUsageWindowCooldownSeconds
	}
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyUserConcurrencyWaitTimeoutSeconds])); err == nil && v > 0 {
		result.UserConcurrencyWaitTimeoutSeconds = v
	} else {
		result.UserConcurrencyWaitTimeoutSeconds = defaultUserConcurrencyWaitTimeoutSeconds
	}

	// 解析浮点数类型
	if balance, err := strconv.ParseFloat(settings[SettingKeyDefaultBalance], 64); err == nil {
		result.DefaultBalance = balance
	} else {
		result.DefaultBalance = s.cfg.Default.UserBalance
	}

	// 敏感信息直接返回，方便测试连接时使用
	result.SMTPPassword = settings[SettingKeySMTPPassword]
	result.TurnstileSecretKey = settings[SettingKeyTurnstileSecretKey]

	// Model fallback settings
	result.EnableModelFallback = settings[SettingKeyEnableModelFallback] == "true"
	result.FallbackModelAnthropic = s.getStringOrDefault(settings, SettingKeyFallbackModelAnthropic, "claude-3-5-sonnet-20241022")
	result.FallbackModelOpenAI = s.getStringOrDefault(settings, SettingKeyFallbackModelOpenAI, "gpt-4o")
	result.FallbackModelGemini = s.getStringOrDefault(settings, SettingKeyFallbackModelGemini, "gemini-2.5-pro")
	result.FallbackModelAntigravity = s.getStringOrDefault(settings, SettingKeyFallbackModelAntigravity, "gemini-2.5-pro")

	return result
}

func (s *SettingService) parseKVItems(raw string) []KVItem {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []KVItem{}
	}
	var items []KVItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []KVItem{}
	}
	return s.normalizeKVItems(items)
}

func (s *SettingService) normalizeKVItems(items []KVItem) []KVItem {
	out := make([]KVItem, 0, len(items))
	for _, item := range items {
		k := strings.TrimSpace(item.K)
		v := strings.TrimSpace(item.V)
		if k == "" && v == "" {
			continue
		}
		out = append(out, KVItem{K: k, V: v})
	}
	return out
}

func (s *SettingService) marshalKVItems(items []KVItem) string {
	normalized := s.normalizeKVItems(items)
	b, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func (s *SettingService) parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return s.normalizeStringList(items)
	}
	// Backward/compat: allow comma/space/newline separated values.
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	return s.normalizeStringList(parts)
}

func (s *SettingService) normalizeStringList(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (s *SettingService) marshalStringList(items []string) string {
	normalized := s.normalizeStringList(items)
	b, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// getStringOrDefault 获取字符串值或默认值
func (s *SettingService) getStringOrDefault(settings map[string]string, key, defaultValue string) string {
	if value, ok := settings[key]; ok && value != "" {
		return value
	}
	return defaultValue
}

// getStringWithFallback 按「新 key → 旧 key → default」顺序取值（主要用于平滑升级/历史兼容）。
func (s *SettingService) getStringWithFallback(settings map[string]string, key, fallbackKey, defaultValue string) string {
	if value, ok := settings[key]; ok && value != "" {
		return value
	}
	if value, ok := settings[fallbackKey]; ok && value != "" {
		return value
	}
	return defaultValue
}

func normalizeUsageWindowDisablePercent(value int) int {
	if value <= 0 {
		return defaultUsageWindowDisablePercent
	}
	if value > 100 {
		return 100
	}
	return value
}

// IsTurnstileEnabled 检查是否启用 Turnstile 验证
func (s *SettingService) IsTurnstileEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTurnstileEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// GetTurnstileSecretKey 获取 Turnstile Secret Key
func (s *SettingService) GetTurnstileSecretKey(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTurnstileSecretKey)
	if err != nil {
		return ""
	}
	return value
}

// GenerateAdminAPIKey 生成新的管理员 API Key
func (s *SettingService) GenerateAdminAPIKey(ctx context.Context) (string, error) {
	// 生成 32 字节随机数 = 64 位十六进制字符
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	key := AdminAPIKeyPrefix + hex.EncodeToString(bytes)

	// 存储到 settings 表
	if err := s.settingRepo.Set(ctx, SettingKeyAdminAPIKey, key); err != nil {
		return "", fmt.Errorf("save admin api key: %w", err)
	}

	return key, nil
}

// GetAdminAPIKeyStatus 获取管理员 API Key 状态
// 返回脱敏的 key、是否存在、错误
func (s *SettingService) GetAdminAPIKeyStatus(ctx context.Context) (maskedKey string, exists bool, err error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if key == "" {
		return "", false, nil
	}

	// 脱敏：显示前 10 位和后 4 位
	if len(key) > 14 {
		maskedKey = key[:10] + "..." + key[len(key)-4:]
	} else {
		maskedKey = key
	}

	return maskedKey, true, nil
}

// GetAdminAPIKey 获取完整的管理员 API Key（仅供内部验证使用）
// 如果未配置返回空字符串和 nil 错误，只有数据库错误时才返回 error
func (s *SettingService) GetAdminAPIKey(ctx context.Context) (string, error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", nil // 未配置，返回空字符串
		}
		return "", err // 数据库错误
	}
	return key, nil
}

// DeleteAdminAPIKey 删除管理员 API Key
func (s *SettingService) DeleteAdminAPIKey(ctx context.Context) error {
	return s.settingRepo.Delete(ctx, SettingKeyAdminAPIKey)
}

// IsModelFallbackEnabled 检查是否启用模型兜底机制
func (s *SettingService) IsModelFallbackEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEnableModelFallback)
	if err != nil {
		return false // Default: disabled
	}
	return value == "true"
}

// GetFallbackModel 获取指定平台的兜底模型
func (s *SettingService) GetFallbackModel(ctx context.Context, platform string) string {
	var key string
	var defaultModel string

	switch platform {
	case PlatformAnthropic:
		key = SettingKeyFallbackModelAnthropic
		defaultModel = "claude-3-5-sonnet-20241022"
	case PlatformOpenAI:
		key = SettingKeyFallbackModelOpenAI
		defaultModel = "gpt-4o"
	case PlatformGemini:
		key = SettingKeyFallbackModelGemini
		defaultModel = "gemini-2.5-pro"
	case PlatformAntigravity:
		key = SettingKeyFallbackModelAntigravity
		defaultModel = "gemini-2.5-pro"
	default:
		return ""
	}

	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil || value == "" {
		return defaultModel
	}
	return value
}
