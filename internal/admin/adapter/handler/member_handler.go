package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type MemberHandler struct {
	mu      sync.RWMutex
	members map[string]map[string]*domain.User
}

type createMemberRequest struct {
	Email string      `json:"email"`
	Name  string      `json:"name"`
	Role  domain.Role `json:"role"`
}

type updateMemberRequest struct {
	Role domain.Role `json:"role"`
}

func NewMemberHandler() *MemberHandler {
	return &MemberHandler{members: make(map[string]map[string]*domain.User)}
}

func (h *MemberHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/organizations/:orgId/members", h.ListMembers)
	g.POST("/organizations/:orgId/members", h.CreateMember)
	g.DELETE("/organizations/:orgId/members/:memberId", h.DeleteMember)
	g.PATCH("/organizations/:orgId/members/:memberId", h.UpdateMemberRole)
	g.POST("/organizations/:orgId/members/:memberId/deactivate", h.DeactivateMember)
}

func (h *MemberHandler) ensureOrg(orgID string) map[string]*domain.User {
	if _, ok := h.members[orgID]; !ok {
		h.members[orgID] = make(map[string]*domain.User)
	}
	return h.members[orgID]
}

func (h *MemberHandler) ListMembers(c echo.Context) error {
	orgID := c.Param("orgId")

	h.mu.RLock()
	orgMembers := h.members[orgID]
	items := make([]*domain.User, 0, len(orgMembers))
	for _, m := range orgMembers {
		cp := *m
		items = append(items, &cp)
	}
	h.mu.RUnlock()

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *MemberHandler) CreateMember(c echo.Context) error {
	orgID := c.Param("orgId")

	var req createMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Role == "" {
		req.Role = domain.RoleDeveloper
	}

	now := time.Now().UTC()
	member := &domain.User{
		ID:        uuid.NewString(),
		Email:     req.Email,
		Name:      req.Name,
		Role:      req.Role,
		OrgID:     orgID,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	h.mu.Lock()
	orgMembers := h.ensureOrg(orgID)
	orgMembers[member.ID] = member
	h.mu.Unlock()

	return c.JSON(http.StatusCreated, member)
}

func (h *MemberHandler) DeleteMember(c echo.Context) error {
	orgID := c.Param("orgId")
	memberID := c.Param("memberId")

	h.mu.Lock()
	orgMembers := h.ensureOrg(orgID)
	delete(orgMembers, memberID)
	h.mu.Unlock()

	return c.NoContent(http.StatusNoContent)
}

func (h *MemberHandler) UpdateMemberRole(c echo.Context) error {
	orgID := c.Param("orgId")
	memberID := c.Param("memberId")

	var req updateMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	orgMembers := h.ensureOrg(orgID)
	member, ok := orgMembers[memberID]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "member not found")
	}
	member.Role = req.Role
	member.UpdatedAt = time.Now().UTC()

	return c.JSON(http.StatusOK, member)
}

func (h *MemberHandler) DeactivateMember(c echo.Context) error {
	orgID := c.Param("orgId")
	memberID := c.Param("memberId")

	h.mu.Lock()
	defer h.mu.Unlock()
	orgMembers := h.ensureOrg(orgID)
	member, ok := orgMembers[memberID]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "member not found")
	}
	member.IsActive = false
	member.UpdatedAt = time.Now().UTC()

	return c.JSON(http.StatusOK, member)
}
