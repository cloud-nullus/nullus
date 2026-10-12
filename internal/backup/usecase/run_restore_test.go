package usecase

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// 백업을 한 번 돌려 복구 대상을 만든다. 실제 산출물이 스토어에 있어야
// 복구가 그것을 읽는 경로까지 검증된다.
func seedBackup(t *testing.T, mode domain.Mode) (*RestoreUseCase, *trace, *fakeRepo, *fakeDumper, *fakeKV, *fakeScaler, string) {
	return seedBackupWithShared(t, mode, nil)
}

// seedBackupWithShared 는 공용 자원 복원이 실패하는 경우를 흉내 낼 수 있다.
func seedBackupWithShared(t *testing.T, mode domain.Mode, sharedApplyErr error) (*RestoreUseCase, *trace, *fakeRepo, *fakeDumper, *fakeKV, *fakeScaler, string) {
	return seedBackupWithComponents(t, mode, sharedApplyErr, nil)
}

// seedBackupWithComponents 는 공용 컨트롤러 보장 대역까지 끼운다. nil 이면 배선 없음.
func seedBackupWithComponents(t *testing.T, mode domain.Mode, sharedApplyErr error, sharedComponents port.SharedComponentsEnsurer) (*RestoreUseCase, *trace, *fakeRepo, *fakeDumper, *fakeKV, *fakeScaler, string) {
	t.Helper()
	tr := &trace{}
	repo := newFakeRepo()
	store := newFakeStore(tr)
	dumper := &fakeDumper{tr: tr, schema: domain.SchemaState{Version: 74}}
	kv := &fakeKV{tr: tr}
	scaler := &fakeScaler{tr: tr, workloads: []domain.Workload{
		{Kind: "Deployment", Namespace: "nullus", Name: "gitlab", Replicas: 2},
		// 오퍼레이터가 CR 에서 만드는 것 — 정지는 하되 복구의 재개는 건너뛴다.
		{Kind: "StatefulSet", Namespace: "nullus", Name: "prometheus-kps", Replicas: 1, OwnerManaged: true},
	}}
	archiver := &fakeArchiver{tr: tr, pvcs: []domain.VolumeSpec{{Name: "data-gitlab", SizeBytes: 100}}}
	targets := DBTargets{
		Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
		Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
	}

	buc := NewBackupUseCase(BackupDeps{
		Repo: repo, Dumper: dumper, KV: kv, Scaler: scaler, Archiver: archiver,
		Resources: &fakeResources{tr: tr}, Shared: &fakeShared{tr: tr, payload: "shared"},
		Sealer: fakeSealer{}, Store: store,
		Notifier: &fakeNotifier{tr: tr}, Targets: targets,
	})
	run, err := buc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus", Mode: mode,
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusSucceeded, run.Status)

	tr.steps = nil // 백업 단계는 지우고 복구 순서만 본다

	ruc := NewRestoreUseCase(RestoreDeps{
		Repo: repo, Dumper: dumper, KV: kv, Scaler: scaler, Archiver: archiver,
		Resources: &fakeResources{tr: tr}, Shared: &fakeShared{tr: tr, applyErr: sharedApplyErr},
		SharedComponents: sharedComponents,
		Namespaces:       &fakeNamespaces{tr: tr},
		Connections:      &fakeResetter{tr: tr},
		Sealer:           fakeSealer{}, Store: store,
		Targets: targets,
		TokenSources: &fakeTokenLister{refs: []port.TokenSourceRef{
			{ID: "ts-1", Path: "kv/nullus/dev/org-1/cicd/github/api-token"},
		}},
		Prereq: func(context.Context) domain.Prerequisites {
			return domain.Prerequisites{
				EncryptionKeyPresent: true, BackupSealKeyPresent: true,
				DestinationCredsPresent: true, DestinationReachable: true,
			}
		},
	})
	return ruc, tr, repo, dumper, kv, scaler, run.ID
}

