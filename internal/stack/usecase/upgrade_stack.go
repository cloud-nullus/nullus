package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 업그레이드할 수 있는 릴리스 → 도구. 지금은 Argo CD 하나다(설계 §7 Phase 1).
var upgradeToolByRelease = map[string]string{"argo-cd": domain.UpgradeToolArgoCD}

const (
	// upgradeExecutionTimeout 은 백그라운드 실행 전체의 상한이다. Helm 호출 한 번이 훅 · 대기 ·
	// --atomic 되돌리기로 각각 30분까지 쓸 수 있고, 그 뒤 명시적 되돌리기와 상태 검사 두 번이 온다.
	upgradeExecutionTimeout = 3 * time.Hour
	// upgradePersistTimeout 은 상태 저장 한 번의 상한이다. 실행 ctx 가 끝났어도 결과는 남긴다.
	upgradePersistTimeout = 10 * time.Second
	// upgradePersistAttempts 는 상태 저장의 시도 횟수다 — 끝 상태를 잃으면 성공한 실행이 박동
	// 끊김으로 "중단" 이 된다.
	upgradePersistAttempts = 3
)

type UpgradeExecutorFactory func(kubeconfig []byte) port.ChartUpgradeExecutor

type UpgradeStackDeps struct {
	Stacks      port.StackRepository
	Runs        port.UpgradeRepository
	Kubeconfigs port.KubeconfigProvider
	Releases    ReleaseManagerFactory
	Executors   UpgradeExecutorFactory
	// Backups 가 없으면(백업 기능 꺼짐) 백업이 필요한 번들은 막힌다 — 안전 조건을 완화하지 않는다.
	Backups      port.UpgradeBackupLookup
	BackupMaxAge time.Duration
	Health       port.UpgradeHealthVerifier
	// Vulnerabilities 는 스택의 설치 이미지 스캔 결과에서 릴리스별 CVE 를 읽는다. 없으면 후보는 번들이
	// 고치는 CVE 목록만 싣고, 그중 무엇이 이 스택에 있는지는 비운다.
	Vulnerabilities port.ReleaseVulnerabilityLookup
	Now             func() time.Time
	NewID           func() string
	// Launch 는 실행을 요청 밖으로 넘긴다. 기본은 goroutine 이다.
	Launch func(job func())
	// Context 는 API 프로세스의 수명이다. 종료 때 취소되면 Helm 이 --atomic 으로 스스로 되돌리기
	// 시작하고, 실행은 사람에게 넘긴다. 요청 ctx 와는 묶지 않는다 — 화면을 닫아도 이어진다.
	Context context.Context
	// HeartbeatInterval 은 실행 중 박동 주기다(기본 1분). domain.UpgradeHeartbeatTimeout 보다 짧아야 한다.
	HeartbeatInterval time.Duration
	// RetryBackoff 는 상태 저장 재시도의 첫 대기다(기본 1초, 두 배씩).
	RetryBackoff time.Duration
	Logger       *slog.Logger
}

// UpgradeStack 은 검증된 번들로 설치된 OSS 를 올린다. 사전 점검은 요청 안에서, Helm 적용 ·
// 상태 검사 · 되돌리기는 요청 밖에서 한다 — 화면을 닫아도 서버에서 끝난다.
type UpgradeStack struct {
	d    UpgradeStackDeps
	jobs sync.WaitGroup
}

