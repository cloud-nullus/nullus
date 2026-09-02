package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// 복구 실행. 설계 §6.1 (nullus-plan#75)
//
// 복구는 "dump 를 되돌리는 것" 이 아니다. 순서를 지키지 않으면 각 단계가
// 서로를 깨뜨린다. 이 파일이 강제하는 순서:
//
//	0  전제 확인      — 키가 없으면 아무것도 하지 않는다 (F1)
//	1  무결성 검증
//	2  스키마 정합성  — 백업 > 현재이거나 dirty 면 차단
//	3  PVC 재생성
//	4  볼륨 복원      — 워크로드는 아직 0
//	5  리소스 복원
//	6  DB 복원
//	7  금고 복원
//	8  워크로드 재개  — 반드시 볼륨 복원 뒤 (F4)
//	9  참조 정합성    — 경고하되 중단하지 않는다

type RestoreDeps struct {
	Repo         port.BackupRepository
	Dumper       port.DBDumper
	KV           port.KVExporter
	Scaler       port.WorkloadScaler
	Archiver     port.VolumeArchiver
	Resources    port.ResourceDumper
	Sealer       port.Sealer
	Store        port.ArtifactStore
	TokenSources port.TokenSourceLister
	Targets      DBTargets

	// Prereq 는 실행 시점의 전제 조건을 읽어온다. 함수로 둔 이유는 키의
	// 존재 여부가 프로세스 수명 동안 바뀔 수 있기 때문이다.
	Prereq func(context.Context) domain.Prerequisites

	Logger *slog.Logger
}

type RestoreUseCase struct {
	d RestoreDeps
}

func NewRestoreUseCase(d RestoreDeps) *RestoreUseCase {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.Prereq == nil {
		d.Prereq = func(context.Context) domain.Prerequisites { return domain.Prerequisites{} }
	}
	return &RestoreUseCase{d: d}
}

type RunRestoreRequest struct {
	BackupRunID string
	OrgID       string
	Namespace   string
	Mode        domain.RestoreMode
}

func (uc *RestoreUseCase) Run(ctx context.Context, req RunRestoreRequest) (*domain.RestoreRun, error) {
	rr := domain.NewRestoreRun(req.BackupRunID, req.Mode)
	if err := uc.d.Repo.CreateRestore(ctx, rr); err != nil {
		return nil, fmt.Errorf("복구 실행 기록 생성: %w", err)
	}

	if err := uc.run(ctx, req, rr); err != nil {
		rr.Fail(err.Error())
		_ = uc.d.Repo.UpdateRestore(ctx, rr)
		return rr, err
	}
	rr.Succeed()
	_ = uc.d.Repo.UpdateRestore(ctx, rr)
	return rr, nil
}

