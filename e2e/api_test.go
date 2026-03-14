package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scenario 1: Organization + Cluster 등록 흐름
func TestScenario1_OrgAndCluster(t *testing.T) {
	// 1. POST /api/v1/orgs → 201
	status, resp := doRequest(t, http.MethodPost, "/api/v1/orgs", map[string]any{
		"name":   "Test Org",
		"slug":   "test-org",
		"domain": "test.io",
	})
	assertStatus(t, status, http.StatusCreated)
	orgData := parseData(t, resp)
	orgID := getString(t, orgData, "id")
	assert.Equal(t, "Test Org", orgData["name"])

	// 2. GET /api/v1/orgs/:id → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/orgs/"+orgID, nil)
	assertStatus(t, status, http.StatusOK)
	gotOrg := parseData(t, resp)
	assert.Equal(t, orgID, gotOrg["id"])

	// 3. PUT /api/v1/orgs/:id → 200
	status, resp = doRequest(t, http.MethodPut, "/api/v1/orgs/"+orgID, map[string]any{
		"name":   "Test Org Updated",
		"domain": "updated-test.io",
	})
	assertStatus(t, status, http.StatusOK)
	updatedOrg := parseData(t, resp)
	assert.Equal(t, "Test Org Updated", updatedOrg["name"])
	assert.Equal(t, "updated-test.io", updatedOrg["domain"])

	// 4. POST /api/v1/clusters → 201
	status, resp = doRequest(t, http.MethodPost, "/api/v1/clusters", map[string]any{
		"name":     "prod-cluster",
		"type":     "pipeline",
		"endpoint": "https://k8s.prod.example.com",
		"org_id":   orgID,
	})
	assertStatus(t, status, http.StatusCreated)
	clusterData := parseData(t, resp)
	clusterID := getString(t, clusterData, "id")
	assert.Equal(t, "prod-cluster", clusterData["name"])

	// 5. GET /api/v1/clusters → 200, 1개
	status, resp = doRequest(t, http.MethodGet, "/api/v1/clusters?org_id="+orgID, nil)
	assertStatus(t, status, http.StatusOK)
	clusters := parseDataSlice(t, resp)
	assert.Len(t, clusters, 1)

	// 6. GET /api/v1/clusters/:id → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/clusters/"+clusterID, nil)
	assertStatus(t, status, http.StatusOK)
	gotCluster := parseData(t, resp)
	assert.Equal(t, clusterID, gotCluster["id"])

	// 7. POST /api/v1/clusters/:id/verify → 200
	status, resp = doRequest(t, http.MethodPost, "/api/v1/clusters/"+clusterID+"/verify", nil)
	assertStatus(t, status, http.StatusOK)
	verifiedCluster := parseData(t, resp)
	assert.Equal(t, clusterID, verifiedCluster["id"])
}

// Scenario 2: Stack 템플릿 → 설정 → 배포 흐름
func TestScenario2_StackDeployFlow(t *testing.T) {
	// 1. GET /api/v1/templates → 200, 3개
	status, resp := doRequest(t, http.MethodGet, "/api/v1/templates", nil)
	assertStatus(t, status, http.StatusOK)
	templates := parseDataSlice(t, resp)
	assert.Len(t, templates, 3)

	// 2. GET /api/v1/templates/gitlab-allinone-v1 → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/templates/gitlab-allinone-v1", nil)
	assertStatus(t, status, http.StatusOK)
	tmpl := parseData(t, resp)
	assert.Equal(t, "gitlab-allinone-v1", tmpl["id"])

	// 3. POST /api/v1/stacks → 201
	status, resp = doRequest(t, http.MethodPost, "/api/v1/stacks", map[string]any{
		"name":           "my-stack",
		"cluster_id":     "cluster-001",
		"golden_path_id": "gitlab-allinone-v1",
		"config":         map[string]any{},
	})
	assertStatus(t, status, http.StatusCreated)
	stackData := parseData(t, resp)
	stackID := getString(t, stackData, "id")
	assert.Equal(t, "my-stack", stackData["name"])

	// 4. GET /api/v1/stacks → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/stacks", nil)
	assertStatus(t, status, http.StatusOK)
	stacks := parseDataSlice(t, resp)
	assert.GreaterOrEqual(t, len(stacks), 1)

	// 5. GET /api/v1/stacks/:id → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/stacks/"+stackID, nil)
	assertStatus(t, status, http.StatusOK)
	gotStack := parseData(t, resp)
	assert.Equal(t, stackID, gotStack["id"])

	// 6. POST /api/v1/stacks/:id/deploy → 202
	status, resp = doRequest(t, http.MethodPost, "/api/v1/stacks/"+stackID+"/deploy", nil)
	assertStatus(t, status, http.StatusAccepted)
	require.NotNil(t, resp)

	// 7. GET /api/v1/stacks/:id/status → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/stacks/"+stackID+"/status", nil)
	assertStatus(t, status, http.StatusOK)
	statusData := parseData(t, resp)
	assert.Equal(t, stackID, statusData["stack_id"])
	assert.NotEmpty(t, statusData["state"])
}

