package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UAT-1: Junior DevOps "미정" 시나리오
// 미정이는 새 프로젝트를 시작한다.
func TestUAT1_Mijeong_JuniorDevOps(t *testing.T) {
	// 1. Organization 생성 ("my-team")
	status, resp := doRequest(t, http.MethodPost, "/api/v1/orgs", map[string]any{
		"name":   "my-team",
		"slug":   "my-team",
		"domain": "my-team.internal",
	})
	require.Equal(t, http.StatusCreated, status, "organization 생성 실패")
	orgData := parseData(t, resp)
	orgID := getString(t, orgData, "id")
	assert.Equal(t, "my-team", orgData["name"])

	// 2. 클러스터 등록 ("prod-cluster", pipeline 타입)
	status, resp = doRequest(t, http.MethodPost, "/api/v1/clusters", map[string]any{
		"name":     "prod-cluster",
		"type":     "pipeline",
		"endpoint": "https://k8s.prod.my-team.internal",
		"org_id":   orgID,
	})
	require.Equal(t, http.StatusCreated, status, "클러스터 등록 실패")
	clusterData := parseData(t, resp)
	clusterID := getString(t, clusterData, "id")
	assert.Equal(t, "prod-cluster", clusterData["name"])

	// 3. Golden Path "GitLab All-in-One" 템플릿 조회
	status, resp = doRequest(t, http.MethodGet, "/api/v1/templates/gitlab-allinone-v1", nil)
	require.Equal(t, http.StatusOK, status, "템플릿 조회 실패")
	tmpl := parseData(t, resp)
	assert.Equal(t, "gitlab-allinone-v1", tmpl["id"])

	// 4. 스택 생성 (템플릿 기반)
	status, resp = doRequest(t, http.MethodPost, "/api/v1/stacks", map[string]any{
		"name":           "mijeong-stack",
		"cluster_id":     clusterID,
		"golden_path_id": "gitlab-allinone-v1",
		"config":         map[string]any{},
	})
	require.Equal(t, http.StatusCreated, status, "스택 생성 실패")
	stackData := parseData(t, resp)
	stackID := getString(t, stackData, "id")
	assert.Equal(t, "mijeong-stack", stackData["name"])

	// 5. 리소스 예상량 계산 (개발자 20명, 러너 4개)
	status, resp = doRequest(t, http.MethodPost, "/api/v1/resources/estimate", map[string]any{
		"tools": []map[string]any{
			{"name": "gitlab-ce", "instances": 1},
			{"name": "gitlab-runner", "instances": 4},
			{"name": "argocd", "instances": 1},
		},
		"workload": map[string]any{
			"developers":         20,
			"concurrent_runners": 4,
			"weekly_commits":     50,
			"build_frequency":    "on-push",
		},
	})
	require.Equal(t, http.StatusOK, status, "리소스 예상량 계산 실패")
	require.NotNil(t, resp)

	// 6. 스택 배포 시작
	status, resp = doRequest(t, http.MethodPost, "/api/v1/stacks/"+stackID+"/deploy", nil)
	require.Equal(t, http.StatusAccepted, status, "스택 배포 시작 실패")
	require.NotNil(t, resp)

	// 7. 배포 상태 확인 → installing 또는 이후 상태
	status, resp = doRequest(t, http.MethodGet, "/api/v1/stacks/"+stackID+"/status", nil)
	require.Equal(t, http.StatusOK, status, "배포 상태 확인 실패")
	statusData := parseData(t, resp)
	assert.Equal(t, stackID, statusData["stack_id"])
	state, ok := statusData["state"].(string)
	require.True(t, ok, "state 필드가 문자열이어야 함")
	assert.NotEmpty(t, state, "state가 비어있으면 안 됨")
}

// UAT-2: Developer "지은" 시나리오
// 지은이는 앱을 배포한다.
func TestUAT2_Jieun_Developer(t *testing.T) {
	// 1. CI/CD 템플릿 목록 조회
	status, resp := doRequest(t, http.MethodGet, "/api/v1/cicd/templates", nil)
	require.Equal(t, http.StatusOK, status, "CI/CD 템플릿 목록 조회 실패")
	templates := parseDataSlice(t, resp)
	require.Len(t, templates, 3, "CI/CD 템플릿 3개여야 함")

	// 2. "Web Backend" 템플릿으로 파이프라인 생성
	status, resp = doRequest(t, http.MethodPost, "/api/v1/pipelines", map[string]any{
		"name":         "jieun-backend-pipeline",
		"template_id":  "web-backend-v1",
		"cluster_id":   "cluster-jieun",
		"namespace":    "jieun-app",
		"app_type":     "backend",
		"git_repo_url": "https://gitlab.example.com/jieun/backend",
	})
	require.Equal(t, http.StatusCreated, status, "파이프라인 생성 실패")
	pipelineData := parseData(t, resp)
	pipelineID := getString(t, pipelineData, "id")
	assert.Equal(t, "jieun-backend-pipeline", pipelineData["name"])

	// 3. 파이프라인 배포 실행
	status, resp = doRequest(t, http.MethodPost, "/api/v1/pipelines/"+pipelineID+"/deploy", map[string]any{
		"version":     "v2.1.0",
		"deployed_by": "jieun",
	})
	require.Equal(t, http.StatusCreated, status, "파이프라인 배포 실패")
	require.NotNil(t, resp)

	// 4. 배포 이력 확인
	status, resp = doRequest(t, http.MethodGet, "/api/v1/pipelines/"+pipelineID+"/deployments", nil)
	require.Equal(t, http.StatusOK, status, "배포 이력 조회 실패")
	deployments := parseDataSlice(t, resp)
	assert.GreaterOrEqual(t, len(deployments), 1, "배포 이력이 1개 이상이어야 함")
}