func (uc *RestoreUseCase) run(ctx context.Context, req RunRestoreRequest, rr *domain.RestoreRun) error {
	// ── 0. 전제 확인 ──
	//
	// 이 검사가 다른 어떤 것보다 먼저다. 키 없이 진행하면 "복구된 것처럼
	// 보이는 망가진 상태" 가 되고, 그때는 되돌릴 방법도 없다.
	if err := domain.CheckPrerequisites(uc.d.Prereq(ctx)); err != nil {
		return err
	}

	backup, err := uc.d.Repo.GetRun(ctx, req.BackupRunID)
	if err != nil {
		return err
	}
	if backup.Status != domain.StatusSucceeded && backup.Status != domain.StatusPartial {
		return fmt.Errorf("복구할 수 없는 백업입니다: 상태 %s", backup.Status)
	}
	if err := rr.Start(); err != nil {
		return err
	}

	arts, err := uc.d.Repo.ListArtifacts(ctx, backup.ID)
	if err != nil {
		return fmt.Errorf("산출물 목록 조회: %w", err)
	}

	// ── 1. 무결성 검증 ──
	if err := uc.verifyArtifacts(ctx, arts); err != nil {
		return err
	}

	// ── 2. 스키마 정합성 ──
	current, err := uc.d.Dumper.SchemaState(ctx, uc.d.Targets.Platform)
	if err != nil {
		return fmt.Errorf("현재 스키마 버전 조회: %w", err)
	}
	rr.SchemaCheck = domain.CheckSchemaVersion(backup.SchemaVersion, current)
	_ = uc.d.Repo.UpdateRestore(ctx, rr)
	if !rr.SchemaCheck.Allowed {
		return domain.ErrSchemaVersionMismatch(rr.SchemaCheck.Reason)
	}

	manifest, err := manifestFromMap(backup.Manifest)
	if err != nil {
		return fmt.Errorf("매니페스트 해석: %w", err)
	}

	touchStack := req.Mode != domain.ModePlatformOnly
	touchPlatform := req.Mode != domain.ModeStackOnly

	// ── 3~5. 스택: PVC → 볼륨 → 리소스. 워크로드는 아직 띄우지 않는다 ──
	if touchStack {
		if err := uc.restoreStackData(ctx, req, manifest, arts); err != nil {
			return err
		}
	}

	// ── 6~7. 플랫폼: DB → 금고 ──
	if touchPlatform {
		if err := uc.restorePlatform(ctx, arts); err != nil {
			return err
		}
	}

	// ── 8. 워크로드 재개. 반드시 볼륨 복원 뒤다 ──
	//
	// 순서를 뒤집으면 도구들이 빈 디스크를 보고 재초기화한다 — GitLab 이
	// 새 인스턴스가 되는 순간 복원은 무의미해진다 (§9 F4).
	if touchStack {
		uc.resumeWorkloads(ctx, manifest)
	}

	// ── 9. 참조 정합성. 경고하되 중단하지 않는다 ──
	rr.IntegrityReport = uc.checkIntegrity(ctx, req, backup)
	return nil
}

func (uc *RestoreUseCase) verifyArtifacts(ctx context.Context, arts []*domain.Artifact) error {
	for _, a := range arts {
		_, sum, err := uc.d.Store.Stat(ctx, locationKey(a.Location))
		if err != nil {
			return domain.ErrChecksumMismatch(fmt.Sprintf("%s: %v", a.Component, err))
		}
		if a.ChecksumSHA256 != "" && sum != a.ChecksumSHA256 {
			return domain.ErrChecksumMismatch(fmt.Sprintf("%s: 기대 %s, 실제 %s", a.Component, a.ChecksumSHA256, sum))
		}
	}
	return nil
}

func (uc *RestoreUseCase) restoreStackData(ctx context.Context, req RunRestoreRequest, m domain.Manifest, arts []*domain.Artifact) error {
	// 3. PVC 재생성 — 매니페스트의 크기·StorageClass 대로.
	for _, v := range m.Volumes {
		if err := uc.d.Archiver.EnsurePVC(ctx, req.Namespace, v); err != nil {
			return fmt.Errorf("PVC 재생성(%s): %w", v.Name, err)
		}
	}

	// 4. 볼륨 복원.
	for _, a := range arts {
		if a.Component != domain.ComponentVolume {
			continue
		}
		if err := uc.pullStream(ctx, a, func(r io.Reader) error {
			return uc.d.Archiver.Restore(ctx, req.Namespace, a.ResourceName, r)
		}); err != nil {
			return fmt.Errorf("볼륨 복원(%s): %w", a.ResourceName, err)
		}
	}

	// 5. 리소스 복원.
	for _, a := range arts {
		if a.Component != domain.ComponentNamespaceResources {
			continue
		}
		if err := uc.pullStream(ctx, a, func(r io.Reader) error {
			return uc.d.Resources.Apply(ctx, req.Namespace, r)
		}); err != nil {
			return fmt.Errorf("네임스페이스 리소스 복원: %w", err)
		}
	}
	return nil
}

