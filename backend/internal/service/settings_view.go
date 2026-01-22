package service

type KVItem struct {
	K string `json:"k"`
	V string `json:"v"`
}

type SystemSettings struct {
	RegistrationEnabled bool
	EmailVerifyEnabled  bool

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string
	SMTPUseTLS   bool

	TurnstileEnabled   bool
	TurnstileSiteKey   string
	TurnstileSecretKey string

	SiteName         string
	SiteLogo         string
	SiteSubtitle     string
	APIBaseURL       string
	ContactInfo      string
	AfterSaleContact []KVItem
	DocURL           string

	DefaultConcurrency      int
	DefaultBalance          float64
	GatewayRetrySwitchAfter int
	DailyUsageRefreshTime   string

	// Model fallback configuration
	EnableModelFallback      bool   `json:"enable_model_fallback"`
	FallbackModelAnthropic   string `json:"fallback_model_anthropic"`
	FallbackModelOpenAI      string `json:"fallback_model_openai"`
	FallbackModelGemini      string `json:"fallback_model_gemini"`
	FallbackModelAntigravity string `json:"fallback_model_antigravity"`
}

type PublicSettings struct {
	RegistrationEnabled bool
	EmailVerifyEnabled  bool
	TurnstileEnabled    bool
	TurnstileSiteKey    string
	SiteName            string
	SiteLogo            string
	SiteSubtitle        string
	APIBaseURL          string
	ContactInfo         string
	AfterSaleContact    []KVItem
	DocURL              string
	Version             string
}