func TestRunRestore_full_성공(t *testing.T) {
	uc, _, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, rr.Status)
	assert.True(t, rr.SchemaCheck.Allowed)
}

// 설계 §9 F4 / §6.1 — 볼륨을 채우기 전에 워크로드가 뜨면 도구들이 빈 디스크를
// 보고 재초기화한다. GitLab 이 새 인스턴스가 되는 순간 복원은 무의미해진다.
func TestRunRestore_볼륨_복원이_워크로드_재개보다_먼저다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)

	vol := tr.indexOf("vol.restore:data-gitlab")
	up := tr.indexOf("scale.up:gitlab")
	require.NotEqual(t, -1, vol, "볼륨 복원이 실행돼야 한다")
	require.NotEqual(t, -1, up, "워크로드가 재개돼야 한다")
	assert.Less(t, vol, up, "데이터를 먼저, 프로세스를 나중에")
}

func TestRunRestore_PVC를_먼저_만든다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Less(t, tr.indexOf("vol.ensure:data-gitlab"), tr.indexOf("vol.restore:data-gitlab"))
}

// 새 클러스터나 지운 네임스페이스로 복구하면 네임스페이스부터 없다. 만들지 않으면 첫 PVC
// 생성이 "namespaces not found" 로 실패한다 — 백업이 멀쩡해도 복구가 시작조차 못 한다.
func TestRunRestore_네임스페이스를_PVC보다_먼저_만든다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)

	ns := tr.indexOf("ns.ensure:nullus")
	require.NotEqual(t, -1, ns, "복구할 네임스페이스를 보장해야 한다")
	assert.Less(t, ns, tr.indexOf("vol.ensure:data-gitlab"))
	assert.Less(t, ns, tr.indexOf("res.apply"))
}

// 복구로 게이트웨이 주소가 바뀌면(다른 클러스터·ClusterIP 없는 옛 백업) 러너·Argo CD 의 hostAliases 가
// 옛 주소를 가리킨다. 워크로드를 다시 띄운 뒤 지금 주소로 맞추고, 결과를 복구 기록에 남긴다.
func TestRunRestore_워크로드_재개_뒤_게이트웨이_주소를_다시_맞춘다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	repin := &fakeGatewayAliases{tr: tr, ip: "10.96.200.9", updated: []string{"argo-cd", "gitlab-runner"}}
	uc.d.GatewayAliases = repin
	// 복구를 시작할 때(DB 를 되돌리기 전) 붙은 클러스터다. full 복구가 DB 의 클러스터 정보를 백업
	// 시점으로 돌려도 재설정은 실제로 되돌린 클러스터를 봐야 한다.
	uc.d.StackKubeconfig = []byte("restored-cluster")

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)

	assert.Equal(t, "stack-1", repin.stackID)
	assert.Equal(t, "nullus", repin.namespace)
	assert.Equal(t, "restored-cluster", repin.kubeconfig)
	assert.Less(t, tr.indexOf("scale.up:gitlab"), tr.indexOf("gateway.repin"), "게이트웨이 Service 가 선 뒤에 읽는다")
	require.NotNil(t, rr.IntegrityReport.GatewayAliases)
	assert.Equal(t, "10.96.200.9", rr.IntegrityReport.GatewayAliases.GatewayIP)
	assert.Equal(t, []string{"argo-cd", "gitlab-runner"}, rr.IntegrityReport.GatewayAliases.Updated)
}

// 재설정은 덤이다 — 실패해도 복구는 성공으로 두고 이유를 기록에 남긴다. 데이터는 이미 되돌아왔다.
func TestRunRestore_게이트웨이_주소_재설정이_실패해도_복구는_성공한다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	uc.d.GatewayAliases = &fakeGatewayAliases{tr: tr, err: errors.New("게이트웨이 Service 가 없다")}

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, rr.Status)
	require.NotNil(t, rr.IntegrityReport.GatewayAliases)
	assert.Contains(t, rr.IntegrityReport.GatewayAliases.Error, "게이트웨이 Service 가 없다")
}