func NewUpgradeStack(d UpgradeStackDeps) *UpgradeStack {
	if d.Now == nil {
		d.Now = func() time.Time { return time.Now().UTC() }
	}
	if d.NewID == nil {
		d.NewID = uuid.NewString
	}
	if d.Launch == nil {
		d.Launch = func(job func()) { go job() }
	}
	if d.BackupMaxAge <= 0 {
		d.BackupMaxAge = 24 * time.Hour
	}
	if d.Context == nil {
		d.Context = context.Background()
	}
	if d.HeartbeatInterval <= 0 {
		d.HeartbeatInterval = time.Minute
	}
	if d.RetryBackoff <= 0 {
		d.RetryBackoff = time.Second
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &UpgradeStack{d: d}
}

// Wait 는 백그라운드 실행이 끝나기를 ctx 가 끝날 때까지 기다린다 — API 종료 처리에서 부른다.
func (uc *UpgradeStack) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		uc.jobs.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type resolvedUpgrade struct {
	stack      *domain.Stack
	kubeconfig []byte
	releases   port.HelmReleaseManager
	executor   port.ChartUpgradeExecutor
	namespace  string
}

func (uc *UpgradeStack) resolve(ctx context.Context, stackID string) (*resolvedUpgrade, error) {
	d := uc.d
	if d.Stacks == nil || d.Runs == nil || d.Kubeconfigs == nil || d.Releases == nil || d.Executors == nil || d.Health == nil {
		return nil, fmt.Errorf("업그레이드 기능이 구성되지 않았다")
	}
	stack, err := d.Stacks.GetByID(ctx, strings.TrimSpace(stackID))
	if err != nil || stack == nil {
		return nil, ErrStackNotFound
	}
	kubeconfig, err := d.Kubeconfigs.GetKubeconfig(ctx, stack.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("클러스터 %s 에 kubeconfig 가 등록되어 있지 않다", stack.ClusterID)
	}
	ns := strings.TrimSpace(stack.Namespace)
	if ns == "" {
		ns = "nullus"
	}
	return &resolvedUpgrade{stack: stack, kubeconfig: kubeconfig, releases: d.Releases(kubeconfig),
		executor: d.Executors(kubeconfig), namespace: ns}, nil
}

// Candidates 는 스택에 실제로 설치된 릴리스(DB 가 아니라 클러스터)와 거기서 갈 수 있는 검증 경로,
// 마지막 실행을 돌려준다.
func (uc *UpgradeStack) Candidates(ctx context.Context, stackID string) ([]domain.UpgradeCandidate, error) {
	r, err := uc.resolve(ctx, stackID)
	if err != nil {
		return nil, err
	}
	releases, err := r.releases.ListReleases(ctx, r.namespace)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	out := []domain.UpgradeCandidate{}
	for _, rel := range releases {
		tool, ok := upgradeToolByRelease[rel.ReleaseName]
		if !ok {
			continue
		}
		bundles, err := uc.d.Runs.ListVerifiedBundles(ctx, tool, rel.ChartVersion)
		if err != nil {
			return nil, fmt.Errorf("list upgrade bundles: %w", err)
		}
		bundle := domain.SelectUpgradeBundle(bundles)
		candidate := domain.NewUpgradeCandidate(tool, rel.ReleaseName, rel.ChartVersion, rel.AppVersion,
			rel.Revision, rel.Status, bundle, uc.d.Now())
		candidate.FixesFound = uc.fixesFound(ctx, r.stack.ID, rel.ReleaseName, bundle)
		last, err := uc.lastRun(ctx, r.stack.ID, tool)
		if err != nil {
			return nil, err
		}
		candidate.LastRun = last
		out = append(out, candidate)
	}
	return out, nil
}

// fixesFound 는 번들이 고치는 CVE 중 이 스택의 그 릴리스 이미지에서 지금 발견된 것이다. 읽지 못하면
// 비운다 — 후보 목록을 막을 일이 아니다.
func (uc *UpgradeStack) fixesFound(ctx context.Context, stackID, release string, bundle *domain.UpgradeBundle) []string {
	if uc.d.Vulnerabilities == nil || bundle == nil || len(bundle.FixesCVEs) == 0 {
		return nil
	}
	present, err := uc.d.Vulnerabilities.VulnerabilityIDsByRelease(ctx, stackID, release)
	if err != nil {
		uc.d.Logger.Warn("업그레이드 후보: 설치 이미지 취약점을 읽지 못했습니다", "stack_id", stackID, "release", release, "error", err)
		return nil
	}
	return domain.BundleFixesFound(bundle, present)
}

// lastRun 은 화면이 보여 줄 실행이다. 진행 중인 실행이 있으면 그것이다 — 진행 중에 들어와 막힌
// 시도가 더 나중이어도 진행 표시를 가리면 안 된다. 없으면 가장 최근 실행이다.
func (uc *UpgradeStack) lastRun(ctx context.Context, stackID, tool string) (*domain.UpgradeRun, error) {
	active, err := uc.d.Runs.ActiveRun(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("active upgrade run: %w", err)
	}
	if active != nil && active.Tool == tool {
		return uc.closeIfStale(ctx, active), nil
	}
	last, err := uc.d.Runs.LatestRun(ctx, stackID, tool)
	if err != nil {
		return nil, fmt.Errorf("latest upgrade run: %w", err)
	}
	return uc.closeIfStale(ctx, last), nil
}

// Preflight 는 클러스터를 바꾸지 않고 실행 가능 여부를 점검한다.
func (uc *UpgradeStack) Preflight(ctx context.Context, stackID, bundleID string) ([]domain.UpgradeCheck, error) {
	r, bundle, release, err := uc.resolveBundle(ctx, stackID, bundleID)
	if err != nil {
		return nil, err
	}
	checks, _, err := uc.preflight(ctx, r, bundle, release)
	return checks, err
}

func (uc *UpgradeStack) preflight(ctx context.Context, r *resolvedUpgrade, bundle *domain.UpgradeBundle, release *port.ReleaseInfo) ([]domain.UpgradeCheck, map[string]any, error) {
	checks := []domain.UpgradeCheck{}
	add := func(ok bool, name, passMsg, failMsg, fix string) {
		if ok {
			checks = append(checks, domain.PassedCheck(name, passMsg))
		} else {
			checks = append(checks, domain.BlockedCheck(name, failMsg, fix))
		}
	}

	add(r.stack.State == domain.StateCompleted, "stack_state", "스택 설치 완료",
		fmt.Sprintf("스택이 설치 완료 상태가 아닙니다(현재 %s).", r.stack.State),
		"설치 · 재시도 · 삭제가 끝난 뒤 다시 시도하세요.")

	active, err := uc.d.Runs.ActiveRun(ctx, r.stack.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("check active upgrade: %w", err)
	}
	active = uc.closeIfStale(ctx, active)
	busy := active != nil && active.Active()
	busyMsg := ""
	if busy {
		busyMsg = fmt.Sprintf("다른 업그레이드가 진행 중입니다(%s → %s, %s 단계).", active.SourceChartVersion, active.TargetChartVersion, active.CurrentStep)
	}
	add(!busy, "active_run", "진행 중인 업그레이드 없음", busyMsg, "진행 중인 업그레이드가 끝난 뒤 다시 시도하세요.")

	add(release.Status == "deployed", "release_status", "Helm 릴리스 상태: deployed",
		"Helm 릴리스 상태: "+release.Status, "pending · failed 상태를 정리해 deployed 로 돌린 뒤 다시 시도하세요.")

	add(release.ChartVersion == bundle.SourceChartVersion, "version_path",
		fmt.Sprintf("차트 %s → %s (검증된 경로)", release.ChartVersion, bundle.TargetChartVersion),
		fmt.Sprintf("설치된 차트 %s 는 이 번들의 출발 버전 %s 와 다릅니다.", release.ChartVersion, bundle.SourceChartVersion),
		"현재 버전에서 출발하는 검증 번들을 고르세요.")

	if bundle.RequiresBackup {
		checks = append(checks, uc.backupCheck(ctx, r.stack.ID))
	}

	if domain.UpgradeChecksBlocked(checks) {
		return checks, nil, nil
	}
	values, err := r.releases.GetValues(ctx, release.ReleaseName, releaseNamespace(release, r.namespace))
	if err != nil {
		checks = append(checks, domain.BlockedCheck("helm_dry_run", "설치 values 를 읽지 못했습니다: "+err.Error(), "클러스터 연결을 확인하세요."))
		return checks, nil, nil
	}
	_, err = r.executor.UpgradeChart(ctx, chartUpgradeRequest(bundle, release, r.namespace, values, true))
	if err != nil {
		checks = append(checks, domain.BlockedCheck("helm_dry_run", err.Error(), "차트와 values 호환성을 확인하세요."))
		return checks, nil, nil
	}
	checks = append(checks, domain.PassedCheck("helm_dry_run", "Helm server-side dry-run 통과"))
	return checks, values, nil
}

func (uc *UpgradeStack) backupCheck(ctx context.Context, stackID string) domain.UpgradeCheck {
	if uc.d.Backups == nil {
		return domain.BlockedCheck("backup", "백업 기능이 꺼져 있어 복구 가능성을 확인할 수 없습니다.", "backup.enabled 를 켜고 이 스택의 백업을 받으세요.")
	}
	finishedAt, found, err := uc.d.Backups.LatestRestorableBackup(ctx, stackID)
	if err != nil {
		return domain.BlockedCheck("backup", "백업 상태를 확인하지 못했습니다: "+err.Error(), "잠시 뒤 다시 시도하세요.")
	}
	return domain.EvaluateUpgradeBackup(finishedAt, found, uc.d.Now(), uc.d.BackupMaxAge)
}

type StartUpgradeInput struct{ StackID, BundleID, RequestedBy, Reason, IdempotencyKey string }

// Start 는 사전 점검을 거쳐 실행을 만들고, 통과했으면 Helm 적용을 백그라운드로 넘긴다.
// 돌려주는 실행은 upgrading(진행 중) 또는 blocked(막힘)이다 — 결과는 GetRun 으로 본다.
func (uc *UpgradeStack) Start(ctx context.Context, in StartUpgradeInput) (*domain.UpgradeRun, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, domain.ErrUpgradeReasonRequired
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return nil, domain.ErrUpgradeIdempotencyKeyRequired
	}
	// 재시도(같은 키)는 클러스터를 다시 보지 않고 처음 실행을 돌려준다(UPG-10).
	if len(in.IdempotencyKey) > domain.MaxUpgradeIdempotencyKeyLength {
		return nil, domain.ErrUpgradeIdempotencyKeyInvalid
	}
	if uc.d.Runs != nil {
		if prior, err := uc.d.Runs.FindRunByIdempotencyKey(ctx, strings.TrimSpace(in.StackID), in.IdempotencyKey); err != nil {
			return nil, err
		} else if prior != nil {
			return uc.replay(ctx, prior, in.BundleID)
		}
	}
	r, bundle, release, err := uc.resolveBundle(ctx, in.StackID, in.BundleID)
	if err != nil {
		return nil, err
	}

	checks, values, err := uc.preflight(ctx, r, bundle, release)
	if err != nil {
		return nil, err
	}
	run, err := domain.NewUpgradeRun(domain.NewUpgradeRunInput{
		ID: uc.d.NewID(), StackID: r.stack.ID, Bundle: *bundle,
		SourceChartVersion: release.ChartVersion, PreviousRevision: release.Revision,
		RequestedBy: in.RequestedBy, Reason: in.Reason, IdempotencyKey: in.IdempotencyKey,
		Checks: checks, Now: uc.d.Now(),
	})
	if err != nil {
		return nil, err
	}
	if err := uc.d.Runs.CreateRun(ctx, run); err != nil {
		if errors.Is(err, port.ErrUpgradeIdempotencyConflict) {
			// 같은 키의 다른 요청이 먼저 저장했다. 그 실행이 답이다.
			prior, findErr := uc.d.Runs.FindRunByIdempotencyKey(ctx, r.stack.ID, in.IdempotencyKey)
			if findErr == nil && prior != nil {
				return uc.replay(ctx, prior, in.BundleID)
			}
		}
		return nil, err
	}
	if run.Status != domain.UpgradeStatusUpgrading {
		return run, nil
	}

	// 돌려준 실행은 응답이 직렬화하므로 백그라운드는 사본을 쓴다.
	job := *run
	uc.jobs.Add(1)
	uc.d.Launch(func() {
		defer uc.jobs.Done()
		execCtx, cancel := context.WithTimeout(uc.d.Context, upgradeExecutionTimeout)
		defer cancel()
		stopHeartbeat := uc.heartbeat(execCtx, job.ID)
		defer stopHeartbeat()
		uc.execute(execCtx, r, bundle, release, values, &job)
	})
	return run, nil
}

