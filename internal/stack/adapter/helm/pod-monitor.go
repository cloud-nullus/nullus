package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type podListSnapshot struct {
	Items []podSnapshotItem `json:"items"`
}

type podSnapshotItem struct {
	Metadata podSnapshotMetadata `json:"metadata"`
	Status   podSnapshotStatus   `json:"status"`
}

type podSnapshotMetadata struct {
	Name string `json:"name"`
}

type podSnapshotStatus struct {
	Phase             string               `json:"phase"`
	PodIP             string               `json:"podIP"`
	ContainerStatuses []podContainerStatus `json:"containerStatuses"`
}

type podContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
}

func (o *Orchestrator) StartStepRuntimeTail(ctx context.Context, stackID, step string, emit func(level, message string)) (stop func()) {
	_ = stackID
	if emit == nil {
		return nil
	}
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	spec, ok := o.chartSpecForStep(step)
	if !ok {
		return nil
	}
	spec = o.resolveChartSpecForStep(step, spec)
	if strings.TrimSpace(spec.ChartName) == "" {
		return nil
	}

	namespace := o.namespace
	if strings.TrimSpace(spec.Namespace) != "" {
		namespace = spec.Namespace
	}
	releaseName := o.releaseNameForSpec(spec)
	if strings.TrimSpace(releaseName) == "" {
		return nil
	}

	tailCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		seen := make(map[string]struct{})
		emitTail := func() {
			output, err := o.runKubectl(tailCtx,
				"logs",
				"-n", namespace,
				"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName),
				"--all-containers=true",
				"--tail=40",
				"--prefix=true",
			)
			if err != nil {
				return
			}
			for _, line := range strings.Split(string(output), "\n") {
				msg := strings.TrimSpace(line)
				if msg == "" {
					continue
				}
				if _, ok := seen[msg]; ok {
					continue
				}
				if len(seen) > 4000 {
					seen = map[string]struct{}{}
				}
				seen[msg] = struct{}{}
				emit("info", fmt.Sprintf("container stdout: %s", msg))
			}
		}

		emitTail()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-tailCtx.Done():
				return
			case <-ticker.C:
				emitTail()
			}
		}
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (o *Orchestrator) StepRuntimeLogs(ctx context.Context, stackID, step string) (infos []string, warns []string) {
	_ = stackID

	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil, nil
	}

	spec, ok := o.chartSpecForStep(step)
	if !ok {
		return nil, nil
	}
	spec = o.resolveChartSpecForStep(step, spec)
	if strings.TrimSpace(spec.ChartName) == "" {
		return nil, nil
	}

	namespace := o.namespace
	if strings.TrimSpace(spec.Namespace) != "" {
		namespace = spec.Namespace
	}
	releaseName := o.releaseNameForSpec(spec)
	if strings.TrimSpace(releaseName) == "" {
		return nil, nil
	}

	snapshot, err := o.releasePodSnapshot(ctx, releaseName, namespace)
	if err != nil {
		return nil, []string{fmt.Sprintf("pod snapshot unavailable for release %s: %v", releaseName, err)}
	}

	if len(snapshot.Items) == 0 {
		return []string{fmt.Sprintf("pod snapshot: no pods found yet for release %s in namespace %s", releaseName, namespace)}, nil
	}

	const maxPodLines = 12
	infos = append(infos, fmt.Sprintf("pod snapshot for release %s in namespace %s (%d pods)", releaseName, namespace, len(snapshot.Items)))
	for idx, pod := range snapshot.Items {
		if idx >= maxPodLines {
			infos = append(infos, fmt.Sprintf("... %d additional pods omitted", len(snapshot.Items)-maxPodLines))
			break
		}
		readyCount := 0
		restartCount := 0
		for _, container := range pod.Status.ContainerStatuses {
			if container.Ready {
				readyCount++
			}
			restartCount += container.RestartCount
		}
		infos = append(infos, fmt.Sprintf(
			"pod=%s phase=%s ready=%d/%d restarts=%d ip=%s",
			pod.Metadata.Name,
			strings.TrimSpace(pod.Status.Phase),
			readyCount,
			len(pod.Status.ContainerStatuses),
			restartCount,
			strings.TrimSpace(pod.Status.PodIP),
		))
	}

	return infos, nil
}

func (o *Orchestrator) releasePodSnapshot(ctx context.Context, releaseName, namespace string) (*podListSnapshot, error) {
	selectors := releaseLabelSelectors(releaseName)
	for _, selector := range selectors {
		output, err := o.runKubectl(ctx,
			"get", "pods",
			"-n", namespace,
			"-l", selector,
			"-o", "json",
		)
		if err != nil {
			return nil, err
		}

		var snapshot podListSnapshot
		if err := json.Unmarshal(output, &snapshot); err != nil {
			return nil, err
		}
		if len(snapshot.Items) > 0 {
			return &snapshot, nil
		}
	}

	return &podListSnapshot{}, nil
}

