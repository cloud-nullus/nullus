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
)

// stubGoldenPathRepository 는 저장 규칙(중복 거부·없으면 not found)을 실제 저장소와 같게 흉내낸다.
// 핸들러가 그 에러를 어떤 상태 코드로 옮기는지가 이 테스트의 관심사다.
type stubGoldenPathRepository struct {
	order   []string
	store   map[string]*domain.CICDGoldenPath
	listErr error
}

func newStubGoldenPathRepository(seed ...*domain.CICDGoldenPath) *stubGoldenPathRepository {
	r := &stubGoldenPathRepository{store: map[string]*domain.CICDGoldenPath{}}
	for _, gp := range seed {
		copied := *gp
		r.store[gp.ID] = &copied
		r.order = append(r.order, gp.ID)
	}
	return r
}

func (r *stubGoldenPathRepository) GetByID(_ context.Context, id string) (*domain.CICDGoldenPath, error) {
	gp, ok := r.store[id]
	if !ok {
		return nil, fmt.Errorf("CI/CD golden path %q: %w", id, domain.ErrGoldenPathNotFound)
	}
	copied := *gp
	return &copied, nil
}

func (r *stubGoldenPathRepository) List(_ context.Context) ([]*domain.CICDGoldenPath, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]*domain.CICDGoldenPath, 0, len(r.order))
	for _, id := range r.order {
		copied := *r.store[id]
		out = append(out, &copied)
	}
	return out, nil
}

func (r *stubGoldenPathRepository) Create(_ context.Context, gp *domain.CICDGoldenPath) error {
	if _, ok := r.store[gp.ID]; ok {
		return fmt.Errorf("CI/CD golden path %q: %w", gp.ID, domain.ErrGoldenPathAlreadyExists)
	}
	copied := *gp
	r.store[gp.ID] = &copied
	r.order = append(r.order, gp.ID)
	return nil
}

func (r *stubGoldenPathRepository) Update(_ context.Context, gp *domain.CICDGoldenPath) error {
	if _, ok := r.store[gp.ID]; !ok {
		return fmt.Errorf("CI/CD golden path %q: %w", gp.ID, domain.ErrGoldenPathNotFound)
	}
	copied := *gp
	r.store[gp.ID] = &copied
	return nil
}

