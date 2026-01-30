package service

// Status constants
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusError    = "error"
	StatusBanned   = "banned"
	StatusUnused   = "unused"
	StatusUsed     = "used"
	StatusExpired  = "expired"
)

// Role constants
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Platform constants
const (
	PlatformAnthropic   = "anthropic"
	PlatformOpenAI      = "openai"
	PlatformGemini      = "gemini"
	PlatformAntigravity = "antigravity"
)

// Account type constants
const (
	AccountTypeOAuth      = "oauth"       // OAuth类型账号（full scope: profile + inference）
	AccountTypeSetupToken = "setup-token" // Setup Token类型账号（inference only scope）
	AccountTypeAPIKey     = "apikey"      // API Key类型账号
)

// Redeem type constants
const (
	RedeemTypeBalance      = "balance"
	RedeemTypeConcurrency  = "concurrency"
	RedeemTypeSubscription = "subscription"
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminBalance     = "admin_balance"     // 管理员调整余额
	AdjustmentTypeAdminConcurrency = "admin_concurrency" // 管理员调整并发数
)

// Group subscription type constants
const (
	SubscriptionTypeStandard     = "standard"     // 标准计费模式（按余额扣费）
	SubscriptionTypeSubscription = "subscription" // 订阅模式（按限额控制）
)

// Subscription status constants
const (
	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusSuspended = "suspended"
)

// Setting keys
const (
	// 注册设置
	SettingKeyRegistrationEnabled = "registration_enabled" // 是否开放注册
	SettingKeyEmailVerifyEnabled  = "email_verify_enabled" // 是否开启邮件验证

	// 邮件服务设置
	SettingKeySMTPHost     = "smtp_host"      // SMTP服务器地址
	SettingKeySMTPPort     = "smtp_port"      // SMTP端口
	SettingKeySMTPUsername = "smtp_username"  // SMTP用户名
	SettingKeySMTPPassword = "smtp_password"  // SMTP密码（加密存储）
	SettingKeySMTPFrom     = "smtp_from"      // 发件人地址
	SettingKeySMTPFromName = "smtp_from_name" // 发件人名称
	SettingKeySMTPUseTLS   = "smtp_use_tls"   // 是否使用TLS

	// Cloudflare Turnstile 设置
	SettingKeyTurnstileEnabled   = "turnstile_enabled"    // 是否启用 Turnstile 验证
	SettingKeyTurnstileSiteKey   = "turnstile_site_key"   // Turnstile Site Key
	SettingKeyTurnstileSecretKey = "turnstile_secret_key" // Turnstile Secret Key

	// OEM设置
	SettingKeySiteName             = "site_name"              // 网站名称
	SettingKeySiteLogo             = "site_logo"              // 网站Logo (base64)
	SettingKeySiteSubtitle         = "site_subtitle"          // 网站副标题
	SettingKeyAPIBaseURL           = "api_base_url"           // API端点地址（用于客户端配置和导入）
	SettingKeyContactInfo          = "contact_info"           // 客服联系方式
	SettingKeyAfterSaleContact     = "after_sale_contact"     // 售后联系方式（KV JSON 数组）
	SettingKeyDocURL               = "doc_url"                // 文档链接
	SettingKeyRedeemDeliveryText   = "redeem_delivery_text"   // 兑换码发货文案（支持 ${redeemCodes} 占位符）
	SettingKeyAttractPopupTitle    = "attract_popup_title"    // 引流弹窗标题
	SettingKeyAttractPopupMarkdown = "attract_popup_markdown" // 引流弹窗 Markdown 文案

	// 历史兼容：早期版本使用 qq_group_popup_*，保留读取/写入以平滑升级（后续可迁移后移除）。
	SettingKeyLegacyAttractPopupTitle    = "qq_group_popup_title"
	SettingKeyLegacyAttractPopupMarkdown = "qq_group_popup_markdown"

	// 默认配置
	SettingKeyDefaultConcurrency                = "default_concurrency"                   // 新用户默认并发量
	SettingKeyDefaultBalance                    = "default_balance"                       // 新用户默认余额
	SettingKeyGatewayRetrySwitchAfter           = "gateway_retry_switch_after"            // 重试多少次后切换账号调度
	SettingKeyAuth401CooldownSeconds            = "auth_401_cooldown_seconds"             // 上游鉴权401临时冷却时间（秒）
	SettingKeyUsageWindowDisablePercent         = "usage_window_disable_percent"          // 用量窗口达到多少百分比触发临时不可调度
	SettingKeyUsageWindowCooldownSeconds        = "usage_window_cooldown_seconds"         // 用量窗口超限临时不可调度时间（秒）
	SettingKeyUserConcurrencyWaitTimeoutSeconds = "user_concurrency_wait_timeout_seconds" // 用户并发槽位等待超时（秒）

	// 管理员 API Key
	SettingKeyAdminAPIKey = "admin_api_key" // 全局管理员 API Key（用于外部系统集成）

	// Gemini 配额策略（JSON）
	SettingKeyGeminiQuotaPolicy = "gemini_quota_policy"

	// Model fallback settings
	SettingKeyEnableModelFallback      = "enable_model_fallback"
	SettingKeyFallbackModelAnthropic   = "fallback_model_anthropic"
	SettingKeyFallbackModelOpenAI      = "fallback_model_openai"
	SettingKeyFallbackModelGemini      = "fallback_model_gemini"
	SettingKeyFallbackModelAntigravity = "fallback_model_antigravity"

	// 告警设置
	SettingKeyAlertEmails          = "alert_emails"           // JSON array of recipient emails
	SettingKeyAlertCooldownMinutes = "alert_cooldown_minutes" // int minutes, 0 means no cooldown
)

// AdminAPIKeyPrefix is the prefix for admin API keys (distinct from user "sk-" keys).
const AdminAPIKeyPrefix = "admin-"
