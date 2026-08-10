// Package toolhealth derives live OSS health for the platform dashboard from the
// pods the installed stacks are actually running.
//
// 무엇이 설치되었는지는 스택 모듈의 공개 도메인(domain.InstalledToolWorkloads)에
// 물어본다. 목록을 여기서 다시 적으면 스택 상세 화면과 대시보드가 서로 다른 답을
// 하게 된다 — 실제로 Harbor/Nexus 가 그렇게 누락된 적이 있다.
package toolhealth

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	obskube "github.com/cloud-nullus/draft/internal/observability/adapter/kube"
	obsdomain "github.com/cloud-nullus/draft/internal/observability/domain"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

// StackLister is the slice of the stack repository this reader needs.
type StackLister interface {
	List(ctx context.Context, orgID string, includeDeleted bool) ([]*stackdomain.Stack, error)
}

// KubeconfigProvider returns the decrypted kubeconfig bytes for a cluster.
type KubeconfigProvider interface {
	GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error)
}

const (
	statusRunning = "running"
	statusWarning = "warning"
	statusError   = "error"
)

// statusSeverity orders health so that merging many stacks keeps the worst state.
var statusSeverity = map[string]int{statusRunning: 0, statusWarning: 1, statusError: 2}

// unhealthyWaitingReasons 는 "기다리는 중" 이 아니라 "고장" 으로 봐야 하는 사유다.
var unhealthyWaitingReasons = map[string]struct{}{
	"crashloopbackoff":           {},
	"imagepullbackoff":           {},
	"errimagepull":               {},
	"createcontainererror":       {},
	"invalidimagename":           {},
	"createcontainerconfigerror": {},
}

// Reader implements port.ToolHealthRepository against live cluster state.
type Reader struct {
	stacks     StackLister
	kubeconfig KubeconfigProvider
	// listPods 는 테스트에서 갈아끼우기 위해 필드로 둔다.
	listPods func(ctx context.Context, kubeconfig []byte, namespace string) ([]obskube.PodInfo, error)
}

// New constructs a Reader.
func New(stacks StackLister, kubeconfig KubeconfigProvider) *Reader {
	return &Reader{
		stacks:     stacks,
		kubeconfig: kubeconfig,
		listPods:   obskube.ListPodsInNamespace,
	}
}

// ListToolHealth reports one row per distinct OSS across every completed stack.
func (r *Reader) ListToolHealth(ctx context.Context, orgID string) ([]obsdomain.ToolHealth, error) {
	stacks, err := r.stacks.List(ctx, orgID, false)
	if err != nil {
		return nil, err
	}

	merged := map[string]obsdomain.ToolHealth{}
	podCache := map[string][]obskube.PodInfo{}

	for _, stack := range stacks {
		if stack == nil || stack.State != stackdomain.StateCompleted {
			continue
		}

		workloads := stackdomain.InstalledToolWorkloads(stackdomain.DecodeStackConfig(stack.Config))
		if len(workloads) == 0 {
			continue
		}

		pods, ok := r.podsFor(ctx, podCache, stack.ClusterID, stack.Namespace)
		if !ok {
			continue
		}

		for _, w := range workloads {
			health := obsdomain.ToolHealth{
				Name:    w.Name,
				Version: w.Version,
				Status:  healthForWorkload(w, pods),
			}
			if prev, seen := merged[w.Name]; !seen || statusSeverity[health.Status] > statusSeverity[prev.Status] {
				merged[w.Name] = health
			}
		}
	}

	out := make([]obsdomain.ToolHealth, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// podsFor loads a namespace's pods once per cluster+namespace. A cluster we cannot
// reach yields ok=false so its stacks are skipped rather than reported as broken.
func (r *Reader) podsFor(
	ctx context.Context,
	cache map[string][]obskube.PodInfo,
	clusterID, namespace string,
) ([]obskube.PodInfo, bool) {
	if clusterID == "" || namespace == "" {
		return nil, false
	}

	key := clusterID + "/" + namespace
	if cached, ok := cache[key]; ok {
		return cached, cached != nil
	}

	kubeconfig, err := r.kubeconfig.GetKubeconfig(ctx, clusterID)
	if err != nil || len(kubeconfig) == 0 {
		slog.Warn("tool health: kubeconfig unavailable", "cluster_id", clusterID, "error", err)
		cache[key] = nil
		return nil, false
	}

	pods, err := r.listPods(ctx, kubeconfig, namespace)
	if err != nil {
		slog.Warn("tool health: list pods failed",
			"cluster_id", clusterID, "namespace", namespace, "error", err)
		cache[key] = nil
		return nil, false
	}

	// 빈 슬라이스도 "조회는 됐다" 는 뜻이라 nil 과 구분해 캐시한다.
	if pods == nil {
		pods = []obskube.PodInfo{}
	}
	cache[key] = pods
	return pods, true
}

// healthForWorkload collapses a tool's pods into one status.
func healthForWorkload(w stackdomain.ToolWorkload, pods []obskube.PodInfo) string {
	matched := make([]obskube.PodInfo, 0, len(pods))
	for _, pod := range pods {
		if !matchesAnyPrefix(pod.Name, w.NamePrefixes) {
			continue
		}
		// 일회성 Job 은 Succeeded 로 끝나고 Ready 가 아니다. 건강도에 넣으면
		// 정상인 도구가 영구히 warning 으로 보인다.
		if isCompletedOneShotPod(pod) {
			continue
		}
		matched = append(matched, pod)
	}

	if len(matched) == 0 {
		return statusWarning
	}

	ready := 0
	for _, pod := range matched {
		if isUnhealthy(pod) {
			return statusError
		}
		if pod.Ready {
			ready++
		}
	}
	if ready == len(matched) {
		return statusRunning
	}
	return statusWarning
}

func isUnhealthy(pod obskube.PodInfo) bool {
	if strings.EqualFold(strings.TrimSpace(pod.Phase), "Failed") {
		return true
	}
	_, bad := unhealthyWaitingReasons[strings.ToLower(strings.TrimSpace(pod.Status))]
	return bad
}

func isCompletedOneShotPod(pod obskube.PodInfo) bool {
	if !strings.EqualFold(strings.TrimSpace(pod.Phase), "Succeeded") {
		return false
	}
	name := strings.ToLower(pod.Name)
	return strings.Contains(name, "migrations") || strings.Contains(name, "job")
}

func matchesAnyPrefix(name string, prefixes []string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, p := range prefixes {
		if strings.HasPrefix(lower, strings.ToLower(strings.TrimSpace(p))) {
			return true
		}
	}
	return false
}
