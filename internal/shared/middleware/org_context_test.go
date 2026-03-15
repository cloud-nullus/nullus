package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgContextMiddleware_ExtractsOrgIDFromCurrentUser(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("current_user", &admindomain.User{ID: "u1", OrgID: "org-123"})

	hit := false
	h := OrgContextMiddleware()(func(c echo.Context) error {
		hit = true
		orgID, ok := OrgIDFromContext(c.Request().Context())
		require.True(t, ok)
		assert.Equal(t, "org-123", orgID)
		return c.NoContent(http.StatusNoContent)
	})

	err := h(c)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestOrgContextMiddleware_ExtractsOrgIDFromUserMap(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", map[string]any{"id": "u2", "org_id": "org-map"})

	h := OrgContextMiddleware()(func(c echo.Context) error {
		orgID, ok := OrgIDFromContext(c.Request().Context())
		require.True(t, ok)
		assert.Equal(t, "org-map", orgID)
		return c.NoContent(http.StatusNoContent)
	})

	require.NoError(t, h(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
