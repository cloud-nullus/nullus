package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
)

// CompatibilityHandler handles HTTP requests for compatibility matrix operations.
type CompatibilityHandler struct {
	compatRepo            port.CompatibilityRepository
	validateCompatibility *usecase.ValidateCompatibility
	manageCompatibility   *usecase.ManageCompatibility // nil → admin CRUD routes disabled
	auditSink             audit.Sink                   // nil → audit logging skipped
}

// CompatibilityHandlerOption configures optional CRUD/audit integration.
type CompatibilityHandlerOption func(*CompatibilityHandler)

// WithManageCompatibility enables the admin CRUD routes registered by
// RegisterAdminRoutes. Without this option, only the public read + validate
// routes are registered.
func WithManageCompatibility(u *usecase.ManageCompatibility) CompatibilityHandlerOption {
	return func(h *CompatibilityHandler) { h.manageCompatibility = u }
}

// WithCompatibilityAuditSink attaches an audit.Sink so Create/Update/Delete
// calls emit audit entries. Optional — nil sinks are silently skipped.
func WithCompatibilityAuditSink(s audit.Sink) CompatibilityHandlerOption {
	return func(h *CompatibilityHandler) { h.auditSink = s }
}

// NewCompatibilityHandler constructs a CompatibilityHandler.
func NewCompatibilityHandler(
	compatRepo port.CompatibilityRepository,
	validateCompatibility *usecase.ValidateCompatibility,
	opts ...CompatibilityHandlerOption,
) *CompatibilityHandler {
	h := &CompatibilityHandler{
		compatRepo:            compatRepo,
		validateCompatibility: validateCompatibility,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes registers compatibility routes on the given Echo group.
//
// POST /:stackId/validate is the canonical per-stack validate endpoint. When
// the request body omits stack_id / tools, the handler infers stack_id from
// the path parameter and lets ValidateCompatibility run in persisted mode.
func (h *CompatibilityHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/compatibility", h.GetMatrix)
	g.POST("/:stackId/validate", h.Validate)
}

// RegisterAdminRoutes adds the admin CRUD endpoints for compatibility
// matrices. F8-Phase5 (재개): no-op when ManageCompatibility was not wired
// via WithManageCompatibility.
func (h *CompatibilityHandler) RegisterAdminRoutes(g *echo.Group) {
	if h.manageCompatibility == nil {
		return
	}
	g.POST("/compatibility/matrices", h.CreateMatrix)
	g.PUT("/compatibility/matrices/:id", h.UpdateMatrix)
	g.DELETE("/compatibility/matrices/:id", h.DeleteMatrix)
}

// matrixPayload is the JSON shape for Create/Update bodies. Mirrors
// domain.CompatibilityMatrix but uses snake_case for the external wire.
type matrixPayload struct {
	ID         string                             `json:"id"`
	Name       string                             `json:"name"`
	Status     string                             `json:"status"`
	Kubernetes k8sPayload                         `json:"kubernetes"`
	Tools      map[string]toolPayload             `json:"tools"`
}

type k8sPayload struct {
	Min         string `json:"min"`
	Max         string `json:"max"`
	Recommended string `json:"recommended"`
}

type toolPayload struct {
	Name          string   `json:"name"`
	HelmVersion   string   `json:"helm_version"`
	AppVersion    string   `json:"app_version"`
	MinK8sVersion string   `json:"min_k8s_version,omitempty"`
	ArchSupport   []string `json:"arch_support,omitempty"`
	Tier          string   `json:"tier,omitempty"`
}

func (p *matrixPayload) toDomain() *domain.CompatibilityMatrix {
	m := &domain.CompatibilityMatrix{
		ID:     p.ID,
		Name:   p.Name,
		Status: p.Status,
		Kubernetes: domain.KubernetesCompat{
			Min:         p.Kubernetes.Min,
			Max:         p.Kubernetes.Max,
			Recommended: p.Kubernetes.Recommended,
		},
		Tools: make(map[string]domain.ToolVersion, len(p.Tools)),
	}
	for cat, t := range p.Tools {
		m.Tools[cat] = domain.ToolVersion{
			Name:          t.Name,
			HelmVersion:   t.HelmVersion,
			AppVersion:    t.AppVersion,
			MinK8sVersion: t.MinK8sVersion,
			ArchSupport:   append([]string(nil), t.ArchSupport...),
			Tier:          t.Tier,
		}
	}
	return m
}

// matrixToPayload is the reverse — used when serialising the response so
// the client sees the same snake_case shape it sent in.
func matrixToPayload(m *domain.CompatibilityMatrix) matrixPayload {
	p := matrixPayload{
		ID:     m.ID,
		Name:   m.Name,
		Status: m.Status,
		Kubernetes: k8sPayload{
			Min: m.Kubernetes.Min, Max: m.Kubernetes.Max, Recommended: m.Kubernetes.Recommended,
		},
		Tools: make(map[string]toolPayload, len(m.Tools)),
	}
	for cat, t := range m.Tools {
		p.Tools[cat] = toolPayload{
			Name:          t.Name,
			HelmVersion:   t.HelmVersion,
			AppVersion:    t.AppVersion,
			MinK8sVersion: t.MinK8sVersion,
			ArchSupport:   append([]string(nil), t.ArchSupport...),
			Tier:          t.Tier,
		}
	}
	return p
}

// mapCompatibilityError bridges use-case / repository errors to HTTP codes.
func mapCompatibilityError(c echo.Context, err error) error {
	if err == nil {
		return nil
	}
	// Validation error → 400.
	if usecase.IsValidationError(err) {
		var ve *usecase.CompatibilityValidationError
		if errors.As(err, &ve) {
			return errorResponse(c, http.StatusBadRequest, ve.Code(), ve.Error())
		}
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_VALIDATION", err.Error())
	}
	if errors.Is(err, port.ErrCompatibilityMatrixExists) {
		return errorResponse(c, http.StatusConflict, "COMPATIBILITY_MATRIX_EXISTS", err.Error())
	}
	if errors.Is(err, port.ErrCompatibilityMatrixNotFound) {
		return errorResponse(c, http.StatusNotFound, "COMPATIBILITY_MATRIX_NOT_FOUND", err.Error())
	}
	return errorResponse(c, http.StatusInternalServerError, "COMPATIBILITY_INTERNAL", err.Error())
}

// CreateMatrix handles POST /admin/compatibility/matrices.
func (h *CompatibilityHandler) CreateMatrix(c echo.Context) error {
	var req matrixPayload
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_REQUEST_INVALID", err.Error())
	}
	m := req.toDomain()
	if err := h.manageCompatibility.Create(c.Request().Context(), m); err != nil {
		return mapCompatibilityError(c, err)
	}
	h.logAudit(c, "compatibility_matrix_create", m)
	return c.JSON(http.StatusCreated, matrixToPayload(m))
}

// UpdateMatrix handles PUT /admin/compatibility/matrices/:id.
func (h *CompatibilityHandler) UpdateMatrix(c echo.Context) error {
	pathID := c.Param("id")
	var req matrixPayload
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_REQUEST_INVALID", err.Error())
	}
	if req.ID == "" {
		req.ID = pathID
	}
	if req.ID != pathID {
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_REQUEST_INVALID",
			"path id and body id must match")
	}
	m := req.toDomain()
	if err := h.manageCompatibility.Update(c.Request().Context(), m); err != nil {
		return mapCompatibilityError(c, err)
	}
	h.logAudit(c, "compatibility_matrix_update", m)
	return c.JSON(http.StatusOK, matrixToPayload(m))
}

