package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/cloud-nullus/draft/internal/admin/adapter/kube"
	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/pkg/crypto"
)

var namespaceListerFn = listNamespacesFromKubeconfig

// ClusterHandler handles HTTP requests for clusters.
type ClusterHandler struct {
	clusterUC     *usecase.ClusterUseCase
	audit         audit.Sink
	encryptionKey []byte
}

// NewClusterHandler creates a new ClusterHandler.
func NewClusterHandler(clusterUC *usecase.ClusterUseCase, auditLogger ...audit.Sink) *ClusterHandler {
	var logger audit.Sink
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &ClusterHandler{
		clusterUC:     clusterUC,
		audit:         logger,
		encryptionKey: []byte(os.Getenv("ENCRYPTION_KEY")),
	}
}

type registerClusterRequest struct {
	Name          string               `json:"name"`
	Type          domain.ClusterType   `json:"type"`
	Types         []domain.ClusterType `json:"types"`
	CloudProvider domain.CloudProvider `json:"cloud_provider"`
	Endpoint      string               `json:"endpoint"`
	OrgID         string               `json:"org_id"`
	Kubeconfig    string               `json:"kubeconfig"`
}

type updateClusterRequest struct {
	Name          string               `json:"name"`
	Type          domain.ClusterType   `json:"type,omitempty"`
	Types         []domain.ClusterType `json:"types,omitempty"`
	CloudProvider domain.CloudProvider `json:"cloud_provider,omitempty"`
	Endpoint      string               `json:"endpoint"`
	Kubeconfig    string               `json:"kubeconfig,omitempty"`
}

type verifyClusterDraftRequest struct {
	Endpoint   string `json:"endpoint"`
	Kubeconfig string `json:"kubeconfig"`
}

type clusterResponse struct {
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	Type              domain.ClusterType      `json:"type"`
	Types             []domain.ClusterType    `json:"types"`
	CloudProvider     domain.CloudProvider    `json:"cloud_provider"`
	Endpoint          string                  `json:"endpoint"`
	ConnectionStatus  domain.ConnectionStatus `json:"connection_status"`
	OrgID             string                  `json:"org_id"`
	NodeArchitectures []string                `json:"node_architectures"`
	Kubeconfig        string                  `json:"kubeconfig,omitempty"`
	CreatedAt         any                     `json:"created_at"`
	UpdatedAt         any                     `json:"updated_at"`
}

type clusterNamespaceResponseItem struct {
	Name string `json:"name"`
}

type clusterPodResponseItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Node      string `json:"node"`
}

type clusterMonitoringSummaryResponse struct {
	TotalNodes               int   `json:"total_nodes"`
	ReadyNodes               int   `json:"ready_nodes"`
	TotalPods                int   `json:"total_pods"`
	ReadyPods                int   `json:"ready_pods"`
	CPURequestMillicores     int64 `json:"cpu_request_millicores"`
	CPULimitMillicores       int64 `json:"cpu_limit_millicores"`
	CPUAllocatableMillicores int64 `json:"cpu_allocatable_millicores"`
	CPUUsageMillicores       int64 `json:"cpu_usage_millicores"`
	MemoryRequestMiB         int64 `json:"memory_request_mib"`
	MemoryLimitMiB           int64 `json:"memory_limit_mib"`
	MemoryAllocatableMiB     int64 `json:"memory_allocatable_mib"`
	MemoryUsageMiB           int64 `json:"memory_usage_mib"`
}

func toClusterResponse(cluster *domain.Cluster, kubeconfig string) clusterResponse {
	archs := cluster.NodeArchitectures
	if archs == nil {
		archs = []string{}
	}
	return clusterResponse{
		ID:                cluster.ID,
		Name:              cluster.Name,
		Type:              cluster.Type,
		Types:             domain.NormalizeClusterTypes(cluster.Types, cluster.Type),
		CloudProvider:     cluster.CloudProvider,
		Endpoint:          cluster.Endpoint,
		ConnectionStatus:  cluster.ConnectionStatus,
		OrgID:             cluster.OrgID,
		NodeArchitectures: archs,
		Kubeconfig:        kubeconfig,
		CreatedAt:         cluster.CreatedAt,
		UpdatedAt:         cluster.UpdatedAt,
	}
}

