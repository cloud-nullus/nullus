package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

type fakeStackRepo struct {
	stacks []*stackdomain.Stack
	err    error
	gotOrg string
	gotDel bool
}

func (f *fakeStackRepo) List(_ context.Context, orgID string, includeDeleted bool) ([]*stackdomain.Stack, error) {
	f.gotOrg, f.gotDel = orgID, includeDeleted
	return f.stacks, f.err
}

// 설치가 끝난 스택만 백업 세트에 들어간다 — 반쯤 깔린 것, 실패한 것, 네임스페이스 없는 기록은 뺀다.
func TestStackLister_설치_완료_스택만_준다(t *testing.T) {
	repo := &fakeStackRepo{stacks: []*stackdomain.Stack{
		{ID: "stk_done", Namespace: "nullus-a", State: stackdomain.StateCompleted},
		{ID: "stk_installing", Namespace: "nullus-b", State: stackdomain.StateInstalling},
		{ID: "stk_failed", Namespace: "nullus-c", State: stackdomain.StateFailed},
		{ID: "stk_ghost", Namespace: "", State: stackdomain.StateCompleted},
		nil,
		{ID: "stk_done2", Namespace: "nullus-d", State: stackdomain.StateCompleted},
	}}

	got, err := NewStackLister(repo).ListCompletedStacks(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, []domain.StackRef{
		{ID: "stk_done", Namespace: "nullus-a"},
		{ID: "stk_done2", Namespace: "nullus-d"},
	}, got)
	assert.Equal(t, "org-1", repo.gotOrg)
	assert.False(t, repo.gotDel, "지운 스택은 읽지 않는다")
}

func TestStackLister_저장소_오류는_그대로_알린다(t *testing.T) {
	repo := &fakeStackRepo{err: errors.New("db down")}
	_, err := NewStackLister(repo).ListCompletedStacks(context.Background(), "org-1")
	require.Error(t, err)
}

// 배선되지 않았으면 빈 목록이 아니라 오류다 — 빈 목록은 "플랫폼만 뜬 세트" 를 전체 백업으로 보이게 한다.
func TestStackLister_배선되지_않으면_오류다(t *testing.T) {
	_, err := NewStackLister(nil).ListCompletedStacks(context.Background(), "org-1")
	require.Error(t, err)
	var nilLister *StackLister
	_, err = nilLister.ListCompletedStacks(context.Background(), "org-1")
	require.Error(t, err)
}