// UAT-3: Admin 관리 시나리오
// 관리자가 플랫폼을 설정한다.
func TestUAT3_Admin_PlatformSetup(t *testing.T) {
	// 1. Organization 생성
	status, resp := doRequest(t, http.MethodPost, "/api/v1/orgs", map[string]any{
		"name":   "admin-org",
		"slug":   "admin-org-uat3",
		"domain": "admin.internal",
	})
	require.Equal(t, http.StatusCreated, status, "Organization 생성 실패")
	orgData := parseData(t, resp)
	orgID := getString(t, orgData, "id")

	// 2. Organization 수정 (도메인 추가)
	status, resp = doRequest(t, http.MethodPut, "/api/v1/orgs/"+orgID, map[string]any{
		"name":   "admin-org",
		"domain": "admin.company.internal",
	})
	require.Equal(t, http.StatusOK, status, "Organization 수정 실패")
	updatedOrg := parseData(t, resp)
	assert.Equal(t, "admin.company.internal", updatedOrg["domain"])

	// 3. 클러스터 2개 등록 (pipeline, target)
	status, resp = doRequest(t, http.MethodPost, "/api/v1/clusters", map[string]any{
		"name":     "pipeline-cluster",
		"type":     "pipeline",
		"endpoint": "https://k8s.pipeline.admin.internal",
		"org_id":   orgID,
	})
	require.Equal(t, http.StatusCreated, status, "pipeline 클러스터 등록 실패")
	parseData(t, resp)

	status, resp = doRequest(t, http.MethodPost, "/api/v1/clusters", map[string]any{
		"name":     "target-cluster",
		"type":     "target",
		"endpoint": "https://k8s.target.admin.internal",
		"org_id":   orgID,
	})
	require.Equal(t, http.StatusCreated, status, "target 클러스터 등록 실패")
	parseData(t, resp)

	// 4. 클러스터 목록 조회 → 2개
	status, resp = doRequest(t, http.MethodGet, "/api/v1/clusters?org_id="+orgID, nil)
	require.Equal(t, http.StatusOK, status, "클러스터 목록 조회 실패")
	clusters := parseDataSlice(t, resp)
	assert.Len(t, clusters, 2, "클러스터가 2개여야 함")

	// 5. 호환성 매트릭스 확인
	status, resp = doRequest(t, http.MethodGet, "/api/v1/compatibility/matrix", nil)
	require.Equal(t, http.StatusOK, status, "호환성 매트릭스 조회 실패")
	matrices := parseDataSlice(t, resp)
	assert.GreaterOrEqual(t, len(matrices), 1, "호환성 매트릭스가 1개 이상이어야 함")

	// 6. 알림 규칙 생성 (CPU 80% 초과)
	status, resp = doRequest(t, http.MethodPost, "/api/v1/alerts/rules", map[string]any{
		"name":      "CPU 80% Alert",
		"condition": "cpu_usage > threshold",
		"threshold": 80.0,
		"channel":   "slack",
		"enabled":   true,
	})
	require.Equal(t, http.StatusCreated, status, "알림 규칙 생성 실패")
	ruleData := parseData(t, resp)
	assert.Equal(t, "CPU 80% Alert", ruleData["name"])
	assert.Equal(t, 80.0, ruleData["threshold"])

	// 7. 대시보드 조회
	status, resp = doRequest(t, http.MethodGet, "/api/v1/monitoring/dashboard", nil)
	require.Equal(t, http.StatusOK, status, "대시보드 조회 실패")
	dashboard := parseData(t, resp)
	assert.NotNil(t, dashboard["cluster_metrics"], "cluster_metrics가 있어야 함")
	assert.NotNil(t, dashboard["pipeline_metrics"], "pipeline_metrics가 있어야 함")
	assert.NotNil(t, dashboard["tool_health"], "tool_health가 있어야 함")
}