// RegisterCluster handles POST /api/v1/clusters.
func (h *ClusterHandler) RegisterCluster(c echo.Context) error {
	var req registerClusterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	kubeconfigText := strings.TrimSpace(req.Kubeconfig)
	var encryptedKubeconfig []byte
	if kubeconfigText != "" {
		if len(h.encryptionKey) != 32 {
			return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
		}
		// Try base64 decode first; if it fails, treat as raw YAML.
		kubeconfigBytes := []byte(req.Kubeconfig)
		if decoded, err := base64.StdEncoding.DecodeString(req.Kubeconfig); err == nil {
			kubeconfigBytes = decoded
		}
		kubeconfigText = string(kubeconfigBytes)
		encrypted, err := crypto.Encrypt(h.encryptionKey, kubeconfigBytes)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to encrypt kubeconfig")
		}
		encryptedKubeconfig = []byte(encrypted)
	}

	orgID := strings.TrimSpace(req.OrgID)
	if orgID == "" {
		firstOrg, err := h.clusterUC.GetFirstOrgID(c.Request().Context())
		if err != nil || strings.TrimSpace(firstOrg) == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "organization is required: create an organization first or provide org_id")
		}
		orgID = strings.TrimSpace(firstOrg)
	}

	cluster, err := h.clusterUC.RegisterCluster(c.Request().Context(), usecase.RegisterClusterInput{
		Name:          req.Name,
		Type:          req.Type,
		Types:         req.Types,
		CloudProvider: req.CloudProvider,
		Endpoint:      req.Endpoint,
		OrgID:         orgID,
	})
	if err != nil {
		return err
	}

	if len(encryptedKubeconfig) > 0 {
		if err := h.clusterUC.SaveKubeconfig(c.Request().Context(), cluster.ID, encryptedKubeconfig); err != nil {
			// Keep cluster state consistent: if kubeconfig persistence failed, roll back created cluster.
			_ = h.clusterUC.DeleteCluster(c.Request().Context(), cluster.ID)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save kubeconfig")
		}
		// Best-effort node architecture discovery. Discovery failures do not
		// roll back the registration — the cluster is stored with
		// connection_status=connection_failed and the user can Refresh later.
		if refreshed, refreshErr := h.clusterUC.RefreshDiscovery(c.Request().Context(), cluster.ID); refreshErr == nil {
			cluster = refreshed
		}
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "create",
			ResourceType: "cluster",
			ResourceID:   cluster.ID,
			Details: map[string]any{
				"name":           req.Name,
				"type":           req.Type,
				"types":          req.Types,
				"cloud_provider": req.CloudProvider,
				"endpoint":       req.Endpoint,
				"org_id":         orgID,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusCreated, toClusterResponse(cluster, kubeconfigText))
}

