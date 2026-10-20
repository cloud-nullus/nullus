package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

type scanResultStores struct {
	images  *repository.MemoryImageScanResultRepository
	sast    *repository.MemorySASTResultRepository
	deps    *repository.MemoryDependencyScanResultRepository
	secrets *repository.MemorySecretScanResultRepository
}

func newScanResultStores() scanResultStores {
	return scanResultStores{
		images:  repository.NewMemoryImageScanResultRepository(),
		sast:    repository.NewMemorySASTResultRepository(),
		deps:    repository.NewMemoryDependencyScanResultRepository(),
		secrets: repository.NewMemorySecretScanResultRepository(),
	}
}

// principalOrg 는 인증 미들웨어가 하는 일 — 로그인한 사용자를 컨텍스트에 둔다 — 을 흉내 낸다.
func principalOrg(orgID string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if orgID != "" {
				c.Set("current_user", map[string]any{"org_id": orgID})
			}
			return next(c)
		}
	}
}

type vulnBundles struct{ raw []byte }

func (v vulnBundles) For(context.Context, string) (*port.SCMBundle, error) {
	return &port.SCMBundle{CIArtifacts: rawArtifacts(v)}, nil
}

type rawArtifacts struct{ raw []byte }

func (a rawArtifacts) ReadArtifact(context.Context, port.CIArtifactRef) ([]byte, bool, error) {
	return a.raw, true, nil
}

