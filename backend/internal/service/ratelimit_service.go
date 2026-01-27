package service

import (
	"bytes"
	"context"
	"encoding/json"

	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DueGin/FluxCode/internal/config"
	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
	"github.com/tidwall/gjson"
)

// RateLimitService 处理限流和过载状态管理
type RateLimitService struct {
	accountRepo        AccountRepository
	usageRepo          UsageLogRepository
	cfg                *config.Config
	settingService     *SettingService
	geminiQuotaService *GeminiQuotaService
	tempUnschedCache   TempUnschedCache
	usageCacheMu       sync.RWMutex
	usageCache         map[int64]*geminiUsageCacheEntry
}

type geminiUsageCacheEntry struct {
	windowStart time.Time
	cachedAt    time.Time
	totals      GeminiUsageTotals
}

const geminiPrecheckCacheTTL = time.Minute

// NewRateLimitService 创建RateLimitService实例
func NewRateLimitService(accountRepo AccountRepository, usageRepo UsageLogRepository, cfg *config.Config, settingService *SettingService, geminiQuotaService *GeminiQuotaService, tempUnschedCache TempUnschedCache) *RateLimitService {
	return &RateLimitService{
		accountRepo:        accountRepo,
		usageRepo:          usageRepo,
		cfg:                cfg,
		settingService:     settingService,
		geminiQuotaService: geminiQuotaService,
		tempUnschedCache:   tempUnschedCache,
		usageCache:         make(map[int64]*geminiUsageCacheEntry),
	}
}

func (s *RateLimitService) auth401Cooldown(ctx context.Context) time.Duration {
	if s == nil || s.settingService == nil {
		return 5 * time.Minute
	}
	seconds := s.settingService.GetAuth401CooldownSeconds(ctx)
	return time.Duration(seconds) * time.Second
}

// HandleUpstreamError 处理上游错误响应，标记账号状态
// 返回是否应该停止该账号的调度
func (s *RateLimitService) HandleUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte) (shouldDisable bool) {
	// apikey 类型账号：检查自定义错误码配置
	// 如果启用且错误码不在列表中，则不处理（不停止调度、不标记限流/过载）
	if !account.ShouldHandleErrorCode(statusCode) {
		applog.Printf("Account %d: error %d skipped (not in custom error codes)", account.ID, statusCode)
		return false
	}

	tempMatched := false
	if statusCode != 429 {
		tempMatched = s.tryTempUnschedulable(ctx, account, statusCode, responseBody)
	}

	switch statusCode {
	case 401:
		shouldDisable = s.handle401(ctx, account, headers, responseBody, tempMatched)
	case 402:
		if tempMatched {
			s.logUpstreamAuthAbnormal(account, statusCode, headers, responseBody, tempMatched, "temp_unsched(rule_matched)", 0)
			shouldDisable = true
			break
		}
		s.logUpstreamAuthAbnormal(account, statusCode, headers, responseBody, tempMatched, "set_error", 0)
		// 支付要求：余额不足或计费问题，停止调度
		s.handleAuthError(ctx, account, s.buildAuthErrorMessage("Payment required (402): insufficient balance or billing issue", headers, responseBody))
		shouldDisable = true
	case 403:
		if tempMatched {
			s.logUpstreamAuthAbnormal(account, statusCode, headers, responseBody, tempMatched, "temp_unsched(rule_matched)", 0)
			shouldDisable = true
			break
		}
		s.logUpstreamAuthAbnormal(account, statusCode, headers, responseBody, tempMatched, "set_error", 0)
		// 禁止访问：停止调度，记录错误
		s.handleAuthError(ctx, account, s.buildAuthErrorMessage("Access forbidden (403): account may be suspended or lack permissions", headers, responseBody))
		shouldDisable = true
	case 429:
		// 429 需要区分：
		// - Quota exceeded：通常会给出明确的 reset_at（或 Retry-After），按“临时不可调度”处理，到期自动恢复。
		// - Rate limit：短暂窗口，标记限流（rate_limit_reset_at），到期自动回到可调度集合。
		if s.handleQuotaExceeded429(ctx, account, headers, responseBody) {
			shouldDisable = true
			break
		}
		s.handle429(ctx, account, headers, responseBody)
		shouldDisable = true
	case 529:
		s.handle529(ctx, account)
		shouldDisable = false
	default:
		// 其他5xx错误：记录但不停止调度
		if statusCode >= 500 {
			applog.Printf("Account %d received upstream error %d", account.ID, statusCode)
		}
		shouldDisable = false
	}

	if tempMatched {
		return true
	}
	return shouldDisable
}

