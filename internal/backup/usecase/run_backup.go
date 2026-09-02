package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// 백업 실행. 설계 §3.4 (nullus-plan#75)
//
// 이 파일의 값어치는 "무엇을 하느냐" 가 아니라 **순서** 에 있다:
//   - 목적지 검사는 정지보다 먼저다 (§9 F7b·F8)
//   - 재개는 무슨 일이 있어도 일어난다 (§9 F3)
//   - 리소스 덤프는 워크로드가 살아 있을 때 한다

// RotationPauser 는 백업 동안 토큰 회전을 멈춘다.
//
// 회전 스케줄러가 5분마다 DB 와 금고를 함께 고쳐쓰기 때문에, 멈추지 않으면
// 두 백업 산출물 사이에 skew 가 생긴다 (§2.1).
type RotationPauser interface {
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
}

type DBTargets struct {
	Platform port.DBTarget
	Keycloak port.DBTarget
}

type BackupDeps struct {
	Repo      port.BackupRepository
	Dumper    port.DBDumper
	KV        port.KVExporter
	Scaler    port.WorkloadScaler
	Archiver  port.VolumeArchiver
	Resources port.ResourceDumper
	Sealer    port.Sealer
	Store     port.ArtifactStore
	Notifier  port.Notifier
	Pauser    RotationPauser
	Targets   DBTargets

	PlatformVersion string
	Logger          *slog.Logger
}

type BackupUseCase struct {
	d BackupDeps
}

func NewBackupUseCase(d BackupDeps) *BackupUseCase {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &BackupUseCase{d: d}
}

type RunBackupRequest struct {
	OrgID     string
	StackID   string
	Namespace string
	Mode      domain.Mode
	Trigger   domain.Trigger
}

func (uc *BackupUseCase) Run(ctx context.Context, req RunBackupRequest) (*domain.BackupRun, error) {
	switch req.Mode {
	case domain.ModeFull, domain.ModePlatformOnly, domain.ModeStackOnly:
	default:
		return nil, domain.ErrInvalidMode(string(req.Mode))
	}
	trigger := req.Trigger
	if trigger == "" {
		trigger = domain.TriggerManual
	}

	scope := domain.ModeComponents(req.Mode)
	run := domain.NewBackupRun(req.OrgID, req.Mode, trigger, scope)
	run.StackID = req.StackID

	needsStack := req.Mode != domain.ModePlatformOnly
	if needsStack && req.Namespace == "" {
		return nil, fmt.Errorf("스택 대상 백업에는 네임스페이스가 필요합니다")
	}

	if err := uc.d.Repo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("백업 실행 기록 생성: %w", err)
	}
	if err := run.Start(); err != nil {
		return nil, err
	}
	_ = uc.d.Repo.UpdateRun(ctx, run)

	res, err := uc.execute(ctx, req, run)
	if err != nil {
		run.Fail(err.Error())
		uc.finish(ctx, run, nil)
		return run, err
	}

	run.Complete(res.results)
	uc.finish(ctx, run, res)
	return run, nil
}

type executeResult struct {
	results   map[domain.Component]error
	artifacts []domain.Artifact
	plan      domain.QuiescePlan
	volumes   []domain.VolumeSpec
	kvPaths   int
	pgServer  string
	pgClient  string
}

