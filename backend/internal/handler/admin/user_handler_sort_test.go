package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DueGin/FluxCode/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubUserAdminService struct {
	listUsersFn func(ctx context.Context, page, pageSize int, filters service.UserListFilters) ([]service.User, int64, error)
}

func (s stubUserAdminService) ListUsers(ctx context.Context, page, pageSize int, filters service.UserListFilters) ([]service.User, int64, error) {
	if s.listUsersFn == nil {
		return nil, 0, nil
	}
	return s.listUsersFn(ctx, page, pageSize, filters)
}

func (s stubUserAdminService) GetUser(context.Context, int64) (*service.User, error) {
	panic("not implemented")
}
func (s stubUserAdminService) CreateUser(context.Context, *service.CreateUserInput) (*service.User, error) {
	panic("not implemented")
}
func (s stubUserAdminService) UpdateUser(context.Context, int64, *service.UpdateUserInput) (*service.User, error) {
	panic("not implemented")
}
func (s stubUserAdminService) DeleteUser(context.Context, int64) error {
	panic("not implemented")
}
func (s stubUserAdminService) UpdateUserBalance(context.Context, int64, float64, string, string) (*service.User, error) {
	panic("not implemented")
}
func (s stubUserAdminService) GetUserAPIKeys(context.Context, int64, int, int) ([]service.APIKey, int64, error) {
	panic("not implemented")
}
func (s stubUserAdminService) GetUserUsageStats(context.Context, int64, string) (any, error) {
	panic("not implemented")
}

func TestUserHandler_List_SortByInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewUserHandler(stubUserAdminService{
		listUsersFn: func(context.Context, int, int, service.UserListFilters) ([]service.User, int64, error) {
			return []service.User{}, 0, nil
		},
	})
	r := gin.New()
	r.GET("/api/v1/admin/users", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?sort_by=__bad__", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_List_SortOrderInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewUserHandler(stubUserAdminService{
		listUsersFn: func(context.Context, int, int, service.UserListFilters) ([]service.User, int64, error) {
			return []service.User{}, 0, nil
		},
	})
	r := gin.New()
	r.GET("/api/v1/admin/users", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?sort_by=balance&sort_order=__bad__", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_List_SortOrderDefaultsToAsc(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotFilters service.UserListFilters
	h := NewUserHandler(stubUserAdminService{
		listUsersFn: func(_ context.Context, _ int, _ int, filters service.UserListFilters) ([]service.User, int64, error) {
			gotFilters = filters
			return []service.User{}, 0, nil
		},
	})
	r := gin.New()
	r.GET("/api/v1/admin/users", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?sort_by=balance", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "balance", gotFilters.SortBy)
	require.Equal(t, "asc", gotFilters.SortOrder)
}
