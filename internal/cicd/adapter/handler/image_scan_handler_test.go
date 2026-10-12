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

// 대시보드(#65)는 스캔 결과 테이블을 직접 조회하지 않는다 — cicd 가 소유하므로
// cicd 의 공개 경로로 읽는다.
func TestPipelineHandler_ListImageScans(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1"})
	deploymentRepo := &mockDeploymentRepository{}
	scans := repository.NewMemoryImageScanResultRepository()
	old := time.Now().Add(-40 * 24 * time.Hour)
	require.NoError(t, scans.Upsert(context.Background(), &domain.ImageScanResult{
		ID: "scan_1", PipelineID: "pip-1", Scanner: "trivy", GateResult: domain.GateResultBlock,
		DBUpdatedAt: &old, ScannedAt: time.Now(),
	}))

	e := echo.New()
	createUC := usecase.NewCreatePipeline(pipelineRepo, newMockPipelineTemplateRepository())
	listUC := usecase.NewListPipelines(pipelineRepo)
	deployUC := usecase.NewDeployPipeline(pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, &noopManifestApplier{})
	h := cicdhandler.NewPipelineHandler(createUC, listUC, deployUC, pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, kube.NewStepTracker(), nil).
		WithImageScans(scans)
	h.RegisterRoutes(e.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/pip-1/image-scans", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []struct {
			ID         string `json:"id"`
			GateResult string `json:"gate_result"`
			// DB 가 낡았으면 화면이 초록불을 켜지 않도록 서버가 판정해 내려준다.
			DBStale bool `json:"db_stale"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Total)
	assert.Equal(t, "block", resp.Items[0].GateResult)
	assert.True(t, resp.Items[0].DBStale, "40일 된 DB 는 낡은 것으로 내려야 한다")
}

// 스캔 결과 저장소가 배선되지 않았으면 빈 목록이 아니라 그 사실을 알린다.
// 빈 목록은 "스캔한 적 없음" 으로 읽힌다.
func TestPipelineHandler_ListImageScans_NotConfigured(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1"})
	e := newPipelineEcho(t, pipelineRepo, newMockPipelineTemplateRepository(), &mockDeploymentRepository{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/pip-1/image-scans", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
