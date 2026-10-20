package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

var secGenerated = time.Date(2026, 10, 20, 9, 12, 0, 0, time.UTC)

type fakeAppSecurity struct {
	view *usecase.AppSecurityView
	err  error
	got  usecase.AppSecurityQuery
}

func (f *fakeAppSecurity) Execute(_ context.Context, q usecase.AppSecurityQuery) (*usecase.AppSecurityView, error) {
	f.got = q
	return f.view, f.err
}

type fakeStackSecurity struct {
	view  *usecase.StackSecurityView
	err   error
	got   usecase.StackSecurityQuery
	calls int
}

func (f *fakeStackSecurity) Execute(_ context.Context, q usecase.StackSecurityQuery) (*usecase.StackSecurityView, error) {
	f.got = q
	f.calls++
	return f.view, f.err
}

// roleGuard 는 배선하는 쪽이 넘기는 역할 검사(authmw.RequireRole("admin", "devops"))와 같은 일을 한다 —
// 관측 모듈은 인증 모듈을 알지 못한다.
func roleGuard(allowed ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get("role").(string)
			for _, r := range allowed {
				if r == role {
					return next(c)
				}
			}
			return c.JSON(http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
		}
	}
}

func serveSecurity(t *testing.T, h *SecurityHandler, role, org, path string) (int, map[string]any) {
	t.Helper()
	e := echo.New()
	g := e.Group("/api/v1/observability", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("role", role)
			if org != "" {
				c.Set("current_user", map[string]any{"org_id": org})
			}
			return next(c)
		}
	})
	h.RegisterRoutes(g, roleGuard("admin", "devops"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return rec.Code, body
}

func appView() *usecase.AppSecurityView {
	at := secGenerated.Add(-2 * time.Hour)
	db := secGenerated.Add(-34 * 24 * time.Hour)
	two, vulns := 2, 4
	app := domain.AppSecurityRecord{
		PipelineID: "pip_app_order-api", Name: "order-api", StackID: "stk_prod", ClusterID: "cl_prod",
		StagesKnown: true,
		HasStage: map[domain.SecurityStage]bool{domain.StageSecretScan: true, domain.StageDependencyScan: true,
			domain.StageSAST: true, domain.StageImageScan: true},
		LatestRun: &domain.AppRun{DeploymentID: "dep_ci_42", Version: "#42", Status: "failed", StartedAt: at},
		Records: map[domain.SecurityStage]*domain.ScanRecord{
			domain.StageSecretScan: {Gate: "block", DeploymentID: "dep_ci_42", Version: "#42", At: at, Findings: &two,
				Rules: []domain.SecretRule{{RuleID: "github-pat", Count: 2}}, Link: "https://ci/gitleaks"},
			domain.StageDependencyScan: {Gate: "block", DeploymentID: "dep_ci_42", Version: "#42", At: at, Targets: &two,
				Counts: &shareddomain.SeverityCounts{Critical: 3, High: 5, Medium: 12}, DBUpdatedAt: &db, Link: "https://ci/dep"},
			domain.StageSAST: {Gate: "warn", DeploymentID: "dep_ci_41", Version: "#41", At: at, Vulnerabilities: &vulns,
				FailedConditions: []string{"new_violations"}, Link: "https://sonar"},
			domain.StageImageScan: {Gate: "error", DeploymentID: "dep_ci_41", Version: "#41", At: at},
		},
	}
	legacy := domain.AppSecurityRecord{PipelineID: "pip_legacy", Name: "legacy"}
	ps := []domain.AppSecurityPosture{domain.EvaluateApp(app, secGenerated), domain.EvaluateApp(legacy, secGenerated)}
	sum, cov := domain.SummarizeApps(ps)
	return &usecase.AppSecurityView{
		GeneratedAt: secGenerated, Summary: sum, Coverage: cov,
		Groups: domain.GroupAppsByStack(ps, map[string]domain.SecurityStackRef{"stk_prod": {ID: "stk_prod", Name: "stack-prod", ClusterID: "cl_prod"}},
			map[string]string{"cl_prod": "prod-a"}),
		Apps: ps,
	}
}

// 응답 모양은 설계 §5.2 그대로다(필드 이름은 화면과의 계약). 모르는 숫자는 빠진다 — 0 으로 채우지 않는다.
func TestSecurityHandler_GetApps_Contract(t *testing.T) {
	apps := &fakeAppSecurity{view: appView()}
	h := NewSecurityHandler(apps, &fakeStackSecurity{})
	code, body := serveSecurity(t, h, "developer", "org-1",
		"/api/v1/observability/security/apps?cluster_id=cl_prod&stack_id=none&q=order&problem_only=true")

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, usecase.AppSecurityQuery{OrgID: "org-1", ClusterID: "cl_prod", StackID: "none", Query: "order", ProblemOnly: true}, apps.got)
	assert.Equal(t, "2026-10-20T09:12:00Z", body["generated_at"])
	assert.Equal(t, false, body["truncated"])
	assert.Equal(t, map[string]any{"apps": float64(2), "with_security_stage": float64(1), "blocked": float64(1),
		"secrets": float64(1), "critical": float64(1), "undetermined": float64(1)}, body["summary"])
	assert.Equal(t, map[string]any{"secret_scan": float64(1), "dependency_scan": float64(1), "sast": float64(1), "image_scan": float64(1)}, body["coverage"])

	groups := body["groups"].([]any)
	require.Len(t, groups, 2)
	assert.Equal(t, map[string]any{"stack_id": "stk_prod", "stack_name": "stack-prod", "cluster_id": "cl_prod", "cluster_name": "prod-a",
		"apps": float64(1), "blocked": float64(1), "secrets": float64(1), "critical": float64(1), "undetermined": float64(1), "problems": float64(1)}, groups[0])
	assert.Equal(t, "", groups[1].(map[string]any)["stack_id"], "스택 없음 묶음은 빈 값이다")
	assert.Equal(t, "", groups[1].(map[string]any)["stack_name"])

	items := body["apps"].([]any)
	require.Len(t, items, 2)
	app := items[0].(map[string]any)
	assert.Equal(t, "pip_app_order-api", app["pipeline_id"])
	assert.Equal(t, "order-api", app["name"])
	assert.Equal(t, "stk_prod", app["stack_id"])
	assert.Equal(t, float64(1), app["rank"])
	assert.Equal(t, map[string]any{"deployment_id": "dep_ci_42", "version": "#42", "status": "failed", "started_at": "2026-10-20T07:12:00Z"}, app["latest_run"])

	cells := app["cells"].(map[string]any)
	assert.Equal(t, map[string]any{"state": "block", "deployment_id": "dep_ci_42", "version": "#42", "at": "2026-10-20T07:12:00Z",
		"stale_run": false, "findings": float64(2), "rules": []any{map[string]any{"rule_id": "github-pat", "count": float64(2)}},
		"link": "https://ci/gitleaks"}, cells["secret_scan"])
	assert.Equal(t, map[string]any{"state": "block", "deployment_id": "dep_ci_42", "version": "#42", "at": "2026-10-20T07:12:00Z",
		"stale_run": false, "targets": float64(2),
		"counts":   map[string]any{"critical": float64(3), "high": float64(5), "medium": float64(12), "low": float64(0), "unknown": float64(0)},
		"db_stale": true, "db_updated_at": "2026-09-16T09:12:00Z", "link": "https://ci/dep"}, cells["dependency_scan"],
		"DB 나이를 보이도록 db_updated_at 을 함께 싣는다")
	assert.Equal(t, map[string]any{"state": "warn", "deployment_id": "dep_ci_41", "version": "#41", "at": "2026-10-20T07:12:00Z",
		"stale_run": true, "vulnerabilities": float64(4), "failed_conditions": []any{"new_violations"}, "link": "https://sonar"}, cells["sast"],
		"모르는 핫스팟 수는 빠진다")
	assert.Equal(t, map[string]any{"state": "error", "deployment_id": "dep_ci_41", "version": "#41", "at": "2026-10-20T07:12:00Z",
		"stale_run": true}, cells["image_scan"], "리포트를 못 읽은 셀은 건수 · DB 칸이 없다")

	legacy := items[1].(map[string]any)
	assert.Nil(t, legacy["latest_run"], "실행이 없으면 null")
	assert.Equal(t, float64(6), legacy["rank"])
	for _, stage := range []string{"secret_scan", "dependency_scan", "sast", "image_scan"} {
		assert.Equal(t, map[string]any{"state": "not_applied"}, legacy["cells"].(map[string]any)[stage], "미적용 셀은 state 만 있다")
	}
}

