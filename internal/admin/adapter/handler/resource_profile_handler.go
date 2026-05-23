package handler

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
)

const defaultDevelopmentOrgID = "11111111-1111-1111-1111-111111111111"

type ResourceProfileHandler struct {
	orgUC *usecase.OrgUseCase
	repo  port.ResourceProfileRepository
	audit audit.Sink
}

func NewResourceProfileHandler(orgUC *usecase.OrgUseCase, repo port.ResourceProfileRepository, auditLogger ...audit.Sink) *ResourceProfileHandler {
	var logger audit.Sink
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &ResourceProfileHandler{orgUC: orgUC, repo: repo, audit: logger}
}

type saveResourceProfileRequest struct {
	Name                     string                            `json:"name"`
	BaseProfile              domain.ResourceProfileBase        `json:"baseProfile"`
	OptionOverrides          map[string]map[string]float64     `json:"optionOverrides"`
	AppliedResourceOverrides map[string]domain.ResourceVector  `json:"appliedResourceOverrides"`
	RowUnits                 map[string]domain.PlanningRowUnit `json:"rowUnits"`
}

func (h *ResourceProfileHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/organization/resource-profiles", h.List)
	g.POST("/organization/resource-profiles", h.Create)
	g.PATCH("/organization/resource-profiles/:id", h.Update)
	g.DELETE("/organization/resource-profiles/:id", h.Delete)
}

func (h *ResourceProfileHandler) List(c echo.Context) error {
	org, err := h.resolveOrg(c)
	if err != nil {
		return err
	}
	profiles, err := h.repo.List(c.Request().Context(), org.ID)
	if err != nil {
		return err
	}
	if len(profiles) == 0 && org.ID != defaultDevelopmentOrgID && strings.EqualFold(os.Getenv("NULLUS_SERVER_MODE"), "development") {
		profiles, err = h.repo.List(c.Request().Context(), defaultDevelopmentOrgID)
		if err != nil {
			return err
		}
	}
	return c.JSON(http.StatusOK, profiles)
}

func (h *ResourceProfileHandler) Create(c echo.Context) error {
	org, err := h.resolveOrg(c)
	if err != nil {
		return err
	}

	var req saveResourceProfileRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := normalizeResourceProfileRequest(&req); err != nil {
		return err
	}

	profile := &domain.OrgResourceProfile{
		ID:                       uuid.New().String(),
		Name:                     req.Name,
		OrgID:                    org.ID,
		BaseProfile:              req.BaseProfile,
		OptionOverrides:          req.OptionOverrides,
		AppliedResourceOverrides: req.AppliedResourceOverrides,
		RowUnits:                 req.RowUnits,
		CreatedAt:                time.Now().UTC(),
	}
	if err := h.repo.Create(c.Request().Context(), profile); err != nil {
		return err
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "create",
			ResourceType: "resource_profile",
			ResourceID:   profile.ID,
			Details: map[string]any{
				"name": profile.Name,
			},
			IPAddress: c.RealIP(),
		})
	}
	return c.JSON(http.StatusCreated, profile)
}

func (h *ResourceProfileHandler) Update(c echo.Context) error {
	org, err := h.resolveOrg(c)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "profile id is required")
	}

	var req saveResourceProfileRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := normalizeResourceProfileRequest(&req); err != nil {
		return err
	}

	profile := &domain.OrgResourceProfile{
		ID:                       id,
		Name:                     req.Name,
		OrgID:                    org.ID,
		BaseProfile:              req.BaseProfile,
		OptionOverrides:          req.OptionOverrides,
		AppliedResourceOverrides: req.AppliedResourceOverrides,
		RowUnits:                 req.RowUnits,
		CreatedAt:                time.Now().UTC(),
	}
	updated, err := h.repo.Update(c.Request().Context(), profile)
	if err != nil {
		return err
	}
	if !updated {
		return echo.NewHTTPError(http.StatusNotFound, "resource profile not found")
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "update",
			ResourceType: "resource_profile",
			ResourceID:   profile.ID,
			Details: map[string]any{
				"name": profile.Name,
			},
			IPAddress: c.RealIP(),
		})
	}
	return c.JSON(http.StatusOK, profile)
}

func (h *ResourceProfileHandler) Delete(c echo.Context) error {
	org, err := h.resolveOrg(c)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "profile id is required")
	}
	if err := h.repo.Delete(c.Request().Context(), org.ID, id); err != nil {
		return err
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "delete",
			ResourceType: "resource_profile",
			ResourceID:   id,
			IPAddress:    c.RealIP(),
		})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ResourceProfileHandler) resolveOrg(c echo.Context) (*domain.Organization, error) {
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = c.QueryParam("orgId")
	}
	if orgID == "" {
		orgID = defaultDevelopmentOrgID
	}
	return h.orgUC.GetOrg(c.Request().Context(), orgID)
}

func normalizeResourceProfileRequest(req *saveResourceProfileRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "profile name is required")
	}
	if !validResourceProfileBase(req.BaseProfile) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid base profile")
	}
	if req.OptionOverrides == nil {
		req.OptionOverrides = map[string]map[string]float64{}
	}
	if req.AppliedResourceOverrides == nil {
		req.AppliedResourceOverrides = map[string]domain.ResourceVector{}
	}
	if req.RowUnits == nil {
		req.RowUnits = map[string]domain.PlanningRowUnit{}
	}
	return nil
}

func validResourceProfileBase(base domain.ResourceProfileBase) bool {
	switch base {
	case domain.ResourceProfileLocal, domain.ResourceProfileStartup, domain.ResourceProfileStandard, domain.ResourceProfileEnterprise:
		return true
	default:
		return false
	}
}
