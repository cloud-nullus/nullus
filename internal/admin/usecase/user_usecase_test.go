package usecase

import (
	"context"
	"testing"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	users       map[string]*domain.User
	memberships map[string]map[string]domain.Role
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:       make(map[string]*domain.User),
		memberships: make(map[string]map[string]domain.Role),
	}
}

func (m *mockUserRepo) Create(_ context.Context, user *domain.User) error {
	m.users[user.ID] = user
	if user.OrgID != "" {
		if _, ok := m.memberships[user.OrgID]; !ok {
			m.memberships[user.OrgID] = make(map[string]domain.Role)
		}
		m.memberships[user.OrgID][user.ID] = user.Role
	}
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

func (m *mockUserRepo) SearchByEmail(ctx context.Context, email string) (*domain.User, error) {
	return m.GetByEmail(ctx, email)
}

func (m *mockUserRepo) ListByOrg(_ context.Context, orgID string) ([]*domain.User, error) {
	users := make([]*domain.User, 0)
	for userID, role := range m.memberships[orgID] {
		if user, ok := m.users[userID]; ok {
			copied := *user
			copied.OrgID = orgID
			copied.Role = role
			users = append(users, &copied)
		}
	}
	return users, nil
}

func (m *mockUserRepo) AddMember(_ context.Context, orgID, userID string, role domain.Role) error {
	if _, ok := m.memberships[orgID]; !ok {
		m.memberships[orgID] = make(map[string]domain.Role)
	}
	m.memberships[orgID][userID] = role
	return nil
}

func (m *mockUserRepo) IsMember(_ context.Context, orgID, userID string) (bool, error) {
	_, ok := m.memberships[orgID][userID]
	return ok, nil
}

func (m *mockUserRepo) Update(_ context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) Delete(_ context.Context, id string) error {
	delete(m.users, id)
	for orgID := range m.memberships {
		delete(m.memberships[orgID], id)
	}
	return nil
}

func TestUserUseCase_ListMembers_ReturnsUsers(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	repo.users["u1"] = &domain.User{ID: "u1", Email: "a@nullus.io", OrgID: "org_1", Role: domain.RoleAdmin, IsActive: true}
	repo.users["u2"] = &domain.User{ID: "u2", Email: "b@nullus.io", OrgID: "org_1", Role: domain.RoleDeveloper, IsActive: true}
	repo.users["u3"] = &domain.User{ID: "u3", Email: "c@nullus.io", OrgID: "org_2", Role: domain.RoleDevOps, IsActive: true}
	require.NoError(t, repo.AddMember(context.Background(), "org_1", "u1", domain.RoleAdmin))
	require.NoError(t, repo.AddMember(context.Background(), "org_1", "u2", domain.RoleDeveloper))
	require.NoError(t, repo.AddMember(context.Background(), "org_2", "u3", domain.RoleDevOps))

	users, err := uc.ListMembers(context.Background(), "org_1")
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserUseCase_InviteMember_CreatesPendingUserDefaults(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	user, err := uc.InviteMember(context.Background(), "org_1", "new@nullus.io", "New Member", domain.RoleDeveloper)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "new@nullus.io", user.Email)
	assert.Equal(t, "New Member", user.Name)
	assert.Equal(t, "org_1", user.OrgID)
	assert.Equal(t, domain.RoleDeveloper, user.Role)
	assert.False(t, user.IsActive)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
	isMember, err := repo.IsMember(context.Background(), "org_1", user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestUserUseCase_InviteMember_AddsExistingUserToOrganization(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	repo.users["u1"] = &domain.User{
		ID:       "u1",
		Email:    "existing@nullus.io",
		Name:     "Existing",
		Role:     domain.RoleDeveloper,
		OrgID:    "org_a",
		IsActive: true,
	}

	user, err := uc.InviteMember(context.Background(), "org_b", "existing@nullus.io", "Existing", domain.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, "u1", user.ID)
	assert.Equal(t, "org_b", user.OrgID)
	assert.Equal(t, domain.RoleAdmin, user.Role)
	assert.True(t, user.IsActive)

	isMember, err := repo.IsMember(context.Background(), "org_b", "u1")
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestUserUseCase_InviteMember_RejectsAlreadyMember(t *testing.T) {
	repo := newMockUserRepo()
	uc := NewUserUseCase(repo)

	repo.users["u1"] = &domain.User{
		ID:       "u1",
		Email:    "existing@nullus.io",
		Name:     "Existing",
		Role:     domain.RoleDeveloper,
		OrgID:    "org_a",
		IsActive: true,
	}
	require.NoError(t, repo.AddMember(context.Background(), "org_a", "u1", domain.RoleDeveloper))

	_, err := uc.InviteMember(context.Background(), "org_a", "existing@nullus.io", "Existing", domain.RoleAdmin)
	require.Error(t, err)

	appErr, ok := err.(*shareddomain.AppError)
	require.True(t, ok)
	assert.Equal(t, "USER_ALREADY_MEMBER", appErr.Code)
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
