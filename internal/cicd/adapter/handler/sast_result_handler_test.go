package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

func newSASTEcho(t *testing.T, results *repository.MemorySASTResultRepository) *echo.Echo {
	t.Helper()
	pipelineRepo := newMockPipelineRepository(&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1"})
	deploymentRepo := &mockDeploymentRepository{}
	e := echo.New()
	createUC := usecase.NewCreatePipeline(pipelineRepo, newMockPipelineTemplateRepository())
	listUC := usecase.NewListPipelines(pipelineRepo)
	deployUC := usecase.NewDeployPipeline(pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, &noopManifestApplier{})
	h := cicdhandler.NewPipelineHandler(createUC, listUC, deployUC, pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, kube.NewStepTracker(), nil)
	if results != nil {
		h = h.WithSASTResults(results)
	}
	h.RegisterRoutes(e.Group("/api/v1"))
	return e
}

// 실행 기록 화면이 실행마다 Quality Gate 판정·걸린 조건·지표를 보여준다. 대시보드(#65)도
// 이 공개 경로로 읽는다 — 결과 테이블은 cicd 가 소유한다.
func TestPipelineHandler_ListSASTResults(t *testing.T) {
	results := repository.NewMemorySASTResultRepository()
	vulns := 3
	require.NoError(t, results.Upsert(context.Background(), &domain.SASTResult{
		ID: "sast_dep_1", PipelineID: "pip-1", DeploymentID: "dep_1", ProjectKey: "orders",
		QualityGateStatus: domain.QualityGateError, GateResult: domain.GateResultBlock,
		Conditions:   []domain.SASTCondition{{Metric: "new_violations", Comparator: "GT", Threshold: "0", Actual: "1", Status: "ERROR"}},
		Metrics:      &domain.SASTMetrics{Vulnerabilities: &vulns},
		DashboardURL: "https://sonarqube.example.com/dashboard?id=orders",
		AnalyzedAt:   time.Now(),
	}))
	e := newSASTEcho(t, results)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/pip-1/sast-results", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Items []struct {
			ID                string `json:"id"`
			DeploymentID      string `json:"deployment_id"`
			GateResult        string `json:"gate_result"`
			QualityGateStatus string `json:"quality_gate_status"`
			Conditions        []struct {
				Metric string `json:"metric"`
				Actual string `json:"actual"`
			} `json:"conditions"`
			Metrics struct {
				Vulnerabilities *int `json:"vulnerabilities"`
				Bugs            *int `json:"bugs"`
			} `json:"metrics"`
			DashboardURL string `json:"dashboard_url"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Total)
	got := resp.Items[0]
	assert.Equal(t, "dep_1", got.DeploymentID, "화면은 실행 기록과 이 값으로 잇는다")
	assert.Equal(t, "block", got.GateResult)
	assert.Equal(t, "ERROR", got.QualityGateStatus)
	require.Len(t, got.Conditions, 1)
	assert.Equal(t, "new_violations", got.Conditions[0].Metric)
	require.NotNil(t, got.Metrics.Vulnerabilities)
	assert.Equal(t, 3, *got.Metrics.Vulnerabilities)
	assert.Nil(t, got.Metrics.Bugs, "모르는 지표를 0 으로 내리면 '버그 0건' 으로 보인다")
	assert.Equal(t, "https://sonarqube.example.com/dashboard?id=orders", got.DashboardURL)
}

func TestPipelineHandler_ListSASTResults_UnknownPipeline(t *testing.T) {
	e := newSASTEcho(t, repository.NewMemorySASTResultRepository())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/missing/sast-results", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// 저장소가 배선되지 않았으면 빈 목록이 아니라 그 사실을 알린다 — 빈 목록은 "분석한 적 없음" 이다.
func TestPipelineHandler_ListSASTResults_NotConfigured(t *testing.T) {
	e := newSASTEcho(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/pip-1/sast-results", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
