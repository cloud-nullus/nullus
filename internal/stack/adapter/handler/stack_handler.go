package handler

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// StackHandler handles HTTP requests for stack operations.
type StackHandler struct {
	createStack   *usecase.CreateStack
	listStacks    *usecase.ListStacks
	deleteStack   *usecase.DeleteStack
	addToolsUC    *usecase.AddToolsUseCase
	updateStack   *usecase.UpdateStack
	manageHistory *usecase.ManageHistory
	stackRepo     port.StackRepository
	audit         audit.Sink
}

// StackHandlerOption configures optional features.
type StackHandlerOption func(*StackHandler)

// WithUpdateStack wires the UpdateStack usecase. Optional so existing
// constructor signature stays backward-compatible.
func WithUpdateStack(uc *usecase.UpdateStack) StackHandlerOption {
	return func(h *StackHandler) { h.updateStack = uc }
}

// NewStackHandler constructs a StackHandler.
func NewStackHandler(
	createStack *usecase.CreateStack,
	listStacks *usecase.ListStacks,
	deleteStack *usecase.DeleteStack,
	addToolsUC *usecase.AddToolsUseCase,
	stackRepo port.StackRepository,
	manageHistory *usecase.ManageHistory,
	auditLogger ...audit.Sink,
) *StackHandler {
	var logger audit.Sink
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &StackHandler{
		createStack:   createStack,
		listStacks:    listStacks,
		deleteStack:   deleteStack,
		addToolsUC:    addToolsUC,
		manageHistory: manageHistory,
		stackRepo:     stackRepo,
		audit:         logger,
	}
}

// WithOptions applies StackHandlerOption values after construction.
func (h *StackHandler) WithOptions(opts ...StackHandlerOption) *StackHandler {
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes registers stack routes on the given Echo group.
func (h *StackHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateStack)
	g.GET("", h.ListStacks)
	g.GET("/:stackId", h.GetStack)
	g.PUT("/:stackId", h.UpdateStack)
	g.DELETE("/:stackId", h.DeleteStack)
	g.PATCH("/:stackId/tools", h.AddTools)
	g.POST("/:stackId/config", h.SaveConfig)
	g.POST("/draft", h.SaveDraft)
}

// updateStackRequest is the PUT body. All fields optional; absent fields are
// left untouched on the stack aggregate.
type updateStackRequest struct {
	Name      *string             `json:"name,omitempty"`
	ClusterID *string             `json:"cluster_id,omitempty"`
	Namespace *string             `json:"namespace,omitempty"`
	Config    *domain.StackConfig `json:"config,omitempty"`
	Tools     []domain.ToolConfig `json:"tools,omitempty"`
}

// UpdateStack handles PUT /api/v1/stacks/:stackId. Phase 4 of F8 follow-up.
// Permits mutation only while state ∈ {pending, failed}; other states return
// 409 STACK_UPDATE_INVALID_STATE.
func (h *StackHandler) UpdateStack(c echo.Context) error {
	if h.updateStack == nil {
		return errorResponse(c, http.StatusNotImplemented, "STACK_UPDATE_DISABLED",
			"update stack usecase not wired")
	}
	id := c.Param("stackId")
	var req updateStackRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_UPDATE_REQUEST_INVALID", err.Error())
	}

	out, err := h.updateStack.Execute(c.Request().Context(), usecase.UpdateStackInput{
		StackID:   id,
		Name:      req.Name,
		ClusterID: req.ClusterID,
		Namespace: req.Namespace,
		Config:    req.Config,
		Tools:     req.Tools,
	})
	if err != nil {
		// State-machine rejections use STACK_UPDATE_INVALID_STATE so the
		// frontend can branch without string matching.
		if strings.Contains(err.Error(), "is not updatable") {
			return errorResponse(c, http.StatusConflict, "STACK_UPDATE_INVALID_STATE", err.Error())
		}
		if strings.Contains(err.Error(), "not found") {
			return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "STACK_UPDATE_FAILED", err.Error())
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "update",
			ResourceType: "stack",
			ResourceID:   id,
			Details:      map[string]any{"state": out.Stack.State},
			IPAddress:    c.RealIP(),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id":    out.Stack.ID,
		"state": out.Stack.State,
	})
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
	if h.manageHistory != nil {
		changedBy := c.Request().Header.Get("X-User-ID")
		if changedBy == "" {
			changedBy = "system"
		}
		if _, err := h.manageHistory.SaveVersion(c.Request().Context(), usecase.SaveVersionInput{
			StackID:      out.Stack.ID,
			Config:       req.Config,
			ChangedBy:    changedBy,
			ChangeReason: "stack created",
		}); err != nil {
			return errorResponse(c, http.StatusInternalServerError, "HISTORY_SAVE_FAILED", err.Error())
		}
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
		if errors.Is(err, usecase.ErrStackNotFound) {
			return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "STACK_DELETE_FAILED", err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// saveConfigRequest is the request body for POST /stacks/:id/config.
type saveConfigRequest struct {
	Config domain.StackConfig `json:"config"`
}

type addToolsRequest struct {
	Tools []domain.ToolConfig `json:"tools"`
}

func (h *StackHandler) AddTools(c echo.Context) error {
	stackID := c.Param("stackId")
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack_id is required")
	}

	var req addToolsRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_TOOLS_INVALID", err.Error())
	}
	if len(req.Tools) == 0 {
		return errorResponse(c, http.StatusBadRequest, "STACK_TOOLS_INVALID", "tools is required")
	}
	if h.addToolsUC == nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_UPDATE_FAILED", "add tools usecase not configured")
	}

	result, err := h.addToolsUC.Execute(c.Request().Context(), usecase.AddToolsInput{
		StackID: stackID,
		Tools:   req.Tools,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrStackNotFound) {
			return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
		}
		if strings.Contains(err.Error(), "already exists") {
			return errorResponse(c, http.StatusBadRequest, "STACK_TOOLS_DUPLICATE", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "STACK_UPDATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, result)
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

	if h.manageHistory != nil {
		changedBy := c.Request().Header.Get("X-User-ID")
		if changedBy == "" {
			changedBy = "system"
		}

		reason := "stack config updated"
		if len(req.Config.YAMLOverrides) > 0 {
			reason = fmt.Sprintf("yaml_view_customization (%d overrides)", len(req.Config.YAMLOverrides))
		}

		if _, err := h.manageHistory.SaveVersion(c.Request().Context(), usecase.SaveVersionInput{
			StackID:      id,
			Config:       req.Config,
			ChangedBy:    changedBy,
			ChangeReason: reason,
		}); err != nil {
			return errorResponse(c, http.StatusInternalServerError, "HISTORY_SAVE_FAILED", err.Error())
		}
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

	return "11111111-1111-1111-1111-111111111111"
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