func (s *RateLimitService) handle401(ctx context.Context, account *Account, headers http.Header, responseBody []byte, tempMatched bool) bool {
	if account == nil {
		return true
	}

	// 账号自定义 temp_unschedulable_rules 已命中：视为“用户显式选择”优先，不再升级为永久 error。
	//（避免规则命中后仍被 shouldHardDisable401 误判并 SetError，导致短暂 401 变成长期不可调度。）
	if tempMatched {
		s.logUpstreamAuthAbnormal(account, 401, headers, responseBody, tempMatched, "temp_unsched(rule_matched)", 0)
		return true
	}

	// 自定义 base_url（中转站/代理/自建网关）场景下，401 经常是中间层误报或短暂异常：
	// - 同一时间大量账号同时 401，通常不可能都在同一时刻失效
	// - 直接 SetError 会导致大量账号永久不可调度，需要人工逐个恢复
	// 因此对“非官方上游”的 401 一律先走短暂冷却，让调度系统自恢复，并打出更明确的决策日志。
	if trusted, _ := s.authUpstreamTrustedFor401(account); !trusted {
		cooldown := s.auth401Cooldown(ctx)
		s.logUpstreamAuthAbnormal(account, 401, headers, responseBody, tempMatched, "temp_unsched(untrusted_upstream)", cooldown)
		s.setTempUnschedulable(ctx, account, 401, headers, responseBody, cooldown)
		return true
	}

	// OAuth/Setup token 账号的 401 很可能是短暂链路/令牌刷新窗口问题：
	// - 若直接 SetError 会导致账号永久不可调度，且 TokenRefreshService 也无法再刷新（只遍历 active 状态）
	// 因此默认先做短暂冷却（temp_unschedulable），再由调度/刷新机制自行恢复。
	if account.IsOAuth() || !s.shouldHardDisable401(account, headers, responseBody) {
		decision := "temp_unsched"
		if account.IsOAuth() {
			decision = "temp_unsched(oauth)"
		}
		cooldown := s.auth401Cooldown(ctx)
		s.logUpstreamAuthAbnormal(account, 401, headers, responseBody, tempMatched, decision, cooldown)
		s.setTempUnschedulable(ctx, account, 401, headers, responseBody, cooldown)
		return true
	}

	s.logUpstreamAuthAbnormal(account, 401, headers, responseBody, tempMatched, "set_error", 0)
	s.handleAuthError(ctx, account, s.buildAuthErrorMessage("Authentication failed (401): invalid or expired credentials", headers, responseBody))
	return true
}

func (s *RateLimitService) shouldHardDisable401(account *Account, headers http.Header, responseBody []byte) bool {
	if account == nil {
		return false
	}

	// OAuth/Setup token：优先走 temp unsched，让刷新服务恢复。
	if account.IsOAuth() {
		return false
	}

	// APIKey + 自定义 base_url：不自动升级为永久 error，避免中转站/代理误报造成大面积账号误伤。
	if account.Type == AccountTypeAPIKey {
		if trusted, _ := s.authUpstreamTrustedFor401(account); !trusted {
			return false
		}
	}

	// APIKey：仅在“高度确定”的情况下才升级为永久 error，避免线上偶发 401 误伤大量账号。
	//（例如链路/代理/上游边缘层偶发返回 401，但凭证实际可用。）

	contentType := ""
	wwwAuthenticate := ""
	if headers != nil {
		contentType = strings.ToLower(strings.TrimSpace(headers.Get("content-type")))
		wwwAuthenticate = strings.ToLower(strings.TrimSpace(headers.Get("www-authenticate")))
	}
	if strings.Contains(contentType, "text/html") {
		return false
	}
	bodyTrimmed := bytes.TrimSpace(responseBody)
	if bytes.HasPrefix(bodyTrimmed, []byte("<!DOCTYPE")) || bytes.HasPrefix(bodyTrimmed, []byte("<html")) {
		return false
	}

	// Prefer structured signal when available (e.g. OpenAI: error.code == invalid_api_key).
	code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.code").String()))
	if code == "invalid_api_key" {
		return true
	}

	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	if msg == "" {
		msg = strings.ToLower(strings.TrimSpace(string(bodyTrimmed)))
	}
	msg = sanitizeSensitiveText(msg)

	// “强特征”关键词（更保守）：只匹配明确指向 API Key 不正确/缺失的文案。
	needles := []string{
		"incorrect api key",
		"incorrect api key provided",
		"invalid api key",
		"invalid_api_key",
		"api key is invalid",
		"api key is not valid",
		"missing api key",
		"no api key provided",
		"invalid x-api-key",
	}
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}

	// www-authenticate 仅作为弱信号：单独出现时不升级为永久 error（避免误判）。
	if strings.Contains(wwwAuthenticate, "invalid") || strings.Contains(wwwAuthenticate, "expired") {
		// 若 message 同时包含 api key 字样，才认为是强信号。
		if strings.Contains(msg, "api key") || strings.Contains(msg, "x-api-key") || strings.Contains(msg, "invalid_api_key") {
			return true
		}
	}

	return false
}

// authUpstreamTrustedFor401 returns whether the upstream is "trusted" enough to hard-disable on 401.
//
// Rationale:
//   - For APIKey accounts using custom base_url (relay/proxy/self-hosted gateways), 401 is frequently
//     a transient or misclassified error from the middle layer.
//   - Hard-disabling (SetError) on a single 401 can mistakenly take down many accounts at once.
//   - For official upstream hosts, we keep the stricter behavior for clearly invalid credentials.
func (s *RateLimitService) authUpstreamTrustedFor401(account *Account) (trusted bool, upstreamHost string) {
	if account == nil {
		return true, ""
	}

	// Non-APIKey requests are sent to official upstream endpoints.
	if account.Type != AccountTypeAPIKey {
		switch account.Platform {
		case PlatformAnthropic:
			return true, "api.anthropic.com"
		case PlatformOpenAI:
			if account.Type == AccountTypeOAuth {
				return true, "chatgpt.com"
			}
			return true, "api.openai.com"
		default:
			return true, ""
		}
	}

	raw := strings.TrimSpace(account.GetCredential("base_url"))

	switch account.Platform {
	case PlatformAnthropic:
		const official = "api.anthropic.com"
		if raw == "" {
			return true, official
		}
		host := baseURLHost(raw)
		if host == "" {
			return false, ""
		}
		return host == official, host

	case PlatformOpenAI:
		const official = "api.openai.com"
		if raw == "" {
			return true, official
		}
		host := baseURLHost(raw)
		if host == "" {
			return false, ""
		}
		return host == official, host

	default:
		// Unknown platform: don't block hard-disable by default.
		return true, baseURLHost(raw)
	}
}