// ListClusters handles GET /api/v1/clusters.
func (h *ClusterHandler) ListClusters(c echo.Context) error {
	orgID := c.QueryParam("org_id")

	clusters, err := h.clusterUC.ListClusters(c.Request().Context(), orgID)
	if err != nil {
		return err
	}

	items := make([]clusterResponse, 0, len(clusters))
	for _, cluster := range clusters {
		items = append(items, toClusterResponse(cluster, ""))
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// GetCluster handles GET /api/v1/clusters/:id.
func (h *ClusterHandler) GetCluster(c echo.Context) error {
	id := c.Param("id")

	cluster, err := h.clusterUC.GetCluster(c.Request().Context(), id)
	if err != nil {
		return err
	}

	encryptedConfig, err := h.clusterUC.GetKubeconfig(c.Request().Context(), id)
	if err != nil {
		return err
	}

	kubeconfig := ""
	if len(encryptedConfig) > 0 {
		if len(h.encryptionKey) != 32 {
			return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
		}
		decrypted, err := crypto.Decrypt(h.encryptionKey, string(encryptedConfig))
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to decrypt kubeconfig")
		}
		kubeconfig = string(decrypted)
	}

	return c.JSON(http.StatusOK, toClusterResponse(cluster, kubeconfig))
}

func (h *ClusterHandler) UpdateCluster(c echo.Context) error {
	id := c.Param("id")

	var req updateClusterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	cluster, err := h.clusterUC.UpdateCluster(c.Request().Context(), id, usecase.UpdateClusterInput{
		Name:          req.Name,
		Type:          req.Type,
		Types:         req.Types,
		CloudProvider: req.CloudProvider,
		Endpoint:      req.Endpoint,
	})
	if err != nil {
		return err
	}
	savedKubeconfigText := ""
	if strings.TrimSpace(req.Kubeconfig) != "" {
		if len(h.encryptionKey) != 32 {
			return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
		}
		kubeconfigBytes := []byte(req.Kubeconfig)
		if decoded, err := base64.StdEncoding.DecodeString(req.Kubeconfig); err == nil {
			kubeconfigBytes = decoded
		}
		savedKubeconfigText = string(kubeconfigBytes)
		encrypted, err := crypto.Encrypt(h.encryptionKey, kubeconfigBytes)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to encrypt kubeconfig")
		}
		if err := h.clusterUC.SaveKubeconfig(c.Request().Context(), cluster.ID, []byte(encrypted)); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save kubeconfig")
		}
		// Re-discover node architectures against the new kubeconfig. Best
		// effort: discovery errors surface as connection_failed, not an HTTP
		// error, so the PATCH call still succeeds.
		if refreshed, refreshErr := h.clusterUC.RefreshDiscovery(c.Request().Context(), cluster.ID); refreshErr == nil {
			cluster = refreshed
		} else if latest, getErr := h.clusterUC.GetCluster(c.Request().Context(), cluster.ID); getErr == nil {
			cluster = latest
		}
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "update",
			ResourceType: "cluster",
			ResourceID:   cluster.ID,
			Details: map[string]any{
				"name":           req.Name,
				"type":           req.Type,
				"types":          req.Types,
				"cloud_provider": req.CloudProvider,
				"endpoint":       req.Endpoint,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusOK, toClusterResponse(cluster, savedKubeconfigText))
}

// DeleteCluster handles DELETE /api/v1/clusters/:id.
func (h *ClusterHandler) DeleteCluster(c echo.Context) error {
	id := c.Param("id")

	if err := h.clusterUC.DeleteCluster(c.Request().Context(), id); err != nil {
		return err
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "delete",
			ResourceType: "cluster",
			ResourceID:   id,
			Details:      map[string]any{},
			IPAddress:    c.RealIP(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *ClusterHandler) VerifyCluster(c echo.Context) error {
	id := c.Param("id")

	encryptedConfig, err := h.clusterUC.GetKubeconfig(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if len(encryptedConfig) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "kubeconfig is not registered for this cluster")
	}
	if len(h.encryptionKey) != 32 {
		return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
	}

	decrypted, err := crypto.Decrypt(h.encryptionKey, string(encryptedConfig))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to decrypt kubeconfig")
	}

	info, err := kube.DiscoverCluster(c.Request().Context(), decrypted)
	if err != nil {
		// Auto-heal flow: refresh discovery metadata, re-read kubeconfig, retry verify once.
		_, _ = h.clusterUC.RefreshDiscovery(c.Request().Context(), id)
		reloaded, reloadErr := h.clusterUC.GetKubeconfig(c.Request().Context(), id)
		if reloadErr == nil && len(reloaded) > 0 {
			if retryDecrypted, decErr := crypto.Decrypt(h.encryptionKey, string(reloaded)); decErr == nil {
				if retryInfo, retryErr := kube.DiscoverCluster(c.Request().Context(), retryDecrypted); retryErr == nil {
					info = retryInfo
					err = nil
				}
			}
		}
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway,
				fmt.Sprintf("cluster verify failed (possible stale endpoint/kubeconfig). run refresh-discovery and retry: %v", err))
		}
	}

	// Persist NodeArchitectures + mark connected via the use case so the
	// cluster row reflects what we just discovered.
	if _, err := h.clusterUC.RefreshDiscovery(c.Request().Context(), id); err != nil {
		// Discovery talked to the cluster successfully above, so this is a
		// persistence error — surface as 500, not 502.
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "verify",
			ResourceType: "cluster",
			ResourceID:   id,
			Details: map[string]any{
				"status":             "connected",
				"version":            info.ServerVersion,
				"node_architectures": info.NodeArchitectures,
				"node_count":         info.NodeCount,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status":             "connected",
		"version":            info.ServerVersion,
		"node_architectures": info.NodeArchitectures,
		"node_count":         info.NodeCount,
	})
}

