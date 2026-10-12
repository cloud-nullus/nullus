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
	Repo      port.BackupRepository
	Dumper    port.DBDumper
	KV        port.KVExporter
	Scaler    port.WorkloadScaler
	Archiver  port.VolumeArchiver
	Resources port.ResourceDumper
	// Shared 는 클러스터 공용 자원을 없을 때만 되살린다. Resources 와 같이 스택 클러스터 어댑터다.
	Shared port.SharedResourceDumper
	// SharedComponents 는 공용 자원을 되살리기 전에 cert-manager·metrics-server 를 다시 세운다(스택 모듈의
	// 설치기). 없으면 컨트롤러가 이미 있다고 보고 건너뛴다 — 없으면 ApplyShared 가 무엇을 깔아야 하는지
	// 말하고 멈춘다.
	SharedComponents port.SharedComponentsEnsurer
	// Namespaces 는 복구할 네임스페이스를 만든다. 없으면 이미 있다고 보고 건너뛴다.
	Namespaces port.NamespaceEnsurer
	// Connections 는 플랫폼 DB 를 되돌린 직후 연결 풀을 비운다 — 옛 테이블을 가리키는 준비된 문장이
	// 다음 질의를 0A000 으로 죽인다. 없으면 건너뛴다(테스트·풀 없는 구성).
	Connections port.ConnectionResetter
	// GatewayAliases 는 복구 뒤 러너·Argo CD hostAliases 를 게이트웨이의 지금 주소로 맞춘다.
	// 없으면 건너뛴다.
	GatewayAliases port.GatewayAliasRepinner
	// StackKubeconfig 는 복구가 붙은 스택 클러스터의 kubeconfig 다. 복구를 시작할 때(DB 를 되돌리기
	// 전) 읽은 것이라, full 복구가 DB 의 클러스터 정보를 백업 시점으로 돌려도 실제 대상을 가리킨다.
	StackKubeconfig []byte
	Sealer          port.Sealer
	Store           port.ArtifactStore
	TokenSources    port.TokenSourceLister
	Targets         DBTargets

	// Prereq 는 실행 시점의 전제 조건을 읽어온다. 함수로 둔 이유는 키의
	// 존재 여부가 프로세스 수명 동안 바뀔 수 있기 때문이다.
	Prereq func(context.Context) domain.Prerequisites

	// Releases 는 대상 네임스페이스에 이미 설치된 차트를 읽는다. 없으면
	// 버전 대조를 건너뛴다 — 대조 없이도 복구는 성립한다.
	Releases port.HelmReleaseLister

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

	// ── 2b. 차트 버전 대조 ──
	//
	// 대상에 이미 다른 버전이 깔려 있으면 복구는 **새 버전 위에 옛 데이터를
	// 얹는다.** 도구가 기동하며 스키마를 올리면 백업 시점으로 돌아가지 않고,
	// 그 변형은 되돌릴 수 없다.
	//
	// 막지는 않는다 — 사용자가 알고 하는 경우(버전 올린 뒤 데이터만 회수)가
	// 있고, 여기서 멈추면 그 길이 막힌다. 대신 무엇이 어긋났는지 남긴다.
	if uc.d.Releases != nil && strings.TrimSpace(req.Namespace) != "" {
		if now, err := uc.d.Releases.ListHelmReleases(ctx, req.Namespace); err != nil {
			uc.d.Logger.Warn("대상 차트 버전을 읽지 못해 대조를 건너뜁니다",
				"namespace", req.Namespace, "error", err)
		} else if drift := domain.CompareHelmReleases(manifest.HelmReleases, now); len(drift) > 0 {
			for _, d := range drift {
				uc.d.Logger.Warn("차트 버전이 백업 시점과 다릅니다 — 기동 시 스키마가 올라가면 되돌릴 수 없습니다",
					"release", d.Release, "backed_up", d.BackedUp, "current", d.Current)
			}
		}
	}

	touchStack := req.Mode != domain.ModePlatformOnly
	touchPlatform := req.Mode != domain.ModeStackOnly

	// ── 3~5. 스택: 공용 컨트롤러·자원 → PVC → 볼륨 → 리소스. 워크로드는 아직 띄우지 않는다 ──
	var sharedComponents *domain.SharedComponentsOutcome
	if touchStack {
		outcome, err := uc.restoreStackData(ctx, req, manifest, arts)
		if err != nil {
			return err
		}
		sharedComponents = outcome
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
	rr.IntegrityReport.SharedComponents = sharedComponents

	// ── 10. 게이트웨이 주소. 워크로드를 띄운 뒤 지금 주소로 hostAliases 를 맞춘다 ──
	//
	// Service 는 백업 때의 ClusterIP 로 되살아나지만, 그 IP 를 쓸 수 없거나(다른 클러스터) 백업에
	// IP 가 없으면(옛 백업) 주소가 바뀐다. 실패해도 복구는 성공이다 — 데이터는 되돌아왔고, 결과를
	// 기록에 남겨 손으로 맞출 수 있게 한다.
	if touchStack && uc.d.GatewayAliases != nil && strings.TrimSpace(backup.StackID) != "" {
		rr.IntegrityReport.GatewayAliases = uc.repinGatewayAliases(ctx, backup.StackID, req.Namespace)
	}
	return nil
}

