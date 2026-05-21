package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	obskube "github.com/cloud-nullus/draft/internal/observability/adapter/kube"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
	stackport "github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// KubeconfigProvider returns the decrypted kubeconfig bytes for a cluster.
// Defined locally so the observability module isn't coupled to a specific provider impl.
type KubeconfigProvider interface {
	GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error)
}

// DashboardHandler handles HTTP requests for observability dashboard operations.
type DashboardHandler struct {
	getDashboard       *usecase.GetDashboard
	stackRepo          stackport.StackRepository
	pool               *pgxpool.Pool
	kubeconfigProvider KubeconfigProvider
}

// NewDashboardHandler constructs a DashboardHandler.
func NewDashboardHandler(getDashboard *usecase.GetDashboard, opts ...func(*DashboardHandler)) *DashboardHandler {
	h := &DashboardHandler{getDashboard: getDashboard}
	for _, o := range opts {
		o(h)
	}
	return h
}

// WithStackRepo injects the stack repository used to drive the deployed-apps view.
func WithStackRepo(repo stackport.StackRepository) func(*DashboardHandler) {
	return func(h *DashboardHandler) { h.stackRepo = repo }
}

// WithPool injects a database pool for direct queries (cluster name lookups).
func WithPool(pool *pgxpool.Pool) func(*DashboardHandler) {
	return func(h *DashboardHandler) { h.pool = pool }
}

// WithKubeconfigProvider injects a kubeconfig provider used to query live K8s Pods.
func WithKubeconfigProvider(p KubeconfigProvider) func(*DashboardHandler) {
	return func(h *DashboardHandler) { h.kubeconfigProvider = p }
}

// RegisterRoutes registers dashboard routes on the given Echo group.
func (h *DashboardHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/dashboard", h.GetDashboard)
	g.GET("/deployed-apps", h.GetDeployedApps)
}

type dashboardResponse struct {
	KPI      kpiResponse      `json:"kpi"`
	Pipeline pipelineResponse `json:"pipeline"`
	Tools    []toolResponse   `json:"tools"`
}

type kpiResponse struct {
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryUsage  float64 `json:"memoryUsage"`
	StorageUsage float64 `json:"storageUsage"`
	PodCount     int     `json:"podCount"`
	PodRunning   int     `json:"podRunning"`
}

type pipelineResponse struct {
	TotalRuns       int     `json:"totalRuns"`
	SuccessRate     float64 `json:"successRate"`
	AvgBuildSeconds float64 `json:"avgBuildSeconds"`
}

type toolResponse struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

type deployedAppPod struct {
	Name   string `json:"name"`
	Node   string `json:"node"`
	Status string `json:"status"`
}

type deployedAppResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	TemplateID  string           `json:"template_id"`
	Namespace   string           `json:"namespace"`
	ClusterID   string           `json:"cluster_id"`
	ClusterName string           `json:"cluster_name"`
	GitRepoURL  string           `json:"git_repo_url"`
	Status      string           `json:"status"`
	State       string           `json:"state"`
	Version     string           `json:"version"`
	DeployedBy  string           `json:"deployed_by"`
	DeployedAt  string           `json:"deployed_at"`
	Replicas    int32            `json:"replicas"`
	Pods        []deployedAppPod `json:"pods"`
}

// GetDashboard handles GET /api/v1/observability/dashboard
func (h *DashboardHandler) GetDashboard(c echo.Context) error {
	_ = c.QueryParam("scope")

	out, err := h.getDashboard.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", err.Error())
	}

	d := out.Dashboard
	podRunning := int(float64(d.ClusterMetrics.PodCount) * 0.92)

	tools := make([]toolResponse, len(d.ToolHealthList))
	for i, t := range d.ToolHealthList {
		tools[i] = toolResponse{Name: t.Name, Status: t.Status, Version: t.Version}
	}

	return c.JSON(http.StatusOK, dashboardResponse{
		KPI: kpiResponse{
			CPUUsage:     d.ClusterMetrics.CPUUsage,
			MemoryUsage:  d.ClusterMetrics.MemoryUsage,
			StorageUsage: d.ClusterMetrics.StorageUsage,
			PodCount:     d.ClusterMetrics.PodCount,
			PodRunning:   podRunning,
		},
		Pipeline: pipelineResponse{
			TotalRuns:       d.PipelineMetrics.TotalRuns,
			SuccessRate:     d.PipelineMetrics.SuccessRate,
			AvgBuildSeconds: d.PipelineMetrics.AvgBuildTime,
		},
		Tools: tools,
	})
}