func newScanResultsEcho(t *testing.T, callerOrg string, stores scanResultStores) *echo.Echo {
	t.Helper()
	pipelineRepo := newMockPipelineRepository(&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1", StackID: "stk-1"})
	deploymentRepo := &mockDeploymentRepository{}
	createUC := usecase.NewCreatePipeline(pipelineRepo, newMockPipelineTemplateRepository())
	listUC := usecase.NewListPipelines(pipelineRepo)
	deployUC := usecase.NewDeployPipeline(pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, &noopManifestApplier{})
	raw, err := os.ReadFile("../../../shared/domain/testdata/trivy-report-node16.json")
	require.NoError(t, err)
	h := cicdhandler.NewPipelineHandler(createUC, listUC, deployUC, pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, kube.NewStepTracker(), nil).
		WithImageScans(stores.images).
		WithSASTResults(stores.sast).
		WithDependencyScans(stores.deps).
		WithSecretScans(stores.secrets).
		WithScanVulnerabilities(usecase.NewScanVulnerabilities(pipelineRepo, stores.images, vulnBundles{raw: raw}))
	e := echo.New()
	h.RegisterRoutes(e.Group("/api/v1/cicd", principalOrg(callerOrg)))
	return e
}

func getScanResults(t *testing.T, e *echo.Echo, path string, headers map[string]string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// 의존성 스캔 결과는 이미지 스캔과 같은 모양으로 내준다. 건수 · 검사 대상 수는 모르면 빠진다 — 0 으로 채우면
// "취약점 0건 · 대상 없음" 으로 읽힌다. DB 가 낡았는지는 서버가 판정한다.
func TestPipelineHandler_ListDependencyScans(t *testing.T) {
	stores := newScanResultStores()
	now := time.Now().UTC().Truncate(time.Second)
	fresh := now.Add(-time.Hour)
	two := 2
	require.NoError(t, stores.deps.Upsert(context.Background(), &domain.DependencyScanResult{
		ID: "depscan_dep_2", PipelineID: "pip-1", DeploymentID: "dep_2", Scanner: "trivy", ScannerVersion: "0.58.1",
		DBUpdatedAt: &fresh, Counts: &domain.SeverityCounts{Critical: 3, High: 5}, Targets: &two,
		GateResult: domain.GateResultBlock, ReportURI: "https://ci/report", ScannedAt: now,
		ReportRef: &domain.ScanReportRef{JobName: "orders", BuildNumber: 2, Path: port.DependencyScanReportFile},
	}))
	require.NoError(t, stores.deps.Upsert(context.Background(), &domain.DependencyScanResult{
		ID: "depscan_dep_1", PipelineID: "pip-1", DeploymentID: "dep_1", Scanner: "trivy",
		GateResult: domain.GateResultError, ScannedAt: now.Add(-time.Hour),
	}))
	e := newScanResultsEcho(t, "org-1", stores)

	code, body := getScanResults(t, e, "/api/v1/cicd/pipelines/pip-1/dependency-scans", nil)
	require.Equal(t, http.StatusOK, code, string(body))

	var resp struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Equal(t, 2, resp.Total)
	first := resp.Items[0]
	assert.Equal(t, "depscan_dep_2", first["id"])
	assert.Equal(t, "pip-1", first["pipeline_id"])
	assert.Equal(t, "dep_2", first["deployment_id"])
	assert.Equal(t, "trivy", first["scanner"])
	assert.Equal(t, "0.58.1", first["scanner_version"])
	assert.Equal(t, "block", first["gate_result"])
	assert.Equal(t, false, first["db_stale"])
	assert.Equal(t, float64(2), first["targets"])
	assert.Equal(t, map[string]any{"critical": float64(3), "high": float64(5), "medium": float64(0), "low": float64(0), "unknown": float64(0)}, first["counts"])
	assert.Equal(t, "https://ci/report", first["report_uri"])
	assert.NotContains(t, first, "report_ref", "리포트 위치는 내부용이다")
	assert.NotEmpty(t, first["db_updated_at"])
	assert.NotEmpty(t, first["scanned_at"])

	second := resp.Items[1]
	assert.Equal(t, "error", second["gate_result"])
	assert.NotContains(t, second, "counts", "모르는 건수는 0 이 아니라 빠진다")
	assert.NotContains(t, second, "targets")
	assert.NotContains(t, second, "scanner_version")
	assert.NotContains(t, second, "report_uri")
	assert.Equal(t, true, second["db_stale"], "DB 시각을 모르면 낡은 것으로 본다")
}

// 시크릿 탐지 결과는 탐지 위치뿐이다. rules · findings 는 늘 배열이고, 탐지 건수는 모르면 빠진다.
func TestPipelineHandler_ListSecretScans(t *testing.T) {
	stores := newScanResultStores()
	now := time.Now().UTC().Truncate(time.Second)
	two := 2
	require.NoError(t, stores.secrets.Upsert(context.Background(), &domain.SecretScanResult{
		ID: "secretscan_dep_2", PipelineID: "pip-1", DeploymentID: "dep_2", Scanner: "gitleaks", FindingsCount: &two,
		Rules: []domain.SecretRuleCount{{RuleID: "github-pat", Count: 2}},
		Findings: []domain.SecretFinding{
			{RuleID: "github-pat", File: "config/app.env", StartLine: 3, Commit: "9f1c2ab", Fingerprint: "9f1c2ab:config/app.env:github-pat:3"},
			{RuleID: "github-pat", File: "README.md", StartLine: 12, Fingerprint: "README.md:github-pat:12"},
		},
		GateResult: domain.GateResultBlock, ReportURI: "https://ci/gitleaks", ScannedAt: now,
	}))
	require.NoError(t, stores.secrets.Upsert(context.Background(), &domain.SecretScanResult{
		ID: "secretscan_dep_1", PipelineID: "pip-1", DeploymentID: "dep_1", Scanner: "gitleaks",
		GateResult: domain.GateResultError, ScannedAt: now.Add(-time.Hour),
	}))
	e := newScanResultsEcho(t, "org-1", stores)

	code, body := getScanResults(t, e, "/api/v1/cicd/pipelines/pip-1/secret-scans", nil)
	require.Equal(t, http.StatusOK, code, string(body))

	var resp struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Equal(t, 2, resp.Total)
	first := resp.Items[0]
	assert.Equal(t, "secretscan_dep_2", first["id"])
	assert.Equal(t, "dep_2", first["deployment_id"])
	assert.Equal(t, "gitleaks", first["scanner"])
	assert.Equal(t, float64(2), first["findings_count"])
	assert.Equal(t, "block", first["gate_result"])
	assert.Equal(t, []any{map[string]any{"rule_id": "github-pat", "count": float64(2)}}, first["rules"])
	findings := first["findings"].([]any)
	require.Len(t, findings, 2)
	assert.Equal(t, map[string]any{"rule_id": "github-pat", "file": "config/app.env", "start_line": float64(3),
		"commit": "9f1c2ab", "fingerprint": "9f1c2ab:config/app.env:github-pat:3"}, findings[0])
	assert.NotContains(t, findings[1], "commit", "작업 트리 검사는 커밋을 모른다")

	second := resp.Items[1]
	assert.NotContains(t, second, "findings_count", "모르는 탐지 건수는 0 이 아니라 빠진다")
	assert.Equal(t, []any{}, second["rules"])
	assert.Equal(t, []any{}, second["findings"])
}

// 저장소가 배선되지 않았으면 빈 목록이 아니라 그 사실을 알린다.
func TestPipelineHandler_ListDependencyAndSecretScans_NotConfigured(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1"})
	e := newPipelineEcho(t, pipelineRepo, newMockPipelineTemplateRepository(), &mockDeploymentRepository{})
	for _, path := range []string{"/api/v1/pipelines/pip-1/dependency-scans", "/api/v1/pipelines/pip-1/secret-scans"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, path)
	}
}

// 다른 조직의 파이프라인 스캔 결과는 없는 것과 같게 404 다(Security Dashboard 설계 §8 G4). 보안 대시보드가
// 링크하는 단건 조회가 다른 조직의 결과로 가는 길이 되면 안 된다.
func TestPipelineHandler_ScanReads_OtherOrgIsNotFound(t *testing.T) {
	stores := newScanResultStores()
	require.NoError(t, stores.images.Upsert(context.Background(), &domain.ImageScanResult{
		ID: "scan_1", PipelineID: "pip-1", GateResult: domain.GateResultWarn, ScannedAt: time.Now(),
		ReportRef: &domain.ScanReportRef{JobName: "orders", BuildNumber: 7, Path: "trivy-report.json"},
	}))
	paths := []string{
		"/api/v1/cicd/pipelines/pip-1/image-scans",
		"/api/v1/cicd/pipelines/pip-1/image-scans/scan_1/vulnerabilities",
		"/api/v1/cicd/pipelines/pip-1/sast-results",
		"/api/v1/cicd/pipelines/pip-1/dependency-scans",
		"/api/v1/cicd/pipelines/pip-1/secret-scans",
	}

	other := newScanResultsEcho(t, "org-2", stores)
	same := newScanResultsEcho(t, "org-1", stores)
	devMode := newScanResultsEcho(t, "", stores)
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			code, body := getScanResults(t, other, path, nil)
			assert.Equal(t, http.StatusNotFound, code, string(body))
			assert.Contains(t, string(body), "PIPELINE_NOT_FOUND")
			assert.NotContains(t, string(body), "org-1", "다른 조직의 것이 있다는 사실도 알리지 않는다")

			code, body = getScanResults(t, same, path, nil)
			assert.Equal(t, http.StatusOK, code, string(body))

			// principal 이 없으면(인증을 끈 개발 모드) X-Org-ID 를 따른다.
			code, _ = getScanResults(t, devMode, path, map[string]string{"X-Org-ID": "org-2"})
			assert.Equal(t, http.StatusNotFound, code)
			// 신원도 헤더도 없으면 다른 cicd 조회 · 스택 모듈처럼 기본 조직으로 비교한다 — 비교를 건너뛰면
			// 조직 클레임이 없는 토큰이 헤더 없이 아무 조직의 결과나 읽는다.
			code, _ = getScanResults(t, devMode, path, nil)
			assert.Equal(t, http.StatusNotFound, code, "org-1 파이프라인은 기본 조직의 것이 아니다")
			code, _ = getScanResults(t, devMode, path, map[string]string{"X-Org-ID": "org-1"})
			assert.Equal(t, http.StatusOK, code)

			// principal 이 헤더보다 먼저다 — 헤더로 다른 조직을 흉내 낼 수 없다.
			code, _ = getScanResults(t, other, path, map[string]string{"X-Org-ID": "org-1"})
			assert.Equal(t, http.StatusNotFound, code)
		})
	}
}
