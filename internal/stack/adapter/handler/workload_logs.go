package handler

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/labstack/echo/v4"
)

// 배포된 애플리케이션의 로그.
//
// 스택 배포 로그(deploy_handler.go)나 CI 스텝 로그와는 다른 것이다. 저것들은
// Nullus 가 만든 기록이고, 이건 앱 컨테이너가 stdout 으로 뱉은 것이다. 지표가
// "얼마나 쓰는가" 를 말한다면 로그는 "무엇을 하다 죽었는가" 를 말한다.
//
// 파드별로 나눠 보여주지 않고 시간순으로 섞는다. 나눠 두면 요청이 어느 파드로
// 갔는지를 사람이 맞춰 봐야 한다 — kubectl logs -l 이 같은 이유로 섞는다.

const (
	// defaultLogTailLines 는 파드 하나에서 가져오는 줄 수다. 크게 잡으면 파드가
	// 여럿일 때 응답이 급격히 커진다.
	defaultLogTailLines = 100
	maxLogTailLines     = 500

	// maxLogPods 는 한 번에 읽는 파드 수 상한이다. 파드마다 요청이 하나씩 나가므로
	// 스택에 앱이 많으면 응답이 느려진다.
	maxLogPods = 12

	// maxLogLines 는 섞은 뒤 남기는 줄 수다.
	maxLogLines = 500
)

