package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/labstack/echo/v4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type StackMonitoringHandler struct {
	stackRepo           port.StackRepository
	kubeconfigProvider  port.KubeconfigProvider
	collectMonitoringFn func(ctx context.Context, stack *domain.Stack, kubeconfig []byte) (*stackMonitoringResponse, error)
}

func NewStackMonitoringHandler(stackRepo port.StackRepository, kubeconfigProvider port.KubeconfigProvider) *StackMonitoringHandler {
	return &StackMonitoringHandler{
		stackRepo:           stackRepo,
		kubeconfigProvider:  kubeconfigProvider,
		collectMonitoringFn: collectStackMonitoring,
	}
}

func (h *StackMonitoringHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/:stackId/monitoring", h.GetMonitoring)
}

func (h *StackMonitoringHandler) GetMonitoring(c echo.Context) error {
	stackID := strings.TrimSpace(c.Param("stackId"))
	if stackID == "" {
		return errorResponse(c, 400, "STACK_ID_REQUIRED", "stack_id is required")
	}

	stack, err := h.stackRepo.GetByID(c.Request().Context(), stackID)
	if err != nil || stack == nil {
		return errorResponse(c, 404, "STACK_NOT_FOUND", "stack not found")
	}

	if h.kubeconfigProvider == nil {
		return errorResponse(c, 500, "KUBECONFIG_PROVIDER_NOT_CONFIGURED", "kubeconfig provider not configured")
	}

	kubeconfig, err := h.kubeconfigProvider.GetKubeconfig(c.Request().Context(), stack.ClusterID)
	if err != nil {
		return errorResponse(c, 500, "KUBECONFIG_LOAD_FAILED", err.Error())
	}
	if len(kubeconfig) == 0 {
		return errorResponse(c, 400, "KUBECONFIG_NOT_REGISTERED", "kubeconfig is not registered for this cluster")
	}

	out, err := h.collectMonitoringFn(c.Request().Context(), stack, kubeconfig)
	if err != nil {
		return errorResponse(c, 502, "STACK_MONITORING_FAILED", err.Error())
	}

	return c.JSON(200, out)
}

type stackMonitoringResponse struct {
	StackID           string                    `json:"stack_id"`
	Namespace         string                    `json:"namespace"`
	Timestamp         string                    `json:"timestamp"`
	Summary           monitoringSummary         `json:"summary"`
	PodStatusCounts   []namedCount              `json:"pod_status_counts"`
	InstalledResource []installedResourceStatus `json:"installed_resources"`
	OSSStatuses       []ossMonitoringStatus     `json:"oss_statuses"`
}

type monitoringSummary struct {
	TotalPods            int   `json:"total_pods"`
	ReadyPods            int   `json:"ready_pods"`
	CPURequestMillicores int64 `json:"cpu_request_millicores"`
	CPULimitMillicores   int64 `json:"cpu_limit_millicores"`
	CPUUsageMillicores   int64 `json:"cpu_usage_millicores"`
	MemoryRequestMiB     int64 `json:"memory_request_mib"`
	MemoryLimitMiB       int64 `json:"memory_limit_mib"`
	MemoryUsageMiB       int64 `json:"memory_usage_mib"`
	UsageAvailable       bool  `json:"usage_available"`
}

type namedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type installedResourceStatus struct {
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	DesiredReplicas   int32  `json:"desired_replicas"`
	ReadyReplicas     int32  `json:"ready_replicas"`
	AvailableReplicas int32  `json:"available_replicas"`
	Status            string `json:"status"`
}

type ossMonitoringStatus struct {
	Key       string                `json:"key"`
	Name      string                `json:"name"`
	Version   string                `json:"version"`
	Enabled   bool                  `json:"enabled"`
	Status    string                `json:"status"`
	PodCount  int                   `json:"pod_count"`
	ReadyPods int                   `json:"ready_pods"`
	Pods      []podMonitoringStatus `json:"pods"`
}

