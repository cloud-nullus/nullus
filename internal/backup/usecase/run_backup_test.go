package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

func newBackupFixture(t *testing.T) (*BackupUseCase, *trace, *fakeRepo, *fakeStore, *fakeScaler, *fakeArchiver, *fakeNotifier) {
	t.Helper()
	tr := &trace{}
	repo := newFakeRepo()
	store := newFakeStore(tr)
	scaler := &fakeScaler{tr: tr, workloads: []domain.Workload{
		{Kind: "Deployment", Namespace: "nullus", Name: "gitlab", Replicas: 2},
	}}
	archiver := &fakeArchiver{tr: tr, pvcs: []domain.VolumeSpec{{Name: "data-gitlab", SizeBytes: 100}}}
	notifier := &fakeNotifier{tr: tr}

	uc := NewBackupUseCase(BackupDeps{
		Repo:      repo,
		Dumper:    &fakeDumper{tr: tr},
		KV:        &fakeKV{tr: tr},
		Scaler:    scaler,
		Archiver:  archiver,
		Resources: &fakeResources{tr: tr}, Shared: &fakeShared{tr: tr},
		Sealer:   fakeSealer{},
		Store:    store,
		Notifier: notifier,
		Pauser:   &fakePauser{tr: tr},
		Targets: DBTargets{
			Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
			Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
		},
	})
	return uc, tr, repo, store, scaler, archiver, notifier
}

func TestRunBackup_full_모드_성공(t *testing.T) {
	uc, tr, repo, _, _, _, notifier := newBackupFixture(t)

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, run.Status)

	arts, _ := repo.ListArtifacts(context.Background(), run.ID)
	assert.Len(t, arts, 5, "platform/keycloak/kv/resources/volume 다섯 건")

	assert.Equal(t, []domain.Status{domain.StatusSucceeded}, notifier.called)
	assert.NotNil(t, run.QuiesceStartedAt)
	assert.NotNil(t, run.QuiesceEndedAt)
	_ = tr
}

// 설계 §9 F7b·F8 — 정지 창을 소비하고도 산출물을 못 만드는 것이 최악의
// 조합이다. 멈추기 전에 실패할 수 있는 것은 전부 먼저 검사한다.
func TestRunBackup_목적지_검사는_정지보다_먼저다(t *testing.T) {
	uc, tr, _, _, _, _, _ := newBackupFixture(t)

	_, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)

	pre := tr.indexOf("store.preflight")
	down := tr.indexOf("scale.down:gitlab")
	require.NotEqual(t, -1, pre)
	require.NotEqual(t, -1, down)
	assert.Less(t, pre, down, "preflight 가 정지보다 앞이어야 한다")
}

func TestRunBackup_목적지_검사_실패시_정지하지_않는다(t *testing.T) {
	uc, tr, _, store, _, _, _ := newBackupFixture(t)
	store.preflightErr = errors.New("bucket 없음")

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.Error(t, err)
	assert.Equal(t, domain.StatusFailed, run.Status)
	assert.Equal(t, -1, tr.indexOf("scale.down:gitlab"), "정지 창을 열지 않았어야 한다")
	assert.Nil(t, run.QuiesceStartedAt)
}

// 설계 §9 F3 — 최악의 시나리오는 백업하려다 서비스를 못 살리는 것이다.
func TestRunBackup_아카이브가_실패해도_반드시_재개한다(t *testing.T) {
	uc, tr, _, _, _, archiver, _ := newBackupFixture(t)
	archiver.archiveErr = errors.New("tar 실패")

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err, "부분 실패는 에러가 아니라 partial 이다")
	assert.Equal(t, domain.StatusPartial, run.Status)

	assert.NotEqual(t, -1, tr.indexOf("scale.up:gitlab"), "실패해도 재개해야 한다")
	assert.NotNil(t, run.QuiesceEndedAt)
}

func TestRunBackup_리소스_덤프는_정지_전에_한다(t *testing.T) {
	// 워크로드가 살아 있을 때 떠야 실제 배포 상태가 담긴다.
	uc, tr, _, _, _, _, _ := newBackupFixture(t)

	_, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Less(t, tr.indexOf("res.dump"), tr.indexOf("scale.down:gitlab"))
}

func TestRunBackup_platform_only_는_정지하지_않는다(t *testing.T) {
	// 볼륨을 뜨지 않는 백업은 무중단이어야 한다.
	uc, tr, repo, _, _, _, _ := newBackupFixture(t)

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus", Mode: domain.ModePlatformOnly,
	})
	require.NoError(t, err)

	assert.Equal(t, -1, tr.indexOf("scale.down:gitlab"))
	assert.Nil(t, run.QuiesceStartedAt, "정지 창이 없다")

	arts, _ := repo.ListArtifacts(context.Background(), run.ID)
	assert.Len(t, arts, 3, "platform/keycloak/kv 세 건")
}

