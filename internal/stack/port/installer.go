package port

import "context"

// HelmInstallRequest contains parameters for a Helm install operation.
type HelmInstallRequest struct {
	ReleaseName string         `json:"release_name"`
	ChartName   string         `json:"chart_name"`
	RepoURL     string         `json:"repo_url"`
	Version     string         `json:"version"`
	Namespace   string         `json:"namespace"`
	Values      map[string]any `json:"values"`
}

// HelmInstallResult contains the result of a Helm install operation.
type HelmInstallResult struct {
	ReleaseName string `json:"release_name"`
	Namespace   string `json:"namespace"`
	Status      string `json:"status"`
	Revision    int    `json:"revision"`
}

// HelmInstaller defines the interface for Helm chart operations.
type HelmInstaller interface {
	Install(ctx context.Context, req HelmInstallRequest) (*HelmInstallResult, error)
	Uninstall(ctx context.Context, releaseName, namespace string) error
	Status(ctx context.Context, releaseName, namespace string) (*HelmInstallResult, error)
}

// StepExecutor maps install step names to real infrastructure operations.
// Nil-safe: the usecase falls back to simulation when executor is nil.
type StepExecutor interface {
	ExecuteStep(ctx context.Context, stackID, step, phase string) error
}
