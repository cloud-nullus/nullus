package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryHandler_GetVersionsDiff_200(t *testing.T) {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	historyRepo := stackrepo.NewMemoryHistoryRepository()
	err := historyRepo.SaveVersion(t.Context(), &domain.StackVersion{
		ID:        "v1",
		StackID:   "stack-1",
		Version:   1,
		ChangedBy: "alice",
		CreatedAt: time.Now(),
		Config: domain.StackConfig{
			Pipeline: domain.PipelineConfig{
				CIPlatform: domain.ToolSelection{Name: "GitLab CI", Version: "17.7.0", Enabled: true},
			},
		},
	})
	require.NoError(t, err)
	err = historyRepo.SaveVersion(t.Context(), &domain.StackVersion{
		ID:        "v2",
		StackID:   "stack-1",
		Version:   2,
		ChangedBy: "alice",
		CreatedAt: time.Now(),
		Config: domain.StackConfig{
			Pipeline: domain.PipelineConfig{
				CIPlatform: domain.ToolSelection{Name: "GitHub Actions", Version: "external", Enabled: true},
			},
		},
	})
	require.NoError(t, err)

	handler := stackhandler.NewHistoryHandler(
		historyRepo,
		stackrepo.NewMemoryStackRepository(),
		usecase.NewManageHistory(historyRepo),
	)

	v1 := e.Group("/api/v1")
	stacks := v1.Group("/stacks")
	handler.RegisterRoutes(stacks)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/stack-1/history/diff?versionA=1&versionB=2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Added   map[string]any    `json:"added"`
		Removed map[string]any    `json:"removed"`
		Changed map[string][2]any `json:"changed"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	assert.Empty(t, body.Added)
	assert.Empty(t, body.Removed)
	assert.Equal(t, [2]any{"GitLab CI", "GitHub Actions"}, body.Changed["pipeline.ci_platform.name"])
	assert.Equal(t, [2]any{"17.7.0", "external"}, body.Changed["pipeline.ci_platform.version"])
}
