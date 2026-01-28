package service

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DueGin/FluxCode/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type captureHandler struct {
	mu       sync.Mutex
	messages []string
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, r.Message)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *captureHandler) Messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.messages))
	copy(out, h.messages)
	return out
}

func TestGatewayService_HandleErrorResponse_LogsUpstreamErrorFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &captureHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Key("ctx_user_email"), "alice@example.com"))

	upstreamBody := `{"type":"error","error":{"type":"rate_limit_error","message":"Too many requests","code":"rate_limit"}}`
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"X-Request-Id": []string{"req_upstream_123"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}

	svc := &GatewayService{}
	_, err := svc.handleErrorResponse(c.Request.Context(), resp, c, &Account{ID: 7, Platform: PlatformAnthropic, Type: AccountTypeOAuth})
	require.Error(t, err)

	msgs := h.Messages()
	require.NotEmpty(t, msgs)

	found := false
	for _, msg := range msgs {
		if strings.Contains(msg, "request_id=req_upstream_123") &&
			strings.Contains(msg, "user_email=alice@example.com") &&
			strings.Contains(msg, "upstream_error_code=rate_limit") &&
			strings.Contains(msg, `upstream_error_message="Too many requests"`) {
			found = true
			break
		}
	}
	require.True(t, found, "expected upstream error log to include required fields, got: %v", msgs)
}

func TestOpenAIGatewayService_HandleErrorResponse_LogsUpstreamErrorFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &captureHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Key("ctx_user_email"), "alice@example.com"))

	upstreamBody := `{"error":{"message":"Too many requests","type":"rate_limit_error","code":"rate_limit"}}`
	headers := http.Header{"X-Request-Id": []string{"req_openai_456"}}

	svc := &OpenAIGatewayService{}
	_, err := svc.handleErrorResponse(c.Request.Context(), http.StatusTooManyRequests, headers, []byte(upstreamBody), c, &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeOAuth})
	require.Error(t, err)

	msgs := h.Messages()
	require.NotEmpty(t, msgs)

	found := false
	for _, msg := range msgs {
		if strings.Contains(msg, "request_id=req_openai_456") &&
			strings.Contains(msg, "user_email=alice@example.com") &&
			strings.Contains(msg, "upstream_error_code=rate_limit") &&
			strings.Contains(msg, `upstream_error_message="Too many requests"`) {
			found = true
			break
		}
	}
	require.True(t, found, "expected upstream error log to include required fields, got: %v", msgs)
}