// RefreshDiscovery is the explicit "re-probe this cluster now" endpoint
// used by the UI refresh button and by scheduled sweeps. It simply re-runs
// the same discovery flow that Register/Update trigger implicitly.
func (h *ClusterHandler) RefreshDiscovery(c echo.Context) error {
	id := c.Param("id")

	cluster, err := h.clusterUC.RefreshDiscovery(c.Request().Context(), id)
	if err != nil {
		if cluster != nil {
			// Discovery failed but the cluster row was updated (connection_failed).
			// Return 200 with the new state so the UI can display the status
			// change without treating the HTTP call as an error.
			return c.JSON(http.StatusOK, toClusterResponse(cluster, ""))
		}
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "refresh_discovery",
			ResourceType: "cluster",
			ResourceID:   id,
			Details: map[string]any{
				"node_architectures": cluster.NodeArchitectures,
			},
			IPAddress: c.RealIP(),
		})
	}
	return c.JSON(http.StatusOK, toClusterResponse(cluster, ""))
}

func (h *ClusterHandler) VerifyClusterDraft(c echo.Context) error {
	var req verifyClusterDraftRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	kubeconfigText := strings.TrimSpace(req.Kubeconfig)
	if kubeconfigText == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kubeconfig is required")
	}

	kubeconfigBytes := []byte(kubeconfigText)
	if decoded, err := base64.StdEncoding.DecodeString(kubeconfigText); err == nil {
		kubeconfigBytes = decoded
	}

	info, err := kube.DiscoverCluster(c.Request().Context(), kubeconfigBytes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status":             "connected",
		"version":            info.ServerVersion,
		"node_architectures": info.NodeArchitectures,
		"node_count":         info.NodeCount,
	})
}

func (h *ClusterHandler) ListNamespaces(c echo.Context) error {
	id := c.Param("id")

	encryptedConfig, err := h.clusterUC.GetKubeconfig(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if len(encryptedConfig) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "kubeconfig is not registered for this cluster")
	}
	if len(h.encryptionKey) != 32 {
		return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
	}

	decrypted, err := crypto.Decrypt(h.encryptionKey, string(encryptedConfig))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to decrypt kubeconfig")
	}

	namespaces, err := namespaceListerFn(decrypted)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	systemNamespaces := map[string]struct{}{
		"kube-system":        {},
		"kube-public":        {},
		"kube-node-lease":    {},
		"local-path-storage": {},
	}

	items := make([]clusterNamespaceResponseItem, 0, len(namespaces))
	for _, name := range namespaces {
		if _, isSystem := systemNamespaces[name]; isSystem {
			continue
		}
		items = append(items, clusterNamespaceResponseItem{Name: name})
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *ClusterHandler) GetMonitoringSummary(c echo.Context) error {
	id := c.Param("id")

	encryptedConfig, err := h.clusterUC.GetKubeconfig(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if len(encryptedConfig) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "kubeconfig is not registered for this cluster")
	}
	if len(h.encryptionKey) != 32 {
		return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
	}

	decrypted, err := crypto.Decrypt(h.encryptionKey, string(encryptedConfig))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to decrypt kubeconfig")
	}

	summary, err := getClusterMonitoringSummary(c.Request().Context(), decrypted)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	return c.JSON(http.StatusOK, summary)
}