func (uc *BackupUseCase) execute(ctx context.Context, req RunBackupRequest, run *domain.BackupRun) (*executeResult, error) {
	out := &executeResult{results: map[domain.Component]error{}}

	// 스키마 버전은 복구 시 정합성 판단의 기준이다 (§6.2).
	if st, err := uc.d.Dumper.SchemaState(ctx, uc.d.Targets.Platform); err == nil {
		run.SchemaVersion = st.Version
	}

	// ── 목적지 검사. 반드시 정지보다 먼저 (§9 F7b·F8) ──
	//
	// 필요 용량은 정적 계산이 아니라 실행 시점 PVC 조회로 구한다 — GitLab·
	// Gitea 는 설치기가 크기를 지정하지 않아 차트 기본값을 따른다 (§1.5).
	var required int64
	if uc.inScope(run, domain.ComponentVolume) {
		pvcs, err := uc.d.Archiver.ListPVCs(ctx, req.Namespace)
		if err != nil {
			return nil, fmt.Errorf("PVC 목록 조회: %w", err)
		}
		out.volumes = pvcs
		for _, v := range pvcs {
			required += v.SizeBytes
		}
	}
	if err := uc.d.Store.Preflight(ctx, required); err != nil {
		return nil, domain.ErrDestinationUnavailable(err.Error())
	}

	// ── 1. 리소스 덤프 — 워크로드가 살아 있을 때 떠야 실제 상태가 담긴다 ──
	if uc.inScope(run, domain.ComponentNamespaceResources) {
		art, err := uc.putStream(ctx, run, domain.ComponentNamespaceResources, "",
			key(run.ID, "namespace-resources.yaml"),
			func(w io.Writer) (int64, error) { return uc.d.Resources.Dump(ctx, req.Namespace, w) })
		out.results[domain.ComponentNamespaceResources] = err
		if err == nil {
			out.artifacts = append(out.artifacts, art)
		}
	}

	// ── 2~6. 정지 창 ──
	if run.RequiresQuiesce() {
		if err := uc.withQuiesce(ctx, req, run, out); err != nil {
			return nil, err
		}
	}

	// ── 7. DB / 금고. 무중단으로 돈다 (컨트롤 플레인은 멈추지 않는다) ──
	//
	// 순서는 금고 → DB 다. 금고를 먼저 뜨면 "DB 는 아는데 금고엔 없다" 방향의
	// 불일치만 남고, 그것은 §6.4 검사로 탐지된다 (§2.1).
	if uc.inScope(run, domain.ComponentOpenBaoKV) {
		var count int
		art, err := uc.putStream(ctx, run, domain.ComponentOpenBaoKV, "",
			key(run.ID, "openbao-kv.json"),
			func(w io.Writer) (int64, error) {
				r, e := uc.d.KV.Export(ctx, req.StackID, w)
				count = r.PathCount
				return r.Bytes, e
			})
		out.results[domain.ComponentOpenBaoKV] = err
		if err == nil {
			out.kvPaths = count
			out.artifacts = append(out.artifacts, art)
		}
	}

	for _, spec := range []struct {
		comp   domain.Component
		target port.DBTarget
		file   string
	}{
		{domain.ComponentPlatformDB, uc.d.Targets.Platform, "platform-db.dump"},
		{domain.ComponentKeycloakDB, uc.d.Targets.Keycloak, "keycloak-db.dump"},
	} {
		if !uc.inScope(run, spec.comp) {
			continue
		}
		// 대상이 설정되지 않았으면 여기서 분명히 말한다.
		//
		// 그냥 두면 pg_dump 가 로컬 소켓에 붙으려다 "socket ... No such file"
		// 로 죽는다 — 무엇이 잘못됐는지 알 수 없는 메시지다. 리허설에서
		// 실제로 그렇게 됐고, 기본 설정이 빈 값이라 모든 백업이 partial 이
		// 될 수 있었다.
		if strings.TrimSpace(spec.target.Host) == "" {
			out.results[spec.comp] = fmt.Errorf(
				"%s 대상이 설정되지 않았습니다 (backup 설정의 데이터베이스 항목을 채우세요)", spec.comp)
			continue
		}
		art, err := uc.putStream(ctx, run, spec.comp, "", key(run.ID, spec.file),
			func(w io.Writer) (int64, error) {
				r, e := uc.d.Dumper.Dump(ctx, spec.target, w)
				out.pgClient = r.ClientVersion
				return r.BytesWritten, e
			})
		out.results[spec.comp] = err
		if err == nil {
			out.artifacts = append(out.artifacts, art)
		}
	}
	if v, err := uc.d.Dumper.ServerVersion(ctx, uc.d.Targets.Platform); err == nil {
		out.pgServer = v
	}

	return out, nil
}

// withQuiesce 는 정지 창을 열고 반드시 닫는다.
//
// 재개는 defer 로 보장한다. 이 함수 안에서 무엇이 실패하든 워크로드는
// 되살아나야 한다 — 백업하려다 서비스를 못 살리는 것이 최악이다 (§9 F3).
func (uc *BackupUseCase) withQuiesce(ctx context.Context, req RunBackupRequest, run *domain.BackupRun, out *executeResult) error {
	workloads, err := uc.d.Scaler.List(ctx, req.Namespace)
	if err != nil {
		return fmt.Errorf("워크로드 목록 조회: %w", err)
	}
	plan := domain.NewQuiescePlan(workloads)
	out.plan = plan
	if plan.IsEmpty() {
		// 멈출 것이 없으면 정지 창을 열지 않는다.
		out.results[domain.ComponentVolume] = uc.archiveVolumes(ctx, req, run, out)
		return nil
	}

	if uc.d.Pauser != nil {
		if err := uc.d.Pauser.Pause(ctx); err != nil {
			uc.d.Logger.Warn("회전 스케줄러 정지 실패 — skew 가 생길 수 있습니다", "error", err)
		}
	}

	run.BeginQuiesce()
	defer func() {
		// 역순 재개. 실패해도 남은 대상은 계속 시도한다.
		for _, t := range plan.ResumeOrder() {
			if err := uc.d.Scaler.Scale(ctx, t, t.OriginalReplicas); err != nil {
				uc.d.Logger.Error("워크로드 재개 실패 — 수동 조치가 필요합니다",
					"kind", t.Kind, "name", t.Name, "replicas", t.OriginalReplicas, "error", err)
			}
		}
		if uc.d.Pauser != nil {
			if err := uc.d.Pauser.Resume(ctx); err != nil {
				uc.d.Logger.Error("회전 스케줄러 재개 실패", "error", err)
			}
		}
		run.EndQuiesce()
	}()

	for _, t := range plan.Targets {
		if err := uc.d.Scaler.Scale(ctx, t, t.ScaleDownTo); err != nil {
			return fmt.Errorf("워크로드 정지 실패(%s/%s): %w", t.Kind, t.Name, err)
		}
	}
	if err := uc.d.Scaler.WaitStopped(ctx, req.Namespace, plan.Targets); err != nil {
		return fmt.Errorf("파드 종료 대기: %w", err)
	}

	out.results[domain.ComponentVolume] = uc.archiveVolumes(ctx, req, run, out)
	return nil
}

