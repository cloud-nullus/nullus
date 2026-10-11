package handler

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

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/usecase"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	sharedmw "github.com/cloud-nullus/draft/internal/shared/middleware"
)

type stubRepo struct {
	run     *domain.BackupRun
	runs    []*domain.BackupRun
	arts    []*domain.Artifact
	restore *domain.RestoreRun
	getErr  error
}

func (s *stubRepo) ListRuns(context.Context, string, int) ([]*domain.BackupRun, error) {
	return s.runs, nil
}
func (s *stubRepo) GetRun(_ context.Context, id string) (*domain.BackupRun, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.run, nil
}
func (s *stubRepo) ListArtifacts(context.Context, string) ([]*domain.Artifact, error) {
	return s.arts, nil
}
func (s *stubRepo) GetRestore(context.Context, string) (*domain.RestoreRun, error) {
	return s.restore, nil
}

// stubFactory 는 유스케이스를 만들지 않고 호출 여부만 기록한다.
// 확인 절차가 유스케이스에 닿기 전에 막는지 보는 것이 목적이다.
type stubFactory struct {
	backupCalled  bool
	restoreCalled bool
}

func (f *stubFactory) Backup(context.Context, string) (*usecase.BackupUseCase, error) {
	f.backupCalled = true
	return nil, &shareddomain.AppError{
		Code: "STUB", HTTPStatus: http.StatusTeapot, Message: "유스케이스까지 도달했다",
	}
}
func (f *stubFactory) Restore(context.Context, string) (*usecase.RestoreUseCase, error) {
	f.restoreCalled = true
	return nil, &shareddomain.AppError{
		Code: "STUB", HTTPStatus: http.StatusTeapot, Message: "유스케이스까지 도달했다",
	}
}

func newTestHandler(repo *stubRepo) (*echo.Echo, *BackupHandler, *stubFactory) {
	e := echo.New()
	// 실제 서버와 같은 에러 변환을 쓴다 — AppError 의 HTTPStatus 가 응답
	// 코드로 나가는지까지 봐야 확인 절차가 실제로 400 을 내는지 검증된다.
	e.HTTPErrorHandler = sharedmw.AppErrorHandler
	f := &stubFactory{}
	h := NewBackupHandler(f, nil, repo, "nullus")
	h.RegisterRoutes(e.Group("/api/v1"))
	return e, h, f
}

