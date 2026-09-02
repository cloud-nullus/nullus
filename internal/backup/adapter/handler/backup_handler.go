// Package handler 는 백업/복구 HTTP 어댑터다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §7.2 (nullus-plan#75)
package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/usecase"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// UseCaseFactory 는 스택별 유스케이스를 만든다.
//
// 백업 대상 스택은 등록된 워크로드 클러스터에 있으므로, 어댑터를 기동
// 시점에 고정할 수 없다. 요청이 온 뒤에야 어느 클러스터인지 알 수 있다.
type UseCaseFactory interface {
	Backup(ctx context.Context, stackID string) (*usecase.BackupUseCase, error)
	Restore(ctx context.Context, stackID string) (*usecase.RestoreUseCase, error)
}

type BackupHandler struct {
	factory   UseCaseFactory
	verify    *usecase.VerifyUseCase
	repo      backupReader
	defaultNS string
}

type backupReader interface {
	ListRuns(ctx context.Context, orgID string, limit int) ([]*domain.BackupRun, error)
	GetRun(ctx context.Context, id string) (*domain.BackupRun, error)
	ListArtifacts(ctx context.Context, backupRunID string) ([]*domain.Artifact, error)
	GetRestore(ctx context.Context, id string) (*domain.RestoreRun, error)
}

func NewBackupHandler(f UseCaseFactory, v *usecase.VerifyUseCase, repo backupReader, defaultNS string) *BackupHandler {
	return &BackupHandler{factory: f, verify: v, repo: repo, defaultNS: defaultNS}
}

func (h *BackupHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/backups", h.CreateBackup)
	g.GET("/backups", h.ListBackups)
	g.GET("/backups/:id", h.GetBackup)
	g.POST("/backups/:id/verify", h.VerifyBackup)
	g.POST("/restores", h.CreateRestore)
	g.GET("/restores/:id", h.GetRestore)
}

type createBackupRequest struct {
	StackID   string `json:"stack_id"`
	Namespace string `json:"namespace"`
	Mode      string `json:"mode"`
	// Scope 는 사용자가 고른 백업 대상이다 (platform_db·keycloak_db·openbao_kv·
	// ns_resources·volume). 비우면 모드에서 파생한다 — 스케줄과 옛 호출부의 경로다.
	Scope []string `json:"scope,omitempty"`
	// Confirm 은 대상 식별자 재입력이다.
	//
	// 백업이 정지 창을 만든다는 사실을 모르고 누르면 안 된다 — 볼륨을 뜨는
	// 모드에서만 요구한다 (§7.2).
	Confirm string `json:"confirm"`
}

type backupRunResponse struct {
	ID             string          `json:"id"`
	OrgID          string          `json:"org_id"`
	StackID        string          `json:"stack_id,omitempty"`
	Mode           string          `json:"mode"`
	Trigger        string          `json:"trigger"`
	Status         string          `json:"status"`
	SchemaVersion  int             `json:"schema_version"`
	TotalBytes     int64           `json:"total_bytes"`
	Error          string          `json:"error,omitempty"`
	QuiesceSeconds float64         `json:"quiesce_seconds,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	FinishedAt     *time.Time      `json:"finished_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	Manifest       map[string]any  `json:"manifest,omitempty"`
	Artifacts      []artifactBrief `json:"artifacts,omitempty"`
}

