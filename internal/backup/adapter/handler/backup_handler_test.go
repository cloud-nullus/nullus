package handler

import (
	"context"
	"encoding/json"
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
	rec := post(e, "/api/v1/restores", `{"backup_run_id":"b1","confirm":"b1"}`)
	assert.True(t, f.restoreCalled)
	assert.Equal(t, http.StatusTeapot, rec.Code)
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