func post(e *echo.Echo, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// 설계 §7.2 — 백업이 다운타임을 만든다는 사실을 모르고 누르면 안 된다.
func TestCreateBackup_정지창이_생기면_확인을_요구한다(t *testing.T) {
	e, _, f := newTestHandler(&stubRepo{})

	rec := post(e, "/api/v1/backups", `{"mode":"full","namespace":"nullus"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "BACKUP_CONFIRM_REQUIRED")
	assert.False(t, f.backupCalled, "확인 전에 유스케이스를 부르면 안 된다")
}

func TestCreateBackup_확인이_맞으면_진행한다(t *testing.T) {
	e, _, f := newTestHandler(&stubRepo{})

	rec := post(e, "/api/v1/backups", `{"mode":"full","namespace":"nullus","confirm":"nullus"}`)

	assert.True(t, f.backupCalled)
	assert.Equal(t, http.StatusTeapot, rec.Code, "확인을 통과해 유스케이스까지 갔다")
}

func TestCreateBackup_platform_only_는_확인이_필요없다(t *testing.T) {
	// 볼륨을 뜨지 않으므로 정지 창이 없다 — 무중단이다.
	e, _, f := newTestHandler(&stubRepo{})

	rec := post(e, "/api/v1/backups", `{"mode":"platform_only"}`)

	assert.True(t, f.backupCalled)
	assert.Equal(t, http.StatusTeapot, rec.Code)
}

func TestCreateBackup_기본_네임스페이스로_확인한다(t *testing.T) {
	e, _, _ := newTestHandler(&stubRepo{})
	rec := post(e, "/api/v1/backups", `{"mode":"full","confirm":"nullus"}`)
	assert.Equal(t, http.StatusTeapot, rec.Code)
}

// 설계 §7.2 — 복구는 파괴적이다.
func TestCreateRestore_확인이_없으면_거부한다(t *testing.T) {
	e, _, f := newTestHandler(&stubRepo{run: &domain.BackupRun{ID: "b1"}})

	rec := post(e, "/api/v1/restores", `{"backup_run_id":"b1","mode":"full"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "RESTORE_CONFIRM_REQUIRED")
	assert.False(t, f.restoreCalled, "확인 전에 유스케이스를 부르면 안 된다")
}

func TestCreateRestore_확인이_틀리면_거부한다(t *testing.T) {
	e, _, f := newTestHandler(&stubRepo{run: &domain.BackupRun{ID: "b1"}})
	rec := post(e, "/api/v1/restores", `{"backup_run_id":"b1","confirm":"b2"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, f.restoreCalled)
}

func TestCreateRestore_backup_run_id_가_없으면_거부한다(t *testing.T) {
	e, _, _ := newTestHandler(&stubRepo{})
	rec := post(e, "/api/v1/restores", `{"confirm":"x"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateRestore_확인이_맞으면_진행한다(t *testing.T) {
	e, _, f := newTestHandler(&stubRepo{run: &domain.BackupRun{ID: "b1", StackID: "s1"}})
	rec := post(e, "/api/v1/restores", `{"backup_run_id":"b1","namespace":"nullus-gitlab","confirm":"b1"}`)
	assert.True(t, f.restoreCalled)
	assert.Equal(t, http.StatusTeapot, rec.Code)
}

// 스택 백업을 스택까지 되돌리는데 네임스페이스가 없으면 묻는다. 플랫폼 기본 네임스페이스로 떨어뜨리면
// 복구가 그 이름으로 네임스페이스를 만들고 거기에 볼륨·리소스를 풀어 놓는다 — 스택과 무관한 곳이다.
func TestCreateRestore_스택_백업인데_네임스페이스가_없으면_거부한다(t *testing.T) {
	for _, mode := range []string{"", "full", "stack_only"} {
		e, _, f := newTestHandler(&stubRepo{run: &domain.BackupRun{ID: "b1", StackID: "s1"}})
		rec := post(e, "/api/v1/restores", `{"backup_run_id":"b1","mode":"`+mode+`","confirm":"b1"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "mode=%q", mode)
		assert.Contains(t, rec.Body.String(), "RESTORE_NAMESPACE_REQUIRED")
		assert.False(t, f.restoreCalled)
	}
}

// 플랫폼만 되돌리거나 스택 없는 백업이면 네임스페이스를 쓰지 않는다 — 묻지 않는다.
func TestCreateRestore_스택을_건드리지_않으면_네임스페이스를_묻지_않는다(t *testing.T) {
	e, _, f := newTestHandler(&stubRepo{run: &domain.BackupRun{ID: "b1", StackID: "s1"}})
	rec := post(e, "/api/v1/restores", `{"backup_run_id":"b1","mode":"platform_only","confirm":"b1"}`)
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.True(t, f.restoreCalled)

	e, _, f = newTestHandler(&stubRepo{run: &domain.BackupRun{ID: "b1"}})
	rec = post(e, "/api/v1/restores", `{"backup_run_id":"b1","mode":"full","confirm":"b1"}`)
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.True(t, f.restoreCalled)
}

func TestGetBackup_매니페스트와_산출물을_함께_준다(t *testing.T) {
	run := domain.NewBackupRun("org-1", domain.ModeFull, domain.TriggerManual, nil)
	run.ID = "b1"
	run.Manifest = map[string]any{"schema_version": 74}
	repo := &stubRepo{run: run, arts: []*domain.Artifact{
		{Component: domain.ComponentPlatformDB, Location: "s3://b/x", ChecksumSHA256: "abc", SizeBytes: 10},
	}}
	e, _, _ := newTestHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/b1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body backupRunResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "b1", body.ID)
	require.Len(t, body.Artifacts, 1)
	assert.Equal(t, "abc", body.Artifacts[0].ChecksumSHA256)
	assert.NotEmpty(t, body.Manifest)
}

func TestListBackups_목록에는_매니페스트를_싣지_않는다(t *testing.T) {
	// 목록에 매니페스트를 다 실으면 응답이 커지고, 대부분의 화면은 쓰지 않는다.
	run := domain.NewBackupRun("org-1", domain.ModeFull, domain.TriggerManual, nil)
	run.ID = "b1"
	run.Manifest = map[string]any{"schema_version": 74}
	e, _, _ := newTestHandler(&stubRepo{runs: []*domain.BackupRun{run}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Items []backupRunResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Nil(t, body.Items[0].Manifest)
}

func TestToResponse_정지창_시간을_노출한다(t *testing.T) {
	// 사용자가 실제로 감수한 다운타임이다. 이력에 남아야 정지 창 정책을
	// 조정할 근거가 생긴다 (§7.1).
	run := domain.NewBackupRun("o", domain.ModeFull, domain.TriggerManual, nil)
	require.NoError(t, run.Start())
	run.BeginQuiesce()
	run.EndQuiesce()

	res := toResponse(run, nil, false)
	assert.GreaterOrEqual(t, res.QuiesceSeconds, float64(0))
	assert.NotNil(t, res.StartedAt)
}

func TestCreateBackup_볼륨을_빼면_확인이_필요없다(t *testing.T) {
	// full 모드라도 고른 대상에 볼륨이 없으면 멈추지 않는다. 멈추지도 않는데
	// 확인을 강요하면 그 문구는 의미 없는 절차가 되고, 정작 진짜 멈추는
	// 백업에서도 사용자가 습관적으로 넘긴다.
	e, _, f := newTestHandler(&stubRepo{})

	rec := post(e, "/api/v1/backups",
		`{"mode":"full","namespace":"nullus","scope":["platform_db","keycloak_db"]}`)

	assert.True(t, f.backupCalled)
	assert.Equal(t, http.StatusTeapot, rec.Code)
}

func TestCreateBackup_볼륨을_고르면_확인을_요구한다(t *testing.T) {
	// 모드가 stack_only 든 full 이든, 볼륨이 들어가면 서비스가 멈춘다.
	e, _, f := newTestHandler(&stubRepo{})

	rec := post(e, "/api/v1/backups",
		`{"mode":"stack_only","namespace":"nullus","scope":["volume"]}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "BACKUP_CONFIRM_REQUIRED")
	assert.False(t, f.backupCalled)
}

func TestCreateBackup_모르는_대상은_거부한다(t *testing.T) {
	// 조용히 버리면 고른 것과 백업된 것이 달라지고, 그 사실은 복구할 때
	// 드러난다 — 그때는 늦다.
	e, _, f := newTestHandler(&stubRepo{})

	rec := post(e, "/api/v1/backups",
		`{"mode":"full","namespace":"nullus","confirm":"nullus","scope":["platform_db","없는것"]}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "BACKUP_INVALID_COMPONENT")
	assert.False(t, f.backupCalled)
}

// ── 조직 해석 ────────────────────────────────────────────────────────────
//
// 백업은 조직 단위로 보인다 — 목록은 로그인 사용자의 조직으로만 거른다. 헤더에
// 적은 조직이 없는 조직이면 백업은 만들어지되 어떤 화면에도 보이지 않는다.
// 실제로 그랬다(문서의 유령 조직 00000000-…-0001).

type stubOrgChecker struct {
	exists bool
	err    error
	asked  []string
}

func (s *stubOrgChecker) OrganizationExists(_ context.Context, id string) (bool, error) {
	s.asked = append(s.asked, id)
	return s.exists, s.err
}

// recordingRepo 는 어느 조직으로 목록을 물었는지 남긴다.
type recordingRepo struct {
	stubRepo
	listedOrg string
}

func (r *recordingRepo) ListRuns(_ context.Context, orgID string, _ int) ([]*domain.BackupRun, error) {
	r.listedOrg = orgID
	return r.runs, nil
}

func newOrgTestHandler(repo *recordingRepo, orgs *stubOrgChecker) (*echo.Echo, *stubFactory) {
	e := echo.New()
	e.HTTPErrorHandler = sharedmw.AppErrorHandler
	f := &stubFactory{}
	h := NewBackupHandler(f, nil, repo, "nullus")
	if orgs != nil {
		h.WithOrganizationChecker(orgs)
	}
	h.RegisterRoutes(e.Group("/api/v1"))
	return e, f
}

func getWithOrg(e *echo.Echo, path, org string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if org != "" {
		req.Header.Set("X-Org-ID", org)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func postWithOrg(e *echo.Echo, path, body, org string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if org != "" {
		req.Header.Set("X-Org-ID", org)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// 인증이 꺼진 개발 모드에는 principal 도 헤더도 없다. 다른 핸들러처럼 시드
// 기본 조직으로 떨어져야 한다 — 빈 조직으로 UUID 오류를 내던 것이 종전이다.
func TestResolveOrgID_아무것도_없으면_기본_조직이다(t *testing.T) {
	repo := &recordingRepo{}
	e, _ := newOrgTestHandler(repo, &stubOrgChecker{exists: true})

	rec := getWithOrg(e, "/api/v1/backups", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, shareddomain.DefaultOrgID(), repo.listedOrg)
}

func TestResolveOrgID_없는_조직_헤더는_거부한다(t *testing.T) {
	repo := &recordingRepo{}
	orgs := &stubOrgChecker{exists: false}
	e, f := newOrgTestHandler(repo, orgs)
	ghost := "00000000-0000-0000-0000-000000000001"

	rec := postWithOrg(e, "/api/v1/backups", `{"mode":"platform_only"}`, ghost)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "ORG_NOT_FOUND")
	assert.Contains(t, rec.Body.String(), ghost)
	assert.False(t, f.backupCalled, "없는 조직으로 백업을 만들면 안 된다")
	assert.Equal(t, []string{ghost}, orgs.asked)

	list := getWithOrg(e, "/api/v1/backups", ghost)
	assert.Equal(t, http.StatusBadRequest, list.Code, "목록도 같은 규칙이다")
}

func TestResolveOrgID_있는_조직_헤더는_통과한다(t *testing.T) {
	repo := &recordingRepo{}
	e, f := newOrgTestHandler(repo, &stubOrgChecker{exists: true})

	rec := postWithOrg(e, "/api/v1/backups", `{"mode":"platform_only"}`, "org-real")
	assert.Equal(t, http.StatusTeapot, rec.Code, "유스케이스까지 간다")
	assert.True(t, f.backupCalled)

	list := getWithOrg(e, "/api/v1/backups", "org-real")
	assert.Equal(t, http.StatusOK, list.Code)
	assert.Equal(t, "org-real", repo.listedOrg)
}

// 조직 확인이 실패하면 거절도 통과도 아니다 — 조회 오류를 그대로 알린다.
// 통과시키면 유령 조직이 다시 새고, 거절하면 DB 장애가 "조직 없음" 으로 보인다.
func TestResolveOrgID_조직_확인_실패는_오류로_알린다(t *testing.T) {
	repo := &recordingRepo{}
	e, f := newOrgTestHandler(repo, &stubOrgChecker{err: errors.New("db down")})

	rec := postWithOrg(e, "/api/v1/backups", `{"mode":"platform_only"}`, "org-real")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "ORG_NOT_FOUND")
	assert.False(t, f.backupCalled)
}

// 인증된 사용자의 조직이 헤더보다 앞선다 — 헤더는 호출자가 적는 값이다.
func TestResolveOrgID_principal_이_헤더보다_앞선다(t *testing.T) {
	repo := &recordingRepo{}
	orgs := &stubOrgChecker{exists: false}
	e := echo.New()
	e.HTTPErrorHandler = sharedmw.AppErrorHandler
	h := NewBackupHandler(&stubFactory{}, nil, repo, "nullus").WithOrganizationChecker(orgs)
	g := e.Group("/api/v1", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("current_user", &struct{ OrgID string }{OrgID: "org-principal"})
			return next(c)
		}
	})
	h.RegisterRoutes(g)

	rec := getWithOrg(e, "/api/v1/backups", "00000000-0000-0000-0000-000000000001")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "org-principal", repo.listedOrg)
	assert.Empty(t, orgs.asked, "principal 의 조직은 헤더 검증을 타지 않는다")
}

// 확인기가 배선되지 않은 구성(테스트·옛 조립)은 종전대로 헤더를 믿는다.
func TestResolveOrgID_확인기가_없으면_헤더를_믿는다(t *testing.T) {
	repo := &recordingRepo{}
	e, _ := newOrgTestHandler(repo, nil)

	rec := getWithOrg(e, "/api/v1/backups", "org-any")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "org-any", repo.listedOrg)
}