// replay 는 같은 키의 재시도에 처음 실행을 돌려준다. 다른 번들이면 재시도가 아니다.
func (uc *UpgradeStack) replay(ctx context.Context, prior *domain.UpgradeRun, bundleID string) (*domain.UpgradeRun, error) {
	if prior.BundleID != strings.TrimSpace(bundleID) {
		return nil, domain.ErrUpgradeIdempotencyKeyReused
	}
	return uc.closeIfStale(ctx, prior), nil
}

// heartbeat 는 실행이 끝날 때까지 박동(updated_at)을 남긴다. 실행이 다른 곳에서 닫혔으면 멈춘다.
func (uc *UpgradeStack) heartbeat(ctx context.Context, runID string) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(uc.d.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), upgradePersistTimeout)
				err := uc.d.Runs.Heartbeat(saveCtx, runID, uc.d.Now())
				cancel()
				if errors.Is(err, domain.ErrUpgradeRunClosed) {
					return
				}
				if err != nil {
					uc.d.Logger.Warn("upgrade heartbeat", "run_id", runID, "error", err)
				}
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

// execute 는 Helm 적용 → 상태 검사 → (실패 시) 되돌리기 → 되돌린 상태 검사를 한다.
// 실행이 다른 곳에서 닫혔으면(save 가 false) 거기서 멈춘다 — 닫힌 실행을 되살리거나 그 사이
// 시작된 다른 작업 위로 되돌리지 않는다.
func (uc *UpgradeStack) execute(ctx context.Context, r *resolvedUpgrade, bundle *domain.UpgradeBundle, release *port.ReleaseInfo, values map[string]any, run *domain.UpgradeRun) {
	result, err := r.executor.UpgradeChart(ctx, chartUpgradeRequest(bundle, release, r.namespace, values, false))
	if err != nil {
		uc.recover(ctx, r, bundle, release, run, err)
		return
	}
	run.EnterHealthCheck(result.Revision, uc.d.Now())
	if !uc.save(ctx, run) {
		return
	}
	if result.Status != "deployed" {
		uc.recover(ctx, r, bundle, release, run, fmt.Errorf("Helm 릴리스 상태가 %s 다", result.Status))
		return
	}
	if err := uc.d.Health.Verify(ctx, r.kubeconfig, releaseNamespace(release, r.namespace), *bundle); err != nil {
		uc.recover(ctx, r, bundle, release, run, fmt.Errorf("상태 검사 실패: %w", err))
		return
	}
	run.Succeed(uc.d.Now())
	uc.save(ctx, run)
}