// 플랫폼만 되돌리는 복구는 스택 네임스페이스를 건드리지 않는다.
func TestRunRestore_플랫폼만이면_네임스페이스를_만들지_않는다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Mode: domain.ModePlatformOnly,
	})
	require.NoError(t, err)
	assert.Equal(t, -1, tr.indexOf("ns.ensure:nullus"))
}

func TestRunRestore_플랫폼만이면_게이트웨이_주소를_건드리지_않는다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	uc.d.GatewayAliases = &fakeGatewayAliases{tr: tr}

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Mode: domain.ModePlatformOnly,
	})
	require.NoError(t, err)
	assert.Equal(t, -1, tr.indexOf("gateway.repin"))
	assert.Nil(t, rr.IntegrityReport.GatewayAliases)
}

// 설계 §6.1 0단계 — 키 없이 진행하면 "복구된 것처럼 보이는 망가진 상태" 가 된다.
func TestRunRestore_전제조건_미충족이면_아무것도_하지_않는다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	uc.d.Prereq = func(context.Context) domain.Prerequisites {
		return domain.Prerequisites{BackupSealKeyPresent: true, DestinationCredsPresent: true, DestinationReachable: true}
	}

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY")
	assert.Equal(t, domain.StatusFailed, rr.Status)
	assert.Empty(t, tr.all(), "어떤 단계도 실행되지 않았어야 한다")
}

// 설계 §6.2 — 구버전 코드가 신버전 스키마를 읽는 것은 정의되지 않은 동작이다.
func TestRunRestore_백업_스키마가_더_높으면_차단(t *testing.T) {
	uc, tr, repo, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	run, _ := repo.GetRun(context.Background(), backupID)
	run.SchemaVersion = 999

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.Error(t, err)
	assert.False(t, rr.SchemaCheck.Allowed)
	assert.Equal(t, -1, tr.indexOf("db.restore:platform_db"), "DB 를 건드리지 않았어야 한다")
}

func TestRunRestore_dirty_면_차단(t *testing.T) {
	uc, _, _, dumper, _, _, backupID := seedBackup(t, domain.ModeFull)
	dumper.schema = domain.SchemaState{Version: 74, Dirty: true}

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.Error(t, err)
}

// 설계 §6.3 — users.password_hash 경로가 살아 있으면 관리자는 들어갈 수 있다.
func TestRunRestore_Keycloak_실패는_중단시키지_않는다(t *testing.T) {
	uc, tr, _, dumper, _, _, backupID := seedBackup(t, domain.ModeFull)
	dumper.restoreErr = nil

	// keycloak 만 실패시키기 위해 복원 훅을 감싼다.
	orig := uc.d.Dumper
	uc.d.Dumper = &selectiveFailDumper{DBDumper: orig, failOn: domain.ComponentKeycloakDB}

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err, "Keycloak 실패로 복구 전체를 멈추지 않는다")
	assert.Equal(t, domain.StatusSucceeded, rr.Status)
	assert.NotEqual(t, -1, tr.indexOf("scale.up:gitlab"), "나머지 복구는 계속된다")
}

// 설계 §6.4 — dangling 은 조용히 지나갔다가 파이프라인 실행 시점에 드러난다.
func TestRunRestore_참조_정합성_검사_결과를_남긴다(t *testing.T) {
	uc, _, _, _, kv, _, backupID := seedBackup(t, domain.ModeFull)
	kv.missing = map[string]bool{"kv/nullus/dev/org-1/cicd/github/api-token": true}

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull, OrgID: "org-1",
	})
	require.NoError(t, err, "dangling 은 경고이지 중단 사유가 아니다")
	require.True(t, rr.IntegrityReport.HasIssues())
	assert.Equal(t, "ts-1", rr.IntegrityReport.Dangling[0].TokenSourceID)
	assert.Equal(t, 1, rr.IntegrityReport.CheckedPaths)
}