// 빈 조직도 배열이다(null 이 아니다). problem_only 기본값은 false 다.
func TestSecurityHandler_GetApps_EmptyAndDefaults(t *testing.T) {
	apps := &fakeAppSecurity{view: &usecase.AppSecurityView{GeneratedAt: secGenerated, Truncated: true}}
	code, body := serveSecurity(t, NewSecurityHandler(apps, &fakeStackSecurity{}), "developer", "org-1", "/api/v1/observability/security/apps")
	require.Equal(t, http.StatusOK, code)
	assert.False(t, apps.got.ProblemOnly)
	assert.Equal(t, []any{}, body["groups"])
	assert.Equal(t, []any{}, body["apps"])
	assert.Equal(t, true, body["truncated"])
}

// 조직은 로그인한 사용자의 것이다. 없으면(인증을 끈 개발 모드) 기본 조직이다.
func TestSecurityHandler_OrgFromPrincipal(t *testing.T) {
	apps := &fakeAppSecurity{view: &usecase.AppSecurityView{}}
	stacks := &fakeStackSecurity{view: &usecase.StackSecurityView{}}
	h := NewSecurityHandler(apps, stacks)
	serveSecurity(t, h, "admin", "", "/api/v1/observability/security/apps")
	assert.Equal(t, shareddomain.DefaultOrgID(), apps.got.OrgID)
	serveSecurity(t, h, "admin", "org-9", "/api/v1/observability/security/stacks?cluster_id=cl_1&stack_id=stk_1")
	assert.Equal(t, usecase.StackSecurityQuery{OrgID: "org-9", ClusterID: "cl_1", StackID: "stk_1"}, stacks.got)
}