func TestRunBackup_정지시_회전_스케줄러도_멈춘다(t *testing.T) {
	// 설계 §2.1 — 회전이 DB 와 금고를 5분마다 함께 고쳐써서 skew 를 만든다.
	uc, tr, _, _, _, _, _ := newBackupFixture(t)

	_, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)

	assert.Less(t, tr.indexOf("rotation.pause"), tr.indexOf("scale.down:gitlab"))
	assert.NotEqual(t, -1, tr.indexOf("rotation.resume"))
}

func TestRunBackup_매니페스트에_비밀값이_없다(t *testing.T) {
	uc, _, repo, _, _, _, _ := newBackupFixture(t)

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)

	stored, err := repo.GetRun(context.Background(), run.ID)
	require.NoError(t, err)
	require.NotEmpty(t, stored.Manifest)
	assertNoSecrets(t, stored.Manifest)
}

func TestRunBackup_실패시_알림한다(t *testing.T) {
	uc, _, _, store, _, _, notifier := newBackupFixture(t)
	store.preflightErr = errors.New("연결 불가")

	_, _ = uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	assert.Equal(t, []domain.Status{domain.StatusFailed}, notifier.called,
		"백업 실패보다 실패를 모르는 것이 더 나쁘다")
}

func TestRunBackup_잘못된_모드(t *testing.T) {
	uc, _, _, _, _, _, _ := newBackupFixture(t)
	_, err := uc.Run(context.Background(), RunBackupRequest{OrgID: "o", Mode: "nonsense"})
	require.Error(t, err)
}

func TestRunBackup_스택_대상인데_네임스페이스가_없으면_거부(t *testing.T) {
	uc, _, _, _, _, _, _ := newBackupFixture(t)
	_, err := uc.Run(context.Background(), RunBackupRequest{OrgID: "o", Mode: domain.ModeStackOnly})
	require.Error(t, err)
}

// 리허설에서 걸린 것이다 — 대상이 비어 있으면 pg_dump 가 로컬 소켓에
// 붙으려다 "socket ... No such file" 로 죽는다. 무엇이 잘못됐는지 알 수
// 없는 메시지이고, 기본 설정이 빈 값이라 모든 백업이 partial 이 됐다.
func TestRunBackup_DB_대상이_설정되지_않으면_명확히_알려준다(t *testing.T) {
	uc, _, _, _, _, _, _ := newBackupFixture(t)
	uc.d.Targets.Keycloak = port.DBTarget{Component: domain.ComponentKeycloakDB} // Host 없음

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModeFull,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPartial, run.Status)
	assert.Contains(t, run.Error, "설정되지 않았습니다")
	assert.Contains(t, run.Error, string(domain.ComponentKeycloakDB))
}

// 금고는 스택마다 배포된다. 스택이 없으면 뜰 금고도 없다 — 그것을 실패로
// 세면 스택 설치 전의 플랫폼 백업이 항상 partial 이 된다. 인클러스터
// 리허설에서 실제로 그렇게 됐다.
func TestRunBackup_스택이_없으면_금고를_건너뛴다(t *testing.T) {
	uc, tr, repo, _, _, _, _ := newBackupFixture(t)

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Mode: domain.ModePlatformOnly, // StackID 없음
	})
	require.NoError(t, err)

	assert.Equal(t, domain.StatusSucceeded, run.Status, "없는 것을 못 떴다고 실패가 되면 안 된다")
	assert.Equal(t, -1, tr.indexOf("kv.export"))

	arts, _ := repo.ListArtifacts(context.Background(), run.ID)
	assert.Len(t, arts, 2, "platform/keycloak 두 건만")
}

func TestRunBackup_스택이_있으면_금고를_뜬다(t *testing.T) {
	uc, tr, _, _, _, _, _ := newBackupFixture(t)

	_, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Mode: domain.ModePlatformOnly,
	})
	require.NoError(t, err)
	assert.NotEqual(t, -1, tr.indexOf("kv.export"))
}

