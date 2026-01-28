package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/DueGin/FluxCode/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func userEmailFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxkey.UserEmail).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// ensureRequestIDForLog returns a stable request id for logging and best-effort stores it on gin.Request.Context.
func ensureRequestIDForLog(ctx context.Context, c *gin.Context, rawRequestID string) string {
	if ctx != nil {
		if v, ok := ctx.Value(ctxkey.RequestID).(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if c != nil && c.Request != nil {
		if v, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}

	requestID := normalizeRequestIDWithFallback(rawRequestID, "")
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, requestID))
	}
	return requestID
}

func extractUpstreamErrorForLog(body []byte, fallbackStatus int) (code, message string) {
	code = strings.TrimSpace(gjson.GetBytes(body, "error.status").String())
	if code == "" {
		code = strings.TrimSpace(gjson.GetBytes(body, "error.code").String())
	}
	if code == "" {
		code = strings.TrimSpace(gjson.GetBytes(body, "error.type").String())
	}
	if code == "" && fallbackStatus > 0 {
		code = strconv.Itoa(fallbackStatus)
	}

	message = strings.TrimSpace(gjson.GetBytes(body, "error.message").String())
	if message == "" {
		message = strings.TrimSpace(gjson.GetBytes(body, "message").String())
	}
	if message == "" && len(body) > 0 {
		message = truncateForLog(body, 512)
	}
	return code, message
}