type workloadLogLine struct {
	Pod       string `json:"pod"`
	App       string `json:"app"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

type workloadLogsResponse struct {
	Lines []workloadLogLine `json:"lines"`
	// Pods 는 실제로 읽은 파드다. 화면이 "어디서 온 로그인지" 를 말할 수 있어야 한다.
	Pods []string `json:"pods"`
	// Truncated 는 파드가 상한보다 많아 일부만 읽었음을 알린다.
	Truncated bool `json:"truncated"`
}

// GetWorkloadLogs handles GET /api/v1/stacks/:stackId/workloads/logs.
func (h *StackHandler) GetWorkloadLogs(c echo.Context) error {
	stackID := c.Param("stackId")
	if strings.TrimSpace(stackID) == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack_id is required")
	}

	ctx := c.Request().Context()

	stack, err := h.stackRepo.GetByID(ctx, stackID)
	if err != nil || stack == nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
	}

	tailLines := defaultLogTailLines
	if raw := strings.TrimSpace(c.QueryParam("tailLines")); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil && parsed > 0 {
			tailLines = min(parsed, maxLogTailLines)
		}
	}

	kubeconfig := h.kubeconfigFor(ctx, stack.ClusterID)
	if len(kubeconfig) == 0 {
		// kubeconfig 가 없는 개발 환경에서도 화면은 떠야 한다. 빈 결과를 준다.
		return c.JSON(http.StatusOK, workloadLogsResponse{Lines: []workloadLogLine{}, Pods: []string{}})
	}

	live, err := h.workloadsFor(ctx, kubeconfig, stackID)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "WORKLOAD_LOGS_FAILED", err.Error())
	}

	out, err := collectWorkloadLogs(ctx, kubeconfig, live, tailLines)
	if err != nil {
		// 로그를 못 읽는 것과 화면이 뜨지 않는 것은 다른 문제다.
		slog.Warn("failed to read workload logs", "stack_id", stackID, "error", err)
		return c.JSON(http.StatusOK, workloadLogsResponse{Lines: []workloadLogLine{}, Pods: []string{}})
	}

	return c.JSON(http.StatusOK, out)
}

// kubeconfigFor 는 클러스터의 kubeconfig 를 가져온다. 없으면 빈 값이다.
func (h *StackHandler) kubeconfigFor(ctx context.Context, clusterID string) []byte {
	if h.kubeconfigProvider == nil {
		return nil
	}
	kubeconfig, err := h.kubeconfigProvider.GetKubeconfig(ctx, clusterID)
	if err != nil {
		slog.Warn("failed to load kubeconfig", "cluster_id", clusterID, "error", err)
		return nil
	}
	return kubeconfig
}

func collectWorkloadLogs(
	ctx context.Context,
	kubeconfig []byte,
	live clusterWorkloads,
	tailLines int,
) (workloadLogsResponse, error) {
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return workloadLogsResponse{}, fmt.Errorf("parse kubeconfig: %w", err)
	}
	restCfg.Timeout = 15 * time.Second

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return workloadLogsResponse{}, fmt.Errorf("create kubernetes client: %w", err)
	}

	grouped := groupWorkloadsByApp(live)
	appByPod := map[string]string{}
	for app, workloads := range grouped {
		for _, pod := range workloads.Pods {
			appByPod[pod.Name] = app
		}
	}

	// 파드 순서를 이름으로 고정한다. 맵 순회 순서에 기대면 상한에 걸릴 때마다
	// 다른 파드가 잘려 화면이 깜빡인다.
	pods := append([]livePod(nil), live.Pods...)
	sort.Slice(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })

	truncated := len(pods) > maxLogPods
	if truncated {
		pods = pods[:maxLogPods]
	}

	byPod := make(map[string][]string, len(pods))
	names := make([]string, 0, len(pods))
	tail := int64(tailLines)

	for _, pod := range pods {
		names = append(names, pod.Name)

		stream, err := clientset.CoreV1().
			Pods(pod.Namespace).
			GetLogs(pod.Name, &corev1.PodLogOptions{TailLines: &tail, Timestamps: true}).
			Stream(ctx)
		if err != nil {
			// 방금 뜬 파드나 ImagePullBackOff 파드는 로그가 없다. 그 파드만 건너뛴다.
			slog.Debug("pod logs unavailable", "pod", pod.Name, "error", err)
			continue
		}

		lines := make([]string, 0, tailLines)
		scanner := bufio.NewScanner(stream)
		// 로그 한 줄이 기본 버퍼(64KB)를 넘는 경우가 있다 — 스택트레이스나 JSON 덤프.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		_ = stream.Close()

		byPod[pod.Name] = lines
	}

	return workloadLogsResponse{
		Lines:     mergeLogLines(byPod, appByPod, maxLogLines),
		Pods:      names,
		Truncated: truncated,
	}, nil
}

// mergeLogLines 는 파드별 줄들을 시간순 한 줄기로 섞는다.
//
// 쿠버네티스는 Timestamps: true 일 때 "RFC3339Nano 공백 본문" 으로 준다.
// 타임스탬프가 없는 줄(스택트레이스의 이어지는 줄)은 앞줄의 시각을 물려받는다 —
// 버리면 정작 필요한 부분이 사라지고, 0 값을 주면 맨 앞으로 튄다.
func mergeLogLines(byPod map[string][]string, appByPod map[string]string, limit int) []workloadLogLine {
	type stamped struct {
		at   time.Time
		seq  int
		line workloadLogLine
	}

	all := make([]stamped, 0, limit)
	seq := 0

	// 파드 이름 순으로 훑어야 같은 시각의 줄 순서가 매번 같다.
	podNames := make([]string, 0, len(byPod))
	for pod := range byPod {
		podNames = append(podNames, pod)
	}
	sort.Strings(podNames)

	for _, pod := range podNames {
		var last time.Time
		var lastRaw string
		for _, raw := range byPod[pod] {
			at, message, ok := splitLogTimestamp(raw)
			if ok {
				last, lastRaw = at, at.Format(time.RFC3339Nano)
			} else {
				message = raw
			}
			if strings.TrimSpace(message) == "" {
				continue
			}
			all = append(all, stamped{
				at:  last,
				seq: seq,
				line: workloadLogLine{
					Pod:       pod,
					App:       appByPod[pod],
					Timestamp: lastRaw,
					Message:   message,
				},
			})
			seq++
		}
	}

	// 같은 시각이면 읽은 순서를 지킨다 — 한 파드 안의 줄 순서가 뒤집히면 안 된다.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].at.Equal(all[j].at) {
			return all[i].seq < all[j].seq
		}
		return all[i].at.Before(all[j].at)
	})

	if len(all) > limit {
		all = all[len(all)-limit:]
	}

	out := make([]workloadLogLine, 0, len(all))
	for _, item := range all {
		out = append(out, item.line)
	}
	return out
}

// splitLogTimestamp 는 쿠버네티스가 붙인 앞머리 시각을 떼어 낸다.
func splitTimestampCandidate(raw string) (string, string, bool) {
	space := strings.IndexByte(raw, ' ')
	if space <= 0 {
		return "", raw, false
	}
	return raw[:space], raw[space+1:], true
}

func splitLogTimestamp(raw string) (time.Time, string, bool) {
	candidate, rest, ok := splitTimestampCandidate(raw)
	if !ok {
		return time.Time{}, raw, false
	}
	at, err := time.Parse(time.RFC3339Nano, candidate)
	if err != nil {
		return time.Time{}, raw, false
	}
	return at, rest, true
}
