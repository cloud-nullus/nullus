package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// CI/CD 로 배포된 애플리케이션의 실제 클러스터 상태.
//
// 개편 전에는 이 정보를 DB 의 파이프라인 행에서 지어냈다 — replicas 는 항상 2,
// 포트는 항상 8080, 파드 이름은 배포 시각 해시, 노드는 "<cluster>-node-1".
// 빈 값보다 나쁘다: 비면 "아직 없구나" 로 읽히지만 지어낸 값은 사실로 읽힌다.
//
// 배포 매니페스트가 nullus.io/stack-id 라벨을 붙인다
// (cicd/adapter/scaffold/renderer.go). 네임스페이스로는 스택을 판별할 수 없다 —
// 파이프라인이 default 에 깔 수도 있고 여러 스택이 한 네임스페이스를 공유할 수도
// 있다. 그래서 네임스페이스를 훑지 않고 이 라벨로 클러스터 전체에서 찾는다.
const stackIDLabelKey = "nullus.io/stack-id"

// templateIDLabelKey 는 어느 CI/CD 템플릿에서 나온 앱인지 알려 준다.
// 스캐폴딩이 붙인다 (cicd/adapter/scaffold/renderer.go).
const templateIDLabelKey = "nullus.io/cicd-template-id"

type livePod struct {
	Name      string
	Namespace string
	Phase     string
	Ready     bool
	Node      string
	// TemplateID 는 라벨에서 읽은 값이다. 라벨이 없으면 빈 문자열 —
	// 스캐폴딩 이전에 배포된 앱이 그렇다.
	TemplateID string
	// CPUMillicores / MemoryMiB 는 metrics-server 에서 읽은 실사용량이다.
	// 못 읽었으면 nil 이다 — 0 으로 두면 "안 쓰고 있다" 로 읽힌다.
	CPUMillicores *int64
	MemoryMiB     *int64
}

// podResourceUsage 는 metrics API 가 준 파드 하나의 실사용량이다.
type podResourceUsage struct {
	CPUMillicores int64
	MemoryMiB     int64
}

type liveDeployment struct {
	Name          string
	Namespace     string
	Replicas      int32
	ReadyReplicas int32
}

type liveService struct {
	Name      string
	Namespace string
	Port      int32
}

type clusterWorkloads struct {
	Deployments []liveDeployment
	Pods        []livePod
	Services    []liveService
}

type podCounts struct {
	Running int
	Pending int
	Failed  int
}

// workloadsFor 는 네임스페이스 하나의 배포 앱 상태를 읽는다.
//
// 조회 실패를 에러로 올리지 않는다. 클러스터가 잠시 안 보이는 것과 파이프라인
// 목록을 못 보여주는 것은 다른 문제다 — 목록은 DB 로 그릴 수 있으므로, 여기서는
// 빈 값을 주고 화면이 "실행 중인 워크로드 없음" 으로 표시하게 둔다.
func (h *StackHandler) workloadsFor(ctx context.Context, kubeconfig []byte, stackID string) (clusterWorkloads, error) {
	if len(kubeconfig) == 0 || strings.TrimSpace(stackID) == "" {
		return clusterWorkloads{}, nil
	}

	collect := h.collectWorkloadsFn
	if collect == nil {
		collect = collectClusterWorkloads
	}

	live, err := collect(ctx, kubeconfig, stackID)
	if err != nil {
		slog.Warn("failed to read cicd workloads from cluster",
			"stack_id", stackID, "error", err)
		return clusterWorkloads{}, nil
	}
	return live, nil
}

