//go:build unit

package routes

import (
	"testing"

	"github.com/DueGin/FluxCode/internal/handler"
	adminhandler "github.com/DueGin/FluxCode/internal/handler/admin"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAccountRoutes_DoesNotRegisterCRSSync(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	v1 := r.Group("/api/v1")
	admin := v1.Group("/admin")

	h := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Account: (*adminhandler.AccountHandler)(nil),
			OAuth:   (*adminhandler.OAuthHandler)(nil),
		},
	}

	registerAccountRoutes(admin, h)

	for _, route := range r.Routes() {
		if route.Method == "POST" && route.Path == "/api/v1/admin/accounts/sync/crs" {
			require.Fail(t, "CRS sync route should be removed", "found route: %s %s", route.Method, route.Path)
		}
	}
}
