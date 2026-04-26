package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
)

func TestRequireRole_AllowsAdminForAdminRoute(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(userContextKey, &admindomain.User{ID: "u1", Role: admindomain.RoleAdmin})

	h := RequireRole(admindomain.RoleAdmin)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_RejectsDeveloperForAdminRoute(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(userContextKey, &admindomain.User{ID: "u1", Role: admindomain.RoleDeveloper})

	h := RequireRole(admindomain.RoleAdmin)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"insufficient permissions"}`, rec.Body.String())
}

func TestRequireRole_ReturnsForbiddenJSON(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(userContextKey, &admindomain.User{ID: "u1", Role: admindomain.RoleDeveloper})

	h := RequireRole(admindomain.RoleAdmin)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"insufficient permissions"}`, rec.Body.String())
}

func TestRBACByRouteGroup_RejectsDeveloperOnAdminPath(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(userContextKey, &admindomain.User{ID: "u1", Role: admindomain.RoleDeveloper})

	h := RBACByRouteGroup()(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"insufficient permissions"}`, rec.Body.String())
}

func TestRBACByRouteGroup_AllowsDeveloperOnAlertRead(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/alert-rules", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(userContextKey, &admindomain.User{ID: "u1", Role: admindomain.RoleDeveloper})

	h := RBACByRouteGroup()(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRBACByRouteGroup_RejectsDeveloperOnAlertConfig(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observability/alert-rules", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(userContextKey, &admindomain.User{ID: "u1", Role: admindomain.RoleDeveloper})

	h := RBACByRouteGroup()(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"insufficient permissions"}`, rec.Body.String())
}

func TestRequiredRolesForRoute_UnknownRouteRequiresAdmin(t *testing.T) {
	requiredRoles := RequiredRolesForRoute("/api/v1/unknown/resource", http.MethodGet)
	require.Equal(t, []admindomain.Role{admindomain.RoleAdmin}, requiredRoles)
}

func TestRBACByRouteGroup_RejectsDeveloperOnUnknownRoute(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown/resource", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(userContextKey, &admindomain.User{ID: "u1", Role: admindomain.RoleDeveloper})

	h := RBACByRouteGroup()(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"insufficient permissions"}`, rec.Body.String())
}
