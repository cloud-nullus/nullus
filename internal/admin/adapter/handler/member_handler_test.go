package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMemberUserRepository struct {
	users map[string]*domain.User
	orgs  map[string][]string
}

func newMockMemberUserRepository(seed ...*domain.User) *mockMemberUserRepository {
	users := make(map[string]*domain.User, len(seed))
	orgs := map[string][]string{}
	for _, user := range seed {
		copied := *user
		users[user.ID] = &copied
		orgs[user.OrgID] = append(orgs[user.OrgID], user.ID)
	}
	return &mockMemberUserRepository{users: users, orgs: orgs}
}

func (m *mockMemberUserRepository) Create(_ context.Context, user *domain.User) error {
	copied := *user
	m.users[user.ID] = &copied
	m.orgs[user.OrgID] = append(m.orgs[user.OrgID], user.ID)
	return nil
}

func (m *mockMemberUserRepository) GetByID(_ context.Context, id string) (*domain.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	copied := *user
	return &copied, nil
}

func (m *mockMemberUserRepository) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			copied := *user
			return &copied, nil
		}
	}
	return nil, nil
}

func (m *mockMemberUserRepository) SearchByEmail(ctx context.Context, email string) (*domain.User, error) {
	return m.GetByEmail(ctx, email)
}

func (m *mockMemberUserRepository) ListByOrg(_ context.Context, orgID string) ([]*domain.User, error) {
	ids := m.orgs[orgID]
	result := make([]*domain.User, 0, len(ids))
	for _, id := range ids {
		if user, ok := m.users[id]; ok {
			copied := *user
			result = append(result, &copied)
		}
	}
	return result, nil
}

func (m *mockMemberUserRepository) Update(_ context.Context, user *domain.User) error {
	if _, ok := m.users[user.ID]; !ok {
		return errors.New("user not found")
	}
	copied := *user
	m.users[user.ID] = &copied
	return nil
}

func (m *mockMemberUserRepository) AddMember(_ context.Context, orgID, userID string, role domain.Role) error {
	if user, ok := m.users[userID]; ok {
		user.Role = role
	}
	for _, id := range m.orgs[orgID] {
		if id == userID {
			return nil
		}
	}
	m.orgs[orgID] = append(m.orgs[orgID], userID)
	return nil
}

func (m *mockMemberUserRepository) IsMember(_ context.Context, orgID, userID string) (bool, error) {
	for _, id := range m.orgs[orgID] {
		if id == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockMemberUserRepository) Delete(_ context.Context, id string) error {
	user, ok := m.users[id]
	if !ok {
		return errors.New("user not found")
	}
	delete(m.users, id)

	ids := m.orgs[user.OrgID]
	filtered := make([]string, 0, len(ids))
	for _, existing := range ids {
		if existing != id {
			filtered = append(filtered, existing)
		}
	}
	m.orgs[user.OrgID] = filtered
	return nil
}

func newMemberEcho(repo *mockMemberUserRepository) *echo.Echo {
	e := echo.New()
	userUC := usecase.NewUserUseCase(repo)
	h := adminhandler.NewMemberHandler(userUC)
	v1 := e.Group("/api/v1/admin")
	h.RegisterRoutes(v1)
	return e
}

func TestMemberHandler_List_Success(t *testing.T) {
	now := time.Now().UTC()
	repo := newMockMemberUserRepository(
		&domain.User{ID: "u-1", Email: "a@nullus.io", Name: "Alice", Role: domain.RoleDeveloper, OrgID: "org-1", CreatedAt: now, UpdatedAt: now},
		&domain.User{ID: "u-2", Email: "b@nullus.io", Name: "Bob", Role: domain.RoleDevOps, OrgID: "org-1", CreatedAt: now, UpdatedAt: now},
		&domain.User{ID: "u-3", Email: "c@nullus.io", Name: "Carol", Role: domain.RoleDeveloper, OrgID: "org-2", CreatedAt: now, UpdatedAt: now},
	)
	e := newMemberEcho(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/org-1/members", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []domain.User `json:"items"`
		Total int           `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	require.Len(t, resp.Items, 2)
}

func TestMemberHandler_Invite_Success(t *testing.T) {
	repo := newMockMemberUserRepository()
	e := newMemberEcho(repo)

	body := `{"email":"new-member@nullus.io","name":"New Member","role":"devops"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations/org-1/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp domain.User
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "new-member@nullus.io", resp.Email)
	assert.Equal(t, "New Member", resp.Name)
	assert.Equal(t, domain.RoleDevOps, resp.Role)
	assert.Equal(t, "org-1", resp.OrgID)
	assert.False(t, resp.IsActive)
}

func TestMemberHandler_Invite_InvalidBody(t *testing.T) {
	repo := newMockMemberUserRepository()
	e := newMemberEcho(repo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations/org-1/members", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid request body")
}

func TestMemberHandler_Remove_Success(t *testing.T) {
	now := time.Now().UTC()
	repo := newMockMemberUserRepository(&domain.User{
		ID:        "member-1",
		Email:     "member@nullus.io",
		Role:      domain.RoleDeveloper,
		OrgID:     "org-1",
		CreatedAt: now,
		UpdatedAt: now,
	})
	e := newMemberEcho(repo)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/organizations/org-1/members/member-1", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	_, exists := repo.users["member-1"]
	assert.False(t, exists)
}

func TestMemberHandler_SearchUser_Found(t *testing.T) {
	now := time.Now().UTC()
	repo := newMockMemberUserRepository(&domain.User{
		ID:        "member-1",
		Email:     "member@nullus.io",
		Name:      "Member",
		Role:      domain.RoleDeveloper,
		OrgID:     "org-1",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	e := newMemberEcho(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/search?email=member@nullus.io", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Found bool `json:"found"`
		User  struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			IsActive bool   `json:"is_active"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Found)
	assert.Equal(t, "member-1", resp.User.ID)
	assert.Equal(t, "member@nullus.io", resp.User.Email)
	assert.True(t, resp.User.IsActive)
}

func TestMemberHandler_SearchUser_NotFound(t *testing.T) {
	e := newMemberEcho(newMockMemberUserRepository())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/search?email=missing@nullus.io", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Found bool `json:"found"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Found)
}

func TestMemberHandler_SearchUser_RequiresEmailQuery(t *testing.T) {
	e := newMemberEcho(newMockMemberUserRepository())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/search", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "email query param required")
}