// recover 는 실패한 업그레이드를 릴리스의 지금 모양을 보고 처리한다. 이 실행이 만든 리비전일 때만
// 직전 리비전으로 되돌린다 — 다른 작업(값 편집 · 복구)이 잡고 있는 릴리스를 되돌리면 그 작업을 지운다.
// 되돌렸으면 출발 버전으로 같은 상태 검사를 한다. 확신할 수 없으면 사람에게 넘긴다.
func (uc *UpgradeStack) recover(ctx context.Context, r *resolvedUpgrade, bundle *domain.UpgradeBundle, release *port.ReleaseInfo, run *domain.UpgradeRun, cause error) {
	now := uc.d.Now
	manual := func(reason string, args ...any) {
		run.RequireManualIntervention(fmt.Errorf("%v — "+reason, append([]any{cause}, args...)...), now())
		uc.save(ctx, run)
	}
	// API 종료 · 시간 초과. Helm --atomic 이 스스로 되돌리는 중일 수 있어 그 위에서 또 되돌리지 않는다.
	if ctx.Err() != nil {
		manual("API 종료 또는 시간 초과로 중단됐다. Helm --atomic 이 되돌렸는지 확인하라")
		return
	}
	ns := releaseNamespace(release, r.namespace)
	current, err := uc.currentRelease(ctx, r, release.ReleaseName)
	switch {
	case err != nil:
		manual("릴리스 상태를 확인하지 못해 되돌리지 않았다: %v", err)
		return
	case current.Status == "deployed" && current.ChartVersion == run.SourceChartVersion && current.Revision <= run.PreviousRevision:
		// Helm 이 릴리스를 건드리기 전에 실패했다(차트 내려받기 실패 등) — 되돌릴 것이 없다.
		run.Fail(cause, now())
		uc.save(ctx, run)
		return
	case current.Status == "deployed" && current.ChartVersion == run.SourceChartVersion:
		// --atomic 이 이미 출발 버전으로 되돌렸다. 살아 있는지만 본다.
		run.BeginRollback(cause, now())
		if !uc.save(ctx, run) {
			return
		}
	case current.Revision > run.PreviousRevision && current.ChartVersion == run.TargetChartVersion &&
		(current.Status == "deployed" || current.Status == "failed"):
		// 이 실행이 만든 리비전이다.
		run.BeginRollback(cause, now())
		if !uc.save(ctx, run) {
			return
		}
		if err := r.executor.RollbackRelease(ctx, release.ReleaseName, ns, run.PreviousRevision); err != nil {
			run.RequireManualIntervention(fmt.Errorf("되돌리기 실패: %w", err), now())
			uc.save(ctx, run)
			return
		}
	default:
		manual("릴리스가 예상 밖 상태라(%s, 리비전 %d, 차트 %s) 되돌리지 않았다. 다른 작업이 진행 중일 수 있다",
			current.Status, current.Revision, current.ChartVersion)
		return
	}
	source := *bundle
	source.TargetChartVersion, source.TargetAppVersion = bundle.SourceChartVersion, bundle.SourceAppVersion
	if err := uc.d.Health.Verify(ctx, r.kubeconfig, ns, source); err != nil {
		run.RequireManualIntervention(fmt.Errorf("되돌린 뒤 상태 검사 실패: %w", err), now())
		uc.save(ctx, run)
		return
	}
	run.RolledBack(now())
	uc.save(ctx, run)
}

