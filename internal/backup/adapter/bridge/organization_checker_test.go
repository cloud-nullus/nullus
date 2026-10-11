package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
)

type fakeOrgRepo struct {
	org   *admindomain.Organization
	err   error
	asked []string
}

func (f *fakeOrgRepo) GetByID(_ context.Context, id string) (*admindomain.Organization, error) {
	f.asked = append(f.asked, id)
	return f.org, f.err
}

func TestOrganizationChecker_있는_조직(t *testing.T) {
	repo := &fakeOrgRepo{org: &admindomain.Organization{ID: "org-1"}}
	ok, err := NewOrganizationChecker(repo).OrganizationExists(context.Background(), "org-1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"org-1"}, repo.asked)
}

// admin 저장소는 없는 조직에 (nil, nil) 을 돌려준다 — 그것이 "없음" 이다.
func TestOrganizationChecker_없는_조직(t *testing.T) {
	ok, err := NewOrganizationChecker(&fakeOrgRepo{}).OrganizationExists(context.Background(), "ghost")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOrganizationChecker_조회_오류는_그대로(t *testing.T) {
	_, err := NewOrganizationChecker(&fakeOrgRepo{err: errors.New("db down")}).
		OrganizationExists(context.Background(), "org-1")
	require.Error(t, err)
}

func TestOrganizationChecker_저장소가_없으면_모른다고_한다(t *testing.T) {
	_, err := NewOrganizationChecker(nil).OrganizationExists(context.Background(), "org-1")
	require.Error(t, err)
}
