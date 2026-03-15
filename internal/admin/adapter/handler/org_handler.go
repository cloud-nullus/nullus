package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/labstack/echo/v4"
)

// OrgHandler handles HTTP requests for organizations.
type OrgHandler struct {
	orgUC *usecase.OrgUseCase
}

// NewOrgHandler creates a new OrgHandler.
func NewOrgHandler(orgUC *usecase.OrgUseCase) *OrgHandler {
	return &OrgHandler{orgUC: orgUC}
}

type createOrgRequest struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
}

type updateOrgRequest struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
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
	if orgID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "org id is required")
	}

	org, err := h.orgUC.GetOrg(c.Request().Context(), orgID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, org)
}

// PatchOrganization handles PATCH /api/v1/admin/organization.
func (h *OrgHandler) PatchOrganization(c echo.Context) error {
	orgID := h.resolveOrgID(c)
	if orgID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "org id is required")
	}

	var req updateOrgRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	org, err := h.orgUC.UpdateOrg(c.Request().Context(), orgID, usecase.UpdateOrgInput{
		Name:   req.Name,
		Domain: req.Domain,
	})
	if err != nil {
		return err
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

		return c.JSON(http.StatusCreated, org)
	})
}
