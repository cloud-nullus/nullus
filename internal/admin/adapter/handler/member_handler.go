package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/labstack/echo/v4"
)

type MemberHandler struct {
	userUC *usecase.UserUseCase
	audit  *audit.AuditLogger
}

type createMemberRequest struct {
	Email string      `json:"email"`
	Name  string      `json:"name"`
	Role  domain.Role `json:"role"`
}

type updateMemberRequest struct {
	Role domain.Role `json:"role"`
}

func NewMemberHandler(userUC *usecase.UserUseCase, auditLogger ...*audit.AuditLogger) *MemberHandler {
	var logger *audit.AuditLogger
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &MemberHandler{userUC: userUC, audit: logger}
}

func (h *MemberHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/organizations/:orgId/members", h.ListMembers)
	g.POST("/organizations/:orgId/members", h.CreateMember)
	g.DELETE("/organizations/:orgId/members/:memberId", h.DeleteMember)
	g.PATCH("/organizations/:orgId/members/:memberId", h.UpdateMemberRole)
	g.POST("/organizations/:orgId/members/:memberId/deactivate", h.DeactivateMember)
}

func (h *MemberHandler) ListMembers(c echo.Context) error {
	orgID := c.Param("orgId")
	if h.userUC == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "member service is not configured")
	}

	items, err := h.userUC.ListMembers(c.Request().Context(), orgID)
	if err != nil {
		return err
	}

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
	if h.userUC == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "member service is not configured")
	}

	member, err := h.userUC.InviteMember(c.Request().Context(), orgID, req.Email, req.Role)
	if err != nil {
		return err
	}
	member.Name = req.Name
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "invite",
			ResourceType: "member",
			ResourceID:   member.ID,
			Details: map[string]any{
				"org_id": orgID,
				"email":  req.Email,
				"role":   req.Role,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusCreated, member)
}

func (h *MemberHandler) DeleteMember(c echo.Context) error {
	orgID := c.Param("orgId")
	memberID := c.Param("memberId")
	if h.userUC == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "member service is not configured")
	}

	_ = orgID
	if err := h.userUC.RemoveMember(c.Request().Context(), memberID); err != nil {
		return err
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "remove",
			ResourceType: "member",
			ResourceID:   memberID,
			Details: map[string]any{
				"org_id": orgID,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MemberHandler) UpdateMemberRole(c echo.Context) error {
	orgID := c.Param("orgId")
	memberID := c.Param("memberId")

	var req updateMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if h.userUC == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "member service is not configured")
	}

	_ = orgID
	if err := h.userUC.UpdateRole(c.Request().Context(), memberID, req.Role); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

func (h *MemberHandler) DeactivateMember(c echo.Context) error {
	orgID := c.Param("orgId")
	memberID := c.Param("memberId")
	if h.userUC == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "member service is not configured")
	}

	_ = orgID
	if err := h.userUC.DeactivateUser(c.Request().Context(), memberID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deactivated"})
}
