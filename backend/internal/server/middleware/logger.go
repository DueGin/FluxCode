package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/DueGin/FluxCode/internal/pkg/ctxkey"
	applog "github.com/DueGin/FluxCode/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 结束时间
		endTime := time.Now()

		// 执行时间
		latency := endTime.Sub(startTime)

		// 请求方法
		method := c.Request.Method

		// 请求路径
		path := c.Request.URL.Path

		// 状态码
		statusCode := c.Writer.Status()

		// 客户端IP
		clientIP := c.ClientIP()

		requestID := strings.TrimSpace(c.Writer.Header().Get("x-request-id"))
		if requestID == "" && c.Request != nil {
			if v, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok {
				requestID = strings.TrimSpace(v)
			}
		}
		if requestID == "" {
			requestID = uuid.NewString()
			if c.Request != nil {
				c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, requestID))
			}
		}

		userEmail := ""
		if c.Request != nil {
			if v, ok := c.Request.Context().Value(ctxkey.UserEmail).(string); ok {
				userEmail = strings.TrimSpace(v)
			}
		}

		// 日志格式: 状态码 | 延迟 | IP | 方法 路径 | request_id/user_email（时间由全局 logger 输出）
		applog.Printf("%3d | %13v | %15s | %-7s %s | request_id=%s user_email=%s",
			statusCode,
			latency,
			clientIP,
			method,
			path,
			requestID,
			userEmail,
		)

		// 如果有错误，额外记录错误信息
		if len(c.Errors) > 0 {
			applog.Printf("Errors: %v", c.Errors.String())
		}
	}
}
