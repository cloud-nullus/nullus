package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func doTemplateRequest(t *testing.T, repo *mockTemplateRepository, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := newCICDTemplateEcho(repo)
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	code, _ := resp["error"]["code"].(string)
	return code
}

func seededSampleTemplate() *domain.PipelineTemplate {
	return &domain.PipelineTemplate{
		ID:             "nullus-sample-backend-v1",
		Name:           "Nullus Sample App — Backend",
		Description:    "Go API server",
		AppType:        domain.AppTypeBackend,
		Stages:         []string{"GitClone", "DockerBuild", "ImageLoad", "Deploy"},
		GitRepoURL:     "https://github.com/cloud-nullus/nullus-sample-app",
		DockerfilePath: "backend/Dockerfile",
		DockerContext:  "backend/",
		EnvVars:        map[string]string{"BACKEND_HOST": "sample-backend:8080"},
		CreatedBy:      "seed",
	}
}

func TestCICDTemplateHandler_Create_Success(t *testing.T) {
	repo := newStatefulTemplateRepository()

	rec := doTemplateRequest(t, repo, http.MethodPost, "/api/v1/cicd/templates", `{
		"id": "team-backend-v1", "name": "Team Backend", "description": "팀 표준",
		"app_type": "backend", "stages": ["Build", "Deploy"], "created_by": "alice",
		"git_repo_url": "https://gitlab.example.com/team/backend", "env_vars": {"PORT": "8080"}
	}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var resp domain.PipelineTemplate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "team-backend-v1", resp.ID)
	assert.Equal(t, domain.AppTypeBackend, resp.AppType)
	assert.Equal(t, "alice", resp.CreatedBy)
	assert.Equal(t, "https://gitlab.example.com/team/backend", resp.GitRepoURL)
	assert.Equal(t, map[string]string{"PORT": "8080"}, resp.EnvVars)

	stored, ok := repo.store["team-backend-v1"]
	require.True(t, ok, "template must be handed to the repository")
	assert.Equal(t, []string{"Build", "Deploy"}, stored.Stages)
	assert.Equal(t, "팀 표준", stored.Description)
}

func TestCICDTemplateHandler_Create_InvalidIsRejectedBeforeRepository(t *testing.T) {
	cases := map[string]string{
		"missing name":     `{"id": "x", "app_type": "backend", "stages": ["Build"]}`,
		"unknown app type": `{"id": "x", "name": "X", "app_type": "web-backend", "stages": ["Build"]}`,
		"no stages":        `{"id": "x", "name": "X", "app_type": "backend", "stages": []}`,
		"missing id":       `{"name": "X", "app_type": "backend", "stages": ["Build"]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newStatefulTemplateRepository()
			rec := doTemplateRequest(t, repo, http.MethodPost, "/api/v1/cicd/templates", body)

			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Equal(t, "CICD_TEMPLATE_INVALID", errorCode(t, rec))
			assert.Empty(t, repo.store)
		})
	}
}

func TestCICDTemplateHandler_Create_DuplicateIDIsConflict(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPost, "/api/v1/cicd/templates",
		`{"id": "nullus-sample-backend-v1", "name": "Overwrite", "app_type": "web", "stages": ["Build"]}`)

	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "CICD_TEMPLATE_ALREADY_EXISTS", errorCode(t, rec))
	assert.Equal(t, "Nullus Sample App — Backend", repo.store["nullus-sample-backend-v1"].Name)
}

func TestCICDTemplateHandler_Create_RepositoryFailure(t *testing.T) {
	repo := newStatefulTemplateRepository()
	repo.createErr = errors.New("connection refused")

	rec := doTemplateRequest(t, repo, http.MethodPost, "/api/v1/cicd/templates",
		`{"id": "x", "name": "X", "app_type": "backend", "stages": ["Build"]}`)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "CICD_TEMPLATE_CREATE_FAILED", errorCode(t, rec))
}