func (uc *RestoreUseCase) restorePlatform(ctx context.Context, arts []*domain.Artifact) error {
	for _, a := range arts {
		switch a.Component {
		case domain.ComponentPlatformDB:
			// 플랫폼 DB 실패는 치명적이다 — 여기서 멈춘다.
			if err := uc.pullStream(ctx, a, func(r io.Reader) error {
				return uc.d.Dumper.Restore(ctx, uc.d.Targets.Platform, r)
			}); err != nil {
				return fmt.Errorf("플랫폼 DB 복원: %w", err)
			}
		case domain.ComponentKeycloakDB:
			// Keycloak 실패는 치명적이지 않다 (§6.3). users.password_hash
			// 경로가 살아 있으면 관리자는 포털 ID/PW 로 들어갈 수 있다.
			if err := uc.pullStream(ctx, a, func(r io.Reader) error {
				return uc.d.Dumper.Restore(ctx, uc.d.Targets.Keycloak, r)
			}); err != nil {
				uc.d.Logger.Error("Keycloak DB 복원 실패 — 포털 ID/PW 경로로 진입해야 합니다", "error", err)
			}
		case domain.ComponentOpenBaoKV:
			if err := uc.pullStream(ctx, a, func(r io.Reader) error {
				return uc.d.KV.Import(ctx, "", r)
			}); err != nil {
				uc.d.Logger.Error("금고 복원 실패 — 참조 정합성 검사에서 드러납니다", "error", err)
			}
		}
	}
	return nil
}

func (uc *RestoreUseCase) resumeWorkloads(ctx context.Context, m domain.Manifest) {
	// 역순 재개 — 의존 대상이 먼저 뜬다.
	for i := len(m.Workloads) - 1; i >= 0; i-- {
		w := m.Workloads[i]
		t := domain.QuiesceTarget{
			Kind: w.Kind, Namespace: w.Namespace, Name: w.Name,
			OriginalReplicas: w.OriginalReplicas,
		}
		if err := uc.d.Scaler.Scale(ctx, t, w.OriginalReplicas); err != nil {
			uc.d.Logger.Error("워크로드 재개 실패 — 수동 조치가 필요합니다",
				"kind", w.Kind, "name", w.Name, "error", err)
		}
	}
}

// checkIntegrity 는 DB 가 가리키는 시크릿 경로가 금고에 실제로 있는지 본다.
//
// 없으면(dangling) 해당 토큰의 재등록이 필요하다는 뜻이다. 조용히 넘어가면
// 나중에 파이프라인 실행 시점에 인증 실패로만 드러난다 (§6.4).
func (uc *RestoreUseCase) checkIntegrity(ctx context.Context, req RunRestoreRequest, backup *domain.BackupRun) domain.IntegrityReport {
	report := domain.IntegrityReport{CheckedAt: time.Now().UTC()}
	if uc.d.TokenSources == nil {
		return report
	}
	refs, err := uc.d.TokenSources.ListPaths(ctx, req.OrgID)
	if err != nil {
		uc.d.Logger.Warn("참조 정합성 검사 실패", "error", err)
		return report
	}
	for _, ref := range refs {
		report.CheckedPaths++
		ok, err := uc.d.KV.PathExists(ctx, backup.StackID, ref.Path)
		if err != nil {
			uc.d.Logger.Warn("금고 경로 확인 실패", "path", ref.Path, "error", err)
			continue
		}
		if !ok {
			report.Dangling = append(report.Dangling, domain.DanglingReference{
				TokenSourceID: ref.ID, Path: ref.Path,
			})
		}
	}
	if report.HasIssues() {
		uc.d.Logger.Warn("복구 후 끊어진 시크릿 참조가 있습니다 — 해당 토큰을 재등록해야 합니다",
			"count", len(report.Dangling))
	}
	return report
}

// pullStream 은 store → unseal → consume 을 파이프로 잇는다.
func (uc *RestoreUseCase) pullStream(ctx context.Context, a *domain.Artifact, consume func(io.Reader) error) error {
	rc, err := uc.d.Store.Get(ctx, locationKey(a.Location))
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	pr, pw := io.Pipe()
	go func() {
		err := uc.d.Sealer.Unseal(ctx, rc, pw)
		_ = pw.CloseWithError(err)
	}()
	return consume(pr)
}

// locationKey 는 저장 위치에서 오브젝트 키를 뽑는다.
func locationKey(location string) string {
	if i := strings.Index(location, "://"); i >= 0 {
		rest := location[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			return rest[j+1:]
		}
	}
	return location
}
