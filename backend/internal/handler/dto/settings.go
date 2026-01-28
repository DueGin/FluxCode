package dto

type KVItem struct {
	K string `json:"k"`
	V string `json:"v"`
}

// SystemSettings represents the admin settings API response payload.
type SystemSettings struct {
	RegistrationEnabled bool `json:"registration_enabled"`
	EmailVerifyEnabled  bool `json:"email_verify_enabled"`

	AlertEmails          []string `json:"alert_emails"`
	AlertCooldownMinutes int      `json:"alert_cooldown_minutes"`

	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password,omitempty"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`

	TurnstileEnabled   bool   `json:"turnstile_enabled"`
	TurnstileSiteKey   string `json:"turnstile_site_key"`
	TurnstileSecretKey string `json:"turnstile_secret_key,omitempty"`

	SiteName             string   `json:"site_name"`
	SiteLogo             string   `json:"site_logo"`
	SiteSubtitle         string   `json:"site_subtitle"`
	APIBaseURL           string   `json:"api_base_url"`
	ContactInfo          string   `json:"contact_info"`
	AfterSaleContact     []KVItem `json:"after_sale_contact"`
	DocURL               string   `json:"doc_url"`
	QQGroupPopupTitle    string   `json:"qq_group_popup_title"`
	QQGroupPopupMarkdown string   `json:"qq_group_popup_markdown"`

	DefaultConcurrency                int     `json:"default_concurrency"`
	DefaultBalance                    float64 `json:"default_balance"`
	GatewayRetrySwitchAfter           int     `json:"gateway_retry_switch_after"`
	Auth401CooldownSeconds            int     `json:"auth_401_cooldown_seconds"`
	UsageWindowDisablePercent         int     `json:"usage_window_disable_percent"`
	UsageWindowCooldownSeconds        int     `json:"usage_window_cooldown_seconds"`
	UserConcurrencyWaitTimeoutSeconds int     `json:"user_concurrency_wait_timeout_seconds"`

	// Model fallback configuration
	EnableModelFallback      bool   `json:"enable_model_fallback"`
	FallbackModelAnthropic   string `json:"fallback_model_anthropic"`
	FallbackModelOpenAI      string `json:"fallback_model_openai"`
	FallbackModelGemini      string `json:"fallback_model_gemini"`
	FallbackModelAntigravity string `json:"fallback_model_antigravity"`
}

type PublicSettings struct {
	RegistrationEnabled  bool     `json:"registration_enabled"`
	EmailVerifyEnabled   bool     `json:"email_verify_enabled"`
	TurnstileEnabled     bool     `json:"turnstile_enabled"`
	TurnstileSiteKey     string   `json:"turnstile_site_key"`
	SiteName             string   `json:"site_name"`
	SiteLogo             string   `json:"site_logo"`
	SiteSubtitle         string   `json:"site_subtitle"`
	APIBaseURL           string   `json:"api_base_url"`
	ContactInfo          string   `json:"contact_info"`
	AfterSaleContact     []KVItem `json:"after_sale_contact"`
	DocURL               string   `json:"doc_url"`
	QQGroupPopupTitle    string   `json:"qq_group_popup_title"`
	QQGroupPopupMarkdown string   `json:"qq_group_popup_markdown"`
	Version              string   `json:"version"`
}
