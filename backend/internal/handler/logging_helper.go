package handler

import (
	"fmt"
	"strings"

	"github.com/DueGin/FluxCode/internal/service"
	"github.com/gin-gonic/gin"
)

func formatUserNameForLog(user *service.User, fallbackUserID int64) string {
	if user != nil {
		if name := strings.TrimSpace(user.Username); name != "" {
			return name
		}
		if email := strings.TrimSpace(user.Email); email != "" {
			return email
		}
		if user.ID > 0 {
			return fmt.Sprintf("user_%d", user.ID)
		}
	}
	if fallbackUserID > 0 {
		return fmt.Sprintf("user_%d", fallbackUserID)
	}
	return "unknown"
}

func requestIDSuffix(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if requestID := strings.TrimSpace(c.GetHeader("X-Request-Id")); requestID != "" {
		return " request_id=" + requestID
	}
	if requestID := strings.TrimSpace(c.GetHeader("Request-Id")); requestID != "" {
		return " request_id=" + requestID
	}
	if requestID := strings.TrimSpace(c.Writer.Header().Get("x-request-id")); requestID != "" {
		return " request_id=" + requestID
	}
	return ""
}