func baseURLHost(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	host = strings.TrimPrefix(host, "www.")
	return host
}

func (s *RateLimitService) buildAuthErrorMessage(base string, headers http.Header, responseBody []byte) string {
	msg := strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
	reqID := ""
	wwwAuth := ""
	if headers != nil {
		reqID = firstNonEmptyHeader(headers, "x-request-id", "request-id", "anthropic-request-id")
		wwwAuth = strings.TrimSpace(headers.Get("www-authenticate"))
	}

	msg = sanitizeSensitiveText(msg)
	reqID = sanitizeSensitiveText(reqID)
	wwwAuth = sanitizeSensitiveText(wwwAuth)

	// 控制长度，避免把上游原始内容塞太多进 DB。
	if msg != "" && len(msg) > 512 {
		msg = msg[:512] + "..."
	}
	if reqID != "" && len(reqID) > 128 {
		reqID = reqID[:128] + "..."
	}
	if wwwAuth != "" && len(wwwAuth) > 256 {
		wwwAuth = wwwAuth[:256] + "..."
	}

	out := base
	if msg != "" {
		out += "; upstream_message=" + msg
	}
	if reqID != "" {
		out += "; request_id=" + reqID
	}
	if wwwAuth != "" {
		out += "; www_authenticate=" + wwwAuth
	}
	return out
}

func (s *RateLimitService) setTempUnschedulable(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, cooldown time.Duration) {
	if account == nil || s.accountRepo == nil {
		return
	}
	if cooldown <= 0 {
		cooldown = s.auth401Cooldown(ctx)
	}

	now := time.Now()
	until := now.Add(cooldown)

	requestID := ""
	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
	upstreamMsg = sanitizeSensitiveText(upstreamMsg)
	if headers != nil {
		requestID = firstNonEmptyHeader(headers, "x-request-id", "request-id", "anthropic-request-id")
	}

	reasonMsg := "Upstream auth error, temporary cooldown"
	if upstreamMsg != "" {
		if len(upstreamMsg) > 512 {
			upstreamMsg = upstreamMsg[:512] + "..."
		}
		reasonMsg += "; upstream_message=" + upstreamMsg
	}
	if requestID != "" {
		if len(requestID) > 128 {
			requestID = requestID[:128] + "..."
		}
		reasonMsg += "; request_id=" + requestID
	}

	state := &TempUnschedState{
		UntilUnix:       until.Unix(),
		TriggeredAtUnix: now.Unix(),
		StatusCode:      statusCode,
		RuleIndex:       -1,
		ErrorMessage:    reasonMsg,
	}

	reason := ""
	if raw, err := json.Marshal(state); err == nil {
		reason = string(raw)
	}
	if reason == "" {
		reason = reasonMsg
	}

	if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
		applog.Printf("SetTempUnschedulable failed for account %d: %v", account.ID, err)
		return
	}
	applog.Printf("[TempUnsched] account_id=%d until=%v code=%d reason=%q", account.ID, until, statusCode, truncateForLog([]byte(reasonMsg), 256))

	if s.tempUnschedCache != nil {
		if err := s.tempUnschedCache.SetTempUnsched(ctx, account.ID, state); err != nil {
			applog.Printf("SetTempUnsched cache failed for account %d: %v", account.ID, err)
		}
	}
}

func (s *RateLimitService) logUpstreamAuthAbnormal(account *Account, statusCode int, headers http.Header, responseBody []byte, tempMatched bool, decision string, cooldown time.Duration) {
	if account == nil {
		return
	}

	upstreamTrusted, upstreamHost := s.authUpstreamTrustedFor401(account)

	var requestID, wwwAuthenticate, contentType string
	if headers != nil {
		requestID = firstNonEmptyHeader(headers, "x-request-id", "request-id", "anthropic-request-id")
		wwwAuthenticate = strings.TrimSpace(headers.Get("www-authenticate"))
		contentType = strings.TrimSpace(headers.Get("content-type"))
	}

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
	upstreamMsg = sanitizeSensitiveText(upstreamMsg)
	if upstreamMsg != "" && len(upstreamMsg) > 512 {
		upstreamMsg = upstreamMsg[:512] + "..."
	}

	bodySummary := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		bodySummary = truncateForLog(responseBody, s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes)
		bodySummary = sanitizeSensitiveText(bodySummary)
	}

	proxyAddr := ""
	if account.Proxy != nil {
		proxyAddr = account.Proxy.Protocol + "://" + account.Proxy.Host + ":" + strconv.Itoa(account.Proxy.Port)
	}

	cooldownStr := ""
	if cooldown > 0 {
		cooldownStr = cooldown.String()
	}

	applog.Printf(
		"[WARN] [UpstreamAuth] code=%d decision=%s cooldown=%q upstream_trusted=%t upstream_host=%q account_id=%d account_name=%q platform=%s type=%s proxy_id=%v proxy=%q request_id=%q www_authenticate=%q content_type=%q upstream_message=%q body=%q temp_unsched_matched=%t",
		statusCode,
		decision,
		cooldownStr,
		upstreamTrusted,
		upstreamHost,
		account.ID,
		account.Name,
		account.Platform,
		account.Type,
		account.ProxyID,
		proxyAddr,
		requestID,
		wwwAuthenticate,
		contentType,
		upstreamMsg,
		bodySummary,
		tempMatched,
	)
}

