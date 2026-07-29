package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
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

// 인증이 꺼진 개발 모드에는 principal 도 헤더도 없다. 이때 폴백하는 조직은
// 시드 마이그레이션이 실제로 만드는 조직이어야 한다 — 존재하지 않는 UUID 로
// 폴백하면 스택 생성이 stacks_org_id_fkey FK 위반으로 실패한다.
func TestResolveOrgID_DefaultsToSeededOrg(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.Equal(t, "11111111-1111-1111-1111-111111111111", resolveOrgID(c))
}

func TestResolveOrgID_DefaultOrgOverridableByEnv(t *testing.T) {
	t.Setenv("NULLUS_DEFAULT_ORG_ID", "33333333-3333-3333-3333-333333333333")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.Equal(t, "33333333-3333-3333-3333-333333333333", resolveOrgID(c))
}

// 헤더는 기본값보다 우선한다.
func TestResolveOrgID_PrefersHeaderOverDefault(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	req.Header.Set("X-Org-ID", "header-org")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.Equal(t, "header-org", resolveOrgID(c))
}