type artifactBrief struct {
	Component      string `json:"component"`
	ResourceName   string `json:"resource_name,omitempty"`
	Location       string `json:"location"`
	SizeBytes      int64  `json:"size_bytes"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

func toResponse(run *domain.BackupRun, arts []*domain.Artifact, withManifest bool) backupRunResponse {
	res := backupRunResponse{
		ID: run.ID, OrgID: run.OrgID, StackID: run.StackID,
		Mode: string(run.Mode), Trigger: string(run.Trigger), Status: string(run.Status),
		SchemaVersion: run.SchemaVersion, TotalBytes: run.TotalBytes, Error: run.Error,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CreatedAt: run.CreatedAt,
	}
	if run.QuiesceStartedAt != nil && run.QuiesceEndedAt != nil {
		res.QuiesceSeconds = run.QuiesceEndedAt.Sub(*run.QuiesceStartedAt).Seconds()
	}
	if withManifest {
		res.Manifest = run.Manifest
	}
	for _, a := range arts {
		res.Artifacts = append(res.Artifacts, artifactBrief{
			Component: string(a.Component), ResourceName: a.ResourceName,
			Location: a.Location, SizeBytes: a.SizeBytes, ChecksumSHA256: a.ChecksumSHA256,
		})
	}
	return res
}

func (h *BackupHandler) CreateBackup(c echo.Context) error {
	var req createBackupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "요청 본문을 해석할 수 없습니다")
	}
	mode := domain.Mode(req.Mode)
	if mode == "" {
		mode = domain.ModeFull
	}
	ns := req.Namespace
	if ns == "" {
		ns = h.defaultNS
	}

	scope, err := domain.ParseComponents(req.Scope)
	if err != nil {
		return err
	}
	effective := scope
	if len(effective) == 0 {
		effective = domain.ModeComponents(mode)
	}

	// 확인은 **실제로 정지 창이 생길 때만** 요구한다.
	//
	// 모드로 판단하면 full 모드에서 볼륨을 뺀 선택까지 확인을 강요하게 된다 —
	// 멈추지도 않는데 겁을 주면 확인 문구는 의미 없는 절차가 되고, 정작
	// 진짜 멈추는 백업에서도 사용자가 습관적으로 넘긴다.
	if requiresQuiesce(effective) && req.Confirm != ns {
		return &shareddomain.AppError{
			Code:       "BACKUP_CONFIRM_REQUIRED",
			HTTPStatus: http.StatusBadRequest,
			Message:    "이 백업은 대상 워크로드를 잠시 멈춥니다. 확인을 위해 네임스페이스를 입력하세요",
			Detail:     ns,
		}
	}

	uc, err := h.factory.Backup(c.Request().Context(), req.StackID)
	if err != nil {
		return err
	}
	run, err := uc.Run(c.Request().Context(), usecase.RunBackupRequest{
		OrgID: resolveOrgID(c), StackID: req.StackID, Namespace: ns,
		Mode: mode, Trigger: domain.TriggerManual, Scope: scope,
	})
	if err != nil {
		if run != nil {
			return c.JSON(http.StatusAccepted, toResponse(run, nil, false))
		}
		return err
	}
	return c.JSON(http.StatusCreated, toResponse(run, nil, false))
}

func (h *BackupHandler) ListBackups(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	runs, err := h.repo.ListRuns(c.Request().Context(), resolveOrgID(c), limit)
	if err != nil {
		return err
	}
	out := make([]backupRunResponse, 0, len(runs))
	for _, r := range runs {
		out = append(out, toResponse(r, nil, false))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (h *BackupHandler) GetBackup(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	run, err := h.repo.GetRun(ctx, id)
	if err != nil {
		return err
	}
	arts, err := h.repo.ListArtifacts(ctx, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toResponse(run, arts, true))
}

// VerifyBackup 은 복원 없이 산출물이 열리는지만 본다.
//
// 복구 리허설을 상시로 돌릴 수 없으므로, "백업은 되는데 복원이 안 됐다" 에
// 대한 최소한의 상시 방어선이다.
func (h *BackupHandler) VerifyBackup(c echo.Context) error {
	res, err := h.verify.Verify(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, res)
}

type createRestoreRequest struct {
	BackupRunID string `json:"backup_run_id"`
	Namespace   string `json:"namespace"`
	Mode        string `json:"mode"`
	// Confirm 은 대상 백업 ID 재입력이다. 복구는 파괴적이다.
	Confirm string `json:"confirm"`
}

func (h *BackupHandler) CreateRestore(c echo.Context) error {
	var req createRestoreRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "요청 본문을 해석할 수 없습니다")
	}
	if req.BackupRunID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "backup_run_id 가 필요합니다")
	}
	if req.Confirm != req.BackupRunID {
		return &shareddomain.AppError{
			Code:       "RESTORE_CONFIRM_REQUIRED",
			HTTPStatus: http.StatusBadRequest,
			Message:    "복구는 기존 데이터를 덮어씁니다. 확인을 위해 백업 ID 를 다시 입력하세요",
			Detail:     req.BackupRunID,
		}
	}
	mode := domain.RestoreMode(req.Mode)
	if mode == "" {
		mode = domain.ModeFull
	}
	ns := req.Namespace
	if ns == "" {
		ns = h.defaultNS
	}

	backup, err := h.repo.GetRun(c.Request().Context(), req.BackupRunID)
	if err != nil {
		return err
	}
	uc, err := h.factory.Restore(c.Request().Context(), backup.StackID)
	if err != nil {
		return err
	}
	rr, err := uc.Run(c.Request().Context(), usecase.RunRestoreRequest{
		BackupRunID: req.BackupRunID, OrgID: resolveOrgID(c), Namespace: ns, Mode: mode,
	})
	if err != nil {
		if rr != nil {
			return c.JSON(http.StatusConflict, restoreResponse(rr))
		}
		return err
	}
	return c.JSON(http.StatusCreated, restoreResponse(rr))
}

func (h *BackupHandler) GetRestore(c echo.Context) error {
	rr, err := h.repo.GetRestore(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, restoreResponse(rr))
}

func restoreResponse(rr *domain.RestoreRun) map[string]any {
	return map[string]any{
		"id":               rr.ID,
		"backup_run_id":    rr.BackupRunID,
		"mode":             string(rr.Mode),
		"status":           string(rr.Status),
		"schema_check":     rr.SchemaCheck,
		"integrity_report": rr.IntegrityReport,
		"error":            rr.Error,
		"started_at":       rr.StartedAt,
		"finished_at":      rr.FinishedAt,
		"created_at":       rr.CreatedAt,
	}
}

// resolveOrgID 는 인증 미들웨어가 넣은 조직 식별자를 읽는다.
func resolveOrgID(c echo.Context) string {
	if v, ok := c.Get("org_id").(string); ok && v != "" {
		return v
	}
	if v := c.Request().Header.Get("X-Org-Id"); v != "" {
		return v
	}
	return ""
}

// requiresQuiesce 는 고른 대상이 서비스를 멈추게 하는지 본다.
//
// 도메인의 BackupRun.RequiresQuiesce 와 같은 규칙이되, 실행을 만들기 **전에**
// 확인 문구를 요구할지 정해야 하므로 여기서 한 번 더 본다.
func requiresQuiesce(scope []domain.Component) bool {
	for _, c := range scope {
		if c == domain.ComponentVolume {
			return true
		}
	}
	return false
}