var (
	sensitiveQueryParamRegexRL = regexp.MustCompile(`(?i)([?&](?:key|api_key|client_secret|access_token|refresh_token)=)[^&"\\s]+`)
	openAIKeyRegexRL           = regexp.MustCompile(`\\bsk-(?:proj-)?[A-Za-z0-9_-]{10,}\\b`)
	anthropicKeyRegexRL        = regexp.MustCompile(`\\bsk-ant-[A-Za-z0-9_-]{10,}\\b`)
	googleAPIKeyRegexRL        = regexp.MustCompile(`\\bAIza[0-9A-Za-z\\-_]{10,}\\b`)
)

func sanitizeSensitiveText(in string) string {
	if in == "" {
		return in
	}
	out := in
	out = sensitiveQueryParamRegexRL.ReplaceAllString(out, `$1***`)
	out = openAIKeyRegexRL.ReplaceAllString(out, "sk-***")
	out = anthropicKeyRegexRL.ReplaceAllString(out, "sk-ant-***")
	out = googleAPIKeyRegexRL.ReplaceAllString(out, "AIza***")
	return out
}

func firstNonEmptyHeader(h http.Header, keys ...string) string {
	if h == nil {
		return ""
	}
	for _, k := range keys {
		if v := strings.TrimSpace(h.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

// PreCheckUsage proactively checks local quota before dispatching a request.
// Returns false when the account should be skipped.
func (s *RateLimitService) PreCheckUsage(ctx context.Context, account *Account, requestedModel string) (bool, error) {
	if account == nil || account.Platform != PlatformGemini {
		return true, nil
	}
	if s.usageRepo == nil || s.geminiQuotaService == nil {
		return true, nil
	}

	quota, ok := s.geminiQuotaService.QuotaForAccount(ctx, account)
	if !ok {
		return true, nil
	}

	now := time.Now()
	modelClass := geminiModelClassFromName(requestedModel)

	// 1) Daily quota precheck (RPD; resets at PST midnight)
	{
		var limit int64
		if quota.SharedRPD > 0 {
			limit = quota.SharedRPD
		} else {
			switch modelClass {
			case geminiModelFlash:
				limit = quota.FlashRPD
			default:
				limit = quota.ProRPD
			}
		}

		if limit > 0 {
			start := geminiDailyWindowStart(now)
			totals, ok := s.getGeminiUsageTotals(account.ID, start, now)
			if !ok {
				stats, err := s.usageRepo.GetModelStatsWithFilters(ctx, start, now, 0, 0, account.ID)
				if err != nil {
					return true, err
				}
				totals = geminiAggregateUsage(stats)
				s.setGeminiUsageTotals(account.ID, start, now, totals)
			}

			var used int64
			if quota.SharedRPD > 0 {
				used = totals.ProRequests + totals.FlashRequests
			} else {
				switch modelClass {
				case geminiModelFlash:
					used = totals.FlashRequests
				default:
					used = totals.ProRequests
				}
			}

			if used >= limit {
				resetAt := geminiDailyResetTime(now)
				// NOTE:
				// - This is a local precheck to reduce upstream 429s.
				// - Do NOT mark the account as rate-limited here; rate_limit_reset_at should reflect real upstream 429s.
				reason := buildGeminiUsageExceededReason("日", used, limit, resetAt)
				if err := s.SetTempUnschedulableWithReason(ctx, account.ID, resetAt, reason); err != nil {
					applog.Printf("[Gemini PreCheck] SetTempUnschedulable failed for account %d: %v", account.ID, err)
				}
				applog.Printf("[Gemini PreCheck] Account %d reached daily quota (%d/%d), skip until %v", account.ID, used, limit, resetAt)
				return false, nil
			}
		}
	}

	// 2) Minute quota precheck (RPM; fixed window current minute)
	{
		var limit int64
		if quota.SharedRPM > 0 {
			limit = quota.SharedRPM
		} else {
			switch modelClass {
			case geminiModelFlash:
				limit = quota.FlashRPM
			default:
				limit = quota.ProRPM
			}
		}

		if limit > 0 {
			start := now.Truncate(time.Minute)
			stats, err := s.usageRepo.GetModelStatsWithFilters(ctx, start, now, 0, 0, account.ID)
			if err != nil {
				return true, err
			}
			totals := geminiAggregateUsage(stats)

			var used int64
			if quota.SharedRPM > 0 {
				used = totals.ProRequests + totals.FlashRequests
			} else {
				switch modelClass {
				case geminiModelFlash:
					used = totals.FlashRequests
				default:
					used = totals.ProRequests
				}
			}

			if used >= limit {
				resetAt := start.Add(time.Minute)
				// Do not persist "rate limited" status from local precheck. See note above.
				reason := buildGeminiUsageExceededReason("分钟", used, limit, resetAt)
				if err := s.SetTempUnschedulableWithReason(ctx, account.ID, resetAt, reason); err != nil {
					applog.Printf("[Gemini PreCheck] SetTempUnschedulable failed for account %d: %v", account.ID, err)
				}
				applog.Printf("[Gemini PreCheck] Account %d reached minute quota (%d/%d), skip until %v", account.ID, used, limit, resetAt)
				return false, nil
			}
		}
	}

	return true, nil
}

func (s *RateLimitService) getGeminiUsageTotals(accountID int64, windowStart, now time.Time) (GeminiUsageTotals, bool) {
	s.usageCacheMu.RLock()
	defer s.usageCacheMu.RUnlock()

	if s.usageCache == nil {
		return GeminiUsageTotals{}, false
	}

	entry, ok := s.usageCache[accountID]
	if !ok || entry == nil {
		return GeminiUsageTotals{}, false
	}
	if !entry.windowStart.Equal(windowStart) {
		return GeminiUsageTotals{}, false
	}
	if now.Sub(entry.cachedAt) >= geminiPrecheckCacheTTL {
		return GeminiUsageTotals{}, false
	}
	return entry.totals, true
}

func (s *RateLimitService) setGeminiUsageTotals(accountID int64, windowStart, now time.Time, totals GeminiUsageTotals) {
	s.usageCacheMu.Lock()
	defer s.usageCacheMu.Unlock()
	if s.usageCache == nil {
		s.usageCache = make(map[int64]*geminiUsageCacheEntry)
	}
	s.usageCache[accountID] = &geminiUsageCacheEntry{
		windowStart: windowStart,
		cachedAt:    now,
		totals:      totals,
	}
}

// GeminiCooldown returns the fallback cooldown duration for Gemini 429s based on tier.
func (s *RateLimitService) GeminiCooldown(ctx context.Context, account *Account) time.Duration {
	if account == nil {
		return 5 * time.Minute
	}
	if s.geminiQuotaService == nil {
		return 5 * time.Minute
	}
	return s.geminiQuotaService.CooldownForAccount(ctx, account)
}

// handleAuthError 处理认证类错误(401/403)，停止账号调度
func (s *RateLimitService) handleAuthError(ctx context.Context, account *Account, errorMsg string) {
	if err := s.accountRepo.SetError(ctx, account.ID, errorMsg); err != nil {
		applog.Printf("SetError failed for account %d: %v", account.ID, err)
		return
	}
	applog.Printf("Account %d disabled due to auth error: %s", account.ID, errorMsg)
}

// handle429 处理429限流错误
// 解析响应头获取重置时间，标记账号为限流状态
func (s *RateLimitService) handle429(ctx context.Context, account *Account, headers http.Header, responseBody []byte) {
	now := time.Now()
	resetAt := now.Add(5 * time.Minute)
	if parsed := parseRateLimitReset(headers); parsed != nil && parsed.After(now) {
		resetAt = *parsed
	}

	// 标记限流状态
	if err := s.accountRepo.SetRateLimited(ctx, account.ID, resetAt); err != nil {
		applog.Printf("SetRateLimited failed for account %d: %v", account.ID, err)
		return
	}
	if account != nil {
		rateLimitedAt := now
		account.RateLimitedAt = &rateLimitedAt
		account.RateLimitResetAt = &resetAt
	}

	// 仅 Anthropic 的 unified-reset 能反推 5h 窗口。
	if headers != nil && strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-reset")) != "" {
		windowEnd := resetAt
		windowStart := resetAt.Add(-5 * time.Hour)
		if err := s.accountRepo.UpdateSessionWindow(ctx, account.ID, &windowStart, &windowEnd, "rejected"); err != nil {
			applog.Printf("UpdateSessionWindow failed for account %d: %v", account.ID, err)
		}
	}

	reason := buildRateLimitReason(headers, responseBody)
	applog.Printf(
		"[RateLimit] account_id=%d platform=%s type=%s reset_at=%v reason=%q",
		account.ID,
		account.Platform,
		account.Type,
		resetAt,
		truncateForLog([]byte(reason), 256),
	)
}

func (s *RateLimitService) handleQuotaExceeded429(ctx context.Context, account *Account, headers http.Header, responseBody []byte) bool {
	if account == nil || s.accountRepo == nil {
		return false
	}

	code, msg, matched := isQuotaExceededResponse(responseBody)
	if !matched {
		return false
	}

	until := parseRateLimitReset(headers)
	if until == nil {
		until = parseRateLimitResetFromBody(responseBody)
	}
	now := time.Now()
	if until == nil || !until.After(now) {
		fallback := now.Add(30 * time.Minute)
		until = &fallback
	}

	// Quota exceeded 同样按“临时不可调度”处理：记录 reset_at，让调度器在窗口内自动跳过，到期自动恢复。
	// 注意：不要把 schedulable=false 作为超限信号，否则需要人工介入才能恢复调度。
	if err := s.accountRepo.SetRateLimited(ctx, account.ID, *until); err != nil {
		applog.Printf("[QuotaExceeded] SetRateLimited failed for account %d: %v", account.ID, err)
		return false
	}
	if account != nil {
		rateLimitedAt := now
		account.RateLimitedAt = &rateLimitedAt
		account.RateLimitResetAt = until
	}

	reason := buildQuotaExceededReason(code, msg, headers, until)
	applog.Printf(
		"[QuotaExceeded] account_id=%d platform=%s type=%s until=%v reason=%q",
		account.ID,
		account.Platform,
		account.Type,
		*until,
		truncateForLog([]byte(reason), 256),
	)
	return true
}

func isQuotaExceededResponse(body []byte) (string, string, bool) {
	msg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	msgLower := strings.ToLower(msg)

	code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "error.code").String()))
	errType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "error.type").String()))
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "error.status").String()))
	topType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "type").String()))

	if isQuotaExceededCode(code) || isQuotaExceededCode(errType) || isQuotaExceededCode(status) || isQuotaExceededCode(topType) {
		return code, msg, true
	}
	if msgLower == "" {
		return code, msg, false
	}
	if containsQuotaKeyword(msgLower) {
		return code, msg, true
	}
	return code, msg, false
}