// 설치 OSS 축은 admin · devops 만이다 — 개발자는 403 이고 유스케이스까지 닿지 않는다(설계 결정 11). 앱 축은
// 모든 역할이다.
func TestSecurityHandler_StackRouteIsGuarded(t *testing.T) {
	stacks := &fakeStackSecurity{view: &usecase.StackSecurityView{GeneratedAt: secGenerated}}
	h := NewSecurityHandler(&fakeAppSecurity{view: &usecase.AppSecurityView{}}, stacks)

	code, _ := serveSecurity(t, h, "developer", "org-1", "/api/v1/observability/security/stacks")
	assert.Equal(t, http.StatusForbidden, code)
	assert.Zero(t, stacks.calls)

	for _, role := range []string{"admin", "devops"} {
		code, _ = serveSecurity(t, h, role, "org-1", "/api/v1/observability/security/stacks")
		assert.Equal(t, http.StatusOK, code, role)
	}
	code, _ = serveSecurity(t, h, "developer", "org-1", "/api/v1/observability/security/apps")
	assert.Equal(t, http.StatusOK, code, "앱 축은 개발자도 본다")
}

// 스택 행 — 스캔하지 않은 스택의 숫자는 0 이 아니라 null 이다.
func TestSecurityHandler_GetStacks_Contract(t *testing.T) {
	db := secGenerated.Add(-2 * 24 * time.Hour)
	scanned := secGenerated.Add(-3 * time.Hour)
	crit := shareddomain.SeverityCounts{Critical: 1, High: 3}
	inputs := []domain.StackImageInput{
		{StackID: "stk_prod", StackName: "stack-prod", ClusterID: "cl_prod", State: domain.StackScanScanned, ScannedAt: &scanned,
			Images: []domain.StackImage{{Image: "gitlab-toolbox", Exposed: true, Scanned: true, Counts: &crit, Fixable: &crit, DBUpdatedAt: &db}}},
		{StackID: "stk_dev", StackName: "stack-dev", ClusterID: "cl_dev", State: domain.StackScanNotScanned, Reason: "scanner_not_installed"},
	}
	ps := []domain.StackSecurityPosture{domain.EvaluateStack(inputs[0], secGenerated), domain.EvaluateStack(inputs[1], secGenerated)}
	ps[0].ClusterName = "prod-a"
	stacks := &fakeStackSecurity{view: &usecase.StackSecurityView{GeneratedAt: secGenerated, Summary: domain.SummarizeStacks(ps), Stacks: ps}}

	code, body := serveSecurity(t, NewSecurityHandler(&fakeAppSecurity{}, stacks), "devops", "org-1", "/api/v1/observability/security/stacks")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "2026-10-20T09:12:00Z", body["generated_at"])
	assert.Equal(t, map[string]any{"stacks": float64(2), "scanned_stacks": float64(1), "not_scanned_stacks": float64(1),
		"action_images": float64(1), "grade_a_images": float64(1), "grade_b_images": float64(0)}, body["summary"])

	rows := body["stacks"].([]any)
	require.Len(t, rows, 2)
	assert.Equal(t, map[string]any{
		"stack_id": "stk_prod", "stack_name": "stack-prod", "cluster_id": "cl_prod", "cluster_name": "prod-a",
		"state": "scanned", "reason": "", "airgap": false,
		"images": float64(1), "failed_images": float64(0), "shared_images": float64(0),
		"grade_a_images": float64(1), "grade_b_images": float64(0), "grade_c_images": float64(0),
		"fixable":     map[string]any{"critical": float64(1), "high": float64(3), "medium": float64(0), "low": float64(0), "unknown": float64(0)},
		"worst_image": map[string]any{"image": "gitlab-toolbox", "critical": float64(1), "high": float64(3), "exposed": true},
		"scanned_at":  "2026-10-20T06:12:00Z", "db_updated_at": "2026-10-18T09:12:00Z", "db_stale": false,
	}, rows[0])
	dev := rows[1].(map[string]any)
	assert.Equal(t, "not_scanned", dev["state"])
	assert.Equal(t, "scanner_not_installed", dev["reason"])
	for _, k := range []string{"images", "failed_images", "shared_images", "grade_a_images", "grade_b_images", "grade_c_images",
		"fixable", "worst_image", "scanned_at", "db_updated_at", "db_stale"} {
		v, ok := dev[k]
		assert.True(t, ok, k)
		assert.Nil(t, v, "%s 는 0 이 아니라 null 이다", k)
	}
}

func TestSecurityHandler_Errors(t *testing.T) {
	h := NewSecurityHandler(&fakeAppSecurity{err: errors.New("db down")}, &fakeStackSecurity{err: errors.New("db down")})
	code, body := serveSecurity(t, h, "admin", "org-1", "/api/v1/observability/security/apps")
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "SECURITY_APPS_FAILED", body["error"].(map[string]any)["code"])
	code, body = serveSecurity(t, h, "admin", "org-1", "/api/v1/observability/security/stacks")
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "SECURITY_STACKS_FAILED", body["error"].(map[string]any)["code"])
}

func TestUTCPtr(t *testing.T) {
	assert.Nil(t, utcPtr(nil))
	zero := time.Time{}
	assert.Nil(t, utcPtr(&zero))
}