func TestRunBackup_고른_대상만_뜬다(t *testing.T) {
	// full 모드라도 사용자가 볼륨을 뺐으면 볼륨을 뜨지 않는다 — 그리고
	// 그러면 서비스를 멈출 이유도 없다. 이유 없이 멈추면 사용자가 백업을
	// 피하게 되고, 피해진 백업은 없는 백업이다.
	uc, _, _, _, scaler, _, _ := newBackupFixture(t)

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus",
		Mode:  domain.ModeFull,
		Scope: []domain.Component{domain.ComponentPlatformDB},
	})
	require.NoError(t, err)
	assert.Equal(t, []domain.Component{domain.ComponentPlatformDB}, run.Scope)
	assert.False(t, run.RequiresQuiesce(), "볼륨을 빼면 정지 창이 없어야 한다")
	assert.Empty(t, scaler.scaled, "멈출 이유가 없는데 워크로드를 내렸다")
}

func TestRunBackup_고르지_않으면_모드에서_파생한다(t *testing.T) {
	// 스케줄 백업과 옛 호출부는 모드만 넘긴다.
	uc, _, _, _, _, _, _ := newBackupFixture(t)

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Namespace: "nullus",
		Mode: domain.ModePlatformOnly, Trigger: domain.TriggerSchedule,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.ModeComponents(domain.ModePlatformOnly), run.Scope)
}

// 스택 클러스터에 닿을 수단(스택 ID 로 만든 어댑터)이 없으면 리소스·볼륨은 뜰 수 없다. 예전
// 화면은 스택을 보내지 않고 full 을 요청했고, 그 요청은 비어 있는 어댑터를 그대로 불렀다.
// 실행 기록을 만들기 전에, 무엇이 빠졌는지 말하며 거절한다.
func TestRunBackup_스택_클러스터_없이_리소스나_볼륨을_고르면_거부한다(t *testing.T) {
	for _, scope := range [][]domain.Component{
		{domain.ComponentVolume},
		{domain.ComponentNamespaceResources},
		nil, // full 모드에서 파생 — 둘 다 담긴다
	} {
		uc, _, repo, _, _, _, _ := newBackupFixture(t)
		uc.d.Scaler, uc.d.Archiver, uc.d.Resources = nil, nil, nil // 스택 ID 없이 만든 유스케이스

		_, err := uc.Run(context.Background(), RunBackupRequest{
			OrgID: "org-1", Namespace: "nullus-app", Mode: domain.ModeFull, Scope: scope,
		})

		var appErr *shareddomain.AppError
		require.ErrorAs(t, err, &appErr, "scope=%v", scope)
		assert.Equal(t, "BACKUP_STACK_REQUIRED", appErr.Code)
		assert.Equal(t, 400, appErr.HTTPStatus)
		runs, _ := repo.ListRuns(context.Background(), "org-1", 0)
		assert.Empty(t, runs, "거절한 요청이 실행 기록을 남기면 '실행 중' 으로 영영 남는다")
	}
}

// DB 만 고르면 스택이 없어도 된다.
func TestRunBackup_스택_클러스터_없이도_DB_는_뜬다(t *testing.T) {
	uc, _, _, _, _, _, _ := newBackupFixture(t)
	uc.d.Scaler, uc.d.Archiver, uc.d.Resources = nil, nil, nil

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", Mode: domain.ModePlatformOnly,
		Scope: []domain.Component{domain.ComponentPlatformDB, domain.ComponentKeycloakDB},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, run.Status)
}

// ── 공용 자원 ────────────────────────────────────────────────────────────
//
// 클러스터의 마지막 스택을 지우면 Nullus 가 설치한 공용 자원(설치 표시가 붙은 Gateway
// CRD·GatewayClass·내부 CA)이 회수된다. 백업이 그것을 담지 않으면 복구는 네임스페이스
// 리소스를 적용하다 "no matches for kind Gateway" 로 죽는다.

func sharedBackupUC(tr *trace, repo *fakeRepo, shared port.SharedResourceDumper) *BackupUseCase {
	return NewBackupUseCase(BackupDeps{
		Repo: repo, Dumper: &fakeDumper{tr: tr}, KV: &fakeKV{tr: tr},
		Scaler: &fakeScaler{tr: tr}, Archiver: &fakeArchiver{tr: tr},
		Resources: &fakeResources{tr: tr}, Shared: shared,
		Sealer: fakeSealer{}, Store: newFakeStore(tr), Notifier: &fakeNotifier{tr: tr},
		Targets: DBTargets{
			Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
			Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
		},
	})
}

func TestRunBackup_공용_자원을_담는다(t *testing.T) {
	tr := &trace{}
	repo := newFakeRepo()
	uc := sharedBackupUC(tr, repo, &fakeShared{tr: tr, payload: "shared"})

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus", Mode: domain.ModeStackOnly,
		Scope: []domain.Component{domain.ComponentSharedResources, domain.ComponentNamespaceResources},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, run.Status)
	assert.Contains(t, tr.all(), "shared.dump")

	arts, _ := repo.ListArtifacts(context.Background(), run.ID)
	var comps []domain.Component
	for _, a := range arts {
		comps = append(comps, a.Component)
	}
	assert.Contains(t, comps, domain.ComponentSharedResources)
}

