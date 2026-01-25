package admin

import (
	"strings"

	"github.com/DueGin/FluxCode/internal/handler/dto"
	"github.com/DueGin/FluxCode/internal/pkg/response"
	"github.com/DueGin/FluxCode/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService   *service.SettingService
	emailService     *service.EmailService
	turnstileService *service.TurnstileService
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService) *SettingHandler {
	return &SettingHandler{
		settingService:   settingService,
		emailService:     emailService,
		turnstileService: turnstileService,
	}
}

// GetSettings 获取所有系统设置
// GET /api/v1/admin/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	afterSaleContact := make([]dto.KVItem, 0, len(settings.AfterSaleContact))
	for _, item := range settings.AfterSaleContact {
		afterSaleContact = append(afterSaleContact, dto.KVItem{K: item.K, V: item.V})
	}

	response.Success(c, dto.SystemSettings{
		RegistrationEnabled:               settings.RegistrationEnabled,
		EmailVerifyEnabled:                settings.EmailVerifyEnabled,
		AlertEmails:                       settings.AlertEmails,
		AlertCooldownMinutes:              settings.AlertCooldownMinutes,
		SMTPHost:                          settings.SMTPHost,
		SMTPPort:                          settings.SMTPPort,
		SMTPUsername:                      settings.SMTPUsername,
		SMTPPassword:                      settings.SMTPPassword,
		SMTPFrom:                          settings.SMTPFrom,
		SMTPFromName:                      settings.SMTPFromName,
		SMTPUseTLS:                        settings.SMTPUseTLS,
		TurnstileEnabled:                  settings.TurnstileEnabled,
		TurnstileSiteKey:                  settings.TurnstileSiteKey,
		TurnstileSecretKey:                settings.TurnstileSecretKey,
		SiteName:                          settings.SiteName,
		SiteLogo:                          settings.SiteLogo,
		SiteSubtitle:                      settings.SiteSubtitle,
		APIBaseURL:                        settings.APIBaseURL,
		ContactInfo:                       settings.ContactInfo,
		AfterSaleContact:                  afterSaleContact,
		DocURL:                            settings.DocURL,
		DefaultConcurrency:                settings.DefaultConcurrency,
		DefaultBalance:                    settings.DefaultBalance,
		GatewayRetrySwitchAfter:           settings.GatewayRetrySwitchAfter,
		DailyUsageRefreshTime:             settings.DailyUsageRefreshTime,
		Auth401CooldownSeconds:            settings.Auth401CooldownSeconds,
		UsageWindowDisablePercent:         settings.UsageWindowDisablePercent,
		UserConcurrencyWaitTimeoutSeconds: settings.UserConcurrencyWaitTimeoutSeconds,
		EnableModelFallback:               settings.EnableModelFallback,
		FallbackModelAnthropic:            settings.FallbackModelAnthropic,
		FallbackModelOpenAI:               settings.FallbackModelOpenAI,
		FallbackModelGemini:               settings.FallbackModelGemini,
		FallbackModelAntigravity:          settings.FallbackModelAntigravity,
	})
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	// 注册设置
	RegistrationEnabled bool `json:"registration_enabled"`
	EmailVerifyEnabled  bool `json:"email_verify_enabled"`

	// 告警设置
	AlertEmails          []string `json:"alert_emails"`
	AlertCooldownMinutes int      `json:"alert_cooldown_minutes"`

	// 邮件服务设置
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`

	// Cloudflare Turnstile 设置
	TurnstileEnabled   bool   `json:"turnstile_enabled"`
	TurnstileSiteKey   string `json:"turnstile_site_key"`
	TurnstileSecretKey string `json:"turnstile_secret_key"`

	// OEM设置
	SiteName         string       `json:"site_name"`
	SiteLogo         string       `json:"site_logo"`
	SiteSubtitle     string       `json:"site_subtitle"`
	APIBaseURL       string       `json:"api_base_url"`
	ContactInfo      string       `json:"contact_info"`
	AfterSaleContact []dto.KVItem `json:"after_sale_contact"`
	DocURL           string       `json:"doc_url"`

	// 默认配置
	DefaultConcurrency                int     `json:"default_concurrency"`
	DefaultBalance                    float64 `json:"default_balance"`
	GatewayRetrySwitchAfter           int     `json:"gateway_retry_switch_after"`
	DailyUsageRefreshTime             string  `json:"daily_usage_refresh_time"`
	Auth401CooldownSeconds            int     `json:"auth_401_cooldown_seconds"`
	UsageWindowDisablePercent         int     `json:"usage_window_disable_percent"`
	UserConcurrencyWaitTimeoutSeconds int     `json:"user_concurrency_wait_timeout_seconds"`

	// Model fallback configuration
	EnableModelFallback      bool   `json:"enable_model_fallback"`
	FallbackModelAnthropic   string `json:"fallback_model_anthropic"`
	FallbackModelOpenAI      string `json:"fallback_model_openai"`
	FallbackModelGemini      string `json:"fallback_model_gemini"`
	FallbackModelAntigravity string `json:"fallback_model_antigravity"`
}

// UpdateSettings 更新系统设置
// PUT /api/v1/admin/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 验证参数
	if req.DefaultConcurrency < 1 {
		req.DefaultConcurrency = 1
	}
	if req.DefaultBalance < 0 {
		req.DefaultBalance = 0
	}
	if req.GatewayRetrySwitchAfter <= 0 {
		req.GatewayRetrySwitchAfter = 2
	}
	if strings.TrimSpace(req.DailyUsageRefreshTime) == "" {
		req.DailyUsageRefreshTime = "03:00"
	}
	if req.Auth401CooldownSeconds <= 0 {
		req.Auth401CooldownSeconds = 300
	}
	if req.UsageWindowDisablePercent <= 0 {
		req.UsageWindowDisablePercent = 100
	} else if req.UsageWindowDisablePercent > 100 {
		req.UsageWindowDisablePercent = 100
	}
	if req.UserConcurrencyWaitTimeoutSeconds <= 0 {
		req.UserConcurrencyWaitTimeoutSeconds = 30
	}
	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	if req.AlertCooldownMinutes < 0 {
		req.AlertCooldownMinutes = 0
	}

	// Turnstile 参数验证
	if req.TurnstileEnabled {
		// 检查必填字段
		if req.TurnstileSiteKey == "" {
			response.BadRequest(c, "Turnstile Site Key is required when enabled")
			return
		}
		if req.TurnstileSecretKey == "" {
			response.BadRequest(c, "Turnstile Secret Key is required when enabled")
			return
		}

		// 获取当前设置，检查参数是否有变化
		currentSettings, err := h.settingService.GetAllSettings(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		// 当 site_key 或 secret_key 任一变化时验证（避免配置错误导致无法登录）
		siteKeyChanged := currentSettings.TurnstileSiteKey != req.TurnstileSiteKey
		secretKeyChanged := currentSettings.TurnstileSecretKey != req.TurnstileSecretKey
		if siteKeyChanged || secretKeyChanged {
			if err := h.turnstileService.ValidateSecretKey(c.Request.Context(), req.TurnstileSecretKey); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
	}

	settings := &service.SystemSettings{
		RegistrationEnabled:  req.RegistrationEnabled,
		EmailVerifyEnabled:   req.EmailVerifyEnabled,
		AlertEmails:          req.AlertEmails,
		AlertCooldownMinutes: req.AlertCooldownMinutes,
		SMTPHost:             req.SMTPHost,
		SMTPPort:             req.SMTPPort,
		SMTPUsername:         req.SMTPUsername,
		SMTPPassword:         req.SMTPPassword,
		SMTPFrom:             req.SMTPFrom,
		SMTPFromName:         req.SMTPFromName,
		SMTPUseTLS:           req.SMTPUseTLS,
		TurnstileEnabled:     req.TurnstileEnabled,
		TurnstileSiteKey:     req.TurnstileSiteKey,
		TurnstileSecretKey:   req.TurnstileSecretKey,
		SiteName:             req.SiteName,
		SiteLogo:             req.SiteLogo,
		SiteSubtitle:         req.SiteSubtitle,
		APIBaseURL:           req.APIBaseURL,
		ContactInfo:          req.ContactInfo,
		AfterSaleContact: func() []service.KVItem {
			out := make([]service.KVItem, 0, len(req.AfterSaleContact))
			for _, item := range req.AfterSaleContact {
				out = append(out, service.KVItem{K: item.K, V: item.V})
			}
			return out
		}(),
		DocURL:                            req.DocURL,
		DefaultConcurrency:                req.DefaultConcurrency,
		DefaultBalance:                    req.DefaultBalance,
		GatewayRetrySwitchAfter:           req.GatewayRetrySwitchAfter,
		DailyUsageRefreshTime:             req.DailyUsageRefreshTime,
		Auth401CooldownSeconds:            req.Auth401CooldownSeconds,
		UsageWindowDisablePercent:         req.UsageWindowDisablePercent,
		UserConcurrencyWaitTimeoutSeconds: req.UserConcurrencyWaitTimeoutSeconds,
		EnableModelFallback:               req.EnableModelFallback,
		FallbackModelAnthropic:            req.FallbackModelAnthropic,
		FallbackModelOpenAI:               req.FallbackModelOpenAI,
		FallbackModelGemini:               req.FallbackModelGemini,
		FallbackModelAntigravity:          req.FallbackModelAntigravity,
	}

	if err := h.settingService.UpdateSettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedAfterSaleContact := make([]dto.KVItem, 0, len(updatedSettings.AfterSaleContact))
	for _, item := range updatedSettings.AfterSaleContact {
		updatedAfterSaleContact = append(updatedAfterSaleContact, dto.KVItem{K: item.K, V: item.V})
	}

	response.Success(c, dto.SystemSettings{
		RegistrationEnabled:               updatedSettings.RegistrationEnabled,
		EmailVerifyEnabled:                updatedSettings.EmailVerifyEnabled,
		AlertEmails:                       updatedSettings.AlertEmails,
		AlertCooldownMinutes:              updatedSettings.AlertCooldownMinutes,
		SMTPHost:                          updatedSettings.SMTPHost,
		SMTPPort:                          updatedSettings.SMTPPort,
		SMTPUsername:                      updatedSettings.SMTPUsername,
		SMTPPassword:                      updatedSettings.SMTPPassword,
		SMTPFrom:                          updatedSettings.SMTPFrom,
		SMTPFromName:                      updatedSettings.SMTPFromName,
		SMTPUseTLS:                        updatedSettings.SMTPUseTLS,
		TurnstileEnabled:                  updatedSettings.TurnstileEnabled,
		TurnstileSiteKey:                  updatedSettings.TurnstileSiteKey,
		TurnstileSecretKey:                updatedSettings.TurnstileSecretKey,
		SiteName:                          updatedSettings.SiteName,
		SiteLogo:                          updatedSettings.SiteLogo,
		SiteSubtitle:                      updatedSettings.SiteSubtitle,
		APIBaseURL:                        updatedSettings.APIBaseURL,
		ContactInfo:                       updatedSettings.ContactInfo,
		AfterSaleContact:                  updatedAfterSaleContact,
		DocURL:                            updatedSettings.DocURL,
		DefaultConcurrency:                updatedSettings.DefaultConcurrency,
		DefaultBalance:                    updatedSettings.DefaultBalance,
		GatewayRetrySwitchAfter:           updatedSettings.GatewayRetrySwitchAfter,
		DailyUsageRefreshTime:             updatedSettings.DailyUsageRefreshTime,
		Auth401CooldownSeconds:            updatedSettings.Auth401CooldownSeconds,
		UsageWindowDisablePercent:         updatedSettings.UsageWindowDisablePercent,
		UserConcurrencyWaitTimeoutSeconds: updatedSettings.UserConcurrencyWaitTimeoutSeconds,
		EnableModelFallback:               updatedSettings.EnableModelFallback,
		FallbackModelAnthropic:            updatedSettings.FallbackModelAnthropic,
		FallbackModelOpenAI:               updatedSettings.FallbackModelOpenAI,
		FallbackModelGemini:               updatedSettings.FallbackModelGemini,
		FallbackModelAntigravity:          updatedSettings.FallbackModelAntigravity,
	})
}

// TestSMTPRequest 测试SMTP连接请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host" binding:"required"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// TestSMTPConnection 测试SMTP连接
// POST /api/v1/admin/settings/test-smtp
func (h *SettingHandler) TestSMTPConnection(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}

	// 如果未提供密码，从数据库获取已保存的密码
	password := req.SMTPPassword
	if password == "" {
		savedConfig, err := h.emailService.GetSMTPConfig(c.Request.Context())
		if err == nil && savedConfig != nil {
			password = savedConfig.Password
		}
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		UseTLS:   req.SMTPUseTLS,
	}

	err := h.emailService.TestSMTPConnectionWithConfig(config)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "SMTP connection successful"})
}

// SendTestEmailRequest 发送测试邮件请求
type SendTestEmailRequest struct {
	Email        string `json:"email" binding:"required,email"`
	SMTPHost     string `json:"smtp_host" binding:"required"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// SendTestEmail 发送测试邮件
