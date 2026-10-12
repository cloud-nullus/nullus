package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

type vulnPipelineRepo struct{ port.PipelineRepository }

func (vulnPipelineRepo) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	if id != "pip_1" {
		return nil, errors.New("pipeline not found")
	}
	return &domain.Pipeline{ID: "pip_1", Name: "app", StackID: "stk_1"}, nil
}

type vulnArtifacts struct{ raw []byte }

func (a vulnArtifacts) ReadArtifact(context.Context, port.CIArtifactRef) ([]byte, bool, error) {
	return a.raw, true, nil
}

type vulnBundleFactory struct{ raw []byte }

func (f vulnBundleFactory) For(context.Context, string) (*port.SCMBundle, error) {
	return &port.SCMBundle{CIArtifacts: vulnArtifacts{raw: f.raw}}, nil
}

func serveScanVulnerabilities(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile("../../../shared/domain/testdata/trivy-report-node16.json")
	require.NoError(t, err)
	scans := repository.NewMemoryImageScanResultRepository()
	require.NoError(t, scans.Upsert(context.Background(), &domain.ImageScanResult{
		ID: "scan_1", PipelineID: "pip_1", GateResult: domain.GateResultWarn, ScannedAt: time.Now(),
		ReportRef: &domain.ScanReportRef{JobName: "app", BuildNumber: 7, StageID: "11", Path: "trivy-report.json"},
	}))

	h := (&PipelineHandler{}).WithScanVulnerabilities(
		usecase.NewScanVulnerabilities(vulnPipelineRepo{}, scans, vulnBundleFactory{raw: raw}))
	e := echo.New()
	h.RegisterRoutes(e.Group("/api/v1/cicd"))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func TestListScanVulnerabilities(t *testing.T) {
	code, body := serveScanVulnerabilities(t,
		"/api/v1/cicd/pipelines/pip_1/image-scans/scan_1/vulnerabilities?class=library&severity=critical,high&limit=5")

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "available", body["status"])
	assert.Equal(t, float64(5), body["limit"])
	items := body["items"].([]any)
	require.NotEmpty(t, items)
	first := items[0].(map[string]any)
	assert.Equal(t, "library", first["class"])
	assert.Contains(t, []any{"critical", "high"}, first["severity"])
}

func TestListScanVulnerabilities_NotFound(t *testing.T) {
	code, body := serveScanVulnerabilities(t, "/api/v1/cicd/pipelines/pip_1/image-scans/scan_nope/vulnerabilities")
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "IMAGE_SCAN_NOT_FOUND", body["error"].(map[string]any)["code"])

	code, body = serveScanVulnerabilities(t, "/api/v1/cicd/pipelines/pip_x/image-scans/scan_1/vulnerabilities")
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "PIPELINE_NOT_FOUND", body["error"].(map[string]any)["code"])
}

func TestListScanVulnerabilities_NotConfigured(t *testing.T) {
	e := echo.New()
	(&PipelineHandler{}).RegisterRoutes(e.Group("/api/v1/cicd"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cicd/pipelines/pip_1/image-scans/scan_1/vulnerabilities", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