func (uc *BackupUseCase) archiveVolumes(ctx context.Context, req RunBackupRequest, run *domain.BackupRun, out *executeResult) error {
	var firstErr error
	for i, v := range out.volumes {
		art, err := uc.putStream(ctx, run, domain.ComponentVolume, v.Name,
			key(run.ID, "volumes/"+v.Name+".tar"),
			func(w io.Writer) (int64, error) { return uc.d.Archiver.Archive(ctx, req.Namespace, v.Name, w) })
		if err != nil {
			uc.d.Logger.Error("볼륨 아카이브 실패", "pvc", v.Name, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out.volumes[i].ChecksumSHA256 = art.ChecksumSHA256
		out.artifacts = append(out.artifacts, art)
	}
	return firstErr
}

// putStream 은 produce → seal → store 를 파이프로 잇는다.
//
// 중간 파일을 만들지 않는다 — E1 규모에서 디스크 여유가 없다 (§4.3).
func (uc *BackupUseCase) putStream(
	ctx context.Context,
	run *domain.BackupRun,
	comp domain.Component,
	resourceName string,
	objectKey string,
	produce func(io.Writer) (int64, error),
) (domain.Artifact, error) {
	pr, pw := io.Pipe()
	sealed, sealedW := io.Pipe()

	go func() {
		_, err := produce(pw)
		_ = pw.CloseWithError(err)
	}()
	go func() {
		err := uc.d.Sealer.Seal(ctx, pr, sealedW)
		_ = sealedW.CloseWithError(err)
	}()

	res, err := uc.d.Store.Put(ctx, objectKey, sealed)
	if err != nil {
		_ = pr.CloseWithError(err)
		return domain.Artifact{}, err
	}

	art := domain.Artifact{
		BackupRunID:     run.ID,
		Component:       comp,
		ResourceName:    resourceName,
		Location:        res.Location,
		SizeBytes:       res.Bytes,
		ChecksumSHA256:  res.ChecksumSHA256,
		EncryptionKeyID: uc.d.Sealer.KeyID(),
	}
	if err := uc.d.Repo.AddArtifact(ctx, &art); err != nil {
		return domain.Artifact{}, err
	}
	return art, nil
}

func (uc *BackupUseCase) inScope(run *domain.BackupRun, c domain.Component) bool {
	for _, s := range run.Scope {
		if s == c {
			return true
		}
	}
	return false
}

func (uc *BackupUseCase) finish(ctx context.Context, run *domain.BackupRun, res *executeResult) {
	if res != nil {
		m := domain.BuildManifest(run, domain.ManifestInput{
			PlatformVersion:    uc.d.PlatformVersion,
			PGServerVersion:    res.pgServer,
			PGDumpVersion:      res.pgClient,
			EncryptionKeyID:    uc.d.Sealer.KeyID(),
			Plan:               res.plan,
			Volumes:            res.volumes,
			Artifacts:          res.artifacts,
			OpenBaoKVPathCount: res.kvPaths,
		})
		run.Manifest = manifestToMap(m)
		for _, a := range res.artifacts {
			run.TotalBytes += a.SizeBytes
		}
	}
	if err := uc.d.Repo.UpdateRun(ctx, run); err != nil {
		uc.d.Logger.Error("백업 실행 기록 갱신 실패", "run_id", run.ID, "error", err)
	}
	if uc.d.Notifier != nil {
		if err := uc.d.Notifier.NotifyBackupResult(ctx, run); err != nil {
			uc.d.Logger.Warn("백업 결과 알림 실패", "run_id", run.ID, "error", err)
		}
	}
}

func key(runID, name string) string { return "backup-" + runID + "/" + name }
