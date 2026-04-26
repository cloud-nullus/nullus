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

	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stacklog "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

func newDeployEcho(t *testing.T) (*echo.Echo, *stackrepo.MemoryStackRepository) {
	t.Helper()

	e := echo.New()
	repo := stackrepo.NewMemoryStackRepository()
	streamer := stacklog.NewMemoryStreamer()
	install := usecase.NewInstallStack(repo, streamer)
	h := stackhandler.NewDeployHandler(install, repo, streamer)

	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1, e)

	return e, repo
}

func seedStack(t *testing.T, repo *stackrepo.MemoryStackRepository, state domain.DeploymentState) string {
	t.Helper()

	stack := &domain.Stack{
		ID:         "stk-deploy-test",
		Name:       "deploy-test",
		TemplateID: "gitlab-allinone-v1",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		State:      state,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), stack))
	return stack.ID
}

func TestDeployHandler_Deploy_202Accepted(t *testing.T) {
	e, repo := newDeployEcho(t)
	id := seedStack(t, repo, domain.StatePending)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, id, resp["stack_id"])
	assert.Equal(t, "accepted", resp["status"])
	assert.Contains(t, resp["message"], "/ws/deployments/")
}

func TestDeployHandler_Deploy_400WhenStackNotFound(t *testing.T) {
	e, _ := newDeployEcho(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/missing/deploy", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "DEPLOY_FAILED", resp["error"]["code"])
}

func TestDeployHandler_Deploy_400WhenTransitionInvalid(t *testing.T) {
	e, repo := newDeployEcho(t)
	id := seedStack(t, repo, domain.StateCompleted)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "DEPLOY_FAILED", resp["error"]["code"])
	assert.Contains(t, resp["error"]["message"], "invalid state transition")
}