// 담을 것이 없으면(클러스터에 Nullus 가 설치한 공용 자원이 없다) 빈 산출물을 남기지 않는다 —
// 빈 산출물은 "담았다" 로 읽힌다.
func TestRunBackup_공용_자원이_없으면_산출물을_만들지_않는다(t *testing.T) {
	tr := &trace{}
	repo := newFakeRepo()
	uc := sharedBackupUC(tr, repo, &fakeShared{tr: tr, payload: ""})

	run, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus", Mode: domain.ModeStackOnly,
		Scope: []domain.Component{domain.ComponentSharedResources},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, run.Status)

	arts, _ := repo.ListArtifacts(context.Background(), run.ID)
	for _, a := range arts {
		assert.NotEqual(t, domain.ComponentSharedResources, a.Component)
	}
}

// 공용 자원은 스택 클러스터에서 뜬다. 어댑터가 없으면(스택을 고르지 않음) 다른 스택
// 대상과 같이 실행 기록을 만들기 전에 거절한다.
func TestRunBackup_공용_자원도_스택_어댑터가_필요하다(t *testing.T) {
	tr := &trace{}
	uc := sharedBackupUC(tr, newFakeRepo(), nil)

	_, err := uc.Run(context.Background(), RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus", Mode: domain.ModeStackOnly,
		Scope: []domain.Component{domain.ComponentSharedResources},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shared_resources")
}

// ── 요청이 끊긴 정지 창 ──────────────────────────────────────────────────
//
// 백업은 HTTP 요청 안에서 동기로 돈다. 클라이언트가 떠나면(탭 닫음·프록시 타임아웃) 요청
// 컨텍스트가 취소되고, 재개가 그 컨텍스트에 묶여 있으면 첫 호출부터 실패해 워크로드가 0 으로
// 남는다 — 백업하려다 서비스를 못 살리는 것이 최악이다(§9 F3). 실행 기록도 running 으로 남아
// 사용자는 무엇이 일어났는지 모른다.
func TestRunBackup_요청이_끊겨도_워크로드를_재개하고_기록을_닫는다(t *testing.T) {
	tr := &trace{}
	repo := newFakeRepo()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scaler := &fakeScaler{tr: tr, workloads: []domain.Workload{
		{Kind: "Deployment", Namespace: "nullus", Name: "gitlab", Replicas: 2},
	}}
	// 정지 창 한가운데(볼륨 아카이브)에서 요청이 끊긴다.
	archiver := &fakeArchiver{tr: tr, pvcs: []domain.VolumeSpec{{Name: "data-gitlab", SizeBytes: 100}},
		onArchive: cancel}
	uc := NewBackupUseCase(BackupDeps{
		Repo: repo, Dumper: &fakeDumper{tr: tr}, KV: &fakeKV{tr: tr},
		Scaler: scaler, Archiver: archiver,
		Resources: &fakeResources{tr: tr}, Shared: &fakeShared{tr: tr},
		Sealer: fakeSealer{}, Store: newFakeStore(tr), Notifier: &fakeNotifier{tr: tr},
		Pauser: &fakePauser{tr: tr},
		Targets: DBTargets{
			Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
			Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
		},
	})

	run, _ := uc.Run(ctx, RunBackupRequest{
		OrgID: "org-1", StackID: "stack-1", Namespace: "nullus", Mode: domain.ModeStackOnly,
		Scope: []domain.Component{domain.ComponentVolume},
	})
	require.NotNil(t, run)

	assert.NotEqual(t, domain.StatusSucceeded, run.Status, "끊긴 백업이 성공일 수는 없다")
	assert.Contains(t, scaler.scaled, "gitlab=2", "원래 replica 로 되살려야 한다")
	assert.NotContains(t, tr.all(), "scale.canceled:gitlab=2", "재개는 요청 컨텍스트에 묶이면 안 된다")
	assert.Contains(t, tr.all(), "rotation.resume", "회전 스케줄러도 다시 돌아야 한다")
	require.NotEmpty(t, repo.updatedStatus)
	last := repo.updatedStatus[len(repo.updatedStatus)-1]
	assert.NotEqual(t, domain.StatusRunning, last, "실행 기록이 running 으로 남으면 안 된다")
	assert.Equal(t, run.Status, last)
}