func (uc *RestoreUseCase) repinGatewayAliases(ctx context.Context, stackID, namespace string) *domain.GatewayAliasesOutcome {
	ip, updated, err := uc.d.GatewayAliases.RepinGatewayAliases(ctx, stackID, namespace, uc.d.StackKubeconfig)
	out := &domain.GatewayAliasesOutcome{GatewayIP: ip, Updated: updated}
	if err != nil {
		out.Error = err.Error()
		uc.d.Logger.Warn("게이트웨이 주소를 러너·Argo CD 에 맞추지 못했습니다 — hostAliases 를 손으로 맞춰야 합니다",
			"stack_id", stackID, "namespace", namespace, "error", err)
		return out
	}
	if len(updated) > 0 {
		uc.d.Logger.Info("게이트웨이 주소가 바뀌어 러너·Argo CD 의 hostAliases 를 맞췄습니다",
			"stack_id", stackID, "gateway_ip", ip, "releases", updated)
	}
	return out
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

func (uc *RestoreUseCase) restoreStackData(ctx context.Context, req RunRestoreRequest, m domain.Manifest, arts []*domain.Artifact) (*domain.SharedComponentsOutcome, error) {
	// 2c. 네임스페이스 — 새 클러스터나 지운 네임스페이스로 되돌리면 없다. 리소스 덤프는
	// 네임스페이스 안의 것만 담으므로 여기서 만들지 않으면 첫 PVC 생성부터 실패한다.
	if uc.d.Namespaces != nil {
		if err := uc.d.Namespaces.EnsureNamespace(ctx, req.Namespace, req.BackupRunID); err != nil {
			return nil, fmt.Errorf("네임스페이스 준비(%s): %w", req.Namespace, err)
		}
	}

	var shared []*domain.Artifact
	for _, a := range arts {
		if a.Component == domain.ComponentSharedResources {
			shared = append(shared, a)
		}
	}
	if len(shared) > 0 && uc.d.Shared == nil {
		return nil, fmt.Errorf("클러스터 공용 자원 복원이 배선되지 않았습니다")
	}

	// 2d. 내부 CA Secret 선복원 — 컨트롤러보다 **먼저**. cert-manager 설치 단계는 끝에 내부 CA 를
	// 부트스트랩하는데, 그때 Secret 이 있으면 옛 키를 채택하고 없으면 새 키를 발급한다. 새 키가
	// 생긴 뒤에는 없을-때만-만들기(2f)가 옛 키를 되살릴 수 없다. Secret 은 CRD 가 필요 없다.
	for _, a := range shared {
		if err := uc.pullStream(ctx, a, func(r io.Reader) error {
			return uc.d.Shared.ApplySharedSecrets(ctx, r)
		}); err != nil {
			return nil, fmt.Errorf("내부 CA Secret 선복원: %w", err)
		}
	}

	// 2e. 공용 컨트롤러 — cert-manager·metrics-server 를 설치기로 다시 세운다. 클러스터의 마지막
	// 스택을 지우면 회수되는데, 컨트롤러는 설치물이라 백업이 담지 않는다. 설치기는 이미 있으면 다시
	// 쓰므로 다른 스택이 쓰는 클러스터에서도 안전하다. cert-manager 실패는 여기서 멈춘다 — 빈
	// 네임스페이스와 CA Secret 말고는 아직 아무것도 건드리지 않았다.
	var outcome *domain.SharedComponentsOutcome
	if uc.d.SharedComponents != nil {
		steps, warnings, err := uc.d.SharedComponents.EnsureSharedComponents(ctx, uc.d.StackKubeconfig, req.Namespace)
		if err != nil {
			return nil, fmt.Errorf("공용 컨트롤러 재설치: %w", err)
		}
		outcome = &domain.SharedComponentsOutcome{Steps: steps, Warnings: warnings}
		for _, w := range warnings {
			uc.d.Logger.Warn("공용 컨트롤러를 다 세우지 못했습니다 — 복구는 계속합니다", "warning", w)
		}
	}

	// 2f. 클러스터 공용 자원 — 네임스페이스 리소스가 기대는 것(Gateway CRD·GatewayClass·내부 CA
	// ClusterIssuer·Certificate)을 볼륨보다 **먼저**, 없을 때만 되살린다.
	//
	// 네임스페이스 준비 뒤다 — 내부 CA 가 스택 네임스페이스에 있는 구성(cert-manager 를 스택
	// 네임스페이스에 깐 경우)에서 공용 자원 복원이 먼저 네임스페이스를 만들면 복구 라벨 없이 생긴다.
	// 실패하면 여기서 멈춘다. 볼륨까지 되돌린 뒤 Gateway CR 에서 "no matches for kind" 로 죽으면
	// 되돌릴 방법이 없다.
	for _, a := range shared {
		if err := uc.pullStream(ctx, a, func(r io.Reader) error {
			return uc.d.Shared.ApplyShared(ctx, r)
		}); err != nil {
			return nil, fmt.Errorf("클러스터 공용 자원 복원: %w", err)
		}
	}

	// 3. PVC 재생성 — 매니페스트의 크기·StorageClass 대로.
	for _, v := range m.Volumes {
		if err := uc.d.Archiver.EnsurePVC(ctx, req.Namespace, v, req.BackupRunID); err != nil {
			return nil, fmt.Errorf("PVC 재생성(%s): %w", v.Name, err)
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
			return nil, fmt.Errorf("볼륨 복원(%s): %w", a.ResourceName, err)
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
			return nil, fmt.Errorf("네임스페이스 리소스 복원: %w", err)
		}
	}
	return outcome, nil
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
			// 테이블이 새로 만들어졌다 — 풀이 들고 있던 준비된 문장은 옛것을 가리킨다. 이 뒤의
			// 질의(토큰 참조 검사·스택 조회·복구 기록 갱신)가 새 연결을 쓰게 비운다.
			if uc.d.Connections != nil {
				uc.d.Connections.Reset()
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
		if w.OwnerManaged {
			// 오퍼레이터가 CR 에서 다시 만드는 것(Prometheus·Alertmanager StatefulSet). 덤프가 담지
			// 않았으므로 지금은 없거나 오퍼레이터가 막 만드는 중이다 — 조정하려 들면 "not found" 를
			// 수동 조치 경보로 남긴다. 오퍼레이터가 CR 의 replicas 로 세운다.
			uc.d.Logger.Info("소유자가 다시 만드는 워크로드는 재개하지 않습니다", "kind", w.Kind, "name", w.Name)
			continue
		}
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