// 설계 §6.5 — platform-only 는 스택을 건드리지 않는다.
func TestRunRestore_platform_only_는_스택을_건드리지_않는다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModePlatformOnly,
	})
	require.NoError(t, err)

	assert.Equal(t, -1, tr.indexOf("vol.restore:data-gitlab"))
	assert.Equal(t, -1, tr.indexOf("scale.up:gitlab"))
	assert.NotEqual(t, -1, tr.indexOf("db.restore:platform_db"))
}

func TestRunRestore_stack_only_는_DB를_건드리지_않는다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeStackOnly,
	})
	require.NoError(t, err)

	assert.Equal(t, -1, tr.indexOf("db.restore:platform_db"))
	assert.NotEqual(t, -1, tr.indexOf("vol.restore:data-gitlab"))
}

func TestRunRestore_없는_백업(t *testing.T) {
	uc, _, _, _, _, _, _ := seedBackup(t, domain.ModeFull)
	_, err := uc.Run(context.Background(), RunRestoreRequest{BackupRunID: "nope", Mode: domain.ModeFull})
	require.Error(t, err)
}

func TestRunRestore_실패한_백업본은_복구_대상이_아니다(t *testing.T) {
	uc, _, repo, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	run, _ := repo.GetRun(context.Background(), backupID)
	run.Status = domain.StatusFailed

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.Error(t, err)
}

type selectiveFailDumper struct {
	port.DBDumper
	failOn domain.Component
}

func (d *selectiveFailDumper) Restore(ctx context.Context, t port.DBTarget, r io.Reader) error {
	if t.Component == d.failOn {
		return errors.New("keycloak 복원 실패")
	}
	return d.DBDumper.Restore(ctx, t, r)
}

// ── 공용 자원 ────────────────────────────────────────────────────────────

// 공용 자원(Gateway CRD·GatewayClass·내부 CA)은 네임스페이스 리소스가 기대는 것이다.
// 볼륨·리소스보다 먼저 되살려야 하고, 클러스터 범위라 되돌리는 것 자체가 싸다. 네임스페이스
// 준비는 그보다 앞이다 — 공용 자원 복원이 스택 네임스페이스를 먼저 만들면 복구 라벨이 빠진다.
func TestRunRestore_공용_자원을_볼륨보다_먼저_되살린다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)

	shared := tr.indexOf("shared.apply")
	require.GreaterOrEqual(t, shared, 0, "공용 자원을 되살렸어야 한다")
	assert.Less(t, tr.indexOf("ns.ensure:nullus"), shared, "네임스페이스는 복구 라벨과 함께 먼저 선다")
	assert.Less(t, shared, tr.indexOf("vol.ensure:data-gitlab"))
	assert.Less(t, shared, tr.indexOf("res.apply"))
}

// 공용 자원을 되살리지 못하면(예: cert-manager 가 없어 ClusterIssuer 의 CRD 가 없다)
// 아무것도 건드리기 전에 멈춘다. 볼륨까지 되돌린 뒤 죽으면 되돌릴 방법이 없다.
func TestRunRestore_공용_자원_복원이_실패하면_볼륨을_건드리기_전에_멈춘다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackupWithShared(t, domain.ModeFull,
		errors.New("ClusterIssuer 의 CRD 가 없습니다"))

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ClusterIssuer")
	assert.Equal(t, domain.StatusFailed, rr.Status)
	assert.Equal(t, -1, tr.indexOf("vol.ensure:data-gitlab"), "볼륨을 건드리면 안 된다")
	assert.Equal(t, -1, tr.indexOf("vol.restore:data-gitlab"))
	assert.Equal(t, -1, tr.indexOf("db.restore:platform_db"), "플랫폼 DB 도 건드리면 안 된다")
}

