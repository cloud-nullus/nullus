package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type StackMonitoringHandler struct {
	stackRepo           port.StackRepository
	kubeconfigProvider  port.KubeconfigProvider
	collectMonitoringFn func(ctx context.Context, stack *domain.Stack, kubeconfig []byte) (*stackMonitoringResponse, error)
}

const stackNameLabelKey = "nullus.io/stack-name"

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
	TotalPods             int     `json:"total_pods"`
	ReadyPods             int     `json:"ready_pods"`
	CPURequestMillicores  int64   `json:"cpu_request_millicores"`
	CPULimitMillicores    int64   `json:"cpu_limit_millicores"`
	CPUUsageMillicores    int64   `json:"cpu_usage_millicores"`
	MemoryRequestMiB      int64   `json:"memory_request_mib"`
	MemoryLimitMiB        int64   `json:"memory_limit_mib"`
	MemoryUsageMiB        int64   `json:"memory_usage_mib"`
	StorageRequestGiB     int64   `json:"storage_request_gib"`
	StorageLimitGiB       int64   `json:"storage_limit_gib"`
	StorageUsageGiB       float64 `json:"storage_usage_gib"`
	StorageUsageAvailable bool    `json:"storage_usage_available"`
	UsageAvailable        bool    `json:"usage_available"`
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
	Name                 string  `json:"name"`
	Phase                string  `json:"phase"`
	Ready                bool    `json:"ready"`
	RestartCount         int32   `json:"restart_count"`
	NodeName             string  `json:"node_name"`
	CPURequestMillicores int64   `json:"cpu_request_millicores"`
	CPULimitMillicores   int64   `json:"cpu_limit_millicores"`
	CPUUsageMillicores   int64   `json:"cpu_usage_millicores"`
	MemoryRequestMiB     int64   `json:"memory_request_mib"`
	MemoryLimitMiB       int64   `json:"memory_limit_mib"`
	MemoryUsageMiB       int64   `json:"memory_usage_mib"`
	StorageRequestGiB    int64   `json:"storage_request_gib"`
	StorageLimitGiB      int64   `json:"storage_limit_gib"`
	StorageUsageGiB      float64 `json:"storage_usage_gib"`
	Status               string  `json:"status"`
}

type podUsage struct {
	CPUUsageMillicores int64
	MemoryUsageMiB     int64
}

