package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
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
		Resources: &fakeResources{tr: tr},
		Sealer:    fakeSealer{},
		Store:     store,
		Notifier:  notifier,
		Pauser:    &fakePauser{tr: tr},
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
		OrgID: "org-1", Namespace: "nullus", Mode: domain.ModePlatformOnly,
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