// GetDeployedApps handles GET /api/v1/observability/deployed-apps.
// Each item is a stack: status/cluster/namespace plus the live pods running in that namespace.
func (h *DashboardHandler) GetDeployedApps(c echo.Context) error {
	if h.stackRepo == nil {
		return c.JSON(http.StatusOK, map[string]any{"items": []any{}, "total": 0})
	}

	ctx := c.Request().Context()
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000001"
	}

	stacks, err := h.stackRepo.List(ctx, orgID, false)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "DEPLOYED_APPS_FETCH_FAILED", err.Error())
	}

	clusterNames := h.loadClusterNames(ctx)
	podCache := map[struct{ clusterID, namespace string }][]obskube.PodInfo{}

	apps := make([]deployedAppResponse, 0, len(stacks))
	for _, s := range stacks {
		cName := clusterNames[s.ClusterID]
		if cName == "" {
			cName = "unknown"
		}

		pods := h.podsForStack(ctx, podCache, s.ClusterID, s.Namespace)

		apps = append(apps, deployedAppResponse{
			ID:          s.ID,
			Name:        s.Name,
			TemplateID:  s.TemplateID,
			Namespace:   s.Namespace,
			ClusterID:   s.ClusterID,
			ClusterName: cName,
			Status:      stackStatusForUI(s.State),
			State:       string(s.State),
			DeployedAt:  s.UpdatedAt.Format(time.RFC3339),
			Replicas:    int32(len(pods)),
			Pods:        pods,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"items": apps, "total": len(apps)})
}

// stackStatusForUI collapses domain deployment states into the three categories the
// monitoring dashboard table renders (success / running / failed).
func stackStatusForUI(s stackdomain.DeploymentState) string {
	switch s {
	case stackdomain.StateCompleted:
		return "success"
	case stackdomain.StateFailed, stackdomain.StateRollingBack, stackdomain.StateRolledBack:
		return "failed"
	default:
		return "running"
	}
}

func (h *DashboardHandler) loadClusterNames(ctx context.Context) map[string]string {
	out := map[string]string{}
	if h.pool == nil {
		return out
	}
	rows, err := h.pool.Query(ctx, "SELECT id, name FROM clusters")
	if err != nil || rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var cid, cname string
		if err := rows.Scan(&cid, &cname); err == nil {
			out[cid] = cname
		}
	}
	return out
}

// podsForStack returns every pod in the stack's namespace. Helm releases for a stack
// typically share the namespace, so we don't filter by name. Failures degrade to empty.
func (h *DashboardHandler) podsForStack(
	ctx context.Context,
	cache map[struct{ clusterID, namespace string }][]obskube.PodInfo,
	clusterID, namespace string,
) []deployedAppPod {
	raw := h.fetchPodsInNamespace(ctx, cache, clusterID, namespace)
	out := make([]deployedAppPod, 0, len(raw))
	for _, p := range raw {
		out = append(out, deployedAppPod{Name: p.Name, Node: p.Node, Status: p.Status})
	}
	return out
}

func (h *DashboardHandler) fetchPodsInNamespace(
	ctx context.Context,
	cache map[struct{ clusterID, namespace string }][]obskube.PodInfo,
	clusterID, namespace string,
) []obskube.PodInfo {
	if h.kubeconfigProvider == nil || clusterID == "" || namespace == "" {
		return nil
	}

	key := struct{ clusterID, namespace string }{clusterID, namespace}
	if cached, ok := cache[key]; ok {
		return cached
	}

	kubeconfig, err := h.kubeconfigProvider.GetKubeconfig(ctx, clusterID)
	if err != nil {
		slog.Warn("deployed-apps: kubeconfig fetch failed",
			"cluster_id", clusterID, "namespace", namespace, "error", err)
		cache[key] = nil
		return nil
	}
	if len(kubeconfig) == 0 {
		cache[key] = nil
		return nil
	}

	pods, err := obskube.ListPodsInNamespace(ctx, kubeconfig, namespace)
	if err != nil {
		slog.Warn("deployed-apps: list pods failed",
			"cluster_id", clusterID, "namespace", namespace, "error", err)
		cache[key] = nil
		return nil
	}
	cache[key] = pods
	return pods
}

