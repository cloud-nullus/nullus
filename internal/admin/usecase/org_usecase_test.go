package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOrgRepo is an in-memory mock of port.OrgRepository.
type mockOrgRepo struct {
	orgs map[string]*domain.Organization
}

func newMockOrgRepo() *mockOrgRepo {
	return &mockOrgRepo{orgs: make(map[string]*domain.Organization)}
}

func (m *mockOrgRepo) Create(_ context.Context, org *domain.Organization) error {
	m.orgs[org.ID] = org
	return nil
}

func (m *mockOrgRepo) GetByID(_ context.Context, id string) (*domain.Organization, error) {
	return m.orgs[id], nil
}

func (m *mockOrgRepo) List(_ context.Context, limit, offset int) ([]*domain.Organization, error) {
	items := make([]*domain.Organization, 0, len(m.orgs))
	for _, org := range m.orgs {
		items = append(items, org)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*domain.Organization{}, nil
	}

	items = items[offset:]
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	return items, nil
}

func (m *mockOrgRepo) Update(_ context.Context, org *domain.Organization) error {
	if _, ok := m.orgs[org.ID]; !ok {
		return nil
	}
	m.orgs[org.ID] = org
	return nil
}

func (m *mockOrgRepo) GetBySlug(_ context.Context, slug string) (*domain.Organization, error) {
	for _, o := range m.orgs {
		if o.Slug == slug {
			return o, nil
		}
	}
	return nil, nil
}

func TestOrgUseCase_CreateOrg_Success(t *testing.T) {
	repo := newMockOrgRepo()
	uc := NewOrgUseCase(repo)

	org, err := uc.CreateOrg(context.Background(), CreateOrgInput{
		Name:   "Nullus Team",
		Slug:   "nullus-team",
		Domain: "nullus.io",
	})

	require.NoError(t, err)
	assert.Equal(t, "nullus-team", org.Slug)
	assert.Equal(t, "Nullus Team", org.Name)
	assert.Equal(t, domain.OrgStatusActive, org.Status)
	assert.NotEmpty(t, org.ID)
}

func TestOrgUseCase_CreateOrg_InvalidSlug(t *testing.T) {
	repo := newMockOrgRepo()
	uc := NewOrgUseCase(repo)

	_, err := uc.CreateOrg(context.Background(), CreateOrgInput{
		Name: "Bad Org",
		Slug: "INVALID_SLUG",
	})

	require.Error(t, err)
	var appErr *shareddomain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "ORG_CREATE_INVALID_SLUG", appErr.Code)
}

func TestOrgUseCase_CreateOrg_DuplicateSlug(t *testing.T) {
	repo := newMockOrgRepo()
	uc := NewOrgUseCase(repo)

	_, err := uc.CreateOrg(context.Background(), CreateOrgInput{Name: "Org1", Slug: "my-org"})
	require.NoError(t, err)

	_, err = uc.CreateOrg(context.Background(), CreateOrgInput{Name: "Org2", Slug: "my-org"})
	require.Error(t, err)
	var appErr *shareddomain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "ORG_CREATE_SLUG_DUPLICATE", appErr.Code)
}

func TestOrgUseCase_GetOrg_NotFound(t *testing.T) {
	repo := newMockOrgRepo()
	uc := NewOrgUseCase(repo)

	_, err := uc.GetOrg(context.Background(), "nonexistent")

	require.Error(t, err)
	var appErr *shareddomain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "ORG_NOT_FOUND", appErr.Code)
}

func TestOrgUseCase_UpdateOrg_Success(t *testing.T) {
	repo := newMockOrgRepo()
	uc := NewOrgUseCase(repo)

	created, err := uc.CreateOrg(context.Background(), CreateOrgInput{
		Name:   "Original",
		Slug:   "original",
		Domain: "original.io",
	})
	require.NoError(t, err)

	updated, err := uc.UpdateOrg(context.Background(), created.ID, UpdateOrgInput{
		Name:   "Updated",
		Domain: "updated.io",
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "updated.io", updated.Domain)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt))
	_ = time.Now() // ensure time package is used
}

func TestOrgUseCase_GetFirstOrg_Success(t *testing.T) {
	repo := newMockOrgRepo()
	uc := NewOrgUseCase(repo)

	first, err := uc.CreateOrg(context.Background(), CreateOrgInput{Name: "First", Slug: "first", Domain: "first.io"})
	require.NoError(t, err)

	_, err = uc.CreateOrg(context.Background(), CreateOrgInput{Name: "Second", Slug: "second", Domain: "second.io"})
	require.NoError(t, err)

	org, err := uc.GetFirstOrg(context.Background())
	require.NoError(t, err)
	assert.Equal(t, first.ID, org.ID)
}

func TestOrgUseCase_GetFirstOrg_NotFound(t *testing.T) {
	repo := newMockOrgRepo()
	uc := NewOrgUseCase(repo)

	_, err := uc.GetFirstOrg(context.Background())
	require.Error(t, err)

	var appErr *shareddomain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "ORG_NOT_FOUND", appErr.Code)
}
