package helm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

type Orchestrator struct {
	installer   port.HelmInstaller
	rollback    *RollbackManager
	kubeconfig  []byte
	namespace   string
	chartConfig map[string]ChartSpec
	stepOrder   map[string]int
	orderedStep []string
	mu          sync.Mutex
	progress    map[string]int
}

type ChartSpec struct {
	ChartName string
	RepoURL   string
	Version   string
	Namespace string
	Values    map[string]any
}

func NewOrchestrator(installer port.HelmInstaller, kubeconfig []byte, namespace string) *Orchestrator {
	return &Orchestrator{
		installer:  installer,
		rollback:   &RollbackManager{},
		kubeconfig: kubeconfig,
		namespace:  namespace,
		chartConfig: map[string]ChartSpec{
			"installing_cert_manager": {
				ChartName: "cert-manager",
				RepoURL:   "https://charts.jetstack.io",
				Version:   "v1.16.3",
				Values:    DefaultValues("installing_cert_manager"),
			},
			"installing_minio": {
				ChartName: "minio",
				RepoURL:   "https://charts.min.io/",
				Version:   "5.4.0",
				Values:    DefaultValues("installing_minio"),
			},
			"installing_gitlab": {
				ChartName: "gitlab",
				RepoURL:   "https://charts.gitlab.io/",
				Version:   "8.7.2",
				Values:    DefaultValues("installing_gitlab"),
			},
			"installing_argocd": {
				ChartName: "argo-cd",
				RepoURL:   "https://argoproj.github.io/argo-helm",
				Version:   "7.7.16",
				Values:    DefaultValues("installing_argocd"),
			},
			"installing_runner": {
				ChartName: "gitlab-runner",
				RepoURL:   "https://charts.gitlab.io/",
				Version:   "0.72.0",
				Values:    DefaultValues("installing_runner"),
			},
			"installing_prometheus": {
				ChartName: "kube-prometheus-stack",
				RepoURL:   "https://prometheus-community.github.io/helm-charts",
				Version:   "69.3.0",
				Values:    DefaultValues("installing_prometheus"),
			},
			"installing_grafana": {
				ChartName: "grafana",
				RepoURL:   "https://grafana.github.io/helm-charts",
				Version:   "8.9.0",
				Values:    DefaultValues("installing_grafana"),
			},
			"integration_check": {},
		},
		stepOrder: map[string]int{
			"installing_cert_manager": 0,
			"installing_minio":        1,
			"installing_gitlab":       2,
			"installing_argocd":       3,
			"installing_runner":       4,
			"installing_prometheus":   5,
			"installing_grafana":      6,
			"integration_check":       7,
		},
		orderedStep: []string{
			"installing_cert_manager",
			"installing_minio",
			"installing_gitlab",
			"installing_argocd",
			"installing_runner",
			"installing_prometheus",
			"installing_grafana",
			"integration_check",
		},
		progress: make(map[string]int),
	}
}

func (o *Orchestrator) ExecuteStep(ctx context.Context, stackID, step, phase string) error {
	_ = stackID
	_ = phase

	spec, ok := o.chartConfig[step]
	if !ok {
		return fmt.Errorf("unknown step %q", step)
	}

	order, ok := o.stepOrder[step]
	if !ok {
		return fmt.Errorf("step order not defined for %q", step)
	}
	if err := o.ensureOrder(stackID, step, order); err != nil {
		return err
	}

	if step == "integration_check" {
		o.markCompleted(stackID, order)
		return nil
	}

	namespace := o.namespace
	if spec.Namespace != "" {
		namespace = spec.Namespace
	}

	result, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: spec.ChartName,
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   namespace,
		Values:      spec.Values,
	})
	if err != nil {
		return fmt.Errorf("install step %s: %w", step, err)
	}
	if result != nil {
		o.rollback.Push(result.ReleaseName)
	}
	o.markCompleted(stackID, order)
	return nil
}

func (o *Orchestrator) ensureOrder(stackID, step string, order int) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if stackID == "" {
		return nil
	}
	current := o.progress[stackID]
	if _, ok := o.progress[stackID]; !ok {
		current = -1
	}
	if order != current+1 {
		expectedIdx := current + 1
		expected := ""
		if expectedIdx >= 0 && expectedIdx < len(o.orderedStep) {
			expected = o.orderedStep[expectedIdx]
		}
		return fmt.Errorf("out of order step %q for stack %s: expected %q", step, stackID, expected)
	}
	return nil
}

func (o *Orchestrator) markCompleted(stackID string, order int) {
	if stackID == "" {
		return
	}
	o.mu.Lock()
	o.progress[stackID] = order
	o.mu.Unlock()
}