func (o *Orchestrator) waitForReleaseRollouts(ctx context.Context, releaseName, namespace string) error {
	resources := []string{"deployments", "statefulsets", "daemonsets"}
	selectors := releaseLabelSelectors(releaseName)
	rolloutTimeout := "180s"
	if strings.TrimSpace(releaseName) == "gitlab" {
		rolloutTimeout = "600s"
	}
	for _, resourceType := range resources {
		for _, selector := range selectors {
			output, err := o.runKubectl(ctx,
				"get", resourceType,
				"-n", namespace,
				"-l", selector,
				"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`,
			)
			if err != nil {
				return err
			}
			for _, rawName := range strings.Split(string(output), "\n") {
				name := strings.TrimSpace(rawName)
				if name == "" {
					continue
				}
				resource := strings.TrimSuffix(resourceType, "s") + "/" + name

				// OnDelete 전략 워크로드는 rollout status 로 기다릴 수 없다.
				// (OpenBao 차트의 StatefulSet 이 여기 해당한다.)
				// 준비 여부는 readyReplicas 로 직접 확인한다.
				if strategy, err := o.workloadUpdateStrategy(ctx, namespace, resource); err == nil &&
					isRolloutStatusUnsupportedStrategy(strategy) {
					if err := o.waitForWorkloadReadyReplicas(ctx, namespace, resource, rolloutTimeout); err != nil {
						return err
					}
					continue
				}

				if _, err := o.runKubectl(ctx, "rollout", "status", "-n", namespace, resource, "--timeout="+rolloutTimeout); err != nil {
					// 전략 조회가 실패했거나 그 사이 값이 바뀐 경우의 방어선.
					if isRolloutStatusUnsupportedError(err) {
						if readyErr := o.waitForWorkloadReadyReplicas(ctx, namespace, resource, rolloutTimeout); readyErr != nil {
							return readyErr
						}
						continue
					}
					return err
				}
			}
		}
	}
	return nil
}

// isRolloutStatusUnsupportedStrategy 는 kubectl rollout status 로 대기할 수
// 없는 업데이트 전략인지 판단한다. 빈 값은 각 워크로드의 기본값
// (RollingUpdate)이므로 지원 대상으로 본다.
func isRolloutStatusUnsupportedStrategy(strategy string) bool {
	return strings.EqualFold(strings.TrimSpace(strategy), "OnDelete")
}

// isRolloutStatusUnsupportedError 는 kubectl 이 돌려준 에러가 전략 미지원인지 본다.
func isRolloutStatusUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "only available for rollingupdate strategy type")
}

// workloadUpdateStrategy 는 워크로드의 업데이트 전략 타입을 읽는다.
// Deployment 는 .spec.strategy.type, StatefulSet/DaemonSet 은
// .spec.updateStrategy.type 을 쓰므로 둘 다 조회해 비어 있지 않은 값을 쓴다.
func (o *Orchestrator) workloadUpdateStrategy(ctx context.Context, namespace, resource string) (string, error) {
	output, err := o.runKubectl(ctx,
		"get", resource,
		"-n", namespace,
		"-o", `jsonpath={.spec.updateStrategy.type}{.spec.strategy.type}`,
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// waitForWorkloadReadyReplicas 는 rollout status 를 쓸 수 없는 워크로드를 위해
// readyReplicas 가 desired 에 도달할 때까지 기다린다.
func (o *Orchestrator) waitForWorkloadReadyReplicas(ctx context.Context, namespace, resource, timeout string) error {
	// kubectl wait 는 --for=jsonpath 로 동일한 대기를 서버 사이드에서 수행한다.
	// desired replica 수를 먼저 읽어 그 값에 도달할 때까지 기다린다.
	desired, err := o.runKubectl(ctx, "get", resource, "-n", namespace, "-o", `jsonpath={.spec.replicas}`)
	if err != nil {
		return err
	}
	want := strings.TrimSpace(string(desired))
	if want == "" || want == "0" {
		return nil
	}

	if _, err := o.runKubectl(ctx,
		"wait", resource,
		"-n", namespace,
		"--for=jsonpath={.status.readyReplicas}="+want,
		"--timeout="+timeout,
	); err != nil {
		return fmt.Errorf("wait for %s ready replicas (%s): %w", resource, want, err)
	}
	return nil
}

func releaseLabelSelectors(releaseName string) []string {
	name := strings.TrimSpace(releaseName)
	if name == "" {
		return []string{""}
	}
	return []string{
		fmt.Sprintf("app.kubernetes.io/instance=%s", name),
		fmt.Sprintf("release=%s", name),
	}
}