func isQuotaExceededCode(code string) bool {
	switch code {
	case "insufficient_quota",
		"quota_exceeded",
		"billing_hard_limit_reached",
		"billing_hard_limit",
		"hard_limit_reached",
		"account_limit_reached":
		return true
	default:
		return false
	}
}

func containsQuotaKeyword(msgLower string) bool {
	if msgLower == "" {
		return false
	}
	keywords := []string{
		"insufficient quota",
		"quota exceeded",
		"exceeded your current quota",
		"exceeded your quota",
		"exceeded the quota",
		"hard limit",
		"billing hard limit",
		"usage limit",
		"usage cap",
		"plan limit",
		"out of quota",
		"out of credits",
		"out of credit",
		"insufficient credits",
		"account limit",
		"quota has been exceeded",
		"quota exceeded for",
		"exceeded the allowed",
		"额度",
		"配额",
		"超限",
		"限额",
		"余额不足",
	}
	for _, keyword := range keywords {
		if strings.Contains(msgLower, keyword) {
			return true
		}
	}
	return false
}

func parseRateLimitReset(headers http.Header) *time.Time {
	if headers == nil {
		return nil
	}
	keys := []string{
		"anthropic-ratelimit-unified-reset",
		"retry-after",
		"x-ratelimit-reset-requests",
		"x-ratelimit-reset-tokens",
		"x-ratelimit-reset",
		"ratelimit-reset",
	}
	for _, key := range keys {
		value := strings.TrimSpace(headers.Get(key))
		if value == "" {
			continue
		}
		if resetAt := parseResetValue(value); resetAt != nil {
			return resetAt
		}
	}
	return nil
}

