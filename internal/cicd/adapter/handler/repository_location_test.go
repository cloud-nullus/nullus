package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

type fakeRepositoryLocator struct {
	loc   *port.RepositoryLocation
	err   error
	calls []string
}

func (f *fakeRepositoryLocator) RepositoryLocation(_ context.Context, stackID string) (*port.RepositoryLocation, error) {
	f.calls = append(f.calls, stackID)
	return f.loc, f.err
}

func repositoryLocationEcho(t *testing.T, locator port.RepositoryLocator) *echo.Echo {
	t.Helper()
	pipelineRepo := newMockPipelineRepository()
	deploymentRepo := &mockDeploymentRepository{}
	h := cicdhandler.NewPipelineHandler(
		usecase.NewCreatePipeline(pipelineRepo, newMockPipelineTemplateRepository()),
		usecase.NewListPipelines(pipelineRepo),
		usecase.NewDeployPipeline(pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, &noopManifestApplier{}),
		pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, kube.NewStepTracker(), nil,
	)
	if locator != nil {
		h = h.WithRepositoryLocator(locator)
	}
	e := echo.New()
	h.RegisterRoutes(e.Group("/api/v1/cicd"))
	return e
}

func getRepositoryLocation(e *echo.Echo, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cicd/repository-location"+query, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// 앱 배포 화면은 스택 모드에서 실제로 만들어질 저장소 주소(<웹 주소>/<그룹>/<앱 이름>)를 보여 준다.
func TestPipelineHandler_RepositoryLocation(t *testing.T) {
	locator := &fakeRepositoryLocator{loc: &port.RepositoryLocation{
		Platform: port.SCMPlatformGitLab, GroupPath: "nullus", WebBaseURL: "https://gitlab.nullus.local",
	}}
	rec := getRepositoryLocation(repositoryLocationEcho(t, locator), "?stack_id=stk_1")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, map[string]string{
		"platform": "gitlab", "group_path": "nullus", "web_base_url": "https://gitlab.nullus.local",
	}, body)
	assert.Equal(t, []string{"stk_1"}, locator.calls)
}

func TestPipelineHandler_RepositoryLocation_RequiresStack(t *testing.T) {
	rec := getRepositoryLocation(repositoryLocationEcho(t, &fakeRepositoryLocator{}), "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPipelineHandler_RepositoryLocation_UnknownStack(t *testing.T) {
	locator := &fakeRepositoryLocator{err: fmt.Errorf("%w: stack x not found", port.ErrStackToolsUnavailable)}
	rec := getRepositoryLocation(repositoryLocationEcho(t, locator), "?stack_id=x")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPipelineHandler_RepositoryLocation_OtherFailure(t *testing.T) {
	rec := getRepositoryLocation(repositoryLocationEcho(t, &fakeRepositoryLocator{err: errors.New("no github connection")}), "?stack_id=x")
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "스택은 있지만 저장소 자리를 정할 수 없다(연동 미설정 등)")
}

// 배선되지 않은 구성에서는 화면이 미리보기 없이 동작해야 한다 — 404 가 아니라 501 로 알린다.
func TestPipelineHandler_RepositoryLocation_NotWired(t *testing.T) {
	rec := getRepositoryLocation(repositoryLocationEcho(t, nil), "?stack_id=stk_1")
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}
