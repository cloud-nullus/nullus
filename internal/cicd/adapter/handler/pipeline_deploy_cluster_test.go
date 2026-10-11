package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

// recordingManifestApplier 는 적용 요청이 왔는지만 남긴다.
type recordingManifestApplier struct{ applied int }

func (r *recordingManifestApplier) Apply(context.Context, []byte, []string) error {
	r.applied++
	return nil
}

func (r *recordingManifestApplier) ApplyWithTracking(context.Context, []byte, []string, string, ...int) error {
	r.applied++
	return nil
}

func newDeployClusterEcho(
	pipelineRepo *mockPipelineRepository,
	deployments *mockDeploymentRepository,
	reader port.StackReader,
	applier port.ManifestApplier,
) *echo.Echo {
	h := cicdhandler.NewPipelineHandler(
		usecase.NewCreatePipeline(pipelineRepo, newMockPipelineTemplateRepository(), reader),
		usecase.NewListPipelines(pipelineRepo),
		usecase.NewDeployPipeline(pipelineRepo, deployments, &noopKubeconfigProvider{}, &noopManifestApplier{}),
		pipelineRepo, deployments, &noopKubeconfigProvider{}, kube.NewStepTracker(), nil,
	).WithStackReader(reader).WithManifestApplier(applier)
	e := echo.New()
	h.RegisterRoutes(e.Group("/api/v1"))
	h.RegisterStackRoutes(e.Group("/api/v1/stacks"))
	return e
}

// 목록은 앱이 실제로 서 있는 클러스터를 함께 내려준다. 스택 클러스터 규칙이 생기기
// 전에 만든 스택 파이프라인에는 화면에서 고른 클러스터가 저장돼 있다 — 그 값을
// 보여 주면 정보 탭은 앱이 없는 클러스터를 배포 대상이라 하고, 클러스터 필터는
// 그 클러스터로 찾아야 앱이 나온다.
func TestPipelineHandler_ListByStack_ReportsDeployCluster(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(
		&domain.Pipeline{ID: "pip_legacy", Name: "orders", StackID: "stk_1", ClusterID: "c-chosen"},
		&domain.Pipeline{
			ID: "pip_direct", Name: "hotfix", StackID: "stk_1", ClusterID: "c-chosen",
			ExecutionMode: domain.ExecutionModeEmergencyDirect,
		},
	)
	reader := &stubStackReader{summary: &port.StackSummary{ID: "stk_1", ClusterID: "c-stack"}}
	e := newDeployClusterEcho(pipelineRepo, &mockDeploymentRepository{}, reader, &recordingManifestApplier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/stk_1/pipelines", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []struct {
			ID              string `json:"id"`
			ClusterID       string `json:"cluster_id"`
			DeployClusterID string `json:"deploy_cluster_id"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	got := map[string][2]string{}
	for _, item := range resp.Items {
		got[item.ID] = [2]string{item.ClusterID, item.DeployClusterID}
	}
	assert.Equal(t, [2]string{"c-chosen", "c-stack"}, got["pip_legacy"],
		"Argo CD 가 올린 앱은 스택 클러스터에 있다; 저장된 값은 그대로 둔다")
	assert.Equal(t, [2]string{"c-chosen", "c-chosen"}, got["pip_direct"],
		"플랫폼이 직접 적용한 앱은 파이프라인의 클러스터에 있다")
}

// 직접 배포도 생성과 같은 규칙을 따른다(대상 판정은 usecase.MatchStackTarget). 스택을
// 확인하지 못한 채 요청한 클러스터에 적용하면 기록은 스택 파이프라인(러너·Argo CD 경로)
// 으로 남아, 모니터링·삭제가 스택 클러스터를 뒤지고 앱은 요청한 클러스터에 남는다.
// 어느 경우도 적용하거나 기록하지 않는다.
func TestPipelineHandler_DeployApp_RejectsUnverifiedStack(t *testing.T) {
	cases := map[string]struct {
		reader   *stubStackReader
		wantCode int
		wantBody string
	}{
		"스택 조회 실패": {&stubStackReader{err: errors.New("db down")}, http.StatusInternalServerError, "STACK_LOOKUP_FAILED"},
		"없는 스택":    {&stubStackReader{summary: nil}, http.StatusBadRequest, "STACK_NOT_FOUND"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pipelineRepo := newMockPipelineRepository()
			applier := &recordingManifestApplier{}
			e := newDeployClusterEcho(pipelineRepo, &mockDeploymentRepository{}, tc.reader, applier)

			body := `{"templateId":"go-web-api","appName":"my-api","clusterId":"c-other","stackId":"stk_1","namespace":"apps","gitUrl":"https://github.com/acme/api","replicas":1,"port":8080}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/deploy-app", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.wantBody)
			assert.Equal(t, 0, pipelineRepo.created, "파이프라인 기록을 남기지 않는다")
			assert.Equal(t, 0, applier.applied, "어느 클러스터에도 적용하지 않는다")
		})
	}
}
