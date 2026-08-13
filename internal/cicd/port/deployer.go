package port

import "context"

type ManifestApplier interface {
	Apply(ctx context.Context, kubeconfig []byte, manifests []string) error
	ApplyWithTracking(ctx context.Context, kubeconfig []byte, manifests []string, deploymentID string, stepOffset ...int) error
}

// WorkloadDeleter 는 라벨로 찾은 워크로드를 지운다.
//
// Argo CD 로 배포한 앱은 Application 을 지우면 컨트롤러가 함께 걷어내지만,
// 매니페스트를 직접 적용하는 경로는 지워 줄 주체가 없다. 그쪽 워크로드는
// 이것으로 정리한다.
type WorkloadDeleter interface {
	DeleteByLabel(ctx context.Context, kubeconfig []byte, namespace, selector string) error
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