func (r *stubGoldenPathRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.store[id]; !ok {
		return fmt.Errorf("CI/CD golden path %q: %w", id, domain.ErrGoldenPathNotFound)
	}
	delete(r.store, id)
	for i, existing := range r.order {
		if existing == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

func goldenPathServer(repo *stubGoldenPathRepository) *echo.Echo {
	e := echo.New()
	cicdhandler.NewCICDGoldenPathHandler(repo).RegisterRoutes(e.Group("/api/v1/cicd"))
	return e
}

func callGoldenPath(t *testing.T, e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seededGoldenPath() *domain.CICDGoldenPath {
	return &domain.CICDGoldenPath{
		ID:                   "gitlab-allinone-v1",
		Name:                 "GitLab All-in-One",
		EstimatedInstallTime: 90,
		Tools: []domain.CICDTool{
			{Category: "source_repository", Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
		},
	}
}

// 목록은 저장소가 준 순서를 그대로 내보낸다. 카드 순서가 요청마다 바뀌면 고른 것을 다시 찾기 어렵다.
func TestCICDGoldenPathHandler_ListKeepsRepositoryOrder(t *testing.T) {
	repo := newStubGoldenPathRepository(
		&domain.CICDGoldenPath{ID: "gitlab-allinone-v1", Name: "GitLab All-in-One"},
		&domain.CICDGoldenPath{ID: "gitlab-argocd-v1", Name: "GitLab + Argo CD"},
		&domain.CICDGoldenPath{ID: "github-argocd-v1", Name: "GitHub + Argo CD"},
	)

	rec := callGoldenPath(t, goldenPathServer(repo), http.MethodGet, "/api/v1/cicd/golden-paths", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var got []domain.CICDGoldenPath
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 3)
	assert.Equal(t, "gitlab-allinone-v1", got[0].ID)
	assert.Equal(t, "gitlab-argocd-v1", got[1].ID)
	assert.Equal(t, "github-argocd-v1", got[2].ID)
}

// 만든 골든패스는 같은 응답으로 돌아오고 목록에도 바로 들어간다.
func TestCICDGoldenPathHandler_CreateThenList(t *testing.T) {
	repo := newStubGoldenPathRepository(seededGoldenPath())
	e := goldenPathServer(repo)

	body := `{"id":"team-gitops-v1","name":"Team GitOps","estimated_install_time":75,
		"tools":[{"category":"cd_tool","name":"Argo CD","helm_version":"7.7.2","app_version":"2.13.2"}]}`
	rec := callGoldenPath(t, e, http.MethodPost, "/api/v1/cicd/golden-paths", body)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created domain.CICDGoldenPath
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "team-gitops-v1", created.ID)
	require.Len(t, created.Tools, 1)
	assert.Equal(t, "Argo CD", created.Tools[0].Name)

	rec = callGoldenPath(t, e, http.MethodGet, "/api/v1/cicd/golden-paths", "")
	var listed []domain.CICDGoldenPath
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	assert.Len(t, listed, 2)
}

// 같은 ID 로 또 만들면 409 다. 200·201 로 응답하면 화면은 덮어쓴 줄 모른다.
func TestCICDGoldenPathHandler_CreateDuplicateIsConflict(t *testing.T) {
	repo := newStubGoldenPathRepository(seededGoldenPath())

	rec := callGoldenPath(t, goldenPathServer(repo), http.MethodPost, "/api/v1/cicd/golden-paths",
		`{"id":"gitlab-allinone-v1","name":"덮어쓰기 시도"}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "CICD_GOLDEN_PATH_ALREADY_EXISTS")
	assert.Equal(t, "GitLab All-in-One", repo.store["gitlab-allinone-v1"].Name)
}

// ID 가 없으면 400 이다. 빈 ID 로 들어오면 저장소가 빈 키 행을 만든다.
func TestCICDGoldenPathHandler_CreateWithoutIDIsBadRequest(t *testing.T) {
	repo := newStubGoldenPathRepository()

	rec := callGoldenPath(t, goldenPathServer(repo), http.MethodPost, "/api/v1/cicd/golden-paths",
		`{"name":"ID 없음"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "CICD_GOLDEN_PATH_INVALID")
	assert.Empty(t, repo.store)
}

// 이름이 없으면 400 이다. 카드에 제목이 없으면 고를 수 없다.
func TestCICDGoldenPathHandler_CreateWithoutNameIsBadRequest(t *testing.T) {
	repo := newStubGoldenPathRepository()

	rec := callGoldenPath(t, goldenPathServer(repo), http.MethodPost, "/api/v1/cicd/golden-paths",
		`{"id":"no-name-v1"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, repo.store)
}

// 없는 것을 고치거나 지우면 404 다.
func TestCICDGoldenPathHandler_MissingIsNotFound(t *testing.T) {
	e := goldenPathServer(newStubGoldenPathRepository(seededGoldenPath()))

	for _, tc := range []struct{ name, method, body string }{
		{"조회", http.MethodGet, ""},
		{"수정", http.MethodPut, `{"name":"바꿈"}`},
		{"삭제", http.MethodDelete, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := callGoldenPath(t, e, tc.method, "/api/v1/cicd/golden-paths/no-such-path", tc.body)
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Contains(t, rec.Body.String(), "CICD_GOLDEN_PATH_NOT_FOUND")
		})
	}
}

// 고친 뒤에는 고친 값이 조회된다.
func TestCICDGoldenPathHandler_UpdatePersists(t *testing.T) {
	e := goldenPathServer(newStubGoldenPathRepository(seededGoldenPath()))

	rec := callGoldenPath(t, e, http.MethodPut, "/api/v1/cicd/golden-paths/gitlab-allinone-v1",
		`{"name":"GitLab All-in-One (사내)","estimated_install_time":100}`)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = callGoldenPath(t, e, http.MethodGet, "/api/v1/cicd/golden-paths/gitlab-allinone-v1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got domain.CICDGoldenPath
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "GitLab All-in-One (사내)", got.Name)
	assert.Equal(t, 100, got.EstimatedInstallTime)
}

// 저장소가 터지면 500 이다 — 빈 배열로 숨기면 화면은 "골든패스가 없다" 로 읽는다.
func TestCICDGoldenPathHandler_ListErrorIsServerError(t *testing.T) {
	repo := newStubGoldenPathRepository()
	repo.listErr = fmt.Errorf("connection refused")

	rec := callGoldenPath(t, goldenPathServer(repo), http.MethodGet, "/api/v1/cicd/golden-paths", "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "CICD_GOLDEN_PATH_LIST_FAILED")
}

// 쓰기 경로에만 가드가 붙는다. DB 로 옮긴 뒤 삭제가 영구적이 되었으므로,
// 읽기는 열어 두고 바꾸는 쪽만 좁힐 수 있어야 한다.
func TestCICDGoldenPathHandler_WriteGuardsApplyToWritesOnly(t *testing.T) {
	repo := newStubGoldenPathRepository(seededGoldenPath())

	denied := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return c.JSON(http.StatusForbidden, map[string]string{"code": "FORBIDDEN"})
		}
	}

	e := echo.New()
	cicdhandler.NewCICDGoldenPathHandler(repo).RegisterRoutes(e.Group("/api/v1/cicd"), denied)

	// 읽기는 가드를 거치지 않는다.
	rec := callGoldenPath(t, e, http.MethodGet, "/api/v1/cicd/golden-paths", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = callGoldenPath(t, e, http.MethodGet, "/api/v1/cicd/golden-paths/gitlab-allinone-v1", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	// 쓰기는 가드에 막힌다 — 그리고 저장소에 닿지 않는다.
	for _, tc := range []struct{ name, method, target, body string }{
		{"생성", http.MethodPost, "/api/v1/cicd/golden-paths", `{"id":"x-v1","name":"X"}`},
		{"수정", http.MethodPut, "/api/v1/cicd/golden-paths/gitlab-allinone-v1", `{"name":"바꿈"}`},
		{"삭제", http.MethodDelete, "/api/v1/cicd/golden-paths/gitlab-allinone-v1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := callGoldenPath(t, e, tc.method, tc.target, tc.body)
			assert.Equal(t, http.StatusForbidden, rec.Code)
		})
	}

	assert.Len(t, repo.store, 1)
	assert.Equal(t, "GitLab All-in-One", repo.store["gitlab-allinone-v1"].Name)
}

// 가드를 넘기지 않으면 종전처럼 모두 열린다 — development 모드의 동작이다.
func TestCICDGoldenPathHandler_NoGuardsKeepsWritesOpen(t *testing.T) {
	repo := newStubGoldenPathRepository(seededGoldenPath())

	rec := callGoldenPath(t, goldenPathServer(repo), http.MethodDelete,
		"/api/v1/cicd/golden-paths/gitlab-allinone-v1", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, repo.store)
}