func parseRateLimitResetFromBody(body []byte) *time.Time {
	if len(body) == 0 {
		return nil
	}

	if ts := gjson.GetBytes(body, "error.resets_at"); ts.Exists() {
		if unix := ts.Int(); unix > 0 {
			resetAt := time.Unix(unix, 0)
			return &resetAt
		}
	}
	if ts := gjson.GetBytes(body, "resets_at"); ts.Exists() {
		if unix := ts.Int(); unix > 0 {
			resetAt := time.Unix(unix, 0)
			return &resetAt
		}
	}
	if secs := gjson.GetBytes(body, "error.resets_in_seconds"); secs.Exists() {
		if v := secs.Int(); v > 0 {
			resetAt := time.Now().Add(time.Duration(v) * time.Second)
			return &resetAt
		}
	}
	if secs := gjson.GetBytes(body, "resets_in_seconds"); secs.Exists() {
		if v := secs.Int(); v > 0 {
			resetAt := time.Now().Add(time.Duration(v) * time.Second)
			return &resetAt
		}
	}

	return nil
}

func parseResetValue(raw string) *time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
		now := time.Now()
		switch {
		case ts > 1_000_000_000_000:
			resetAt := time.Unix(0, ts*int64(time.Millisecond))
			return &resetAt
		case ts > 1_000_000_000:
			resetAt := time.Unix(ts, 0)
			return &resetAt
		default:
			resetAt := now.Add(time.Duration(ts) * time.Second)
			return &resetAt
		}
	}
	if d, err := time.ParseDuration(value); err == nil {
		resetAt := time.Now().Add(d)
		return &resetAt
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t
	}
	if t, err := http.ParseTime(value); err == nil {
		return &t
	}
	return nil
}

func buildQuotaExceededReason(code, msg string, headers http.Header, resetAt *time.Time) string {
	reqID := ""
	if headers != nil {
		reqID = firstNonEmptyHeader(headers, "x-request-id", "request-id", "anthropic-request-id")
	}
	msg = sanitizeSensitiveText(strings.TrimSpace(msg))
	reqID = sanitizeSensitiveText(strings.TrimSpace(reqID))
	code = sanitizeSensitiveText(strings.TrimSpace(code))

	if msg != "" && len(msg) > 512 {
		msg = msg[:512] + "..."
	}
	if reqID != "" && len(reqID) > 128 {
		reqID = reqID[:128] + "..."
	}
	if code != "" && len(code) > 64 {
		code = code[:64] + "..."
	}

	reason := "Upstream quota exceeded (429)"
	if code != "" {
		reason += "; upstream_code=" + code
	}
	if resetAt == nil {
		resetAt = parseRateLimitReset(headers)
	}
	if resetAt != nil {
		reason += "; reset_at=" + resetAt.Format(time.RFC3339)
	}
	if msg != "" {
		reason += "; upstream_message=" + msg
	}
	if reqID != "" {
		reason += "; request_id=" + reqID
	}
	return reason
}

func buildRateLimitReason(headers http.Header, responseBody []byte) string {
	reqID := ""
	if headers != nil {
		reqID = firstNonEmptyHeader(headers, "x-request-id", "request-id", "anthropic-request-id")
	}
	msg := strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
	msg = sanitizeSensitiveText(msg)
	reqID = sanitizeSensitiveText(strings.TrimSpace(reqID))

	if msg != "" && len(msg) > 512 {
		msg = msg[:512] + "..."
	}
	if reqID != "" && len(reqID) > 128 {
		reqID = reqID[:128] + "..."
	}

	reason := "上游 429 限流，已进入限流冷却"
	if resetAt := parseRateLimitReset(headers); resetAt != nil {
		reason += "，预计 " + resetAt.Format(time.RFC3339) + " 恢复"
	}
	if msg != "" {
		reason += "; upstream_message=" + msg
	}
	if reqID != "" {
		reason += "; request_id=" + reqID
	}
	return reason
}

func buildGeminiUsageExceededReason(windowLabel string, used, limit int64, resetAt time.Time) string {
	label := strings.TrimSpace(windowLabel)
	if label == "" {
		label = "使用"
	}
	return "Gemini " + label + "额度已超限：" +
		strconv.FormatInt(used, 10) + "/" + strconv.FormatInt(limit, 10) +
		"，预计 " + resetAt.Format(time.RFC3339) + " 恢复"
}

