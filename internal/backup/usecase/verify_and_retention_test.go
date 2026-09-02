package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

func TestVerify_정상_백업(t *testing.T) {
	tr := &trace{}
	repo := newFakeRepo()
	store := newFakeStore(tr)
	uc := NewBackupUseCase(BackupDeps{
		Repo: repo, Dumper: &fakeDumper{tr: tr, schema: domain.SchemaState{Version: 74}},
		KV: &fakeKV{tr: tr}, Scaler: &fakeScaler{tr: tr}, Archiver: &fakeArchiver{tr: tr},
		Resources: &fakeResources{tr: tr}, Sealer: fakeSealer{}, Store: store,
		Targets: DBTargets{
			Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
			Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
		},
	})
	run, err := uc.Run(context.Background(), RunBackupRequest{OrgID: "o", StackID: "s1", Mode: domain.ModePlatformOnly})
	require.NoError(t, err)

	v := NewVerifyUseCase(repo, store, &fakeDumper{tr: tr, schema: domain.SchemaState{Version: 74}},
		dbTarget(domain.ComponentPlatformDB, "nullus"), nil)
	res, err := v.Verify(context.Background(), run.ID)
	require.NoError(t, err)
	assert.True(t, res.OK)
	assert.Equal(t, 3, res.CheckedCount)
}

func TestVerify_오브젝트가_사라지면_실패(t *testing.T) {
	// 백업 이력은 있는데 실제 산출물이 없는 상태 — 가장 위험한 착각이다.
	tr := &trace{}
	repo := newFakeRepo()
	store := newFakeStore(tr)
	uc := NewBackupUseCase(BackupDeps{
		Repo: repo, Dumper: &fakeDumper{tr: tr}, KV: &fakeKV{tr: tr},
		Scaler: &fakeScaler{tr: tr}, Archiver: &fakeArchiver{tr: tr},
		Resources: &fakeResources{tr: tr}, Sealer: fakeSealer{}, Store: store,
		Targets: DBTargets{
			Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
			Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
		},
	})
	run, _ := uc.Run(context.Background(), RunBackupRequest{OrgID: "o", StackID: "s1", Mode: domain.ModePlatformOnly})

	store.objects = map[string][]byte{} // 목적지에서 사라졌다

	v := NewVerifyUseCase(repo, store, nil, dbTarget(domain.ComponentPlatformDB, "nullus"), nil)
	res, err := v.Verify(context.Background(), run.ID)
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Len(t, res.MissingObjects, 3)
}

func TestVerify_산출물이_없으면_실패(t *testing.T) {
	repo := newFakeRepo()
	run := domain.NewBackupRun("o", domain.ModeFull, domain.TriggerManual, nil)
	require.NoError(t, repo.CreateRun(context.Background(), run))

	v := NewVerifyUseCase(repo, newFakeStore(&trace{}), nil, dbTarget(domain.ComponentPlatformDB, "n"), nil)
	res, err := v.Verify(context.Background(), run.ID)
	require.NoError(t, err)
	assert.False(t, res.OK)
}

func TestRetention_적용(t *testing.T) {
	tr := &trace{}
	repo := newFakeRepo()
	store := newFakeStore(tr)
	now := time.Now().UTC()
	repo.summaries = []domain.RunSummary{
		{ID: "r1", CreatedAt: now, Status: domain.StatusSucceeded, TotalBytes: 10},
		{ID: "r2", CreatedAt: now.AddDate(0, 0, -1), Status: domain.StatusSucceeded, TotalBytes: 10},
		{ID: "r3", CreatedAt: now.AddDate(0, 0, -40), Status: domain.StatusSucceeded, TotalBytes: 10},
	}

	uc := NewRetentionUseCase(repo, store, domain.RetentionPolicy{Daily: 2}, nil)
	deleted, err := uc.Apply(context.Background(), "org-1")
	require.NoError(t, err)

	assert.Equal(t, []string{"r3"}, deleted)
	assert.Contains(t, tr.all(), "store.delete:backup-r3/")
	assert.Equal(t, []string{"r3"}, repo.deleted)
}

func TestRetention_오브젝트_삭제_실패시_기록을_지우지_않는다(t *testing.T) {
	// 오브젝트가 남았는데 기록만 지우면 고아 오브젝트가 되고 용량은 계속 찬다.
	tr := &trace{}
	repo := newFakeRepo()
	store := &failingDeleteStore{fakeStore: newFakeStore(tr)}
	repo.summaries = []domain.RunSummary{
		{ID: "keep", CreatedAt: time.Now(), Status: domain.StatusSucceeded},
		{ID: "old", CreatedAt: time.Now().AddDate(0, 0, -30), Status: domain.StatusSucceeded},
	}

	uc := NewRetentionUseCase(repo, store, domain.RetentionPolicy{Daily: 1}, nil)
	deleted, err := uc.Apply(context.Background(), "org-1")
	require.NoError(t, err)

	assert.Empty(t, deleted)
	assert.Empty(t, repo.deleted, "오브젝트를 못 지웠으면 기록도 남긴다")
}
