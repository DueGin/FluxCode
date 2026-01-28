//go:build unit

package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
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

func TestLogger_LogsRequestIDAndUserEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &captureHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := gin.New()
	r.Use(Logger())
	r.GET("/t", func(c *gin.Context) {
		// 模拟认证层写入 email
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Key("ctx_user_email"), "alice@example.com"))
		// 模拟 handler/service 设置 request id
		c.Header("x-request-id", "req_123")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	msgs := h.Messages()
	require.NotEmpty(t, msgs)

	found := false
	for _, msg := range msgs {
		if strings.Contains(msg, "request_id=req_123") && strings.Contains(msg, "user_email=alice@example.com") {
			found = true
			break
		}
	}
	require.True(t, found, "expected request log to include request_id and user_email, got: %v", msgs)
}

func TestLogger_GeneratesRequestIDWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &captureHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := gin.New()
	r.Use(Logger())
	r.GET("/t", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	msgs := h.Messages()
	require.NotEmpty(t, msgs)

	re := regexp.MustCompile(`request_id=([^\s]+)`)
	found := false
	for _, msg := range msgs {
		m := re.FindStringSubmatch(msg)
		if len(m) == 2 && strings.TrimSpace(m[1]) != "" {
			found = true
			break
		}
	}
	require.True(t, found, "expected request log to include non-empty request_id, got: %v", msgs)
}