// handle529 处理529过载错误
// 根据配置设置过载冷却时间
func (s *RateLimitService) handle529(ctx context.Context, account *Account) {
	cooldownMinutes := s.cfg.RateLimit.OverloadCooldownMinutes
	if cooldownMinutes <= 0 {
		cooldownMinutes = 10 // 默认10分钟
	}

	until := time.Now().Add(time.Duration(cooldownMinutes) * time.Minute)
	if err := s.accountRepo.SetOverloaded(ctx, account.ID, until); err != nil {
		applog.Printf("SetOverloaded failed for account %d: %v", account.ID, err)
		return
	}

	applog.Printf("Account %d overloaded until %v", account.ID, until)
}

// UpdateSessionWindow 从成功响应更新5h窗口状态
func (s *RateLimitService) UpdateSessionWindow(ctx context.Context, account *Account, headers http.Header) {
	status := headers.Get("anthropic-ratelimit-unified-5h-status")
	if status == "" {
		return
	}

	// 检查是否需要初始化时间窗口
	// 对于 Setup Token 账号，首次成功请求时需要预测时间窗口
	var windowStart, windowEnd *time.Time
	needInitWindow := account.SessionWindowEnd == nil || time.Now().After(*account.SessionWindowEnd)

	if needInitWindow && (status == "allowed" || status == "allowed_warning") {
		// 预测时间窗口：从当前时间的整点开始，+5小时为结束
		// 例如：现在是 14:30，窗口为 14:00 ~ 19:00
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		end := start.Add(5 * time.Hour)
		windowStart = &start
		windowEnd = &end
		applog.Printf("Account %d: initializing 5h window from %v to %v (status: %s)", account.ID, start, end, status)
	}

	if err := s.accountRepo.UpdateSessionWindow(ctx, account.ID, windowStart, windowEnd, status); err != nil {
		applog.Printf("UpdateSessionWindow failed for account %d: %v", account.ID, err)
	}

	// 如果状态为allowed且之前有限流，说明窗口已重置，清除限流状态
	if status == "allowed" && account.IsRateLimited() {
		if err := s.accountRepo.ClearRateLimit(ctx, account.ID); err != nil {
			applog.Printf("ClearRateLimit failed for account %d: %v", account.ID, err)
		}
	}
}

// ClearRateLimit 清除账号的限流状态
func (s *RateLimitService) ClearRateLimit(ctx context.Context, accountID int64) error {
	return s.accountRepo.ClearRateLimit(ctx, accountID)
}

func (s *RateLimitService) ClearTempUnschedulable(ctx context.Context, accountID int64) error {
	if err := s.accountRepo.ClearTempUnschedulable(ctx, accountID); err != nil {
		return err
	}
	if s.tempUnschedCache != nil {
		if err := s.tempUnschedCache.DeleteTempUnsched(ctx, accountID); err != nil {
			applog.Printf("DeleteTempUnsched failed for account %d: %v", accountID, err)
		}
	}
	return nil
}

// SetTempUnschedulableWithReason marks an account as temporarily unschedulable until the given time.
// This is intended for system-triggered cooldowns (e.g. quota windows) where we want a human-readable reason.
// NOTE: accountRepo.SetTempUnschedulable is "extend-only" (won't shorten an existing until).
func (s *RateLimitService) SetTempUnschedulableWithReason(ctx context.Context, accountID int64, until time.Time, reason string) error {
	if accountID <= 0 {
		return nil
	}
	if s.accountRepo == nil {
		return nil
	}
	if until.IsZero() || !until.After(time.Now()) {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Temporary unschedulable"
	}
	reason = sanitizeSensitiveText(reason)

	if err := s.accountRepo.SetTempUnschedulable(ctx, accountID, until, reason); err != nil {
		return err
	}

	if s.tempUnschedCache != nil {
		now := time.Now()
		state := &TempUnschedState{
			UntilUnix:       until.Unix(),
			TriggeredAtUnix: now.Unix(),
			StatusCode:      0,
			RuleIndex:       -2,
			ErrorMessage:    reason,
		}
		if err := s.tempUnschedCache.SetTempUnsched(ctx, accountID, state); err != nil {
			applog.Printf("SetTempUnsched cache failed for account %d: %v", accountID, err)
		}
	}

	applog.Printf("[TempUnsched] account_id=%d until=%v reason=%q", accountID, until, truncateForLog([]byte(reason), 256))
	return nil
}

func normalizeUnschedulableReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "调度已取消"
	}
	reason = sanitizeSensitiveText(reason)
	if len(reason) > tempUnschedMessageMaxBytes {
		reason = reason[:tempUnschedMessageMaxBytes] + "..."
	}
	return reason
}

func setUnschedulableWithReason(ctx context.Context, repo AccountRepository, account *Account, reason string) error {
	if account == nil || repo == nil {
		return nil
	}
	if account.ID <= 0 {
		return nil
	}
	normalized := normalizeUnschedulableReason(reason)
	if err := repo.SetUnschedulableWithReason(ctx, account.ID, normalized); err != nil {
		return err
	}
	account.Schedulable = false
	account.TempUnschedulableUntil = nil
	account.TempUnschedulableReason = normalized
	return nil
}

func setUnschedulableWithReasonByID(ctx context.Context, repo AccountRepository, accountID int64, reason string) error {
	if repo == nil || accountID <= 0 {
		return nil
	}
	normalized := normalizeUnschedulableReason(reason)
	return repo.SetUnschedulableWithReason(ctx, accountID, normalized)
}