func (uc *UpgradeStack) currentRelease(ctx context.Context, r *resolvedUpgrade, name string) (*port.ReleaseInfo, error) {
	releases, err := r.releases.ListReleases(ctx, r.namespace)
	if err != nil {
		return nil, err
	}
	for i := range releases {
		if releases[i].ReleaseName == name {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", domain.ErrUpgradeReleaseNotFound, name)
}

// GetRun 은 실행 하나를 돌려준다. 오래 갱신이 없는 진행 중 실행은 중단으로 닫는다.
func (uc *UpgradeStack) GetRun(ctx context.Context, stackID, runID string) (*domain.UpgradeRun, error) {
	if uc.d.Runs == nil {
		return nil, fmt.Errorf("업그레이드 기능이 구성되지 않았다")
	}
	run, err := uc.d.Runs.GetRun(ctx, strings.TrimSpace(stackID), strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	return uc.closeIfStale(ctx, run), nil
}

// closeIfStale 은 박동이 끊긴 진행 중 실행을 중단으로 닫는다. 그 사이 박동이 들어와 닫지 못했으면
// 지금 상태를 다시 읽어 돌려준다.
func (uc *UpgradeStack) closeIfStale(ctx context.Context, run *domain.UpgradeRun) *domain.UpgradeRun {
	now := uc.d.Now()
	if run == nil || !run.Stale(now) {
		return run
	}
	closed := *run
	closed.Interrupt(now)
	ok, err := uc.d.Runs.CloseStaleRun(ctx, &closed, now.Add(-domain.UpgradeHeartbeatTimeout))
	if err != nil {
		uc.d.Logger.Error("close stale upgrade run", "run_id", run.ID, "error", err)
		return run
	}
	if ok {
		return &closed
	}
	if fresh, err := uc.d.Runs.GetRun(ctx, run.StackID, run.ID); err == nil {
		return fresh
	}
	return run
}

// save 는 실행 ctx 가 끝났어도 상태를 남긴다. 일시 오류는 다시 시도한다. 실행이 다른 곳에서
// 닫혔으면 false — 호출자는 거기서 멈춘다.
func (uc *UpgradeStack) save(ctx context.Context, run *domain.UpgradeRun) bool {
	var err error
	for attempt := 0; attempt < upgradePersistAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(uc.d.RetryBackoff << (attempt - 1))
		}
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), upgradePersistTimeout)
		err = uc.d.Runs.UpdateRun(saveCtx, run)
		cancel()
		if err == nil {
			return true
		}
		if errors.Is(err, domain.ErrUpgradeRunClosed) {
			uc.d.Logger.Warn("upgrade run was closed elsewhere — stopping", "run_id", run.ID, "status", run.Status)
			return false
		}
	}
	uc.d.Logger.Error("persist upgrade run", "run_id", run.ID, "status", run.Status, "error", err)
	return true
}

