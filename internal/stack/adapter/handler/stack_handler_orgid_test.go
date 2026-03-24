package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestResolveOrgID_PrefersJWTClaims(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks?orgId=query-org", nil)
	req.Header.Set("X-Org-ID", "header-org")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set("user_claims", map[string]any{"org_id": "claims-org"})

	require.Equal(t, "claims-org", resolveOrgID(c))
}

func TestResolveOrgID_UsesCurrentUserOrgID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set("current_user", &admindomain.User{OrgID: "org-from-user"})

	require.Equal(t, "org-from-user", resolveOrgID(c))
}

func TestResolveOrgID_DefaultsToDefaultOrg(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.Equal(t, "11111111-1111-1111-1111-111111111111", resolveOrgID(c))
}