type pvcStorageStats struct {
	RequestGiB int64
	LimitGiB   int64
	UsageGiB   float64
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

	selector := ""
	if stack != nil && strings.TrimSpace(stack.Name) != "" {
		selector = fmt.Sprintf("%s=%s", stackNameLabelKey, strings.TrimSpace(stack.Name))
	}

	listOptions := metav1.ListOptions{}
	if selector != "" {
		listOptions.LabelSelector = selector
	}

	pods, err := clientset.CoreV1().Pods(ns).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	if len(pods.Items) == 0 && selector != "" {
		pods, err = clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list pods fallback: %w", err)
		}
	}

	deployments, err := clientset.AppsV1().Deployments(ns).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	if len(deployments.Items) == 0 && selector != "" {
		deployments, err = clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list deployments fallback: %w", err)
		}
	}

	statefulSets, err := clientset.AppsV1().StatefulSets(ns).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("list statefulsets: %w", err)
	}
	if len(statefulSets.Items) == 0 && selector != "" {
		statefulSets, err = clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list statefulsets fallback: %w", err)
		}
	}

	pvcStatsByName := map[string]pvcStorageStats{}
	if pvcs, pvcErr := clientset.CoreV1().PersistentVolumeClaims(ns).List(ctx, listOptions); pvcErr == nil {
		if len(pvcs.Items) == 0 && selector != "" {
			if fallbackPVCs, fallbackErr := clientset.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{}); fallbackErr == nil {
				pvcs = fallbackPVCs
			}
		}
		pvcStatsByName = buildPVCStorageStats(pvcs.Items)
	}

	if pvcUsageByName, ok := collectPVCStorageUsageByNameWithNodeStats(ctx, clientset, ns, pods.Items); ok {
		for pvcName, usageGiB := range pvcUsageByName {
			stats := pvcStatsByName[pvcName]
			stats.UsageGiB = usageGiB
			pvcStatsByName[pvcName] = stats
		}
	}

	podUsageMap, usageAvailable := collectPodUsageWithKubectl(ctx, kubeconfig, ns)
	stackCfg := extractStackConfig(stack)
	podStatuses, _, _ := toPodMonitoringStatuses(pods.Items, podUsageMap, pvcStatsByName)
	selectedTools := selectedToolTypes(stackCfg)
	podStatuses, podStatusCounts, summary := filterMonitoringToSelectedTools(selectedTools, podStatuses)
	summary.UsageAvailable = usageAvailable
	if summary.TotalPods > 0 {
		summary.StorageUsageAvailable = true
	}

	out := &stackMonitoringResponse{
		StackID:           stack.ID,
		Namespace:         ns,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		Summary:           summary,
		PodStatusCounts:   podStatusCounts,
		InstalledResource: filterInstalledResourcesToSelectedTools(selectedTools, toInstalledResourceStatuses(deployments.Items, statefulSets.Items)),
		OSSStatuses:       toOSSStatuses(selectedTools, podStatuses),
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

func toPodMonitoringStatuses(pods []corev1.Pod, usageByPod map[string]podUsage, pvcStatsByName map[string]pvcStorageStats) ([]podMonitoringStatus, []namedCount, monitoringSummary) {
	out := make([]podMonitoringStatus, 0, len(pods))
	statusCountMap := make(map[string]int)
	var summary monitoringSummary

	for _, pod := range pods {
		cpuReq, cpuLimit, memReq, memLimit := podResourceTotals(pod)
		storageReq, storageLimit, storageUsage := podStorageTotals(pod, pvcStatsByName)
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
			StorageRequestGiB:    storageReq,
			StorageLimitGiB:      storageLimit,
			StorageUsageGiB:      storageUsage,
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

func buildPVCStorageStats(pvcs []corev1.PersistentVolumeClaim) map[string]pvcStorageStats {
	out := make(map[string]pvcStorageStats, len(pvcs))
	for _, pvc := range pvcs {
		name := strings.TrimSpace(pvc.Name)
		if name == "" {
			continue
		}
		req := int64(0)
		if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			req = q.Value() / (1024 * 1024 * 1024)
		}
		lim := int64(0)
		if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			lim = q.Value() / (1024 * 1024 * 1024)
		}
		if lim <= 0 {
			lim = req
		}
		out[name] = pvcStorageStats{RequestGiB: req, LimitGiB: lim}
	}
	return out
}

type nodeStatsSummary struct {
	Pods []nodePodStats `json:"pods"`
}

type nodePodStats struct {
	PodRef struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"podRef"`
	Volume []nodeVolumeStats `json:"volume"`
}

type nodeVolumeStats struct {
	PVCRef *struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"pvcRef"`
	UsedBytes *uint64 `json:"usedBytes"`
}

func collectPVCStorageUsageByNameWithNodeStats(ctx context.Context, clientset *kubernetes.Clientset, namespace string, pods []corev1.Pod) (map[string]float64, bool) {
	if clientset == nil || len(pods) == 0 {
		return map[string]float64{}, false
	}

	nodeNames := make(map[string]struct{})
	podNames := make(map[string]struct{})
	for _, pod := range pods {
		podNames[pod.Name] = struct{}{}
		if node := strings.TrimSpace(pod.Spec.NodeName); node != "" {
			nodeNames[node] = struct{}{}
		}
	}
	if len(nodeNames) == 0 {
		return map[string]float64{}, false
	}

	pvcUsedBytesByName := make(map[string]uint64)
	hit := false

	for nodeName := range nodeNames {
		nodeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		raw, err := clientset.CoreV1().RESTClient().Get().
			Resource("nodes").
			Name(nodeName).
			SubResource("proxy").
			Suffix("stats", "summary").
			Do(nodeCtx).
			Raw()
		cancel()
		if err != nil {
			continue
		}

		var summary nodeStatsSummary
		if err := json.Unmarshal(raw, &summary); err != nil {
			continue
		}

		for _, podStat := range summary.Pods {
			if podStat.PodRef.Namespace != namespace {
				continue
			}
			if _, ok := podNames[podStat.PodRef.Name]; !ok {
				continue
			}

			for _, vol := range podStat.Volume {
				if vol.PVCRef == nil || vol.UsedBytes == nil {
					continue
				}
				if vol.PVCRef.Namespace != "" && vol.PVCRef.Namespace != namespace {
					continue
				}
				if prev, exists := pvcUsedBytesByName[vol.PVCRef.Name]; !exists || *vol.UsedBytes > prev {
					pvcUsedBytesByName[vol.PVCRef.Name] = *vol.UsedBytes
				}
				hit = true
			}
		}
	}

	if !hit {
		return map[string]float64{}, false
	}

	const gib = 1024 * 1024 * 1024
	usageByPVCGiB := make(map[string]float64, len(pvcUsedBytesByName))
	for pvcName, used := range pvcUsedBytesByName {
		usageGiB := float64(used) / gib
		usageByPVCGiB[pvcName] = math.Round(usageGiB*100) / 100
	}
	return usageByPVCGiB, true
}

func podStorageTotals(pod corev1.Pod, pvcStatsByName map[string]pvcStorageStats) (reqGiB int64, limitGiB int64, usageGiB float64) {
	seen := make(map[string]struct{})
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvcName := strings.TrimSpace(vol.PersistentVolumeClaim.ClaimName)
		if pvcName == "" {
			continue
		}
		if _, exists := seen[pvcName]; exists {
			continue
		}
		seen[pvcName] = struct{}{}
		stats, ok := pvcStatsByName[pvcName]
		if !ok {
			continue
		}
		reqGiB += stats.RequestGiB
		limitGiB += stats.LimitGiB
		usageGiB += stats.UsageGiB
	}
	usageGiB = math.Round(usageGiB*100) / 100
	return reqGiB, limitGiB, usageGiB
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

func toOSSStatuses(types []selectedToolType, pods []podMonitoringStatus) []ossMonitoringStatus {
	out := make([]ossMonitoringStatus, 0, len(types))

	for _, t := range types {
		allMatched := make([]podMonitoringStatus, 0)
		for _, pod := range pods {
			if matchesAnyPrefix(strings.ToLower(pod.Name), t.PodNamePrefixes) {
				allMatched = append(allMatched, pod)
			}
		}

		// Exclude one-shot completed Job pods (e.g. gitlab-migrations) from health status calculation.
		// These pods finish with Succeeded and should not degrade OSS status.
		matched := make([]podMonitoringStatus, 0, len(allMatched))
		for _, pod := range allMatched {
			if isOneShotCompletedPod(pod) {
				continue
			}
			matched = append(matched, pod)
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

		var status string
		switch {
		case len(matched) == 0 && len(allMatched) > 0:
			status = "running"
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

func isOneShotCompletedPod(pod podMonitoringStatus) bool {
	phase := strings.ToLower(strings.TrimSpace(pod.Phase))
	if phase != "succeeded" {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(pod.Name))
	return strings.Contains(name, "migrations") || strings.Contains(name, "job")
}

func filterMonitoringToSelectedTools(types []selectedToolType, pods []podMonitoringStatus) ([]podMonitoringStatus, []namedCount, monitoringSummary) {
	if len(types) == 0 || len(pods) == 0 {
		return []podMonitoringStatus{}, []namedCount{}, monitoringSummary{}
	}

	prefixes := make([]string, 0, len(types)*4)
	for _, tool := range types {
		prefixes = append(prefixes, tool.PodNamePrefixes...)
	}

	filtered := make([]podMonitoringStatus, 0, len(pods))
	for _, pod := range pods {
		if matchesAnyPrefix(strings.ToLower(pod.Name), prefixes) {
			filtered = append(filtered, pod)
		}
	}

	return summarizePodMonitoringStatuses(filtered)
}

func summarizePodMonitoringStatuses(pods []podMonitoringStatus) ([]podMonitoringStatus, []namedCount, monitoringSummary) {
	if len(pods) == 0 {
		return []podMonitoringStatus{}, []namedCount{}, monitoringSummary{}
	}

	out := append([]podMonitoringStatus(nil), pods...)
	statusCountMap := make(map[string]int)
	var summary monitoringSummary

	for _, pod := range out {
		summary.TotalPods++
		if pod.Ready {
			summary.ReadyPods++
		}
		summary.CPURequestMillicores += pod.CPURequestMillicores
		summary.CPULimitMillicores += pod.CPULimitMillicores
		summary.CPUUsageMillicores += pod.CPUUsageMillicores
		summary.MemoryRequestMiB += pod.MemoryRequestMiB
		summary.MemoryLimitMiB += pod.MemoryLimitMiB
		summary.MemoryUsageMiB += pod.MemoryUsageMiB
		summary.StorageRequestGiB += pod.StorageRequestGiB
		summary.StorageLimitGiB += pod.StorageLimitGiB
		summary.StorageUsageGiB += pod.StorageUsageGiB

		statusKey := strings.TrimSpace(pod.Phase)
		if statusKey == "" {
			statusKey = "Unknown"
		}
		statusCountMap[statusKey]++
	}

	summary.StorageUsageGiB = math.Round(summary.StorageUsageGiB*100) / 100

	counts := make([]namedCount, 0, len(statusCountMap))
	for k, v := range statusCountMap {
		counts = append(counts, namedCount{Name: k, Count: v})
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].Name < counts[j].Name })
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, counts, summary
}

func filterInstalledResourcesToSelectedTools(types []selectedToolType, resources []installedResourceStatus) []installedResourceStatus {
	if len(types) == 0 || len(resources) == 0 {
		return []installedResourceStatus{}
	}

	prefixes := make([]string, 0, len(types)*4)
	for _, tool := range types {
		prefixes = append(prefixes, tool.ResourceNamePrefixes...)
	}

	filtered := make([]installedResourceStatus, 0, len(resources))
	for _, resource := range resources {
		if matchesAnyPrefix(strings.ToLower(resource.Name), prefixes) {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

type selectedToolType struct {
	Key                  string
	Name                 string
	Version              string
	Enabled              bool
	PodNamePrefixes      []string
	ResourceNamePrefixes []string
}

func selectedToolTypes(cfg domain.StackConfig) []selectedToolType {
	fallbackNameByKey := map[string]string{
		"source_repository":  "gitlab",
		"cd_tool":            "argocd",
		"collection":         "prometheus",
		"visualization":      "grafana",
		"logging_collection": "loki",
		"logging_search":     "opensearch",
		"trace_layer":        "tempo",
		"storage_backend":    "minio",
	}

	tool := func(key string, sel domain.ToolSelection, prefixes ...string) selectedToolType {
		name := strings.TrimSpace(sel.Name)
		if name == "" {
			name = fallbackNameByKey[key]
			if name == "" {
				name = key
			}
		}
		version := strings.TrimSpace(sel.Version)
		if version == "" {
			version = "-"
		}
		return selectedToolType{
			Key:                  key,
			Name:                 name,
			Version:              version,
			Enabled:              sel.Enabled,
			PodNamePrefixes:      prefixes,
			ResourceNamePrefixes: prefixes,
		}
	}

	out := []selectedToolType{
		tool("source_repository", cfg.Artifacts.SourceRepository, "gitlab"),
		tool("cd_tool", cfg.Pipeline.CDTool, "argo-cd-argocd"),
		tool("collection", cfg.Monitoring.Collection, "prometheus", "kube-prometheus-stack", "alertmanager-kube-prometheus-stack"),
		tool("visualization", cfg.Monitoring.Visualization, "grafana"),
		tool("logging_collection", cfg.Logging.Collection, "loki"),
		tool("logging_search", cfg.Logging.Search, "opensearch", "elasticsearch"),
		tool("trace_layer", cfg.Logging.TraceLayer, "tempo", "jaeger"),
		tool("storage_backend", cfg.Artifacts.StorageBackend, "nullus-minio", "minio"),
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
