package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const egovWebSourceJSON = `{
	"repo_url": "https://github.com/eGovFramework/egovframe-web-sample",
	"commit": "158daebd2bfdab951e90ad526c3d2a6b6b42513b",
	"label": "v5.0.0",
	"build_profile": "maven-war"
}`

func TestCICDTemplateHandler_Create_WithSource(t *testing.T) {
	repo := newStatefulTemplateRepository()

	rec := doTemplateRequest(t, repo, http.MethodPost, "/api/v1/cicd/templates", `{
		"id": "egov-web", "name": "eGov Web", "app_type": "backend", "stages": ["Build", "Deploy"],
		"source": `+egovWebSourceJSON+`
	}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	stored := repo.store["egov-web"]
	require.NotNil(t, stored.Source)
	assert.Equal(t, domain.BuildProfileMavenWAR, stored.Source.BuildProfile)
	assert.Equal(t, "v5.0.0", stored.Source.Label)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	source, ok := resp["source"].(map[string]any)
	require.True(t, ok, "응답에 원본이 실려야 화면이 출처를 보여준다: %s", rec.Body.String())
	assert.Equal(t, "158daebd2bfdab951e90ad526c3d2a6b6b42513b", source["commit"])
}

// 태그는 다시 찍힐 수 있다. 커밋 SHA 가 아니면 저장하지 않는다.
func TestCICDTemplateHandler_Create_RejectsTagAsCommit(t *testing.T) {
	repo := newStatefulTemplateRepository()

	rec := doTemplateRequest(t, repo, http.MethodPost, "/api/v1/cicd/templates", `{
		"id": "egov-web", "name": "eGov Web", "app_type": "backend", "stages": ["Build", "Deploy"],
		"source": {"repo_url": "https://github.com/eGovFramework/egovframe-web-sample", "commit": "v5.0.0"}
	}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "CICD_TEMPLATE_INVALID", errorCode(t, rec))
	assert.NotContains(t, repo.store, "egov-web")
}

func TestCICDTemplateHandler_Create_WithoutSourceStaysNil(t *testing.T) {
	repo := newStatefulTemplateRepository()

	rec := doTemplateRequest(t, repo, http.MethodPost, "/api/v1/cicd/templates",
		`{"id": "plain", "name": "Plain", "app_type": "backend", "stages": ["Build", "Deploy"]}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Nil(t, repo.store["plain"].Source)
	assert.NotContains(t, rec.Body.String(), `"source"`, "원본이 없으면 응답에 빈 원본을 싣지 않는다")
}

func egovStoredTemplate() *domain.PipelineTemplate {
	return &domain.PipelineTemplate{
		ID: "egovframe-web-sample-v1", Name: "eGovFrame Web Sample (JSP)", AppType: domain.AppTypeBackend,
		Stages: []string{"Build", "Deploy"},
		Source: &domain.TemplateSource{
			RepoURL: "https://github.com/eGovFramework/egovframe-web-sample",
			Commit:  "158daebd2bfdab951e90ad526c3d2a6b6b42513b", Label: "v5.0.0",
			BuildProfile: domain.BuildProfileMavenWAR,
		},
	}
}

// 템플릿 화면은 원본을 보내지 않는다. 이름만 고쳐도 원본이 지워지면 eGov 템플릿이 빈 스캐폴딩이 된다.
func TestCICDTemplateHandler_Update_KeepsSourceAbsentFromRequest(t *testing.T) {
	repo := newStatefulTemplateRepository(egovStoredTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/egovframe-web-sample-v1",
		`{"name": "eGov Web (renamed)", "app_type": "backend", "stages": ["Build", "Deploy"]}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, repo.store["egovframe-web-sample-v1"].Source)
	assert.Equal(t, "v5.0.0", repo.store["egovframe-web-sample-v1"].Source.Label)
}

func TestCICDTemplateHandler_Update_ReplacesSource(t *testing.T) {
	repo := newStatefulTemplateRepository(egovStoredTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/egovframe-web-sample-v1", `{
		"source": {"repo_url": "https://github.com/eGovFramework/egovframe-web-sample",
		           "commit": "1111111111111111111111111111111111111111", "label": "v5.1.0", "build_profile": "maven-war"}
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "v5.1.0", repo.store["egovframe-web-sample-v1"].Source.Label)
}

// 빈 객체를 보내면 원본을 지운다. null 은 "안 보냄" 과 구분되지 않아 지우는 신호로 쓰지 않는다.
func TestCICDTemplateHandler_Update_EmptySourceClears(t *testing.T) {
	repo := newStatefulTemplateRepository(egovStoredTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/egovframe-web-sample-v1", `{"source": {}}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Nil(t, repo.store["egovframe-web-sample-v1"].Source)
}

// source 일부만 보내면 원본을 지우지 않는다 — label 만 고치려다 원본 전체가 200 응답으로 사라진다.
func TestCICDTemplateHandler_Update_PartialSourceIsRejected(t *testing.T) {
	repo := newStatefulTemplateRepository(egovStoredTemplate())

	rec := doTemplateRequest(t, repo, http.MethodPut, "/api/v1/cicd/templates/egovframe-web-sample-v1",
		`{"source": {"label": "v5.0.1"}}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "CICD_TEMPLATE_INVALID", errorCode(t, rec))
	require.NotNil(t, repo.store["egovframe-web-sample-v1"].Source, "저장된 원본은 그대로다")
	assert.Equal(t, "v5.0.0", repo.store["egovframe-web-sample-v1"].Source.Label)
}

// rejectingArchive 는 github.com 만 받는 원본 아카이브 대역이다.
type rejectingArchive struct{}

func (rejectingArchive) Fetch(context.Context, string, string) ([]port.CommitFile, error) {
	return nil, nil
}

func (rejectingArchive) Supports(repoURL string) error {
	if strings.HasPrefix(repoURL, "https://github.com/") {
		return nil
	}
	return fmt.Errorf("%w: %s", port.ErrSourceUnsupported, repoURL)
}

// 저장은 되는데 파이프라인을 만들 때마다 실패하는 템플릿이 생기지 않게, 저장할 때 가져올 수 있는 원본인지 본다.
func TestCICDTemplateHandler_Create_RejectsUnsupportedSource(t *testing.T) {
	repo := newStatefulTemplateRepository()
	e := echo.New()
	cicdhandler.NewCICDTemplateHandler(repo).WithSourceArchive(rejectingArchive{}).RegisterRoutes(e.Group("/api/v1/cicd"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cicd/templates", strings.NewReader(`{
		"id": "gl", "name": "GitLab source", "app_type": "backend", "stages": ["Build", "Deploy"],
		"source": {"repo_url": "https://gitlab.com/group/project", "commit": "158daebd2bfdab951e90ad526c3d2a6b6b42513b"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "CICD_TEMPLATE_INVALID", errorCode(t, rec))
	assert.Contains(t, rec.Body.String(), "gitlab.com")
	assert.NotContains(t, repo.store, "gl")
}