type podMonitoringStatus struct {
	Name                 string `json:"name"`
	Phase                string `json:"phase"`
	Ready                bool   `json:"ready"`
	RestartCount         int32  `json:"restart_count"`
	NodeName             string `json:"node_name"`
	CPURequestMillicores int64  `json:"cpu_request_millicores"`
	CPULimitMillicores   int64  `json:"cpu_limit_millicores"`
	CPUUsageMillicores   int64  `json:"cpu_usage_millicores"`
	MemoryRequestMiB     int64  `json:"memory_request_mib"`
	MemoryLimitMiB       int64  `json:"memory_limit_mib"`
	MemoryUsageMiB       int64  `json:"memory_usage_mib"`
	Status               string `json:"status"`
}

type podUsage struct {
	CPUUsageMillicores int64
	MemoryUsageMiB     int64
}

func collectStackMonitoring(ctx context.Context, stack *domain.Stack, kubeconfig []byte) (*stackMonitoringResponse, error) {
	ns := strings.TrimSpace(stack.Namespace)
	if ns == "" {
		ns = "nullus"
	}

	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	restCfg.Timeout = 10 * time.Second

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	deployments, err := clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	statefulSets, err := clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list statefulsets: %w", err)
	}

	podUsageMap, usageAvailable := collectPodUsageWithKubectl(ctx, kubeconfig, ns)
	stackCfg := extractStackConfig(stack)
	podStatuses, podStatusCounts, summary := toPodMonitoringStatuses(pods.Items, podUsageMap)
	summary.UsageAvailable = usageAvailable

	out := &stackMonitoringResponse{
		StackID:           stack.ID,
		Namespace:         ns,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		Summary:           summary,
		PodStatusCounts:   podStatusCounts,
		InstalledResource: toInstalledResourceStatuses(deployments.Items, statefulSets.Items),
		OSSStatuses:       toOSSStatuses(stackCfg, podStatuses),
	}

	return out, nil
}

func extractStackConfig(stack *domain.Stack) domain.StackConfig {
	if stack == nil || stack.Config == nil {
		return domain.StackConfig{}
	}

	switch cfg := stack.Config.(type) {
	case domain.StackConfig:
		return cfg
	case *domain.StackConfig:
		if cfg != nil {
			return *cfg
		}
		return domain.StackConfig{}
	default:
		b, err := json.Marshal(cfg)
		if err != nil {
			return domain.StackConfig{}
		}
		var decoded domain.StackConfig
		if err := json.Unmarshal(b, &decoded); err != nil {
			return domain.StackConfig{}
		}
		return decoded
	}
}

func toPodMonitoringStatuses(pods []corev1.Pod, usageByPod map[string]podUsage) ([]podMonitoringStatus, []namedCount, monitoringSummary) {
	out := make([]podMonitoringStatus, 0, len(pods))
	statusCountMap := make(map[string]int)
	var summary monitoringSummary

	for _, pod := range pods {
		cpuReq, cpuLimit, memReq, memLimit := podResourceTotals(pod)
		ready := isPodReady(pod)
		restarts := podRestartCount(pod)
		status := classifyPodStatus(pod)
		usage := usageByPod[pod.Name]

		item := podMonitoringStatus{
			Name:                 pod.Name,
			Phase:                string(pod.Status.Phase),
			Ready:                ready,
			RestartCount:         restarts,
			NodeName:             pod.Spec.NodeName,
			CPURequestMillicores: cpuReq,
			CPULimitMillicores:   cpuLimit,
			CPUUsageMillicores:   usage.CPUUsageMillicores,
			MemoryRequestMiB:     memReq,
			MemoryLimitMiB:       memLimit,
			MemoryUsageMiB:       usage.MemoryUsageMiB,
			Status:               status,
		}

		out = append(out, item)
		summary.TotalPods++
		if ready {
			summary.ReadyPods++
		}
		summary.CPURequestMillicores += cpuReq
		summary.CPULimitMillicores += cpuLimit
		summary.CPUUsageMillicores += usage.CPUUsageMillicores
		summary.MemoryRequestMiB += memReq
		summary.MemoryLimitMiB += memLimit
		summary.MemoryUsageMiB += usage.MemoryUsageMiB

		statusKey := string(pod.Status.Phase)
		if statusKey == "" {
			statusKey = "Unknown"
		}
		statusCountMap[statusKey]++
	}

	counts := make([]namedCount, 0, len(statusCountMap))
	for k, v := range statusCountMap {
		counts = append(counts, namedCount{Name: k, Count: v})
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].Name < counts[j].Name })

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, counts, summary
}

