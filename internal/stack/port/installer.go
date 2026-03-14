package port

import "context"

// HelmInstallRequest contains parameters for a Helm install operation.
type HelmInstallRequest struct {
	ReleaseName string            `json:"release_name"`
	ChartName   string            `json:"chart_name"`
	Version     string            `json:"version"`
	Namespace   string            `json:"namespace"`
	Values      map[string]string `json:"values"`
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
