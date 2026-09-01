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
	t.Helper()
	tr := &trace{}
	repo := newFakeRepo()
	store := newFakeStore(tr)
	dumper := &fakeDumper{tr: tr, schema: domain.SchemaState{Version: 74}}
	kv := &fakeKV{tr: tr}
	scaler := &fakeScaler{tr: tr, workloads: []domain.Workload{
		{Kind: "Deployment", Namespace: "nullus", Name: "gitlab", Replicas: 2},
	}}
	archiver := &fakeArchiver{tr: tr, pvcs: []domain.VolumeSpec{{Name: "data-gitlab", SizeBytes: 100}}}
	targets := DBTargets{
		Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
		Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
	}

	buc := NewBackupUseCase(BackupDeps{
		Repo: repo, Dumper: dumper, KV: kv, Scaler: scaler, Archiver: archiver,
		Resources: &fakeResources{tr: tr}, Sealer: fakeSealer{}, Store: store,
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
		Resources: &fakeResources{tr: tr}, Sealer: fakeSealer{}, Store: store,
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
