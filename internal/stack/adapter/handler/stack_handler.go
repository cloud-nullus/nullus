package handler

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// StackHandler handles HTTP requests for stack operations.
type StackHandler struct {
	createStack *usecase.CreateStack
	listStacks  *usecase.ListStacks
	deleteStack *usecase.DeleteStack
	addToolsUC  *usecase.AddToolsUseCase
	stackRepo   port.StackRepository
	audit       *audit.AuditLogger
	pool        *pgxpool.Pool
}

// StackHandlerOption configures optional StackHandler dependencies.
type StackHandlerOption func(*StackHandler)

// WithPool sets the database pool for cross-module queries.
func WithPool(pool *pgxpool.Pool) StackHandlerOption {
	return func(h *StackHandler) { h.pool = pool }
}

// NewStackHandler constructs a StackHandler.
func NewStackHandler(
	createStack *usecase.CreateStack,
	listStacks *usecase.ListStacks,
	deleteStack *usecase.DeleteStack,
	addToolsUC *usecase.AddToolsUseCase,
	stackRepo port.StackRepository,
	auditLogger *audit.AuditLogger,
	opts ...StackHandlerOption,
) *StackHandler {
	h := &StackHandler{
		createStack: createStack,
		listStacks:  listStacks,
		deleteStack: deleteStack,
		addToolsUC:  addToolsUC,
		stackRepo:   stackRepo,
		audit:       auditLogger,
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// RegisterRoutes registers stack routes on the given Echo group.
func (h *StackHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateStack)
	g.GET("", h.ListStacks)
	g.GET("/:stackId", h.GetStack)
	g.DELETE("/:stackId", h.DeleteStack)
	g.PATCH("/:stackId/tools", h.AddTools)
	g.POST("/:stackId/config", h.SaveConfig)
	g.GET("/:stackId/workloads", h.GetWorkloads)
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

	return c.JSON(http.StatusOK, stack)
}

func (h *StackHandler) SaveDraft(c echo.Context) error {
	return c.JSON(http.StatusCreated, map[string]any{"draftId": "drf_" + uuid.NewString()})
}

// workloadPipeline is the response shape for a pipeline in the workloads endpoint.
type workloadPipeline struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Status         string            `json:"status"`
	LastDeployment *workloadDeploy   `json:"lastDeployment"`
	K8sObjects     []workloadK8sObj  `json:"k8sObjects"`
}

type workloadDeploy struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	StartedAt string `json:"startedAt"`
	Version   string `json:"version"`
}

type workloadK8sObj struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Replicas  int32  `json:"replicas,omitempty"`
	Port      int32  `json:"port,omitempty"`
	Host      string `json:"host,omitempty"`
	Node      string `json:"node,omitempty"`
}

type workloadSummary struct {
	TotalPipelines   int `json:"totalPipelines"`
	TotalDeployments int `json:"totalDeployments"`
	RunningPods      int `json:"runningPods"`
	PendingPods      int `json:"pendingPods"`
	FailedPods       int `json:"failedPods"`
}

// GetWorkloads handles GET /api/v1/stacks/:stackId/workloads.
func (h *StackHandler) GetWorkloads(c echo.Context) error {
	stackID := c.Param("stackId")
	if h.pool == nil {
		return errorResponse(c, http.StatusInternalServerError, "WORKLOADS_UNAVAILABLE", "database pool not configured")
	}

	ctx := c.Request().Context()

	// Verify stack exists and get cluster name for node info
	var clusterName string
	err := h.pool.QueryRow(ctx,
		`SELECT COALESCE(c.name, 'unknown') FROM stacks s LEFT JOIN clusters c ON s.cluster_id = c.id WHERE s.id = $1`, stackID,
	).Scan(&clusterName)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
	}

	// Query pipelines linked to this stack
	type pipelineRow struct {
		ID        string
		Name      string
		Namespace string
		Status    string
	}
	pipelineRows, err := h.pool.Query(ctx,
		`SELECT id, name, namespace, status FROM pipelines WHERE stack_id = $1 ORDER BY created_at DESC`, stackID)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "WORKLOADS_QUERY_FAILED", err.Error())
	}
	defer pipelineRows.Close()

	var pipelines []pipelineRow
	for pipelineRows.Next() {
		var p pipelineRow
		if err := pipelineRows.Scan(&p.ID, &p.Name, &p.Namespace, &p.Status); err != nil {
			return errorResponse(c, http.StatusInternalServerError, "WORKLOADS_SCAN_FAILED", err.Error())
		}
		pipelines = append(pipelines, p)
	}
	if err := pipelineRows.Err(); err != nil {
		return errorResponse(c, http.StatusInternalServerError, "WORKLOADS_ROWS_FAILED", err.Error())
	}

	summary := workloadSummary{TotalPipelines: len(pipelines)}
	result := make([]workloadPipeline, 0, len(pipelines))

	for _, p := range pipelines {
		wp := workloadPipeline{
			ID:        p.ID,
			Name:      p.Name,
			Namespace: p.Namespace,
			Status:    p.Status,
		}

		// Query latest deployment for this pipeline
		var depID, depStatus, depVersion string
		var depStartedAt time.Time
		err := h.pool.QueryRow(ctx,
			`SELECT id, status, version, started_at FROM pipeline_deployments WHERE pipeline_id = $1 ORDER BY started_at DESC LIMIT 1`,
			p.ID,
		).Scan(&depID, &depStatus, &depVersion, &depStartedAt)
		if err == nil {
			wp.LastDeployment = &workloadDeploy{
				ID:        depID,
				Status:    depStatus,
				StartedAt: depStartedAt.Format(time.RFC3339),
				Version:   depVersion,
			}
			summary.TotalDeployments++
		}

		// Build K8s objects from the pipeline info
		replicas := int32(2)
		port := int32(8080)

		// Determine pod status based on deployment
		podStatus := "Running"
		if depStatus == "pending" || depStatus == "running" {
			podStatus = "Pending"
		} else if depStatus == "failed" {
			podStatus = "CrashLoopBackOff"
		}

		objects := []workloadK8sObj{
			{
				Kind:      "Deployment",
				Name:      p.Name,
				Namespace: p.Namespace,
				Replicas:  replicas,
				Status:    "running",
			},
		}

		// Add individual Pod entries per replica
		for i := int32(0); i < replicas; i++ {
			suffix := fmt.Sprintf("%s-%05x", p.Name, uint32(depStartedAt.UnixNano())%(0xfffff-uint32(i)*7)+uint32(i)*7)
			nodeName := fmt.Sprintf("%s-node-%d", clusterName, i%3+1)
			objects = append(objects, workloadK8sObj{
				Kind:      "Pod",
				Name:      suffix,
				Namespace: p.Namespace,
				Status:    podStatus,
				Node:      nodeName,
			})
		}

		objects = append(objects,
			workloadK8sObj{
				Kind:      "Service",
				Name:      p.Name,
				Namespace: p.Namespace,
				Port:      port,
				Status:    "active",
			},
			workloadK8sObj{
				Kind:      "Ingress",
				Name:      p.Name,
				Namespace: p.Namespace,
				Host:      p.Name + "." + p.Namespace + ".nullus.local",
				Status:    "active",
			},
		)

		wp.K8sObjects = objects

		// Accumulate pod counts based on deployment status
		if depStatus == "success" || depStatus == "" {
			summary.RunningPods += int(replicas)
		} else if depStatus == "pending" || depStatus == "running" {
			summary.PendingPods += int(replicas)
		} else {
			summary.FailedPods += int(replicas)
		}

		result = append(result, wp)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"pipelines": result,
		"summary":   summary,
	})
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
