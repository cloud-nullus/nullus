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
				if _, err := o.runKubectl(ctx, "rollout", "status", "-n", namespace, resource, "--timeout="+rolloutTimeout); err != nil {
					return err
				}
			}
		}
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
