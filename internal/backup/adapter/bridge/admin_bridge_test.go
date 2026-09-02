package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
)

type stubRepo struct {
	items []*admindomain.TokenSource
	err   error
}

func (s *stubRepo) ListSources(context.Context, string) ([]*admindomain.TokenSource, error) {
	return s.items, s.err
}

func TestListPaths_경로만_넘긴다(t *testing.T) {
	l := NewTokenSourceLister(&stubRepo{items: []*admindomain.TokenSource{
		{ID: "t1", Path: "kv/nullus/dev/o/cicd/github/api-token"},
		{ID: "t2", Path: "kv/nullus/dev/o/cicd/gitlab/api-token"},
	}})

	refs, err := l.ListPaths(context.Background(), "o")
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, "t1", refs[0].ID)
}

func TestListPaths_빈_경로는_건너뛴다(t *testing.T) {
	// 경로가 없는 항목은 금고에서 확인할 대상이 아니다.
	l := NewTokenSourceLister(&stubRepo{items: []*admindomain.TokenSource{
		{ID: "t1", Path: ""},
		{ID: "t2", Path: "kv/x"},
	}})
	refs, err := l.ListPaths(context.Background(), "o")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, "t2", refs[0].ID)
}

func TestListPaths_repo가_없으면_빈_목록(t *testing.T) {
	refs, err := NewTokenSourceLister(nil).ListPaths(context.Background(), "o")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestListPaths_오류를_전달한다(t *testing.T) {
	_, err := NewTokenSourceLister(&stubRepo{err: errors.New("db down")}).
		ListPaths(context.Background(), "o")
	require.Error(t, err)
}

type stubPausable struct{ paused, resumed int }

func (s *stubPausable) Pause()  { s.paused++ }
func (s *stubPausable) Resume() { s.resumed++ }

func TestRotationPauser(t *testing.T) {
	target := &stubPausable{}
	p := NewRotationPauser(target)

	require.NoError(t, p.Pause(context.Background()))
	require.NoError(t, p.Resume(context.Background()))
	assert.Equal(t, 1, target.paused)
	assert.Equal(t, 1, target.resumed)
}

func TestRotationPauser_대상이_없어도_안전하다(t *testing.T) {
	// 로컬 개발처럼 회전 스케줄러가 없는 구성에서도 백업은 돌아야 한다.
	p := NewRotationPauser(nil)
	require.NoError(t, p.Pause(context.Background()))
	require.NoError(t, p.Resume(context.Background()))
}
