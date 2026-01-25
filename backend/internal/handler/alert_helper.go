package handler

import (
	"strings"

	"github.com/DueGin/FluxCode/internal/server/middleware"
	"github.com/DueGin/FluxCode/internal/service"
	"github.com/gin-gonic/gin"
)

func maybeSendNoAvailableAccountsAlert(alertService *service.AlertService, c *gin.Context, message string) {
	if alertService == nil || c == nil {
		return
	}
	if !isNoAvailableAccountsMessage(message) {
		return
	}

	detail := service.NoAvailableAccountsAlert{
		Message: message,
		Path:    c.Request.URL.Path,
		Method:  c.Request.Method,
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID := subject.UserID
		detail.UserID = &userID
	}
	if apiKey, ok := middleware.GetAPIKeyFromContext(c); ok && apiKey != nil {
		apiKeyID := apiKey.ID
		detail.APIKeyID = &apiKeyID
		if apiKey.GroupID != nil {
			groupID := *apiKey.GroupID
			detail.GroupID = &groupID
		}
		if apiKey.Group != nil {
			detail.Platform = apiKey.Group.Platform
		}
	}
	if detail.Platform == "" {
		if platform, ok := middleware.GetForcePlatformFromContext(c); ok {
			detail.Platform = platform
		}
	}

	alertService.NotifyNoAvailableAccounts(c.Request.Context(), detail)
}

func isNoAvailableAccountsMessage(message string) bool {
	return strings.Contains(strings.ToLower(message), "no available accounts")
}