// 플랫폼만 되돌리는 복구는 스택 클러스터에 붙지 않는다 — 공용 자원도 건드리지 않는다.
func TestRunRestore_platform_only_는_공용_자원을_건드리지_않는다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)

	_, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Mode: domain.ModePlatformOnly,
	})
	require.NoError(t, err)
	assert.Equal(t, -1, tr.indexOf("shared.apply"))
}

// ── 공용 컨트롤러 ──────────────────────────────────────────────────────

// 클러스터의 마지막 스택을 지우면 cert-manager·metrics-server 까지 회수된다. 컨트롤러는 설치라
// 백업이 담지 않는다 — 복구가 설치기를 불러 다시 세운다. 순서가 핵심이다: CA Secret 을 먼저
// 두고(CRD 불필요) cert-manager 를 세워야 부트스트랩이 새 키를 만들지 않고 옛 키를 채택한다.
func TestRunRestore_CA_Secret_을_먼저_두고_컨트롤러를_세운_뒤_공용_자원을_되살린다(t *testing.T) {
	tr0 := &trace{}
	components := &fakeSharedComponents{tr: tr0}
	uc, tr, _, _, _, _, backupID := seedBackupWithComponents(t, domain.ModeFull, nil, components)
	components.tr = tr

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, rr.Status)

	ns := tr.indexOf("ns.ensure:nullus")
	secrets := tr.indexOf("shared.secrets")
	ensure := tr.indexOf("components.ensure")
	shared := tr.indexOf("shared.apply")
	require.GreaterOrEqual(t, secrets, 0)
	require.GreaterOrEqual(t, ensure, 0)
	assert.Less(t, ns, secrets, "네임스페이스가 먼저")
	assert.Less(t, secrets, ensure, "CA Secret 이 cert-manager 보다 먼저 — 부트스트랩이 옛 키를 채택한다")
	assert.Less(t, ensure, shared, "ClusterIssuer·Certificate 는 cert-manager 가 선 뒤")
	assert.Less(t, shared, tr.indexOf("vol.ensure:data-gitlab"))
	require.NotNil(t, rr.IntegrityReport.SharedComponents)
	assert.Equal(t, []string{"installing_cert_manager", "installing_metrics_server"}, rr.IntegrityReport.SharedComponents.Steps)
}

func TestRunRestore_컨트롤러를_세우지_못하면_볼륨을_건드리기_전에_멈춘다(t *testing.T) {
	components := &fakeSharedComponents{tr: &trace{}, err: errors.New("cert-manager chart pull failed")}
	uc, tr, _, _, _, _, backupID := seedBackupWithComponents(t, domain.ModeFull, nil, components)
	components.tr = tr

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cert-manager")
	assert.Equal(t, domain.StatusFailed, rr.Status)
	assert.Equal(t, -1, tr.indexOf("shared.apply"))
	assert.Equal(t, -1, tr.indexOf("vol.ensure:data-gitlab"))
	assert.Equal(t, -1, tr.indexOf("db.restore:platform_db"))
}

