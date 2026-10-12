package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

// seedSet 은 플랫폼 + 스택 둘을 가진 조직의 세트 유스케이스를 만든다. 팩토리는 스택 ID 를 기록하고
// 같은 대역 어댑터로 백업 유스케이스를 만든다.
func seedSet(t *testing.T, stacks []domain.StackRef) (*BackupSetUseCase, *fakeRepo, *[]string, *trace) {
	t.Helper()
	tr := &trace{}
	repo := newFakeRepo()
	store := newFakeStore(tr)
	dumper := &fakeDumper{tr: tr, schema: domain.SchemaState{Version: 92}}
	targets := DBTargets{
		Platform: dbTarget(domain.ComponentPlatformDB, "nullus"),
		Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak"),
	}
	var asked []string
	factory := func(_ context.Context, stackID string) (*BackupUseCase, error) {
		asked = append(asked, stackID)
		deps := BackupDeps{
			Repo: repo, Dumper: dumper, KV: &fakeKV{tr: tr}, Sealer: fakeSealer{}, Store: store,
			Notifier: &fakeNotifier{tr: tr}, Targets: targets,
		}
		if stackID != "" {
			deps.Scaler = &fakeScaler{tr: tr, workloads: []domain.Workload{{Kind: "Deployment", Namespace: "x", Name: "gitlab", Replicas: 1}}}
			deps.Archiver = &fakeArchiver{tr: tr, pvcs: []domain.VolumeSpec{{Name: "data-" + stackID, SizeBytes: 10}}}
			deps.Resources = &fakeResources{tr: tr}
			deps.Shared = &fakeShared{tr: tr, payload: "shared"}
		}
		return NewBackupUseCase(deps), nil
	}
	uc := NewBackupSetUseCase(BackupSetDeps{Repo: repo, Stacks: &fakeStackLister{stacks: stacks}, Factory: factory})
	return uc, repo, &asked, tr
}

// 플랫폼 전체 = 플랫폼 한 번 + 스택마다 한 번, 같은 세트 ID.
func TestRunBackupSet_플랫폼과_모든_스택을_한_세트로_뜬다(t *testing.T) {
	uc, repo, asked, _ := seedSet(t, []domain.StackRef{
		{ID: "stk_b", Namespace: "nullus-b"}, {ID: "stk_a", Namespace: "nullus-a"},
	})

	res, err := uc.Run(context.Background(), RunBackupSetRequest{OrgID: "org-1", Trigger: domain.TriggerManual})
	require.NoError(t, err)
	require.NotEmpty(t, res.SetID)
	assert.Equal(t, domain.StatusSucceeded, res.Status)
	require.Len(t, res.Runs, 3)

	assert.Equal(t, []string{"", "stk_a", "stk_b"}, *asked, "플랫폼 먼저, 스택은 ID 순")
	for _, r := range res.Runs {
		assert.Equal(t, res.SetID, r.SetID, "모두 같은 세트")
		assert.Equal(t, domain.StatusSucceeded, r.Status)
	}
	assert.Equal(t, domain.ModePlatformOnly, res.Runs[0].Mode)
	assert.Equal(t, domain.ModeStackOnly, res.Runs[1].Mode)
	assert.Contains(t, res.Runs[1].Scope, domain.ComponentOpenBaoKV, "스택 금고는 스택 멤버가 담는다")
	assert.Equal(t, "nullus-a", res.Runs[1].Manifest["namespace"], "복구가 읽을 네임스페이스가 매니페스트에 있다")

	stored, err := repo.ListRunsBySet(context.Background(), res.SetID)
	require.NoError(t, err)
	assert.Len(t, stored, 3)
}

// 스택 하나가 실패해도 나머지는 계속 뜬다 — 일부라도 쓸 수 있는 세트가 아무것도 없는 것보다 낫다.
func TestRunBackupSet_한_멤버가_실패해도_나머지는_뜨고_세트는_partial(t *testing.T) {
	uc, _, _, _ := seedSet(t, []domain.StackRef{{ID: "stk_a", Namespace: "nullus-a"}, {ID: "stk_b", Namespace: "nullus-b"}})
	uc.d.Factory = func(ctx context.Context, stackID string) (*BackupUseCase, error) {
		if stackID == "stk_a" {
			return nil, errors.New("클러스터 kubeconfig 를 풀 수 없습니다")
		}
		return seedSetFactory(t, stackID)
	}

	res, err := uc.Run(context.Background(), RunBackupSetRequest{OrgID: "org-1", Trigger: domain.TriggerSchedule})
	require.NoError(t, err, "멤버 실패는 세트 결과에 남고 오류로 돌려주지 않는다")
	assert.Equal(t, domain.StatusPartial, res.Status)
	require.Len(t, res.Failed, 1)
	assert.Contains(t, res.Failed[0], "stk_a")
	assert.Len(t, res.Runs, 2, "플랫폼 + stk_b")
}

func TestRunBackupSet_스택_목록을_못_읽으면_시작하지_않는다(t *testing.T) {
	uc, _, asked, _ := seedSet(t, nil)
	uc.d.Stacks = &fakeStackLister{err: errors.New("db down")}
	_, err := uc.Run(context.Background(), RunBackupSetRequest{OrgID: "org-1"})
	require.Error(t, err)
	assert.Empty(t, *asked, "무엇을 뜰지 모르면 플랫폼만 뜨고 성공으로 꾸미지 않는다")
}

// seedSetFactory 는 부분 실패 테스트가 쓰는, 스택 ID 를 가리지 않는 백업 유스케이스다.
func seedSetFactory(t *testing.T, stackID string) (*BackupUseCase, error) {
	t.Helper()
	tr := &trace{}
	deps := BackupDeps{
		Repo: newFakeRepo(), Dumper: &fakeDumper{tr: tr, schema: domain.SchemaState{Version: 92}},
		KV: &fakeKV{tr: tr}, Sealer: fakeSealer{}, Store: newFakeStore(tr),
		Notifier: &fakeNotifier{tr: tr},
		Targets:  DBTargets{Platform: dbTarget(domain.ComponentPlatformDB, "nullus"), Keycloak: dbTarget(domain.ComponentKeycloakDB, "keycloak")},
	}
	if stackID != "" {
		deps.Scaler = &fakeScaler{tr: tr, workloads: []domain.Workload{{Kind: "Deployment", Namespace: "x", Name: "gitlab", Replicas: 1}}}
		deps.Archiver = &fakeArchiver{tr: tr, pvcs: []domain.VolumeSpec{{Name: "data-" + stackID, SizeBytes: 10}}}
		deps.Resources = &fakeResources{tr: tr}
		deps.Shared = &fakeShared{tr: tr, payload: "shared"}
	}
	return NewBackupUseCase(deps), nil
}
