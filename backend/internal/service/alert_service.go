package service

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
)

const (
	defaultAlertCooldownMinutes = 5
	alertTypeNoAccounts         = "no_available_accounts"
)

type NoAvailableAccountsAlert struct {
	Message  string
	Path     string
	Method   string
	Platform string
	UserID   *int64
	APIKeyID *int64
	GroupID  *int64
}

type AlertService struct {
	emailService *EmailService
	settingRepo  SettingRepository
	siteName     string
	lastSent     map[string]time.Time
	mu           sync.Mutex

	cooldownMu      sync.Mutex
	cooldownCache   time.Duration
	cooldownCacheAt time.Time
}

func NewAlertService(emailService *EmailService, settingRepo SettingRepository) *AlertService {
	siteName := strings.TrimSpace(os.Getenv("WEB_TITLE"))
	if siteName == "" {
		siteName = "FluxCode"
	}
	return &AlertService{
		emailService: emailService,
		settingRepo:  settingRepo,
		siteName:     siteName,
		lastSent:     make(map[string]time.Time),
	}
}

func (s *AlertService) NotifyNoAvailableAccounts(ctx context.Context, detail NoAvailableAccountsAlert) {
	if s == nil || s.emailService == nil {
		return
	}
	recipients := s.getRecipients(ctx)
	if len(recipients) == 0 {
		return
	}
	if !s.shouldSend(ctx, alertTypeNoAccounts) {
		return
	}

	subject := fmt.Sprintf("[%s] 号池异常告警", s.siteName)
	body := s.buildNoAvailableAccountsBody(detail)
	s.sendEmailAsync(recipients, subject, body)
}

func (s *AlertService) shouldSend(ctx context.Context, key string) bool {
	cooldown := s.getCooldown(ctx)
	if cooldown <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if last, ok := s.lastSent[key]; ok && now.Sub(last) < cooldown {
		return false
	}
	s.lastSent[key] = now
	return true
}

func (s *AlertService) sendEmailAsync(recipients []string, subject, body string) {
	recipients = append([]string(nil), recipients...)
	go func() {
		alertCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for _, to := range recipients {
			if err := s.emailService.SendEmail(alertCtx, to, subject, body); err != nil {
				applog.Printf("[Alert] Failed to send alert email to %s: %v", to, err)
			}
		}
	}()
}

func (s *AlertService) getRecipients(ctx context.Context) []string {
	if s != nil && s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeyAlertEmails); err == nil {
			if recipients := parseAlertRecipientsFromSetting(raw); len(recipients) > 0 {
				return recipients
			}
		}
	}
	return nil
}

func (s *AlertService) getCooldown(ctx context.Context) time.Duration {
	const cacheTTL = 30 * time.Second

	s.cooldownMu.Lock()
	if !s.cooldownCacheAt.IsZero() && time.Since(s.cooldownCacheAt) < cacheTTL {
		v := s.cooldownCache
		s.cooldownMu.Unlock()
		return v
	}
	s.cooldownMu.Unlock()

	minutes := defaultAlertCooldownMinutes
	if s != nil && s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeyAlertCooldownMinutes); err == nil {
			raw = strings.TrimSpace(raw)
			if raw != "" {
				if v, err := strconv.Atoi(raw); err == nil {
					minutes = v
				}
			}
		}
	}
	if minutes <= 0 {
		minutes = 0
	}
	d := time.Duration(minutes) * time.Minute

	s.cooldownMu.Lock()
	s.cooldownCache = d
	s.cooldownCacheAt = time.Now()
	s.cooldownMu.Unlock()
	return d
}

func (s *AlertService) buildNoAvailableAccountsBody(detail NoAvailableAccountsAlert) string {
	var builder strings.Builder
	builder.WriteString("<div>")
	builder.WriteString("<p>号池异常：没有可用账号。</p>")
	builder.WriteString(fmt.Sprintf("<p>时间：%s</p>", time.Now().Format("2006-01-02 15:04:05")))
	if detail.Method != "" || detail.Path != "" {
		builder.WriteString(fmt.Sprintf("<p>请求：%s %s</p>", detail.Method, detail.Path))
	}
	if detail.Platform != "" {
		builder.WriteString(fmt.Sprintf("<p>平台：%s</p>", html.EscapeString(detail.Platform)))
	}
	if detail.UserID != nil {
		builder.WriteString(fmt.Sprintf("<p>UserID：%d</p>", *detail.UserID))
	}
	if detail.GroupID != nil {
		builder.WriteString(fmt.Sprintf("<p>GroupID：%d</p>", *detail.GroupID))
	}
	if detail.APIKeyID != nil {
		builder.WriteString(fmt.Sprintf("<p>APIKeyID：%d</p>", *detail.APIKeyID))
	}
	if detail.Message != "" {
		builder.WriteString(fmt.Sprintf("<p>详情：%s</p>", html.EscapeString(detail.Message)))
	}
	builder.WriteString("</div>")
	return builder.String()
}

func parseAlertRecipientsFromSetting(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return normalizeAlertRecipients(items)
	}
	// Backward/compat: allow comma/space/newline separated values.
	return parseAlertRecipients(raw)
}

func normalizeAlertRecipients(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		addr := strings.TrimSpace(item)
		if addr == "" {
			continue
		}
		key := strings.ToLower(addr)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, addr)
	}
	return out
}

func parseAlertRecipients(value string) []string {
	if value == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	recipients := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		addr := strings.TrimSpace(field)
		if addr == "" {
			continue
		}
		key := strings.ToLower(addr)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		recipients = append(recipients, addr)
	}
	return recipients
}
