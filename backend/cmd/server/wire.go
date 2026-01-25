//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"net/http"
	"time"

	"github.com/DueGin/FluxCode/ent"
	"github.com/DueGin/FluxCode/internal/config"
	"github.com/DueGin/FluxCode/internal/handler"
	"github.com/DueGin/FluxCode/internal/repository"
	"github.com/DueGin/FluxCode/internal/server"
	"github.com/DueGin/FluxCode/internal/server/middleware"
	"github.com/DueGin/FluxCode/internal/service"

	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

type Application struct {
	Server  *http.Server
	Cleanup func()
}

func initializeApplication(buildInfo handler.BuildInfo) (*Application, error) {
	wire.Build(
		// Infrastructure layer ProviderSets
		config.ProviderSet,

		// Business layer ProviderSets
		repository.ProviderSet,
		service.ProviderSet,
		middleware.ProviderSet,
		handler.ProviderSet,

		// Server layer ProviderSet
		server.ProviderSet,

		// BuildInfo provider
		provideServiceBuildInfo,

		// Cleanup function provider
		provideCleanup,

		// Application struct
		wire.Struct(new(Application), "Server", "Cleanup"),
	)
	return nil, nil
}

func provideServiceBuildInfo(buildInfo handler.BuildInfo) service.BuildInfo {
	return service.BuildInfo{
		Version:   buildInfo.Version,
		BuildType: buildInfo.BuildType,
	}
}

func provideCleanup(
	entClient *ent.Client,
	rdb *redis.Client,
	tokenRefresh *service.TokenRefreshService,
	pricing *service.PricingService,
	emailQueue *service.EmailQueueService,
	usageQueue *service.UsageQueueService,
	billingCache *service.BillingCacheService,
	accountExpirationWorker *service.AccountExpirationWorker,
	subscriptionExpirationWorker *service.SubscriptionExpirationWorker,
	dailyUsageRefreshWorker *service.DailyUsageRefreshWorker,
	rateLimitReactivateWorker *service.RateLimitReactivateWorker,
	oauth *service.OAuthService,
	openaiOAuth *service.OpenAIOAuthService,
	geminiOAuth *service.GeminiOAuthService,
	antigravityOAuth *service.AntigravityOAuthService,
) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Cleanup steps in reverse dependency order
		cleanupSteps := []struct {
			name string
			fn   func() error
		}{
			{"SubscriptionExpirationWorker", func() error {
				subscriptionExpirationWorker.Stop()
				return nil
			}},
			{"AccountExpirationWorker", func() error {
				accountExpirationWorker.Stop()
				return nil
			}},
				{"DailyUsageRefreshWorker", func() error {
					dailyUsageRefreshWorker.Stop()
					return nil
				}},
				{"RateLimitReactivateWorker", func() error {
					rateLimitReactivateWorker.Stop()
					return nil
				}},
				{"TokenRefreshService", func() error {
					tokenRefresh.Stop()
					return nil
				}},
			{"PricingService", func() error {
				pricing.Stop()
				return nil
			}},
			{"EmailQueueService", func() error {
				emailQueue.Stop()
				return nil
			}},
			{"UsageQueueService", func() error {
				usageQueue.Stop()
				return nil
			}},
			{"BillingCacheService", func() error {
				billingCache.Stop()
				return nil
			}},
			{"OAuthService", func() error {
				oauth.Stop()
				return nil
			}},
			{"OpenAIOAuthService", func() error {
				openaiOAuth.Stop()
				return nil
			}},
			{"GeminiOAuthService", func() error {
				geminiOAuth.Stop()
				return nil
			}},
			{"AntigravityOAuthService", func() error {
				antigravityOAuth.Stop()
				return nil
			}},
			{"Redis", func() error {
				return rdb.Close()
			}},
			{"Ent", func() error {
				return entClient.Close()
			}},
		}

		for _, step := range cleanupSteps {
			if err := step.fn(); err != nil {
				applog.Printf("[Cleanup] %s failed: %v", step.name, err)
				// Continue with remaining cleanup steps even if one fails
			} else {
				applog.Printf("[Cleanup] %s succeeded", step.name)
			}
		}

		// Check if context timed out
		select {
		case <-ctx.Done():
			applog.Printf("[Cleanup] Warning: cleanup timed out after 10 seconds")
		default:
			applog.Printf("[Cleanup] All cleanup steps completed")
		}
	}
}