// DeleteMatrix handles DELETE /admin/compatibility/matrices/:id.
func (h *CompatibilityHandler) DeleteMatrix(c echo.Context) error {
	id := c.Param("id")
	if err := h.manageCompatibility.Delete(c.Request().Context(), id); err != nil {
		return mapCompatibilityError(c, err)
	}
	h.logAudit(c, "compatibility_matrix_delete", &domain.CompatibilityMatrix{ID: id})
	return c.NoContent(http.StatusNoContent)
}

func (h *CompatibilityHandler) logAudit(c echo.Context, action string, m *domain.CompatibilityMatrix) {
	if h.auditSink == nil || m == nil {
		return
	}
	details := map[string]any{
		"status":     m.Status,
		"tool_count": len(m.Tools),
	}
	_ = h.auditSink.Log(c.Request().Context(), audit.AuditEntry{
		UserID:       c.Request().Header.Get("X-User-ID"),
		Action:       action,
		ResourceType: "compatibility_matrix",
		ResourceID:   m.ID,
		Details:      details,
		IPAddress:    c.RealIP(),
	})
}

// GetMatrix handles GET /api/v1/compatibility/matrix.
func (h *CompatibilityHandler) GetMatrix(c echo.Context) error {
	matrices, err := h.compatRepo.GetAll(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "COMPATIBILITY_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, matrices)
}

// validateRequest is the request body for POST /stacks/:stackId/validate.
//
// All fields are optional; the handler applies path-to-body inference so a
// caller hitting /stacks/abc/validate with an empty body still runs the
// persisted-mode gate.
//
//   - Tools: explicit {category: toolName} map. Non-empty body wins over
//     stack-persisted tools (F8-F3).
//   - StackID: falls back to the :stackId path param when omitted.
//   - ClusterID / NodeArchitectures: F8 Task 3 Pre-Deploy Gate inputs for
//     cross-checking per-tool ArchSupport against the worker fleet.
type validateRequest struct {
	Tools             map[string]string `json:"tools"`
	StackID           string            `json:"stack_id,omitempty"`
	ClusterID         string            `json:"cluster_id,omitempty"`
	NodeArchitectures []string          `json:"node_architectures,omitempty"`
}

// Validate handles POST /api/v1/stacks/:stackId/validate.
func (h *CompatibilityHandler) Validate(c echo.Context) error {
	var req validateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_REQUEST_INVALID", err.Error())
	}

	// Path → body inference: :stackId on the route populates the persisted
	// mode input when the body omits it. Keeps backward compatibility with
	// callers that already include stack_id in the JSON body.
	if req.StackID == "" {
		if pathID := c.Param("stackId"); pathID != "" {
			req.StackID = pathID
		}
	}

	out, err := h.validateCompatibility.Execute(c.Request().Context(), usecase.ValidateCompatibilityInput{
		Tools:             req.Tools,
		StackID:           req.StackID,
		ClusterID:         req.ClusterID,
		NodeArchitectures: req.NodeArchitectures,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_REQUEST_INVALID", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"compatible":         out.Compatible,
		"matrix":             out.Matrix,
		"message":            out.Message,
		"overall":            out.Overall,
		"issues":             out.Issues,
		"node_architectures": out.NodeArchitectures,
		"checkedAt":          out.CheckedAt.Format(time.RFC3339),
	})
}