func collectPodUsageWithKubectl(ctx context.Context, kubeconfig []byte, namespace string) (map[string]podUsage, bool) {
	if len(kubeconfig) == 0 {
		return map[string]podUsage{}, false
	}

	tmp, err := os.CreateTemp("", "nullus-monitoring-kubeconfig-*.yaml")
	if err != nil {
		return map[string]podUsage{}, false
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	if err := os.WriteFile(path, kubeconfig, 0o600); err != nil {
		return map[string]podUsage{}, false
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx,
		"kubectl",
		"--kubeconfig", path,
		"-n", namespace,
		"top", "pod", "--no-headers",
	)
	out, err := cmd.Output()
	if err != nil {
		return map[string]podUsage{}, false
	}

	usage := make(map[string]podUsage)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		cpu, cpuErr := parseCPUToMillicores(parts[1])
		mem, memErr := parseMemoryToMiB(parts[2])
		if cpuErr != nil || memErr != nil {
			continue
		}
		usage[parts[0]] = podUsage{
			CPUUsageMillicores: cpu,
			MemoryUsageMiB:     mem,
		}
	}

	return usage, len(usage) > 0
}

func parseCPUToMillicores(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty cpu usage")
	}
	if strings.HasSuffix(raw, "m") {
		v, err := strconv.ParseInt(strings.TrimSuffix(raw, "m"), 10, 64)
		if err != nil {
			return 0, err
		}
		return v, nil
	}
	cores, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	return int64(cores * 1000), nil
}

func parseMemoryToMiB(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty memory usage")
	}

	if strings.HasSuffix(raw, "Mi") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "Mi"), 64)
		if err != nil {
			return 0, err
		}
		return int64(v), nil
	}
	if strings.HasSuffix(raw, "Gi") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "Gi"), 64)
		if err != nil {
			return 0, err
		}
		return int64(v * 1024), nil
	}
	if strings.HasSuffix(raw, "Ki") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "Ki"), 64)
		if err != nil {
			return 0, err
		}
		return int64(v / 1024), nil
	}
	if strings.HasSuffix(raw, "M") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "M"), 64)
		if err != nil {
			return 0, err
		}
		return int64(v), nil
	}
	if strings.HasSuffix(raw, "G") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "G"), 64)
		if err != nil {
			return 0, err
		}
		return int64(v * 1024), nil
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	return int64(v / (1024 * 1024)), nil
}

func podResourceTotals(pod corev1.Pod) (cpuReqMillicores, cpuLimitMillicores, memReqMiB, memLimitMiB int64) {
	for _, c := range pod.Spec.Containers {
		if q := c.Resources.Requests.Cpu(); q != nil {
			cpuReqMillicores += q.MilliValue()
		}
		if q := c.Resources.Limits.Cpu(); q != nil {
			cpuLimitMillicores += q.MilliValue()
		}
		if q := c.Resources.Requests.Memory(); q != nil {
			memReqMiB += q.Value() / (1024 * 1024)
		}
		if q := c.Resources.Limits.Memory(); q != nil {
			memLimitMiB += q.Value() / (1024 * 1024)
		}
	}
	return
}

func isPodReady(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func podRestartCount(pod corev1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return restarts
}

func classifyPodStatus(pod corev1.Pod) string {
	if pod.Status.Phase == corev1.PodFailed {
		return "error"
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := strings.ToLower(strings.TrimSpace(cs.State.Waiting.Reason))
			if strings.Contains(reason, "crashloop") || strings.Contains(reason, "error") || strings.Contains(reason, "imagepull") {
				return "error"
			}
			return "warning"
		}
	}
	if pod.Status.Phase == corev1.PodRunning && isPodReady(pod) {
		return "running"
	}
	if pod.Status.Phase == corev1.PodSucceeded {
		return "running"
	}
	return "warning"
}