func collectClusterWorkloads(ctx context.Context, kubeconfig []byte, stackID string) (clusterWorkloads, error) {
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return clusterWorkloads{}, fmt.Errorf("parse kubeconfig: %w", err)
	}
	restCfg.Timeout = 10 * time.Second

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return clusterWorkloads{}, fmt.Errorf("create kubernetes client: %w", err)
	}

	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", stackIDLabelKey, stackID),
	}

	// 네임스페이스를 비워 클러스터 전체에서 찾는다. 스택이 파이프라인마다 다른
	// 네임스페이스에 앱을 깔 수 있으므로 한 곳만 보면 놓친다.
	const allNamespaces = ""

	out := clusterWorkloads{}

	deployments, err := clientset.AppsV1().Deployments(allNamespaces).List(ctx, opts)
	if err != nil {
		return clusterWorkloads{}, fmt.Errorf("list deployments: %w", err)
	}
	for _, d := range deployments.Items {
		replicas := int32(0)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		out.Deployments = append(out.Deployments, liveDeployment{
			Name:          d.Name,
			Namespace:     d.Namespace,
			Replicas:      replicas,
			ReadyReplicas: d.Status.ReadyReplicas,
		})
	}

	pods, err := clientset.CoreV1().Pods(allNamespaces).List(ctx, opts)
	if err != nil {
		return clusterWorkloads{}, fmt.Errorf("list pods: %w", err)
	}
	for _, p := range pods.Items {
		out.Pods = append(out.Pods, livePod{
			Name:       p.Name,
			Namespace:  p.Namespace,
			Phase:      podPhaseLabel(p),
			Ready:      podIsReady(p),
			Node:       p.Spec.NodeName,
			TemplateID: p.Labels[templateIDLabelKey],
		})
	}

	services, err := clientset.CoreV1().Services(allNamespaces).List(ctx, opts)
	if err != nil {
		return clusterWorkloads{}, fmt.Errorf("list services: %w", err)
	}
	for _, s := range services.Items {
		port := int32(0)
		if len(s.Spec.Ports) > 0 {
			port = s.Spec.Ports[0].Port
		}
		out.Services = append(out.Services, liveService{
			Name:      s.Name,
			Namespace: s.Namespace,
			Port:      port,
		})
	}

	applyPodUsage(&out, collectPodUsage(ctx, restCfg, opts))

	return out, nil
}

// collectPodUsage 는 metrics-server 에서 파드별 실사용량을 읽는다.
//
// 실패를 에러로 올리지 않는다. metrics-server 는 선택 설치라 없는 클러스터가
// 정상이고, 그때도 워크로드 목록 자체는 나와야 한다 — 사용량 칸만 빈다.
//
// 키는 "네임스페이스/이름" 이다. 이름만으로 맞추면 여러 네임스페이스에 같은
// 이름의 파드가 있을 때 섞인다 (앱 이름이 곧 파드 접두사이므로 흔한 상황이다).
func collectPodUsage(ctx context.Context, restCfg *rest.Config, opts metav1.ListOptions) map[string]podResourceUsage {
	metricsClient, err := metricsclient.NewForConfig(restCfg)
	if err != nil {
		slog.Warn("failed to create metrics client", "error", err)
		return nil
	}

	const allNamespaces = ""
	list, err := metricsClient.MetricsV1beta1().PodMetricses(allNamespaces).List(ctx, opts)
	if err != nil {
		// metrics-server 미설치가 가장 흔한 원인이라 Debug 로 남긴다.
		slog.Debug("pod metrics unavailable", "error", err)
		return nil
	}

	usage := make(map[string]podResourceUsage, len(list.Items))
	for _, item := range list.Items {
		var cpu, memory int64
		// 파드의 사용량은 컨테이너 사용량의 합이다. 사이드카가 있으면
		// 컨테이너 하나만 보면 실제보다 적게 나온다.
		for _, container := range item.Containers {
			cpu += container.Usage.Cpu().MilliValue()
			memory += container.Usage.Memory().Value()
		}
		usage[item.Namespace+"/"+item.Name] = podResourceUsage{
			CPUMillicores: cpu,
			MemoryMiB:     memory / (1024 * 1024),
		}
	}
	return usage
}

// applyPodUsage 는 읽어 온 사용량을 파드에 붙인다.
// 못 읽은 파드는 nil 로 남는다 — 0 과 구분해야 한다.
func applyPodUsage(live *clusterWorkloads, usage map[string]podResourceUsage) {
	if len(usage) == 0 {
		return
	}
	for i := range live.Pods {
		found, ok := usage[live.Pods[i].Namespace+"/"+live.Pods[i].Name]
		if !ok {
			continue
		}
		cpu, memory := found.CPUMillicores, found.MemoryMiB
		live.Pods[i].CPUMillicores = &cpu
		live.Pods[i].MemoryMiB = &memory
	}
}

