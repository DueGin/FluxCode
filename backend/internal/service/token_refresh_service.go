package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/DueGin/FluxCode/internal/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const tokenRefreshLockKey = "fluxcode:lock:token_refresh"

var tokenRefreshLockExtendScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0
`)

var tokenRefreshLockReleaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

// TokenRefreshService OAuth token自动刷新服务
// 定期检查并刷新即将过期的token
type TokenRefreshService struct {
	accountRepo AccountRepository
	refreshers  []TokenRefresher
	cfg         *config.TokenRefreshConfig
	rdb         *redis.Client

	stopCh chan struct{}
	wg     sync.WaitGroup

	cycleMu     sync.Mutex
	cycleCancel context.CancelFunc
}

// NewTokenRefreshService 创建token刷新服务
func NewTokenRefreshService(
	accountRepo AccountRepository,
	oauthService *OAuthService,
	openaiOAuthService *OpenAIOAuthService,
	geminiOAuthService *GeminiOAuthService,
	antigravityOAuthService *AntigravityOAuthService,
	rdb *redis.Client,
	cfg *config.Config,
) *TokenRefreshService {
	s := &TokenRefreshService{
		accountRepo: accountRepo,
		cfg:         &cfg.TokenRefresh,
		rdb:         rdb,
		stopCh:      make(chan struct{}),
	}

	// 注册平台特定的刷新器
	s.refreshers = []TokenRefresher{
		NewClaudeTokenRefresher(oauthService),
		NewOpenAITokenRefresher(openaiOAuthService),
		NewGeminiTokenRefresher(geminiOAuthService),
		NewAntigravityTokenRefresher(antigravityOAuthService),
	}

	return s
}

// Start 启动后台刷新服务
func (s *TokenRefreshService) Start() {
	if !s.cfg.Enabled {
		log.Println("[TokenRefresh] Service disabled by configuration")
		return
	}

	s.wg.Add(1)
	go s.refreshLoop()

	log.Printf("[TokenRefresh] Service started (check every %d minutes, refresh %v hours before expiry)",
		s.cfg.CheckIntervalMinutes, s.cfg.RefreshBeforeExpiryHours)
}

// Stop 停止刷新服务
func (s *TokenRefreshService) Stop() {
	s.cycleMu.Lock()
	if s.cycleCancel != nil {
		s.cycleCancel()
	}
	s.cycleMu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	log.Println("[TokenRefresh] Service stopped")
}

// refreshLoop 刷新循环
func (s *TokenRefreshService) refreshLoop() {
	defer s.wg.Done()

	// 计算检查间隔
	checkInterval := time.Duration(s.cfg.CheckIntervalMinutes) * time.Minute
	if checkInterval < time.Minute {
		checkInterval = 5 * time.Minute
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// 启动时立即执行一次检查
	s.processRefresh()

	for {
		select {
		case <-ticker.C:
			s.processRefresh()
		case <-s.stopCh:
			return
		}
	}
}

// processRefresh 执行一次刷新检查
func (s *TokenRefreshService) processRefresh() {
	lockTTL := s.lockTTL()
	lockDeadline := time.Now().Add(lockTTL)

	ctx, cancel := context.WithDeadline(context.Background(), lockDeadline)
	defer cancel()

	s.cycleMu.Lock()
	s.cycleCancel = cancel
	s.cycleMu.Unlock()
	defer func() {
		s.cycleMu.Lock()
		s.cycleCancel = nil
		s.cycleMu.Unlock()
	}()

	unlock, shouldRun := s.acquireDistributedLock(ctx, cancel, lockTTL, lockDeadline)
	if !shouldRun {
		return
	}
	defer unlock()

	// 计算刷新窗口
	refreshWindow := time.Duration(s.cfg.RefreshBeforeExpiryHours * float64(time.Hour))

	// 获取所有active状态的账号
	accounts, err := s.listActiveAccounts(ctx)
	if err != nil {
		log.Printf("[TokenRefresh] Failed to list accounts: %v", err)
		return
	}

	totalAccounts := len(accounts)
	oauthAccounts := 0 // 可刷新的OAuth账号数
	needsRefresh := 0  // 需要刷新的账号数
	refreshed, failed := 0, 0

	for i := range accounts {
		if err := ctx.Err(); err != nil {
			log.Printf("[TokenRefresh] Refresh cycle cancelled: %v", err)
			break
		}
		account := &accounts[i]

		// 遍历所有刷新器，找到能处理此账号的
		for _, refresher := range s.refreshers {
			if err := ctx.Err(); err != nil {
				break
			}
			if !refresher.CanRefresh(account) {
				continue
			}

			oauthAccounts++

			// 检查是否需要刷新
			if !refresher.NeedsRefresh(account, refreshWindow) {
				break // 不需要刷新，跳过
			}

			needsRefresh++

			// 执行刷新
			if err := s.refreshWithRetry(ctx, account, refresher); err != nil {
				log.Printf("[TokenRefresh] Account %d (%s) failed: %v", account.ID, account.Name, err)
				failed++
			} else {
				log.Printf("[TokenRefresh] Account %d (%s) refreshed successfully", account.ID, account.Name)
				refreshed++
			}

			// 每个账号只由一个refresher处理
			break
		}
	}

	// 始终打印周期日志，便于跟踪服务运行状态
	log.Printf("[TokenRefresh] Cycle complete: total=%d, oauth=%d, needs_refresh=%d, refreshed=%d, failed=%d",
		totalAccounts, oauthAccounts, needsRefresh, refreshed, failed)
}

func (s *TokenRefreshService) acquireDistributedLock(ctx context.Context, cancel func(), ttl time.Duration, maxHoldUntil time.Time) (unlock func(), shouldRun bool) {
	// Redis 不可用：退化为无锁执行（无法保证多机互斥，但避免 token 过期）
	if s.rdb == nil {
		return func() {}, true
	}

	token := uuid.NewString()

	ok, err := s.rdb.SetNX(ctx, tokenRefreshLockKey, token, ttl).Result()
	if err != nil {
		log.Printf("[TokenRefresh] Acquire redis lock failed (degraded, will run without lock): %v", err)
		return func() {}, true
	}
	if !ok {
		log.Printf("[TokenRefresh] Skip refresh cycle: another instance holds lock")
		return func() {}, false
	}

	// 续租：如果锁丢失则取消 ctx，尽量减少并发写入风险
	renewStop := make(chan struct{})
	renewDone := make(chan struct{})
	go func() {
		defer close(renewDone)
		interval := ttl / 3
		if interval < 2*time.Second {
			interval = 2 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-renewStop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			remaining := time.Until(maxHoldUntil)
			if remaining <= 0 {
				cancel()
				return
			}

			ttlMs := int64(remaining / time.Millisecond)
			if ttlMs <= 0 {
				cancel()
				return
			}

			res, err := tokenRefreshLockExtendScript.Run(ctx, s.rdb, []string{tokenRefreshLockKey}, token, ttlMs).Int64()
			if err != nil {
				log.Printf("[TokenRefresh] Renew redis lock failed: %v", err)
				continue
			}
			if res == 0 {
				log.Printf("[TokenRefresh] Lost redis lock, cancelling refresh cycle")
				cancel()
				return
			}
		}
	}()

	return func() {
		close(renewStop)
		<-renewDone
		_, _ = tokenRefreshLockReleaseScript.Run(context.Background(), s.rdb, []string{tokenRefreshLockKey}, token).Int64()
	}, true
}

func (s *TokenRefreshService) lockTTL() time.Duration {
	checkInterval := time.Duration(s.cfg.CheckIntervalMinutes) * time.Minute
	if checkInterval < time.Minute {
		checkInterval = 5 * time.Minute
	}

	// 经验值：锁 TTL 至少 10 分钟，且为 checkInterval 的 2 倍（防止执行耗时 > 间隔）。
	ttl := checkInterval * 2
	if ttl < 10*time.Minute {
		ttl = 10 * time.Minute
	}
	if ttl > 2*time.Hour {
		ttl = 2 * time.Hour
	}
	return ttl
}

// listActiveAccounts 获取所有active状态的账号
// 使用ListActive确保刷新所有活跃账号的token（包括临时禁用的）
func (s *TokenRefreshService) listActiveAccounts(ctx context.Context) ([]Account, error) {
	return s.accountRepo.ListActive(ctx)
}

// refreshWithRetry 带重试的刷新
func (s *TokenRefreshService) refreshWithRetry(ctx context.Context, account *Account, refresher TokenRefresher) error {
	var lastErr error

	for attempt := 1; attempt <= s.cfg.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		newCredentials, err := refresher.Refresh(ctx, account)
		if err == nil {
			// 刷新成功，更新账号credentials
			account.Credentials = newCredentials
			if err := s.accountRepo.Update(ctx, account); err != nil {
				return fmt.Errorf("failed to save credentials: %w", err)
			}
			return nil
		}

		// Antigravity 账户：不可重试错误直接标记 error 状态并返回
		if account.Platform == PlatformAntigravity && isNonRetryableRefreshError(err) {
			errorMsg := fmt.Sprintf("Token refresh failed (non-retryable): %v", err)
			if setErr := s.accountRepo.SetError(ctx, account.ID, errorMsg); setErr != nil {
				log.Printf("[TokenRefresh] Failed to set error status for account %d: %v", account.ID, setErr)
			}
			return err
		}

		lastErr = err
		log.Printf("[TokenRefresh] Account %d attempt %d/%d failed: %v",
			account.ID, attempt, s.cfg.MaxRetries, err)

		// 如果还有重试机会，等待后重试
		if attempt < s.cfg.MaxRetries {
			// 指数退避：2^(attempt-1) * baseSeconds
			backoff := time.Duration(s.cfg.RetryBackoffSeconds) * time.Second * time.Duration(1<<(attempt-1))
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	// Antigravity 账户：其他错误仅记录日志，不标记 error（可能是临时网络问题）
	// 其他平台账户：重试失败后标记 error
	if account.Platform == PlatformAntigravity {
		log.Printf("[TokenRefresh] Account %d: refresh failed after %d retries: %v", account.ID, s.cfg.MaxRetries, lastErr)
	} else {
		errorMsg := fmt.Sprintf("Token refresh failed after %d retries: %v", s.cfg.MaxRetries, lastErr)
		if err := s.accountRepo.SetError(ctx, account.ID, errorMsg); err != nil {
			log.Printf("[TokenRefresh] Failed to set error status for account %d: %v", account.ID, err)
		}
	}

	return lastErr
}

// isNonRetryableRefreshError 判断是否为不可重试的刷新错误
// 这些错误通常表示凭证已失效，需要用户重新授权
func isNonRetryableRefreshError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	nonRetryable := []string{
		"invalid_grant",       // refresh_token 已失效
		"invalid_client",      // 客户端配置错误
		"unauthorized_client", // 客户端未授权
		"access_denied",       // 访问被拒绝
	}
	for _, needle := range nonRetryable {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
