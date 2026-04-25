package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/labstack/echo/v4"
)

// OrgHandler handles HTTP requests for organizations.
type OrgHandler struct {
	orgUC *usecase.OrgUseCase
	audit audit.Sink
}

// NewOrgHandler creates a new OrgHandler.
func NewOrgHandler(orgUC *usecase.OrgUseCase, auditLogger ...audit.Sink) *OrgHandler {
	var logger audit.Sink
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &OrgHandler{orgUC: orgUC, audit: logger}
}

type createOrgRequest struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
}

type updateOrgRequest struct {
	Name               string   `json:"name"`
	Domain             string   `json:"domain"`
	ClusterAccessScope []string `json:"clusterAccessScope"`
}

func (h *OrgHandler) resolveOrgID(c echo.Context) string {
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = c.QueryParam("orgId")
	}
	return orgID
}

// GetOrganization handles GET /api/v1/admin/organization.
func (h *OrgHandler) GetOrganization(c echo.Context) error {
	orgID := h.resolveOrgID(c)
	var org *domain.Organization
	var err error
	if orgID == "" {
		org, err = h.orgUC.GetFirstOrg(c.Request().Context())
	} else {
		org, err = h.orgUC.GetOrg(c.Request().Context(), orgID)
	}
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, org)
}

// PatchOrganization handles PATCH /api/v1/admin/organization.
func (h *OrgHandler) PatchOrganization(c echo.Context) error {
	orgID := h.resolveOrgID(c)

	var req updateOrgRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	input := usecase.UpdateOrgInput{
		Name:               req.Name,
		Domain:             req.Domain,
		ClusterAccessScope: req.ClusterAccessScope,
	}
	var org *domain.Organization
	var err error
	if orgID == "" {
		org, err = h.orgUC.UpdateFirstOrg(c.Request().Context(), input)
	} else {
		org, err = h.orgUC.UpdateOrg(c.Request().Context(), orgID, input)
	}
	if err != nil {
		return err
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "update",
			ResourceType: "organization",
			ResourceID:   org.ID,
			Details: map[string]any{
				"name":   req.Name,
				"domain": req.Domain,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusOK, org)
}

// RegisterRoutes registers organization routes on the given group.
func (h *OrgHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/organization", h.GetOrganization)
	g.PATCH("/organization", h.PatchOrganization)

	g.POST("/orgs", func(c echo.Context) error {
		var req createOrgRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		org, err := h.orgUC.CreateOrg(c.Request().Context(), usecase.CreateOrgInput{
			Name:   req.Name,
			Slug:   req.Slug,
			Domain: req.Domain,
		})
		if err != nil {
			return err
		}
		if h.audit != nil {
			_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
				UserID:       c.Request().Header.Get("X-User-ID"),
				Action:       "create",
				ResourceType: "organization",
				ResourceID:   org.ID,
				Details: map[string]any{
					"name":   req.Name,
					"slug":   req.Slug,
					"domain": req.Domain,
				},
				IPAddress: c.RealIP(),
			})
		}

		return c.JSON(http.StatusCreated, org)
	})
}
