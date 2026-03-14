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

// CreateOrg handles POST /api/v1/orgs.
func (h *OrgHandler) CreateOrg(c echo.Context) error {
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

	return c.JSON(http.StatusCreated, map[string]any{"data": org})
}

// GetOrg handles GET /api/v1/orgs/:orgId.
func (h *OrgHandler) GetOrg(c echo.Context) error {
	id := c.Param("orgId")

	org, err := h.orgUC.GetOrg(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"data": org})
}

// UpdateOrg handles PUT /api/v1/orgs/:orgId.
func (h *OrgHandler) UpdateOrg(c echo.Context) error {
	id := c.Param("orgId")

	var req updateOrgRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	org, err := h.orgUC.UpdateOrg(c.Request().Context(), id, usecase.UpdateOrgInput{
		Name:   req.Name,
		Domain: req.Domain,
	})
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"data": org})
}

// RegisterRoutes registers organization routes on the given group.
func (h *OrgHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/orgs", h.CreateOrg)
	g.GET("/orgs/:orgId", h.GetOrg)
	g.PUT("/orgs/:orgId", h.UpdateOrg)
}
