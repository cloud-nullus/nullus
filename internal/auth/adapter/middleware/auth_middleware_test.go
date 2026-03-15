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

func TestAuthMiddleware_WithSession(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "session-user")
	req.Header.Set("X-User-Email", "session@nullus.io")
	req.Header.Set("X-User-Name", "session-name")
	req.Header.Set("X-User-Role", "developer")
	req.Header.Set("X-User-OrgID", "org-1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := AuthMiddleware()(func(c echo.Context) error {
		user, ok := c.Get(userContextKey).(*admindomain.User)
		require.True(t, ok)
		require.NotNil(t, user)
		assert.Equal(t, "session-user", user.ID)
		assert.Equal(t, "session@nullus.io", user.Email)
		assert.Equal(t, admindomain.RoleDeveloper, user.Role)
		assert.Equal(t, "org-1", user.OrgID)
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_NoSession(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := AuthMiddleware()(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"authentication required"}`, rec.Body.String())
}
