package port

import "context"

type ManifestApplier interface {
	Apply(ctx context.Context, kubeconfig []byte, manifests []string) error
}

type KubeconfigProvider interface {
	GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error)
}