// POST /api/v1/admin/settings/send-test-email
func (h *SettingHandler) SendTestEmail(c *gin.Context) {
	var req SendTestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}

	// 如果未提供密码，从数据库获取已保存的密码
	password := req.SMTPPassword
	if password == "" {
		savedConfig, err := h.emailService.GetSMTPConfig(c.Request.Context())
		if err == nil && savedConfig != nil {
			password = savedConfig.Password
		}
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		From:     req.SMTPFrom,
		FromName: req.SMTPFromName,
		UseTLS:   req.SMTPUseTLS,
	}

	siteName := h.settingService.GetSiteName(c.Request.Context())
	subject := "[" + siteName + "] Test Email"
	body := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .content { padding: 40px 30px; text-align: center; }
        .success { color: #10b981; font-size: 48px; margin-bottom: 20px; }
        .footer { background-color: #f8f9fa; padding: 20px; text-align: center; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + siteName + `</h1>
        </div>
        <div class="content">
            <div class="success">✓</div>
            <h2>Email Configuration Successful!</h2>
            <p>This is a test email to verify your SMTP settings are working correctly.</p>
        </div>
        <div class="footer">
            <p>This is an automated test message.</p>
        </div>
    </div>
</body>
</html>
`

	if err := h.emailService.SendEmailWithConfig(config, req.Email, subject, body); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Test email sent successfully"})
}

// GetAdminAPIKey 获取管理员 API Key 状态
// GET /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) GetAdminAPIKey(c *gin.Context) {
	maskedKey, exists, err := h.settingService.GetAdminAPIKeyStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"exists":     exists,
		"masked_key": maskedKey,
	})
}

// RegenerateAdminAPIKey 生成/重新生成管理员 API Key
// POST /api/v1/admin/settings/admin-api-key/regenerate
func (h *SettingHandler) RegenerateAdminAPIKey(c *gin.Context) {
	key, err := h.settingService.GenerateAdminAPIKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"key": key, // 完整 key 只在生成时返回一次
	})
}

// DeleteAdminAPIKey 删除管理员 API Key
// DELETE /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) DeleteAdminAPIKey(c *gin.Context) {
	if err := h.settingService.DeleteAdminAPIKey(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Admin API key deleted"})
}