// metrics-server 를 못 세워도 복구는 성공이다 — 사유는 기록에 남긴다.
func TestRunRestore_컨트롤러_경고는_기록에_남는다(t *testing.T) {
	components := &fakeSharedComponents{tr: &trace{}, warnings: []string{"metrics-server 를 다시 세우지 못했습니다"}}
	uc, tr, _, _, _, _, backupID := seedBackupWithComponents(t, domain.ModeFull, nil, components)
	components.tr = tr

	rr, err := uc.Run(context.Background(), RunRestoreRequest{
		BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, rr.Status)
	require.NotNil(t, rr.IntegrityReport.SharedComponents)
	assert.Equal(t, []string{"metrics-server 를 다시 세우지 못했습니다"}, rr.IntegrityReport.SharedComponents.Warnings)
}

// 플랫폼만 되돌리는 복구는 스택 클러스터에 붙지 않는다.
func TestRunRestore_platform_only_는_컨트롤러를_건드리지_않는다(t *testing.T) {
	components := &fakeSharedComponents{tr: &trace{}}
	uc, tr, _, _, _, _, backupID := seedBackupWithComponents(t, domain.ModeFull, nil, components)
	components.tr = tr

	_, err := uc.Run(context.Background(), RunRestoreRequest{BackupRunID: backupID, Mode: domain.ModePlatformOnly})
	require.NoError(t, err)
	assert.Equal(t, -1, tr.indexOf("components.ensure"))
	assert.Equal(t, -1, tr.indexOf("shared.secrets"))
}

// ── 재개 ───────────────────────────────────────────────────────────────

// 오퍼레이터가 CR 을 읽어 만드는 StatefulSet(Prometheus·Alertmanager)은 덤프에 없고(ownerReferences)
// 복구 뒤 오퍼레이터가 CR 에서 다시 만든다. 재개가 그것을 조정하려 들면 아직 없어서 "수동 조치가
// 필요합니다" 를 남긴다 — 거짓 경보다. 2026-10-12 리허설 세 번 모두 그랬다.
func TestRunRestore_소유자_관리_워크로드는_재개하지_않는다(t *testing.T) {
	uc, tr, _, _, _, scaler, backupID := seedBackup(t, domain.ModeFull)
	scaler.scaled = nil // 백업의 정지·재개는 지우고 복구의 재개만 본다

	_, err := uc.Run(context.Background(), RunRestoreRequest{BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull})
	require.NoError(t, err)

	assert.NotEqual(t, -1, tr.indexOf("scale.up:gitlab"), "보통 워크로드는 재개한다")
	assert.Equal(t, -1, tr.indexOf("scale.up:prometheus-kps"), "오퍼레이터가 다시 만든다 — 재개가 건드리지 않는다")
	for _, s := range scaler.scaled {
		assert.NotContains(t, s, "prometheus-kps")
	}
}

// ── DB 연결 풀 ─────────────────────────────────────────────────────────

// pg_restore --clean 은 테이블을 지우고 다시 만든다. 그 사이 API 의 연결 풀이 들고 있던 준비된
// 문장(prepared statement)은 옛 테이블을 가리켜, 복원 직후 첫 질의가 "cached plan must not change
// result type (SQLSTATE 0A000)" 으로 죽는다 — 2026-10-12 리허설 4차에서 게이트웨이 주소 재설정이
// 그렇게 실패했다(어느 연결이 잡히느냐에 따라 간헐적이다). DB 를 되돌린 직후 풀을 비운다.
func TestRunRestore_플랫폼_DB_를_되돌린_뒤_연결_풀을_비운다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	uc.d.GatewayAliases = &fakeGatewayAliases{tr: tr, ip: "10.96.0.9"}

	_, err := uc.Run(context.Background(), RunRestoreRequest{BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeFull})
	require.NoError(t, err)

	db := tr.indexOf("db.restore:platform_db")
	reset := tr.indexOf("db.reset")
	repin := tr.indexOf("gateway.repin")
	require.GreaterOrEqual(t, db, 0)
	require.GreaterOrEqual(t, reset, 0, "DB 를 되돌린 뒤 풀을 비워야 한다")
	assert.Less(t, db, reset, "되돌린 직후")
	assert.Less(t, reset, repin, "그 뒤의 DB 질의(스택 조회)가 새 연결을 쓴다")
}

func TestRunRestore_stack_only_는_연결_풀을_건드리지_않는다(t *testing.T) {
	uc, tr, _, _, _, _, backupID := seedBackup(t, domain.ModeFull)
	_, err := uc.Run(context.Background(), RunRestoreRequest{BackupRunID: backupID, Namespace: "nullus", Mode: domain.ModeStackOnly})
	require.NoError(t, err)
	assert.Equal(t, -1, tr.indexOf("db.reset"), "DB 를 되돌리지 않았으면 비울 이유가 없다")
}
