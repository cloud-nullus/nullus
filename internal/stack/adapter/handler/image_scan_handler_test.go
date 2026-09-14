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

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type fakeImageScanReporter struct {
	report  *domain.StackImageScanReport
	err     error
	vulnErr error
}

func (f fakeImageScanReporter) Report(context.Context, string) (*domain.StackImageScanReport, error) {
	return f.report, f.err
}

func (f fakeImageScanReporter) Vulnerabilities(_ context.Context, _ string, digest string, filter shareddomain.VulnerabilityFilter) (*shareddomain.VulnerabilityPage, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.vulnErr != nil {
		return nil, f.vulnErr
	}
	page := shareddomain.NewVulnerabilityPage([]shareddomain.ImageVulnerability{
		{ID: "CVE-1", PkgName: "openssl", Severity: "critical", Class: shareddomain.VulnerabilityClassOS, Target: digest},
		{ID: "CVE-2", PkgName: "lodash", Severity: "high", Class: shareddomain.VulnerabilityClassLibrary, Target: "app"},
	}, filter)
	return &page, nil
}

func getImageScans(t *testing.T, reporter fakeImageScanReporter, now time.Time) (int, map[string]any) {
	t.Helper()
	e := echo.New()
	h := NewImageScanHandler(reporter)
	h.now = func() time.Time { return now }
	h.RegisterRoutes(e.Group("/api/v1/stacks"))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stacks/stk_1/image-scans", nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func TestImageScanHandler_Scanned(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	fresh := now.Add(-time.Hour)
	old := now.Add(-40 * 24 * time.Hour)
	counts := shareddomain.SeverityCounts{Critical: 1, High: 2}
	report := domain.BuildImageScanReport("stk_1", "", []domain.StackImageScan{
		{Image: "redis:7", ImageDigest: "sha256:r", Release: "argo-cd", Workloads: []string{"argo-cd-argocd-redis"},
			Status: domain.ImageScanStatusScanned, Counts: &counts, FixableCounts: &shareddomain.SeverityCounts{High: 1},
			ScannerVersion: "0.74.0", DBUpdatedAt: &fresh, ScannedAt: now},
		{Image: "argocd:v2", ImageDigest: "sha256:a", Status: domain.ImageScanStatusFailed, Error: "manifest unknown",
			DBUpdatedAt: &old, ScannedAt: now},
	})

	code, body := getImageScans(t, fakeImageScanReporter{report: &report}, now)

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "stk_1", body["stack_id"])
	assert.Equal(t, "scanned", body["status"])
	assert.Equal(t, "", body["reason"])
	assert.Equal(t, float64(2), body["total"])
	assert.Equal(t, map[string]any{"critical": float64(1), "high": float64(2), "medium": float64(0), "low": float64(0), "unknown": float64(0)}, body["summary"])

	items := body["items"].([]any)
	scanned := items[0].(map[string]any)
	assert.Equal(t, "sha256:r", scanned["image_digest"])
	assert.Equal(t, []any{"argo-cd-argocd-redis"}, scanned["workloads"])
	assert.Equal(t, false, scanned["db_stale"])
	assert.NotNil(t, scanned["fixable_counts"])

	failed := items[1].(map[string]any)
	assert.Equal(t, "failed", failed["status"])
	assert.Equal(t, "manifest unknown", failed["error"])
	// 스캔하지 못한 이미지는 건수를 싣지 않는다 — 0 으로 읽히면 안 된다.
	assert.NotContains(t, failed, "counts")
	assert.Equal(t, true, failed["db_stale"])
}

// 스캔하지 않는 스택도 같은 모양으로 답한다. items 는 null 이 아니라 빈 목록이다.
func TestImageScanHandler_NotScanned(t *testing.T) {
	report := domain.BuildImageScanReport("stk_1", domain.ImageScanReasonAirgap, nil)

	code, body := getImageScans(t, fakeImageScanReporter{report: &report}, time.Now())

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "not_scanned", body["status"])
	assert.Equal(t, "airgap", body["reason"])
	assert.Equal(t, []any{}, body["items"])
	assert.Nil(t, body["summary"])
	assert.Nil(t, body["last_scanned_at"])
	assert.Equal(t, float64(0), body["total"])
}

func TestImageScanHandler_StackNotFound(t *testing.T) {
	code, body := getImageScans(t, fakeImageScanReporter{err: errors.New("stack not found: stk_1")}, time.Now())

	assert.Equal(t, http.StatusNotFound, code)
	assert.NotNil(t, body["error"])
}

func getImageVulnerabilities(t *testing.T, reporter fakeImageScanReporter, query string) (int, map[string]any) {
	t.Helper()
	e := echo.New()
	NewImageScanHandler(reporter).RegisterRoutes(e.Group("/api/v1/stacks"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stacks/stk_1/image-scans/vulnerabilities"+query, nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func TestImageScanHandler_Vulnerabilities(t *testing.T) {
	code, body := getImageVulnerabilities(t, fakeImageScanReporter{}, "?digest=sha256:abc&class=os&limit=10")

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "available", body["status"])
	assert.Equal(t, float64(1), body["total"])
	assert.Equal(t, float64(10), body["limit"])
	item := body["items"].([]any)[0].(map[string]any)
	assert.Equal(t, "CVE-1", item["id"])
	assert.Equal(t, "os", item["class"])
	assert.Equal(t, "sha256:abc", item["target"])
}

func TestImageScanHandler_Vulnerabilities_RequiresDigest(t *testing.T) {
	code, _ := getImageVulnerabilities(t, fakeImageScanReporter{}, "")
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestImageScanHandler_Vulnerabilities_UnknownImage(t *testing.T) {
	code, body := getImageVulnerabilities(t, fakeImageScanReporter{vulnErr: usecase.ErrStackImageScanNotFound}, "?digest=sha256:nope")
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "IMAGE_SCAN_NOT_FOUND", body["error"].(map[string]any)["code"])
}
