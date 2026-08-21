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

// CDApplicationDeleter 는 CD 도구가 배포한 애플리케이션을 지운다.
//
// 도구 이름을 계약에 넣지 않는다. Argo CD 든 다른 것이든 "배포된 애플리케이션을
// 지운다" 는 같은 일이고, 도구가 바뀌면 구현체만 갈아 끼우면 된다.
//
// 애플리케이션만 지우면 Deployment·Service·HTTPRoute 가 클러스터에 남아 앱이
// 계속 돈다. 구현체는 그 도구의 방식대로 배포된 리소스까지 함께 걷어내야 한다
// (Argo CD 는 정리 finalizer 를 붙인다).
type CDApplicationDeleter interface {
	// DeleteApplication 은 애플리케이션이 이미 없으면 성공으로 본다 —
	// 삭제의 목표는 "없는 상태" 이고, 없음을 오류로 올리면 재시도가 끝나지 않는다.
	DeleteApplication(ctx context.Context, kubeconfig []byte, namespace, name string) error
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

// BuildDelegate 는 이미지 빌드를 스택의 CI 러너에 넘긴다.
//
// ImagePreparer 와 짝을 이루는 반대편이다. ImagePreparer 는 플랫폼이 직접
// 빌드하는 장애 대응 경로(emergency_direct)이고, 이쪽은 스택 컴포넌트가
// 실행하는 일반 경로(stack_integrated)다 — CI 가 빌드해 레지스트리에 올리고
// CD 도구가 클러스터에 반영한다. 플랫폼은 실행을 시작시키고 결과를 들여올 뿐
// git clone·docker build·kubectl apply 를 하지 않는다.
type BuildDelegate interface {
	// DelegateBuild 는 러너의 실행을 시작시키고 그 실행의 주소를 돌려준다.
	DelegateBuild(ctx context.Context, opts DelegateBuildOpts) (runURL string, err error)
}

// DelegateBuildOpts 는 어느 스택의 어느 job 을 실행할지다.
type DelegateBuildOpts struct {
	StackID string
	// JobName 은 CI 서버의 job 이름이다. 프로비저닝이 앱 이름으로 만든다.
	JobName string
	// Branch 는 실행할 브랜치다. multibranch job 은 브랜치가 하위 job 이라
	// 이것이 없으면 무엇을 실행할지 정해지지 않는다.
	Branch string
	// DeploymentID 는 진행 상황을 기록할 배포 기록이다.
	DeploymentID string
	// StepIndex 는 이 작업이 기록될 단계 번호다.
	StepIndex int
}

type ClusterTarget struct {
	Kubeconfig  []byte
	ClusterName string
}

type ClusterTargetProvider interface {
	GetTarget(ctx context.Context, clusterID string) (*ClusterTarget, error)
}