// 화면은 이름·설명·유형·단계만 보낸다. 빌드 설정(git_repo_url·dockerfile_path·env_vars)과 만든 사람은
// 요청에 없으면 저장된 값을 그대로 둬야 한다 — 시드 템플릿의 이름만 고쳤는데 Dockerfile 경로가 지워지면 안 된다.
func TestCICDTemplateHandler_Update_KeepsFieldsAbsentFromRequest(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/nullus-sample-backend-v1",
		`{"name": "Sample Backend (renamed)", "description": "renamed", "app_type": "backend", "stages": ["CI", "CD"]}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stored := repo.store["nullus-sample-backend-v1"]
	assert.Equal(t, "Sample Backend (renamed)", stored.Name)
	assert.Equal(t, "renamed", stored.Description)
	assert.Equal(t, []string{"CI", "CD"}, stored.Stages)
	assert.Equal(t, "https://github.com/cloud-nullus/nullus-sample-app", stored.GitRepoURL)
	assert.Equal(t, "backend/Dockerfile", stored.DockerfilePath)
	assert.Equal(t, "backend/", stored.DockerContext)
	assert.Equal(t, map[string]string{"BACKEND_HOST": "sample-backend:8080"}, stored.EnvVars)
	assert.Equal(t, "seed", stored.CreatedBy, "creator is not rewritten on update")

	var resp domain.PipelineTemplate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "backend/Dockerfile", resp.DockerfilePath, "response shows the merged template")
}

func TestCICDTemplateHandler_Update_AppliesBuildConfigWhenSent(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/nullus-sample-backend-v1",
		`{"name": "Sample Backend", "app_type": "web", "stages": ["Build"],
		  "git_repo_url": "", "dockerfile_path": "Dockerfile", "env_vars": {}}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stored := repo.store["nullus-sample-backend-v1"]
	assert.Equal(t, domain.AppTypeWeb, stored.AppType)
	assert.Equal(t, "", stored.GitRepoURL, "explicit empty string clears the field")
	assert.Equal(t, "Dockerfile", stored.DockerfilePath)
	assert.Equal(t, "backend/", stored.DockerContext, "field absent from request is kept")
	assert.Empty(t, stored.EnvVars, "explicit empty object clears env vars")
}

func TestCICDTemplateHandler_Update_PathIDWins(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/nullus-sample-backend-v1",
		`{"id": "other-id", "name": "Renamed", "app_type": "backend", "stages": ["Build"]}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "Renamed", repo.store["nullus-sample-backend-v1"].Name)
	_, created := repo.store["other-id"]
	assert.False(t, created)
}

func TestCICDTemplateHandler_Update_NotFound(t *testing.T) {
	repo := newStatefulTemplateRepository()

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/missing",
		`{"name": "X", "app_type": "backend", "stages": ["Build"]}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "CICD_TEMPLATE_NOT_FOUND", errorCode(t, rec))
	assert.Empty(t, repo.store, "update must not create")
}

func TestCICDTemplateHandler_Update_Invalid(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/nullus-sample-backend-v1",
		`{"name": "X", "app_type": "web-backend", "stages": ["Build"]}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "CICD_TEMPLATE_INVALID", errorCode(t, rec))
	assert.Equal(t, domain.AppTypeBackend, repo.store["nullus-sample-backend-v1"].AppType)
}

func TestCICDTemplateHandler_Update_RepositoryFailure(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())
	repo.updateErr = errors.New("connection refused")

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/nullus-sample-backend-v1",
		`{"name": "X", "app_type": "backend", "stages": ["Build"]}`)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "CICD_TEMPLATE_UPDATE_FAILED", errorCode(t, rec))
}

func TestCICDTemplateHandler_Delete_Success(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())

	rec := doTemplateRequest(t, repo, http.MethodDelete, "/api/v1/cicd/templates/nullus-sample-backend-v1", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, repo.store)
}

func TestCICDTemplateHandler_Delete_NotFound(t *testing.T) {
	repo := newStatefulTemplateRepository()

	rec := doTemplateRequest(t, repo, http.MethodDelete, "/api/v1/cicd/templates/missing", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "CICD_TEMPLATE_NOT_FOUND", errorCode(t, rec))
}

func TestCICDTemplateHandler_Delete_RepositoryFailure(t *testing.T) {
	repo := newStatefulTemplateRepository(seededSampleTemplate())
	repo.deleteErr = errors.New("connection refused")

	rec := doTemplateRequest(t, repo, http.MethodDelete, "/api/v1/cicd/templates/nullus-sample-backend-v1", "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "CICD_TEMPLATE_DELETE_FAILED", errorCode(t, rec))
}

func TestCICDTemplateHandler_Get_NotFound(t *testing.T) {
	repo := newStatefulTemplateRepository()

	rec := doTemplateRequest(t, repo, http.MethodGet, "/api/v1/cicd/templates/missing", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "CICD_TEMPLATE_NOT_FOUND", errorCode(t, rec))
}