func (s *RateLimitService) GetTempUnschedStatus(ctx context.Context, accountID int64) (*TempUnschedState, error) {
	now := time.Now().Unix()
	if s.tempUnschedCache != nil {
		state, err := s.tempUnschedCache.GetTempUnsched(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if state != nil && state.UntilUnix > now {
			return state, nil
		}
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account.TempUnschedulableUntil == nil {
		return nil, nil
	}
	if account.TempUnschedulableUntil.Unix() <= now {
		return nil, nil
	}

	state := &TempUnschedState{
		UntilUnix: account.TempUnschedulableUntil.Unix(),
	}

	if account.TempUnschedulableReason != "" {
		var parsed TempUnschedState
		if err := json.Unmarshal([]byte(account.TempUnschedulableReason), &parsed); err == nil {
			if parsed.UntilUnix == 0 {
				parsed.UntilUnix = state.UntilUnix
			}
			state = &parsed
		} else {
			state.ErrorMessage = account.TempUnschedulableReason
		}
	}

	if s.tempUnschedCache != nil {
		if err := s.tempUnschedCache.SetTempUnsched(ctx, accountID, state); err != nil {
			applog.Printf("SetTempUnsched failed for account %d: %v", accountID, err)
		}
	}

	return state, nil
}

func (s *RateLimitService) HandleTempUnschedulable(ctx context.Context, account *Account, statusCode int, responseBody []byte) bool {
	if account == nil {
		return false
	}
	if statusCode == 429 {
		return false
	}
	if !account.ShouldHandleErrorCode(statusCode) {
		return false
	}
	return s.tryTempUnschedulable(ctx, account, statusCode, responseBody)
}

const tempUnschedBodyMaxBytes = 64 << 10
const tempUnschedMessageMaxBytes = 2048

func (s *RateLimitService) tryTempUnschedulable(ctx context.Context, account *Account, statusCode int, responseBody []byte) bool {
	if account == nil {
		return false
	}
	if !account.IsTempUnschedulableEnabled() {
		return false
	}
	rules := account.GetTempUnschedulableRules()
	if len(rules) == 0 {
		return false
	}
	if statusCode <= 0 || len(responseBody) == 0 {
		return false
	}

	body := responseBody
	if len(body) > tempUnschedBodyMaxBytes {
		body = body[:tempUnschedBodyMaxBytes]
	}
	bodyLower := strings.ToLower(string(body))

	for idx, rule := range rules {
		if rule.ErrorCode != statusCode || len(rule.Keywords) == 0 {
			continue
		}
		matchedKeyword := matchTempUnschedKeyword(bodyLower, rule.Keywords)
		if matchedKeyword == "" {
			continue
		}

		if s.triggerTempUnschedulable(ctx, account, rule, idx, statusCode, matchedKeyword, responseBody) {
			return true
		}
	}

	return false
}

func matchTempUnschedKeyword(bodyLower string, keywords []string) string {
	if bodyLower == "" {
		return ""
	}
	for _, keyword := range keywords {
		k := strings.TrimSpace(keyword)
		if k == "" {
			continue
		}
		if strings.Contains(bodyLower, strings.ToLower(k)) {
			return k
		}
	}
	return ""
}

func (s *RateLimitService) triggerTempUnschedulable(ctx context.Context, account *Account, rule TempUnschedulableRule, ruleIndex int, statusCode int, matchedKeyword string, responseBody []byte) bool {
	if account == nil {
		return false
	}
	if rule.DurationMinutes <= 0 {
		return false
	}

	now := time.Now()
	until := now.Add(time.Duration(rule.DurationMinutes) * time.Minute)

	desc := strings.TrimSpace(rule.Description)
	msg := sanitizeSensitiveText(truncateTempUnschedMessage(responseBody, tempUnschedMessageMaxBytes))
	if desc != "" {
		if matchedKeyword != "" {
			msg = desc + "; matched_keyword=" + matchedKeyword + "; upstream=" + msg
		} else {
			msg = desc + "; upstream=" + msg
		}
		// 再次截断，避免 description + upstream 拼接后过长写入 DB/Redis
		if len(msg) > tempUnschedMessageMaxBytes {
			msg = msg[:tempUnschedMessageMaxBytes] + "..."
		}
	}

	state := &TempUnschedState{
		UntilUnix:       until.Unix(),
		TriggeredAtUnix: now.Unix(),
		StatusCode:      statusCode,
		MatchedKeyword:  matchedKeyword,
		RuleIndex:       ruleIndex,
		ErrorMessage:    msg,
	}

	reason := ""
	if raw, err := json.Marshal(state); err == nil {
		reason = string(raw)
	}
	if reason == "" {
		reason = strings.TrimSpace(state.ErrorMessage)
	}

	if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
		applog.Printf("SetTempUnschedulable failed for account %d: %v", account.ID, err)
		return false
	}

	if s.tempUnschedCache != nil {
		if err := s.tempUnschedCache.SetTempUnsched(ctx, account.ID, state); err != nil {
			applog.Printf("SetTempUnsched cache failed for account %d: %v", account.ID, err)
		}
	}

	descLog := desc
	if len(descLog) > 128 {
		descLog = descLog[:128] + "..."
	}
	applog.Printf("[TempUnsched] account_id=%d until=%v code=%d rule_index=%d keyword=%q desc=%q", account.ID, until, statusCode, ruleIndex, matchedKeyword, descLog)
	return true
}

func truncateTempUnschedMessage(body []byte, maxBytes int) string {
	if maxBytes <= 0 || len(body) == 0 {
		return ""
	}
	if len(body) > maxBytes {
		body = body[:maxBytes]
	}
	return strings.TrimSpace(string(body))
}