// Scenario 3: 호환성 매트릭스
func TestScenario3_CompatibilityMatrix(t *testing.T) {
	// 1. GET /api/v1/compatibility/matrix → 200
	status, resp := doRequest(t, http.MethodGet, "/api/v1/compatibility/matrix", nil)
	assertStatus(t, status, http.StatusOK)
	matrices := parseDataSlice(t, resp)
	assert.GreaterOrEqual(t, len(matrices), 1)

	// 2. POST /api/v1/compatibility/validate → 200, compatible
	status, resp = doRequest(t, http.MethodPost, "/api/v1/compatibility/validate", map[string]any{
		"tools": map[string]string{
			"source_repository": "GitLab CE",
			"ci_platform":       "GitLab CI",
		},
	})
	assertStatus(t, status, http.StatusOK)
	valData := parseData(t, resp)
	assert.Equal(t, true, valData["compatible"])

	// 3. POST /api/v1/compatibility/validate → 200, untested/not-compatible
	status, _ = doRequest(t, http.MethodPost, "/api/v1/compatibility/validate", map[string]any{
		"tools": map[string]string{
			"source_repository": "GitHub",
			"ci_platform":       "GitHub Actions",
		},
	})
	// When no matching matrix is found, compatible=false and status is 200 (OK, not error)
	assert.True(t, status == http.StatusOK || status == http.StatusUnprocessableEntity)
}

// Scenario 4: CI/CD 파이프라인 흐름
func TestScenario4_CICDPipelineFlow(t *testing.T) {
	// 1. GET /api/v1/cicd/templates → 200, 3개
	status, resp := doRequest(t, http.MethodGet, "/api/v1/cicd/templates", nil)
	assertStatus(t, status, http.StatusOK)
	templates := parseDataSlice(t, resp)
	assert.Len(t, templates, 3)

	// 2. POST /api/v1/pipelines → 201
	status, resp = doRequest(t, http.MethodPost, "/api/v1/pipelines", map[string]any{
		"name":        "my-pipeline",
		"template_id": "web-backend-v1",
		"cluster_id":  "cluster-001",
		"namespace":   "default",
		"app_type":    "backend",
		"git_repo_url": "https://gitlab.example.com/my-app",
	})
	assertStatus(t, status, http.StatusCreated)
	pipelineData := parseData(t, resp)
	pipelineID := getString(t, pipelineData, "id")
	assert.Equal(t, "my-pipeline", pipelineData["name"])

	// 3. GET /api/v1/pipelines → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/pipelines", nil)
	assertStatus(t, status, http.StatusOK)
	pipelines := parseDataSlice(t, resp)
	assert.GreaterOrEqual(t, len(pipelines), 1)

	// 4. POST /api/v1/pipelines/:id/deploy → 201 (DeployPipeline returns 201 Created)
	status, resp = doRequest(t, http.MethodPost, "/api/v1/pipelines/"+pipelineID+"/deploy", map[string]any{
		"version":     "v1.0.0",
		"deployed_by": "ci-bot",
	})
	assertStatus(t, status, http.StatusCreated)
	require.NotNil(t, resp)

	// 5. GET /api/v1/pipelines/:id/deployments → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/pipelines/"+pipelineID+"/deployments", nil)
	assertStatus(t, status, http.StatusOK)
	deployments := parseDataSlice(t, resp)
	assert.GreaterOrEqual(t, len(deployments), 1)
}

// Scenario 5: 모니터링 + 알림
func TestScenario5_MonitoringAndAlerts(t *testing.T) {
	// 1. GET /api/v1/monitoring/dashboard → 200
	status, resp := doRequest(t, http.MethodGet, "/api/v1/monitoring/dashboard", nil)
	assertStatus(t, status, http.StatusOK)
	dashboard := parseData(t, resp)
	assert.NotNil(t, dashboard["cluster_metrics"])
	assert.NotNil(t, dashboard["pipeline_metrics"])

	// 2. POST /api/v1/alerts/rules → 201
	status, resp = doRequest(t, http.MethodPost, "/api/v1/alerts/rules", map[string]any{
		"name":      "High CPU Alert",
		"condition": "cpu_usage > threshold",
		"threshold": 80.0,
		"channel":   "slack",
		"enabled":   true,
	})
	assertStatus(t, status, http.StatusCreated)
	ruleData := parseData(t, resp)
	assert.Equal(t, "High CPU Alert", ruleData["name"])

	// 3. GET /api/v1/alerts/rules → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/alerts/rules", nil)
	assertStatus(t, status, http.StatusOK)
	rules := parseDataSlice(t, resp)
	assert.GreaterOrEqual(t, len(rules), 1)

	// 4. GET /api/v1/alerts/history → 200
	status, resp = doRequest(t, http.MethodGet, "/api/v1/alerts/history", nil)
	assertStatus(t, status, http.StatusOK)
	require.NotNil(t, resp)
}

// Scenario 6: Health Check
func TestScenario6_HealthCheck(t *testing.T) {
	status, resp := doRequest(t, http.MethodGet, "/health", nil)
	assertStatus(t, status, http.StatusOK)
	require.NotNil(t, resp)
	assert.Equal(t, "healthy", resp["status"])
}
