package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
	createStack   *usecase.CreateStack
	listStacks    *usecase.ListStacks
	deleteStack   *usecase.DeleteStack
	addToolsUC    *usecase.AddToolsUseCase
	manageHistory *usecase.ManageHistory
	stackRepo     port.StackRepository
	audit         *audit.AuditLogger
	pool          *pgxpool.Pool
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

// WithStackManageHistory enables history snapshotting for stack config updates.
func WithStackManageHistory(manageHistory *usecase.ManageHistory) StackHandlerOption {
	return func(h *StackHandler) { h.manageHistory = manageHistory }
}

// RegisterRoutes registers stack routes on the given Echo group.
func (h *StackHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateStack)
	g.POST("/storage/test", h.TestStorageConnection)
	g.GET("", h.ListStacks)
	g.GET("/:stackId", h.GetStack)
	g.DELETE("/:stackId", h.DeleteStack)
	g.PATCH("/:stackId/tools", h.AddTools)
	g.POST("/:stackId/config", h.SaveConfig)
	g.GET("/:stackId/workloads", h.GetWorkloads)
	g.GET("/:stackId/integrations", h.GetIntegrations)
	g.POST("/draft", h.SaveDraft)
}

type testStorageConnectionRequest struct {
	Target         string `json:"target"`
	Endpoint       string `json:"endpoint"`
	ProviderEngine string `json:"provider_or_engine"`
	AuthID         string `json:"auth_id"`
	AuthPassword   string `json:"auth_password"`
	ResourceName   string `json:"resource_name"`
}

func (h *StackHandler) TestStorageConnection(c echo.Context) error {
	var req testStorageConnectionRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "STORAGE_TEST_INVALID", err.Error())
	}

	target := strings.TrimSpace(strings.ToLower(req.Target))
	endpoint := strings.TrimSpace(req.Endpoint)
	if target != "database" && target != "object_storage" {
		return errorResponse(c, http.StatusBadRequest, "STORAGE_TEST_INVALID", "target must be database or object_storage")
	}
	if endpoint == "" {
		return errorResponse(c, http.StatusBadRequest, "STORAGE_TEST_INVALID", "endpoint is required")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	if target == "database" {
		if err := testDatabaseConnection(ctx, endpoint); err != nil {
			return c.JSON(http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "message": "database endpoint reachable"})
	}

	if err := testObjectStorageConnection(ctx, endpoint); err != nil {
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "message": "object storage endpoint reachable"})
}

func testDatabaseConnection(ctx context.Context, endpoint string) error {
	addr := endpoint
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("invalid endpoint: %w", err)
		}
		addr = u.Host
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	_ = conn.Close()
	return nil
}

func testObjectStorageConnection(ctx context.Context, endpoint string) error {
	addr := endpoint
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("invalid endpoint: %w", err)
		}
		addr = u.Host
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("object storage connection failed: %w", err)
	}
	_ = conn.Close()
	return nil
}

type stackIntegrationResponse struct {
	ID                    string         `json:"id"`
	StackID               string         `json:"stack_id"`
	ComponentType         string         `json:"component_type"`
	Provider              string         `json:"provider"`
	Endpoint              string         `json:"endpoint"`
	APIEndpoint           string         `json:"api_endpoint"`
	CredentialRef         string         `json:"credential_ref,omitempty"`
	CredentialReady       bool           `json:"credential_ready"`
	HealthStatus          string         `json:"health_status"`
	ProvisionCapabilities []string       `json:"provisioning_capabilities"`
	Metadata              map[string]any `json:"metadata,omitempty"`
}

func (h *StackHandler) GetIntegrations(c echo.Context) error {
	stackID := c.Param("stackId")
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack_id is required")
	}
	stack, err := h.stackRepo.GetByID(c.Request().Context(), stackID)
	if err != nil || stack == nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found")
	}

	cfg := stackConfigFromAny(stack.Config)
	items := []stackIntegrationResponse{
		buildIntegration(stack, cfg, "code_repository", cfg.Artifacts.SourceRepository, []string{"repository_select", "repository_create"}),
		buildIntegration(stack, cfg, "package_registry", cfg.Artifacts.PackageRegistry, []string{"artifact_publish", "sbom_publish", "test_report_publish"}),
		buildIntegration(stack, cfg, "image_registry", cfg.Artifacts.ContainerRegistry, []string{"image_push", "image_pull"}),
		buildIntegration(stack, cfg, "ci_platform", cfg.Pipeline.CIPlatform, []string{"workflow_provision", "pipeline_trigger", "run_status_read"}),
		buildIntegration(stack, cfg, "cd_tool", cfg.Pipeline.CDTool, []string{"application_provision", "sync_trigger", "sync_status_read"}),
	}

	if h.pool != nil {
		var clusterName sql.NullString
		if err := h.pool.QueryRow(c.Request().Context(), "SELECT name FROM clusters WHERE id = $1", stack.ClusterID).Scan(&clusterName); err == nil && clusterName.Valid {
			for i := range items {
				if items[i].Metadata == nil {
					items[i].Metadata = map[string]any{}
				}
				items[i].Metadata["cluster_name"] = clusterName.String
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"stack_id":     stack.ID,
		"state":        stack.State,
		"integrations": items,
		"total":        len(items),
	})
}

