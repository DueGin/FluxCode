package handler

import (
	"github.com/DueGin/FluxCode/internal/handler/dto"
	"github.com/DueGin/FluxCode/internal/pkg/response"
	"github.com/DueGin/FluxCode/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 公开设置处理器（无需认证）
type SettingHandler struct {
	settingService *service.SettingService
	version        string
}

// NewSettingHandler 创建公开设置处理器
func NewSettingHandler(settingService *service.SettingService, version string) *SettingHandler {
	return &SettingHandler{
		settingService: settingService,
		version:        version,
	}
}

// GetPublicSettings 获取公开设置
// GET /api/v1/settings/public
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	afterSaleContact := make([]dto.KVItem, 0, len(settings.AfterSaleContact))
	for _, item := range settings.AfterSaleContact {
		afterSaleContact = append(afterSaleContact, dto.KVItem{K: item.K, V: item.V})
	}

	response.Success(c, dto.PublicSettings{
		RegistrationEnabled: settings.RegistrationEnabled,
		EmailVerifyEnabled:  settings.EmailVerifyEnabled,
		TurnstileEnabled:    settings.TurnstileEnabled,
		TurnstileSiteKey:    settings.TurnstileSiteKey,
		SiteName:            settings.SiteName,
		SiteLogo:            settings.SiteLogo,
		SiteSubtitle:        settings.SiteSubtitle,
		APIBaseURL:          settings.APIBaseURL,
		ContactInfo:         settings.ContactInfo,
		AfterSaleContact:    afterSaleContact,
		DocURL:              settings.DocURL,
		Version:             h.version,
	})
}
