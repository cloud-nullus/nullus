package port

import "context"

type ManifestApplier interface {
	Apply(ctx context.Context, kubeconfig []byte, manifests []string) error
	ApplyWithTracking(ctx context.Context, kubeconfig []byte, manifests []string, deploymentID string, stepOffset ...int) error
}

// ArgoApplicationDeleter 는 Argo CD Application 과 그것이 배포한 리소스를 지운다.
//
// Application 만 지우면 Deployment·Service·HTTPRoute 가 클러스터에 남아 앱이
// 계속 돈다. 구현체는 정리 finalizer 를 붙여 Argo CD 가 함께 걷어내게 해야 한다.
type ArgoApplicationDeleter interface {
	// Delete 는 Application 이 이미 없으면 성공으로 본다.
	Delete(ctx context.Context, kubeconfig []byte, namespace, name string) error
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