// podPhaseLabel 은 화면에 쓸 상태 문자열이다. 컨테이너가 대기 중이면 그 이유를
// 쓴다 — Phase 만 보면 CrashLoopBackOff 가 "Running" 으로 보인다.
func podPhaseLabel(p corev1.Pod) string {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil && strings.TrimSpace(cs.State.Waiting.Reason) != "" {
			return cs.State.Waiting.Reason
		}
	}
	return string(p.Status.Phase)
}

func podIsReady(p corev1.Pod) bool {
	for _, cond := range p.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// buildWorkloadObjects 는 조회한 상태를 응답 모양으로 옮긴다.
// 없는 것은 만들지 않는다 — 빈 클러스터는 빈 목록이다.
func buildWorkloadObjects(live clusterWorkloads) ([]workloadK8sObj, podCounts) {
	objects := make([]workloadK8sObj, 0, len(live.Deployments)+len(live.Pods)+len(live.Services))
	counts := podCounts{}

	for _, d := range live.Deployments {
		status := "running"
		if d.ReadyReplicas < d.Replicas {
			status = "progressing"
		}
		objects = append(objects, workloadK8sObj{
			Kind:      "Deployment",
			Name:      d.Name,
			Namespace: d.Namespace,
			Replicas:  d.Replicas,
			Status:    status,
		})
	}

	for _, p := range live.Pods {
		objects = append(objects, workloadK8sObj{
			Kind:          "Pod",
			Name:          p.Name,
			Namespace:     p.Namespace,
			Status:        p.Phase,
			Node:          p.Node,
			TemplateID:    p.TemplateID,
			CPUMillicores: p.CPUMillicores,
			MemoryMiB:     p.MemoryMiB,
		})
		switch {
		case p.Ready && p.Phase == "Running":
			counts.Running++
		case p.Phase == "Pending":
			counts.Pending++
		default:
			counts.Failed++
		}
	}

	for _, s := range live.Services {
		objects = append(objects, workloadK8sObj{
			Kind:      "Service",
			Name:      s.Name,
			Namespace: s.Namespace,
			Port:      s.Port,
			Status:    "active",
		})
	}

	return objects, counts
}

// groupWorkloadsByApp 은 조회한 워크로드를 앱 이름으로 나눈다.
//
// 파이프라인 이름이 곧 앱 이름이고, 매니페스트가 app.kubernetes.io/name 으로 그
// 이름을 붙인다. 파드 이름은 "<앱>-<replicaset>-<suffix>" 라 접두사로 맞춘다 —
// 라벨을 파드까지 읽어 오지만, Deployment/Service 는 이름이 곧 앱 이름이다.
func groupWorkloadsByApp(live clusterWorkloads) map[string]clusterWorkloads {
	out := map[string]clusterWorkloads{}

	for _, d := range live.Deployments {
		w := out[d.Name]
		w.Deployments = append(w.Deployments, d)
		out[d.Name] = w
	}
	for _, s := range live.Services {
		w := out[s.Name]
		w.Services = append(w.Services, s)
		out[s.Name] = w
	}
	for _, p := range live.Pods {
		app := appNameForPod(p.Name, out)
		w := out[app]
		w.Pods = append(w.Pods, p)
		out[app] = w
	}

	return out
}

// appNameForPod 는 파드가 속한 앱을 찾는다. 이미 알려진 앱 중 가장 긴 접두사를
// 고른다 — "demo" 와 "demo-api" 가 함께 있으면 "demo-api-xxxx" 를 앞의 것에
// 붙이면 안 된다.
func appNameForPod(podName string, known map[string]clusterWorkloads) string {
	best := ""
	for app := range known {
		if strings.HasPrefix(podName, app+"-") && len(app) > len(best) {
			best = app
		}
	}
	if best != "" {
		return best
	}
	return podName
}