func toInstalledResourceStatuses(deployments []appsv1.Deployment, statefulSets []appsv1.StatefulSet) []installedResourceStatus {
	out := make([]installedResourceStatus, 0, len(deployments)+len(statefulSets))

	for _, d := range deployments {
		desired := int32(1)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		status := "warning"
		if desired == 0 || (d.Status.ReadyReplicas >= desired && d.Status.AvailableReplicas >= desired) {
			status = "running"
		}
		out = append(out, installedResourceStatus{
			Kind:              "Deployment",
			Name:              d.Name,
			DesiredReplicas:   desired,
			ReadyReplicas:     d.Status.ReadyReplicas,
			AvailableReplicas: d.Status.AvailableReplicas,
			Status:            status,
		})
	}

	for _, s := range statefulSets {
		desired := int32(1)
		if s.Spec.Replicas != nil {
			desired = *s.Spec.Replicas
		}
		status := "warning"
		if desired == 0 || s.Status.ReadyReplicas >= desired {
			status = "running"
		}
		out = append(out, installedResourceStatus{
			Kind:              "StatefulSet",
			Name:              s.Name,
			DesiredReplicas:   desired,
			ReadyReplicas:     s.Status.ReadyReplicas,
			AvailableReplicas: s.Status.ReadyReplicas,
			Status:            status,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Name < out[j].Name
		}
		return out[i].Kind < out[j].Kind
	})

	return out
}

func toOSSStatuses(cfg domain.StackConfig, pods []podMonitoringStatus) []ossMonitoringStatus {
	types := selectedToolTypes(cfg)
	out := make([]ossMonitoringStatus, 0, len(types))

	for _, t := range types {
		matched := make([]podMonitoringStatus, 0)
		for _, pod := range pods {
			if matchesAnyPrefix(strings.ToLower(pod.Name), t.PodNamePrefixes) {
				matched = append(matched, pod)
			}
		}

		ready := 0
		hasError := false
		for _, pod := range matched {
			if pod.Ready {
				ready++
			}
			if pod.Status == "error" {
				hasError = true
			}
		}

		status := "warning"
		switch {
		case len(matched) == 0:
			status = "warning"
		case hasError:
			status = "error"
		case ready == len(matched):
			status = "running"
		default:
			status = "warning"
		}

		out = append(out, ossMonitoringStatus{
			Key:       t.Key,
			Name:      t.Name,
			Version:   t.Version,
			Enabled:   t.Enabled,
			Status:    status,
			PodCount:  len(matched),
			ReadyPods: ready,
			Pods:      matched,
		})
	}

	return out
}

type selectedToolType struct {
	Key             string
	Name            string
	Version         string
	Enabled         bool
	PodNamePrefixes []string
}

func selectedToolTypes(cfg domain.StackConfig) []selectedToolType {
	tool := func(key string, sel domain.ToolSelection, prefixes ...string) selectedToolType {
		name := strings.TrimSpace(sel.Name)
		if name == "" {
			name = key
		}
		version := strings.TrimSpace(sel.Version)
		if version == "" {
			version = "-"
		}
		return selectedToolType{Key: key, Name: name, Version: version, Enabled: sel.Enabled, PodNamePrefixes: prefixes}
	}

	out := []selectedToolType{
		tool("source_repository", cfg.Artifacts.SourceRepository, "gitlab-"),
		tool("cd_tool", cfg.Pipeline.CDTool, "argo-cd-argocd-"),
		tool("collection", cfg.Monitoring.Collection, "prometheus", "kube-prometheus-stack", "alertmanager-kube-prometheus-stack"),
		tool("visualization", cfg.Monitoring.Visualization, "grafana-"),
		tool("logging_collection", cfg.Logging.Collection, "loki-"),
		tool("logging_search", cfg.Logging.Search, "opensearch-", "elasticsearch-"),
		tool("trace_layer", cfg.Logging.TraceLayer, "tempo-", "jaeger-"),
		tool("storage_backend", cfg.Artifacts.StorageBackend, "nullus-minio", "minio-"),
	}

	filtered := make([]selectedToolType, 0, len(out))
	for _, t := range out {
		if !t.Enabled {
			continue
		}
		filtered = append(filtered, t)
	}

	return filtered
}

func matchesAnyPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, strings.ToLower(strings.TrimSpace(p))) {
			return true
		}
	}
	return false
}
