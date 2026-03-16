package handler

import (
	"net/http"
	"reflect"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// StackHandler handles HTTP requests for stack operations.
type StackHandler struct {
	createStack *usecase.CreateStack
	listStacks  *usecase.ListStacks
	deleteStack *usecase.DeleteStack
	stackRepo   port.StackRepository
	audit       *audit.AuditLogger
}

// NewStackHandler constructs a StackHandler.
func NewStackHandler(
	createStack *usecase.CreateStack,
	listStacks *usecase.ListStacks,
	deleteStack *usecase.DeleteStack,
	stackRepo port.StackRepository,
	auditLogger ...*audit.AuditLogger,
) *StackHandler {
	var logger *audit.AuditLogger
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &StackHandler{
		createStack: createStack,
		listStacks:  listStacks,
		deleteStack: deleteStack,
		stackRepo:   stackRepo,
		audit:       logger,
	}
}

// RegisterRoutes registers stack routes on the given Echo group.
func (h *StackHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateStack)
	g.GET("", h.ListStacks)
	g.GET("/:stackId", h.GetStack)
	g.DELETE("/:stackId", h.DeleteStack)
	g.POST("/:stackId/config", h.SaveConfig)
	g.POST("/draft", h.SaveDraft)
}

// createStackRequest is the request body for POST /stacks.
type createStackRequest struct {
	Name       string             `json:"name"`
	ClusterID  string             `json:"cluster_id"`
	Namespace  string             `json:"namespace"`
	TemplateID string             `json:"golden_path_id"`
	Config     domain.StackConfig `json:"config"`
}

// CreateStack handles POST /api/v1/stacks.
func (h *StackHandler) CreateStack(c echo.Context) error {
	var req createStackRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_CONFIG_INVALID", err.Error())
	}

	orgID := resolveOrgID(c)

	out, err := h.createStack.Execute(c.Request().Context(), usecase.CreateStackInput{
		Name:       req.Name,
		OrgID:      orgID,
		ClusterID:  req.ClusterID,
		Namespace:  req.Namespace,
		TemplateID: req.TemplateID,
		Config:     req.Config,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_CONFIG_INVALID", err.Error())
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "create",
			ResourceType: "stack",
			ResourceID:   out.Stack.ID,
			Details: map[string]any{
				"name":        req.Name,
				"org_id":      orgID,
				"cluster_id":  req.ClusterID,
				"namespace":   req.Namespace,
				"template_id": req.TemplateID,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusCreated, map[string]any{"id": out.Stack.ID})
}

// ListStacks handles GET /api/v1/stacks.
func (h *StackHandler) ListStacks(c echo.Context) error {
	orgID := resolveOrgID(c)

	out, err := h.listStacks.Execute(c.Request().Context(), usecase.ListStacksInput{OrgID: orgID})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": out.Stacks, "total": len(out.Stacks)})
}

// GetStack handles GET /api/v1/stacks/:id.
func (h *StackHandler) GetStack(c echo.Context) error {
	id := c.Param("stackId")

	stack, err := h.stackRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, stack)
}

func (h *StackHandler) DeleteStack(c echo.Context) error {
	stackID := c.Param("stackId")
	if err := h.deleteStack.Execute(c.Request().Context(), stackID); err != nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_DELETE_FAILED", err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// saveConfigRequest is the request body for POST /stacks/:id/config.
type saveConfigRequest struct {
	Config domain.StackConfig `json:"config"`
}

// SaveConfig handles POST /api/v1/stacks/:id/config.
func (h *StackHandler) SaveConfig(c echo.Context) error {
	id := c.Param("stackId")

	var req saveConfigRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_CONFIG_INVALID", err.Error())
	}

	stack, err := h.stackRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	}

	stack.Config = req.Config

	if err := h.stackRepo.Update(c.Request().Context(), stack); err != nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_UPDATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, stack)
}

func (h *StackHandler) SaveDraft(c echo.Context) error {
	return c.JSON(http.StatusCreated, map[string]any{"draftId": "drf_" + uuid.NewString()})
}

func resolveOrgID(c echo.Context) string {
	if claims, ok := c.Get("user_claims").(map[string]any); ok {
		if orgID, ok := claims["org_id"].(string); ok && orgID != "" {
			return orgID
		}
	}

	if orgID := orgIDFromPrincipal(c.Get("current_user")); orgID != "" {
		return orgID
	}

	if orgID := c.Request().Header.Get("X-Org-ID"); orgID != "" {
		return orgID
	}

	if orgID := c.QueryParam("orgId"); orgID != "" {
		return orgID
	}

	return "00000000-0000-0000-0000-000000000001"
}

func orgIDFromPrincipal(principal any) string {
	if principal == nil {
		return ""
	}

	v := reflect.ValueOf(principal)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return ""
	}

	orgField := v.FieldByName("OrgID")
	if orgField.IsValid() && orgField.Kind() == reflect.String {
		return orgField.String()
	}

	return ""
}
