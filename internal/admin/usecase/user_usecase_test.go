package usecase

import (
	"context"
	"testing"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	users map[string]*domain.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*domain.User)}
}

func (m *mockUserRepo) Create(_ context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) ListByOrg(_ context.Context, orgID string) ([]*domain.User, error) {
	users := make([]*domain.User, 0)
	for _, user := range m.users {
		if user.OrgID == orgID {
			users = append(users, user)
		}
	}
	return users, nil
}

func (m *mockUserRepo) Update(_ context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) Delete(_ context.Context, id string) error {
	delete(m.users, id)
	return nil
}

func TestUserUseCase_ListMembers_ReturnsUsers(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	repo.users["u1"] = &domain.User{ID: "u1", Email: "a@nullus.io", OrgID: "org_1", Role: domain.RoleAdmin, IsActive: true}
	repo.users["u2"] = &domain.User{ID: "u2", Email: "b@nullus.io", OrgID: "org_1", Role: domain.RoleDeveloper, IsActive: true}
	repo.users["u3"] = &domain.User{ID: "u3", Email: "c@nullus.io", OrgID: "org_2", Role: domain.RoleDevOps, IsActive: true}

	users, err := uc.ListMembers(context.Background(), "org_1")
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserUseCase_InviteMember_CreatesPendingUserDefaults(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	user, err := uc.InviteMember(context.Background(), "org_1", "new@nullus.io", domain.RoleDeveloper)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "new@nullus.io", user.Email)
	assert.Equal(t, "org_1", user.OrgID)
	assert.Equal(t, domain.RoleDeveloper, user.Role)
	assert.False(t, user.IsActive)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

func TestUserUseCase_UpdateRole_ChangesRole(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	repo.users["u1"] = &domain.User{ID: "u1", Email: "a@nullus.io", OrgID: "org_1", Role: domain.RoleDeveloper, IsActive: true}

	err := uc.UpdateRole(context.Background(), "u1", domain.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleAdmin, repo.users["u1"].Role)
}

func TestUserUseCase_DeactivateUser_SetsInactiveStatus(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	repo.users["u1"] = &domain.User{ID: "u1", Email: "a@nullus.io", OrgID: "org_1", Role: domain.RoleDeveloper, IsActive: true}

	err := uc.DeactivateUser(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, repo.users["u1"].IsActive)
}