func (uc *UpgradeStack) resolveBundle(ctx context.Context, stackID, bundleID string) (*resolvedUpgrade, *domain.UpgradeBundle, *port.ReleaseInfo, error) {
	r, err := uc.resolve(ctx, stackID)
	if err != nil {
		return nil, nil, nil, err
	}
	bundle, err := uc.d.Runs.GetVerifiedBundle(ctx, strings.TrimSpace(bundleID))
	if err != nil {
		return nil, nil, nil, err
	}
	releases, err := r.releases.ListReleases(ctx, r.namespace)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list releases: %w", err)
	}
	for i := range releases {
		if releases[i].ReleaseName == bundle.ReleaseName {
			return r, bundle, &releases[i], nil
		}
	}
	return nil, nil, nil, fmt.Errorf("%w: %s", domain.ErrUpgradeReleaseNotFound, bundle.ReleaseName)
}

func chartUpgradeRequest(b *domain.UpgradeBundle, rel *port.ReleaseInfo, stackNamespace string, values map[string]any, dryRun bool) port.ChartUpgradeRequest {
	return port.ChartUpgradeRequest{ReleaseName: rel.ReleaseName, ChartName: b.ChartName, RepoURL: b.RepoURL,
		Version: b.TargetChartVersion, Namespace: releaseNamespace(rel, stackNamespace), Values: values, DryRun: dryRun}
}

func releaseNamespace(rel *port.ReleaseInfo, fallback string) string {
	if ns := strings.TrimSpace(rel.Namespace); ns != "" {
		return ns
	}
	return fallback
}
