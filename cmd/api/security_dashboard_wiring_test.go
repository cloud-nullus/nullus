package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	authmw "github.com/cloud-nullus/draft/internal/auth/adapter/middleware"
	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	obsuc "github.com/cloud-nullus/draft/internal/observability/usecase"
)

// chainOf 는 anchor 에서 시작하는 배선 체인(빈 줄까지)이다.
func chainOf(t *testing.T, src, anchor string) string {
	t.Helper()
	i := strings.Index(src, anchor)
	if i < 0 {
		t.Fatalf("main.go 에서 %q 를 찾지 못했다", anchor)
	}
	chain := src[i:]
	if end := strings.Index(chain, "\n\n"); end >= 0 {
		chain = chain[:end]
	}
	return chain
}

// 의존성 스캔 · 시크릿 탐지 결과는 두 곳에 배선돼야 한다 — 동기화가 남기고 파이프라인 핸들러가 내준다. 한쪽이
// 빠져도 컴파일과 단위 테스트는 통과한다. 보안 대시보드는 관측 그룹에 붙고, 설치 OSS 축에는 역할 검사가 붙는다.
func TestSecurityDashboardIsWired(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)

	for _, want := range []string{
		"cicdrepo.NewPostgresDependencyScanResultRepository(pool)",
		"cicdrepo.NewPostgresSecretScanResultRepository(pool)",
		"cicdrepo.NewPostgresPipelineSecurityReader(pool)",
		"obsbridge.NewSecurityScope(pgStackRepo, clusterRepo)",
		"obsbridge.NewStackSecuritySource(stackImageScanUC)",
		`securityStackGuards = []echo.MiddlewareFunc{authmw.RequireRole("admin", "devops")}`,
		"securityHandler.RegisterRoutes(observability, securityStackGuards...)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("main.go 에 %q 가 없다", want)
		}
	}
	for _, anchor := range []string{"runSyncUC := cicduc.NewSyncPipelineRuns(", "pipelineHandler := cicdhandler.NewPipelineHandler("} {
		chain := chainOf(t, src, anchor)
		for _, with := range []string{"WithDependencyScans(pgDependencyScanRepo)", "WithSecretScans(pgSecretScanRepo)"} {
			if !strings.Contains(chain, with) {
				t.Errorf("%q 체인에 %s 가 없다", anchor, with)
			}
		}
	}
}

type stubAppSecurity struct{}

func (stubAppSecurity) Execute(context.Context, obsuc.AppSecurityQuery) (*obsuc.AppSecurityView, error) {
	return &obsuc.AppSecurityView{}, nil
}

type stubStackSecurity struct{}

func (stubStackSecurity) Execute(context.Context, obsuc.StackSecurityQuery) (*obsuc.StackSecurityView, error) {
	return &obsuc.StackSecurityView{}, nil
}

// main.go 가 쓰는 실제 역할 검사(authmw.RequireRole)로 — 개발자는 설치 OSS 축에서 403, 앱 축은 모두 200.
func TestSecurityDashboard_RoleGuards(t *testing.T) {
	for _, tc := range []struct {
		role       admindomain.Role
		appCode    int
		stacksCode int
	}{
		{admindomain.RoleDeveloper, http.StatusOK, http.StatusForbidden},
		{admindomain.RoleDevOps, http.StatusOK, http.StatusOK},
		{admindomain.RoleAdmin, http.StatusOK, http.StatusOK},
	} {
		e := echo.New()
		g := e.Group("/api/v1/observability", func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("current_user", &admindomain.User{ID: "u1", OrgID: "org-1", Role: tc.role})
				return next(c)
			}
		})
		obshandler.NewSecurityHandler(stubAppSecurity{}, stubStackSecurity{}).
			RegisterRoutes(g, authmw.RequireRole("admin", "devops"))

		for path, want := range map[string]int{
			"/api/v1/observability/security/apps":   tc.appCode,
			"/api/v1/observability/security/stacks": tc.stacksCode,
		} {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != want {
				t.Errorf("%s %s = %d, want %d", tc.role, path, rec.Code, want)
			}
		}
	}
}
