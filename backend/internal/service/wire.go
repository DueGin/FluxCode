package service

import (
	"database/sql"
	"time"

	"github.com/DueGin/FluxCode/ent"
	"github.com/DueGin/FluxCode/internal/config"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// BuildInfo contains build information
type BuildInfo struct {
	Version   string
	BuildType string
}

// ProvidePricingService creates and initializes PricingService
func ProvidePricingService(cfg *config.Config, remoteClient PricingRemoteClient) (*PricingService, error) {
	svc := NewPricingService(cfg, remoteClient)
	if err := svc.Initialize(); err != nil {
		// Pricing service initialization failure should not block startup, use fallback prices
		println("[Service] Warning: Pricing service initialization failed:", err.Error())
	}
	return svc, nil
}

// ProvideUpdateService creates UpdateService with BuildInfo
func ProvideUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, buildInfo BuildInfo) *UpdateService {
	return NewUpdateService(cache, githubClient, buildInfo.Version, buildInfo.BuildType)
}

// ProvideEmailQueueService creates EmailQueueService with default worker count
func ProvideEmailQueueService(rdb *redis.Client, emailService *EmailService) *EmailQueueService {
	return NewEmailQueueService(rdb, emailService, 3)
}

// ProvideUsageQueueService creates UsageQueueService with default worker count
func ProvideUsageQueueService(
	rdb *redis.Client,
	entClient *ent.Client,
	cfg *config.Config,
	billingService *BillingService,
	usageLogRepo UsageLogRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	billingCacheService *BillingCacheService,
	deferredService *DeferredService,
) *UsageQueueService {
	return NewUsageQueueService(
		rdb,
		entClient,
		cfg,
		billingService,
		usageLogRepo,
		userRepo,
		userSubRepo,
		billingCacheService,
		deferredService,
		6,
	)
}

// ProvideTokenRefreshService creates and starts TokenRefreshService
func ProvideTokenRefreshService(
	accountRepo AccountRepository,
	oauthService *OAuthService,
	openaiOAuthService *OpenAIOAuthService,
	geminiOAuthService *GeminiOAuthService,
	antigravityOAuthService *AntigravityOAuthService,
	rdb *redis.Client,
	cfg *config.Config,
) *TokenRefreshService {
	svc := NewTokenRefreshService(accountRepo, oauthService, openaiOAuthService, geminiOAuthService, antigravityOAuthService, rdb, cfg)
	svc.Start()
	return svc
}

// ProvideTimingWheelService creates and starts TimingWheelService
func ProvideTimingWheelService() *TimingWheelService {
	svc := NewTimingWheelService()
	svc.Start()
	return svc
}

// ProvideDeferredService creates and starts DeferredService
func ProvideDeferredService(accountRepo AccountRepository, timingWheel *TimingWheelService) *DeferredService {
	svc := NewDeferredService(accountRepo, timingWheel, 10*time.Second)
	svc.Start()
	return svc
}

func ProvideAccountExpirationWorker(db *sql.DB, timingWheel *TimingWheelService) *AccountExpirationWorker {
	svc := NewAccountExpirationWorker(db, timingWheel, 30*time.Second)
	svc.Start()
	return svc
}

func ProvideSubscriptionExpirationWorker(db *sql.DB, timingWheel *TimingWheelService) *SubscriptionExpirationWorker {
	svc := NewSubscriptionExpirationWorker(db, timingWheel, 1*time.Minute)
	svc.Start()
	return svc
}

func ProvideDailyUsageRefreshWorker(
	db *sql.DB,
	settingService *SettingService,
	accountRepo AccountRepository,
	usageService *AccountUsageService,
	rateLimitService *RateLimitService,
	httpUpstream HTTPUpstream,
) *DailyUsageRefreshWorker {
	svc := NewDailyUsageRefreshWorker(db, settingService, accountRepo, usageService, rateLimitService, httpUpstream)
	if settingService != nil {
		settingService.RegisterDailyUsageRefreshTimeListener(svc.ResetSchedule)
	}
	svc.Start()
	return svc
}

func ProvideRateLimitReactivateWorker(
	db *sql.DB,
	timingWheel *TimingWheelService,
	accountRepo AccountRepository,
	dailyUsageRefreshWorker *DailyUsageRefreshWorker,
) *RateLimitReactivateWorker {
	svc := NewRateLimitReactivateWorker(db, timingWheel, accountRepo, dailyUsageRefreshWorker, 30*time.Second)
	svc.Start()
	return svc
}

// ProvideConcurrencyService creates ConcurrencyService and starts slot cleanup worker.
func ProvideConcurrencyService(cache ConcurrencyCache, accountRepo AccountRepository, cfg *config.Config) *ConcurrencyService {
	svc := NewConcurrencyService(cache)
	if cfg != nil {
		svc.StartSlotCleanupWorker(accountRepo, cfg.Gateway.Scheduling.SlotCleanupInterval)
	}
	return svc
}

// ProviderSet is the Wire provider set for all services
var ProviderSet = wire.NewSet(
	// Core services
	NewAuthService,
	NewUserService,
	NewAPIKeyService,
	NewGroupService,
	NewPricingPlanService,
	NewAccountService,
	NewProxyService,
	NewRedeemService,
	NewUsageService,
	NewDashboardService,
	ProvidePricingService,
	NewBillingService,
	NewBillingCacheService,
	NewAdminService,
	NewGatewayService,
	NewOpenAIGatewayService,
	NewOAuthService,
	NewOpenAIOAuthService,
	NewGeminiOAuthService,
	NewGeminiQuotaService,
	NewAntigravityOAuthService,
	NewGeminiTokenProvider,
	NewGeminiMessagesCompatService,
	NewAntigravityTokenProvider,
	NewAntigravityGatewayService,
	NewRateLimitService,
	NewAccountUsageService,
	NewAccountTestService,
	NewSettingService,
	NewEmailService,
	NewAlertService,
	ProvideEmailQueueService,
	ProvideUsageQueueService,
	NewTurnstileService,
	NewSubscriptionService,
	ProvideConcurrencyService,
	NewIdentityService,
	ProvideUpdateService,
	ProvideTokenRefreshService,
	ProvideTimingWheelService,
	ProvideDeferredService,
	ProvideAccountExpirationWorker,
	ProvideSubscriptionExpirationWorker,
	ProvideDailyUsageRefreshWorker,
	ProvideRateLimitReactivateWorker,
	NewAntigravityQuotaFetcher,
	NewUserAttributeService,
	NewUsageCache,
)