func listNamespacesFromKubeconfig(kubeconfig []byte) ([]string, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	items := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		items = append(items, ns.Name)
	}
	return items, nil
}

func getClusterMonitoringSummary(ctx context.Context, kubeconfig []byte) (*clusterMonitoringSummaryResponse, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	restConfig.Timeout = 10 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	podCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	nodeList, err := clientset.CoreV1().Nodes().List(podCtx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podList, err := clientset.CoreV1().Pods("").List(podCtx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	summary := &clusterMonitoringSummaryResponse{}
	for _, node := range nodeList.Items {
		summary.TotalNodes++
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				summary.ReadyNodes++
				break
			}
		}
		if cpu, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
			summary.CPUAllocatableMillicores += cpu.MilliValue()
		}
		if mem, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
			summary.MemoryAllocatableMiB += mem.Value() / (1024 * 1024)
		}
	}
	// metrics-server (best-effort): cluster 에 metrics API 가 있으면 실제 사용량 합산.
	if mc, err := metricsclient.NewForConfig(restConfig); err == nil {
		if nm, err := mc.MetricsV1beta1().NodeMetricses().List(podCtx, metav1.ListOptions{}); err == nil {
			for _, m := range nm.Items {
				summary.CPUUsageMillicores += m.Usage.Cpu().MilliValue()
				summary.MemoryUsageMiB += m.Usage.Memory().Value() / (1024 * 1024)
			}
		}
	}

	for _, pod := range podList.Items {
		summary.TotalPods++
		if isClusterPodReady(pod) {
			summary.ReadyPods++
		}

		for _, container := range pod.Spec.Containers {
			if req, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				summary.CPURequestMillicores += req.MilliValue()
			}
			if lim, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
				summary.CPULimitMillicores += lim.MilliValue()
			}
			if req, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				summary.MemoryRequestMiB += req.Value() / (1024 * 1024)
			}
			if lim, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
				summary.MemoryLimitMiB += lim.Value() / (1024 * 1024)
			}
		}
	}

	return summary, nil
}

func isClusterPodReady(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (h *ClusterHandler) ListPods(c echo.Context) error {
	id := c.Param("id")

	encryptedConfig, err := h.clusterUC.GetKubeconfig(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if len(encryptedConfig) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "kubeconfig is not registered for this cluster")
	}
	if len(h.encryptionKey) != 32 {
		return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
	}

	decrypted, err := crypto.Decrypt(h.encryptionKey, string(encryptedConfig))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to decrypt kubeconfig")
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(decrypted)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	restConfig.Timeout = 10 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	podCtx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	podList, err := clientset.CoreV1().Pods("").List(podCtx, metav1.ListOptions{})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	items := make([]clusterPodResponseItem, 0, len(podList.Items))
	for _, pod := range podList.Items {
		totalContainers := len(pod.Spec.Containers)
		readyContainers := 0
		var totalRestarts int32 = 0
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyContainers++
			}
			totalRestarts += cs.RestartCount
		}
		items = append(items, clusterPodResponseItem{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Status:    string(pod.Status.Phase),
			Ready:     fmt.Sprintf("%d/%d", readyContainers, totalContainers),
			Restarts:  totalRestarts,
			Node:      pod.Spec.NodeName,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

// RegisterRoutes registers cluster routes on the given group.
func (h *ClusterHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/clusters", h.RegisterCluster)
	g.POST("/clusters/verify", h.VerifyClusterDraft)
	g.GET("/clusters", h.ListClusters)
	g.GET("/clusters/:id", h.GetCluster)
	g.GET("/clusters/:id/namespaces", h.ListNamespaces)
	g.GET("/clusters/:id/monitoring-summary", h.GetMonitoringSummary)
	g.GET("/clusters/:id/pods", h.ListPods)
	g.PATCH("/clusters/:id", h.UpdateCluster)
	g.DELETE("/clusters/:id", h.DeleteCluster)
	g.POST("/clusters/:id/verify", h.VerifyCluster)
	g.POST("/clusters/:id/refresh-discovery", h.RefreshDiscovery)
}
