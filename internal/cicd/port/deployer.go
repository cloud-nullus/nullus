package port

import "context"

type ManifestApplier interface {
	Apply(ctx context.Context, kubeconfig []byte, manifests []string) error
	ApplyWithTracking(ctx context.Context, kubeconfig []byte, manifests []string, deploymentID string, stepOffset ...int) error
}

type KubeconfigProvider interface {
	GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error)
}

type ImagePreparer interface {
	PrepareImage(ctx context.Context, opts PrepareImageOpts) (imageRef string, err error)
}

type PrepareImageOpts struct {
	GitRepoURL       string
	DockerfilePath   string
	DockerContext    string
	ImageName        string
	ClusterName      string
	DeploymentID     string
	RegistryURL      string
	RegistryUsername string
	RegistryPassword string
}

type ClusterTarget struct {
	Kubeconfig  []byte
	ClusterName string
}

type ClusterTargetProvider interface {
	GetTarget(ctx context.Context, clusterID string) (*ClusterTarget, error)
}
