package handler

import (
	"github.com/DueGin/FluxCode/internal/config"
	"github.com/DueGin/FluxCode/internal/handler/dto"
	"github.com/DueGin/FluxCode/internal/pkg/response"
	middleware2 "github.com/DueGin/FluxCode/internal/server/middleware"
	"github.com/DueGin/FluxCode/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	cfg         *config.Config
	authService *service.AuthService
	userService *service.UserService
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(cfg *config.Config, authService *service.AuthService, userService *service.UserService) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		authService: authService,
		userService: userService,
	}
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	VerifyCode     string `json:"verify_code"`
	TurnstileToken string `json:"turnstile_token"`
}

// SendVerifyCodeRequest 发送验证码请求
type SendVerifyCodeRequest struct {
	Email          string `json:"email" binding:"required,email"`
	TurnstileToken string `json:"turnstile_token"`
}

// SendVerifyCodeResponse 发送验证码响应
type SendVerifyCodeResponse struct {
	Message        string `json:"message"`
	Countdown      int    `json:"countdown"` // 倒计时秒数
	TaskID         string `json:"task_id"`
	QueueMessageID string `json:"queue_message_id"`
	SendAck        bool   `json:"send_ack"`
	ConsumeAck     bool   `json:"consume_ack"`
	DeliveryStatus string `json:"delivery_status"`
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	TurnstileToken string `json:"turnstile_token"`
}

// AuthResponse 认证响应格式（匹配前端期望）
type AuthResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	User        *dto.User `json:"user"`
}

// Register handles user registration
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Turnstile 验证（当提供了邮箱验证码时跳过，因为发送验证码时已验证过）
	if req.VerifyCode == "" {
		if err := h.authService.VerifyTurnstile(c.Request.Context(), req.TurnstileToken, c.ClientIP()); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	token, user, err := h.authService.RegisterWithVerification(c.Request.Context(), req.Email, req.Password, req.VerifyCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		User:        dto.UserFromService(user),
	})
}

// SendVerifyCode 发送邮箱验证码
// POST /api/v1/auth/send-verify-code
func (h *AuthHandler) SendVerifyCode(c *gin.Context) {
	var req SendVerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Turnstile 验证
	if err := h.authService.VerifyTurnstile(c.Request.Context(), req.TurnstileToken, c.ClientIP()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result, err := h.authService.SendVerifyCodeAsync(c.Request.Context(), req.Email)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, SendVerifyCodeResponse{
		Message:        "Verification code sent successfully",
		Countdown:      result.Countdown,
		TaskID:         result.TaskID,
		QueueMessageID: result.QueueMessageID,
		SendAck:        result.SendAck,
		ConsumeAck:     result.ConsumeAck,
		DeliveryStatus: result.DeliveryStatus,
	})
}

// GetVerifyCodeStatus 查询验证码发送任务状态
// GET /api/v1/auth/verify-code-status?task_id=xxx
func (h *AuthHandler) GetVerifyCodeStatus(c *gin.Context) {
	taskID := c.Query("task_id")
	if taskID == "" {
		response.BadRequest(c, "task_id is required")
		return
	}

	status, err := h.authService.GetVerifyCodeTaskStatus(c.Request.Context(), taskID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if status == nil {
		response.NotFound(c, "task not found")
		return
	}

	response.Success(c, status)
}

// Login handles user login
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Turnstile 验证
	if err := h.authService.VerifyTurnstile(c.Request.Context(), req.TurnstileToken, c.ClientIP()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	token, user, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		User:        dto.UserFromService(user),
	})
}

// GetCurrentUser handles getting current authenticated user
// GET /api/v1/auth/me
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	type UserResponse struct {
		*dto.User
		RunMode string `json:"run_mode"`
	}

	runMode := config.RunModeStandard
	if h.cfg != nil {
		runMode = h.cfg.RunMode
	}

	response.Success(c, UserResponse{User: dto.UserFromService(user), RunMode: runMode})
}