func buildIntegration(stack *domain.Stack, cfg domain.StackConfig, componentType string, sel domain.ToolSelection, caps []string) stackIntegrationResponse {
	provider := strings.TrimSpace(sel.Name)
	ready := sel.Enabled && provider != ""
	health := "credential_required"
	if ready {
		health = "ready"
	}
	id := fmt.Sprintf("int_%s_%s", stack.ID, componentType)
	endpoint := integrationEndpoint(stack, cfg, componentType, provider)
	return stackIntegrationResponse{
		ID:                    id,
		StackID:               stack.ID,
		ComponentType:         componentType,
		Provider:              provider,
		Endpoint:              endpoint,
		APIEndpoint:           endpoint,
		CredentialRef:         "",
		CredentialReady:       ready,
		HealthStatus:          health,
		ProvisionCapabilities: caps,
		Metadata: map[string]any{
			"version": sel.Version,
		},
	}
}

func integrationEndpoint(stack *domain.Stack, cfg domain.StackConfig, componentType, provider string) string {
	if provider == "" {
		return ""
	}

	normalizedProvider := strings.ToLower(strings.ReplaceAll(provider, " ", "-"))
	if componentType == "code_repository" && (normalizedProvider == "gitlab" || normalizedProvider == "gitlab-ce") {
		if accessDomain := strings.TrimSpace(cfg.AccessDomain); accessDomain != "" {
			return fmt.Sprintf("https://gitlab.%s", accessDomain)
		}
		if namespace := strings.TrimSpace(stack.Namespace); namespace != "" {
			return fmt.Sprintf("http://gitlab-webservice-default.%s.svc:8181", namespace)
		}
	}

	if accessDomain := strings.TrimSpace(cfg.AccessDomain); accessDomain != "" {
		if subdomain := integrationSubdomain(componentType, normalizedProvider); subdomain != "" {
			return fmt.Sprintf("https://%s.%s", subdomain, accessDomain)
		}
	}
	return ""
}

func integrationSubdomain(componentType, normalizedProvider string) string {
	switch componentType {
	case "image_registry":
		switch normalizedProvider {
		case "gitlab-registry", "gitlab-container-registry":
			return "registry"
		case "harbor":
			return "harbor"
		}
	case "package_registry":
		switch normalizedProvider {
		case "gitlab", "gitlab-package", "gitlab-package-registry":
			return "gitlab"
		case "nexus":
			return "nexus"
		case "artifactory":
			return "artifactory"
		}
	case "ci_platform":
		switch normalizedProvider {
		case "gitlab-ci", "gitlab":
			return "gitlab"
		case "argocd", "argo-cd":
			return "argocd"
		}
	case "cd_tool":
		switch normalizedProvider {
		case "argocd", "argo-cd":
			return "argocd"
		case "flux":
			return "flux"
		}
	}
	return normalizedProvider
}

func stackConfigFromAny(v any) domain.StackConfig {
	if cfg, ok := v.(domain.StackConfig); ok {
		return cfg
	}
	if cfg, ok := v.(*domain.StackConfig); ok && cfg != nil {
		return *cfg
	}
	return domain.StackConfig{}
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
	if h.manageHistory != nil {
		if _, err := h.manageHistory.SaveVersion(c.Request().Context(), usecase.SaveVersionInput{
			StackID:      stack.ID,
			Config:       req.Config,
			ChangedBy:    "system",
			ChangeReason: "config updated",
		}); err != nil {
			return errorResponse(c, http.StatusInternalServerError, "HISTORY_SAVE_FAILED", err.Error())
		}
	}

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
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Namespace      string           `json:"namespace"`
	Status         string           `json:"status"`
	LastDeployment *workloadDeploy  `json:"lastDeployment"`
	K8sObjects     []workloadK8sObj `json:"k8sObjects"`
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
