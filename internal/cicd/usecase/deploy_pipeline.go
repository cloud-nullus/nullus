package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/manifests"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	envManifestDeployment = "NULLUS_MANIFEST_DEPLOYMENT"
	envManifestService    = "NULLUS_MANIFEST_SERVICE"
	envManifestIngress    = "NULLUS_MANIFEST_INGRESS"

	envRegistryURL      = "IMAGE_REGISTRY_URL"
	envRegistryUsername = "REGISTRY_USERNAME"
	envRegistryPassword = "REGISTRY_PASSWORD"
)

type DeployPipelineInput struct {
	PipelineID    string
	Version       string
	DeployedBy    string
	ManifestTypes []string
}

type DeployPipelineOutput struct {
	Deployment *domain.Deployment
}

type DeployPipeline struct {
	pipelineRepo          port.PipelineRepository
	deploymentRepo        port.DeploymentRepository
	kubeconfigProvider    port.KubeconfigProvider
	applier               port.ManifestApplier
	imagePreparer         port.ImagePreparer
	clusterTargetProvider port.ClusterTargetProvider
	// buildDelegate 는 빌드를 스택의 CI 러너에 넘긴다. 통합모드 파이프라인의
	// 배포 실행이 이 자리로 간다.
	buildDelegate port.BuildDelegate
	// deployTimeout 은 배포 한 번의 상한이다.
	deployTimeout time.Duration
	// stackReader 는 배포 대상 스택의 요약을 읽는다. 배포되는 앱에 넣어 줄
	// 수집기 주소가 여기서 온다 — 없으면 추적 환경변수를 넣지 않는다.
	stackReader port.StackReader
}

func NewDeployPipeline(
	pipelineRepo port.PipelineRepository,
	deploymentRepo port.DeploymentRepository,
	kubeconfigProvider port.KubeconfigProvider,
	applier port.ManifestApplier,
	opts ...DeployOption,
) *DeployPipeline {
	dp := &DeployPipeline{
		pipelineRepo:       pipelineRepo,
		deploymentRepo:     deploymentRepo,
		kubeconfigProvider: kubeconfigProvider,
		applier:            applier,
		deployTimeout:      defaultDeployTimeout,
	}
	for _, opt := range opts {
		opt(dp)
	}
	return dp
}

type DeployOption func(*DeployPipeline)

func WithImagePreparer(p port.ImagePreparer) DeployOption {
	return func(dp *DeployPipeline) { dp.imagePreparer = p }
}

func WithClusterTargetProvider(p port.ClusterTargetProvider) DeployOption {
	return func(dp *DeployPipeline) { dp.clusterTargetProvider = p }
}

// defaultDeployTimeout 은 배포 한 번이 걸릴 수 있는 최대 시간이다.
//
// 직접 빌드 경로(docker build + push)를 넉넉히 담되, 매달린 호출이 영원히 남지는
// 않게 한다.
const defaultDeployTimeout = 15 * time.Minute

// WithDeployTimeout 은 배포 한 번의 상한을 바꾼다. 0 이하면 기본값을 쓴다.
func WithDeployTimeout(d time.Duration) DeployOption {
	return func(dp *DeployPipeline) {
		if d > 0 {
			dp.deployTimeout = d
		}
	}
}

// WithBuildDelegate 는 러너 위임 경로를 배선한다.
//
// 배선되지 않으면 위임 대상 파이프라인의 배포는 오류로 끝난다. 조용히 직접
// 빌드로 되돌리지 않는다 — API 파드에는 도커 데몬이 없어 그 경로는 실패할 수
// 없는 경로이고, 되돌리면 사용자는 왜 실패했는지 알 수 없다.
func WithBuildDelegate(d port.BuildDelegate) DeployOption {
	return func(dp *DeployPipeline) { dp.buildDelegate = d }
}

// WithStackReader 는 스택 요약 조회를 배선한다. 배선되지 않으면 배포는 그대로
// 되고 추적 환경변수만 빠진다 — 관측 배선 때문에 배포가 막히면 안 된다.
func WithStackReader(r port.StackReader) DeployOption {
	return func(dp *DeployPipeline) { dp.stackReader = r }
}

func includesOptionalManifest(manifestTypes []string, target string) bool {
	if len(manifestTypes) == 0 {
		return true
	}
	for _, manifestType := range manifestTypes {
		if strings.EqualFold(strings.TrimSpace(manifestType), target) {
			return true
		}
	}
	return false
}

// buildStepCount 는 BuildStepPlan 이 매니페스트 단계 앞에 붙이는 빌드 단계 수다.
//
// applyToCluster 가 진행 상황을 **인덱스로** 기록하므로, 이 값과 어긋나면 매니페스트
// 적용 결과가 빌드 단계 이름 위에 찍힌다. 실제로 그런 일이 있었다 — 계획은
// DockerfilePath 만 보고 3개를 붙이는데 applyToCluster 는 imagePreparer 와
// clusterTargetProvider 까지 확인한 뒤에야 오프셋을 3으로 올려서, 이미지 준비기가
// 없는 경로에서 "Git Clone" 단계에 "namespace/... configured" 가 기록됐다.
// 두 곳이 같은 함수를 쓰게 해서 다시 갈라지지 않게 한다.
func buildStepCount(pipeline *domain.Pipeline) int {
	if pipeline.DelegatesBuildToRunner() {
		// 위임 경로에서 플랫폼이 하는 일은 실행을 넘기는 것 하나뿐이다.
		// 그 뒤의 매니페스트 단계는 없다 — 클러스터 반영은 CD 도구가 한다.
		return 1
	}
	if pipeline != nil && pipeline.DockerfilePath != "" {
		return 3
	}
	return 0
}

func BuildStepPlan(pipeline *domain.Pipeline, manifestTypes ...[]string) []string {
	var selected []string
	if len(manifestTypes) > 0 {
		selected = manifestTypes[0]
	}
	// 위임 경로는 플랫폼이 매니페스트를 적용하지 않는다. 러너가 이미지를 올리고
	// 매니페스트 태그를 되커밋하면 Argo CD 가 그 커밋을 동기화한다 — 플랫폼이
	// 같은 리소스를 따로 적용하면 둘이 서로를 덮어쓴다.
	if pipeline.DelegatesBuildToRunner() {
		return []string{"Trigger CI"}
	}

	steps := []string{"Create Namespace", "Create Deployment"}
	if buildStepCount(pipeline) > 0 {
		hasRegistry := pipeline.EnvVars[envRegistryURL] != ""
		imageStep := "Image Load"
		if hasRegistry {
			imageStep = "Image Push"
		}
		steps = append([]string{"Git Clone", "Docker Build", imageStep}, steps...)
	}
	if includesOptionalManifest(selected, "service") {
		steps = append(steps, "Create Service")
	}
	if includesOptionalManifest(selected, "ingress") {
		steps = append(steps, "Create Ingress")
	}
	return steps
}

// Start creates a Deployment record with status=running and returns immediately.
func (uc *DeployPipeline) Start(ctx context.Context, input DeployPipelineInput) (*DeployPipelineOutput, error) {
	if input.PipelineID == "" {
		return nil, fmt.Errorf("pipeline_id is required")
	}
	if input.Version == "" {
		return nil, fmt.Errorf("version is required")
	}

	if _, err := uc.pipelineRepo.GetByID(ctx, input.PipelineID); err != nil {
		return nil, fmt.Errorf("pipeline not found: %w", err)
	}

	deployment := &domain.Deployment{
		ID:         generateID("dep"),
		PipelineID: input.PipelineID,
		Version:    input.Version,
		Status:     domain.DeploymentStatusRunning,
		StartedAt:  time.Now(),
		DeployedBy: input.DeployedBy,
	}

	if err := uc.deploymentRepo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}

	return &DeployPipelineOutput{Deployment: deployment}, nil
}

// ApplyAsync runs the actual K8s deployment in the background.
func (uc *DeployPipeline) ApplyAsync(deploymentID string, manifestTypes ...[]string) {
	// 배포에는 데드라인이 있어야 한다.
	//
	// 예전에는 context.Background() 를 그대로 썼다. 러너 위임 경로는 스택 번들을
	// 조립하며 OpenBao 호출과 Gitea 파드 exec 까지 하는데, 스택이 온전치 않으면
	// 그 호출이 돌아오지 않는다. 그러면 배포 기록이 영원히 running 으로 남고
	// 화면은 "실행 중" 만 보여 준다 — 2026-08-21 운영에서 그랬다.
	timeout := uc.deployTimeout
	if timeout <= 0 {
		timeout = defaultDeployTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 상태 기록은 데드라인 밖에서 한다. 시간을 넘겨 실패한 배포를 "실패" 로
	// 남기려는데 그 쓰기까지 같은 만료된 컨텍스트를 타면 아무것도 남지 않는다.
	recordCtx := context.WithoutCancel(ctx)

	deployment, err := uc.deploymentRepo.GetByID(ctx, deploymentID)
	if err != nil {
		slog.Error("apply: deployment not found", "id", deploymentID, "error", err)
		return
	}

	pipeline, err := uc.pipelineRepo.GetByID(ctx, deployment.PipelineID)
	if err != nil {
		slog.Error("apply: pipeline not found", "id", deployment.PipelineID, "error", err)
		uc.failDeployment(recordCtx, deployment, err)
		return
	}

	var selected []string
	if len(manifestTypes) > 0 {
		selected = manifestTypes[0]
	}
	if deployErr := uc.applyToCluster(ctx, pipeline, deploymentID, selected); deployErr != nil {
		slog.Error("apply: cluster deploy failed", "deployment", deploymentID, "error", deployErr)
		uc.failDeployment(recordCtx, deployment, deployErr)
		return
	}

	completed := time.Now()
	deployment.CompletedAt = &completed
	deployment.Status = domain.DeploymentStatusSuccess
	_ = uc.deploymentRepo.Update(recordCtx, deployment)
	slog.Info("apply: deployment succeeded", "deployment", deploymentID)
}

// Execute runs the full synchronous flow (for tests and backward compat).
func (uc *DeployPipeline) Execute(ctx context.Context, input DeployPipelineInput) (*DeployPipelineOutput, error) {
	out, err := uc.Start(ctx, input)
	if err != nil {
		return nil, err
	}

	uc.ApplyAsync(out.Deployment.ID, input.ManifestTypes)

	updated, err := uc.deploymentRepo.GetByID(ctx, out.Deployment.ID)
	if err != nil {
		return nil, err
	}
	if updated.Status == domain.DeploymentStatusFailed {
		return nil, fmt.Errorf("deployment failed")
	}

	return &DeployPipelineOutput{Deployment: updated}, nil
}

func (uc *DeployPipeline) failDeployment(ctx context.Context, deployment *domain.Deployment, reason error) {
	_ = reason
	completed := time.Now()
	deployment.CompletedAt = &completed
	deployment.Status = domain.DeploymentStatusFailed
	_ = uc.deploymentRepo.Update(ctx, deployment)
}

func (uc *DeployPipeline) applyToCluster(ctx context.Context, pipeline *domain.Pipeline, deploymentID string, manifestTypes []string) error {
	namespace := pipeline.Namespace
	if namespace == "" {
		namespace = "default"
	}

	if pipeline.DelegatesBuildToRunner() {
		return uc.delegateToRunner(ctx, pipeline, deploymentID)
	}

	var imageRef string
	// BuildStepPlan 과 같은 근거로 계산한다. 이미지 준비기 유무와 무관하게
	// 계획에 빌드 단계가 있으면 매니페스트 단계는 그만큼 뒤에서 시작한다.
	stepOffset := buildStepCount(pipeline)

	if pipeline.DockerfilePath != "" && uc.imagePreparer != nil && uc.clusterTargetProvider != nil {
		target, err := uc.clusterTargetProvider.GetTarget(ctx, pipeline.ClusterID)
		if err != nil {
			return fmt.Errorf("get cluster target: %w", err)
		}

		suffixStart := max(len(deploymentID)-8, 0)
		imageName := fmt.Sprintf("%s:%s", pipeline.Name, deploymentID[suffixStart:])

		builtRef, err := uc.imagePreparer.PrepareImage(ctx, port.PrepareImageOpts{
			GitRepoURL:       pipeline.GitRepoURL,
			DockerfilePath:   pipeline.DockerfilePath,
			DockerContext:    pipeline.DockerContext,
			ImageName:        imageName,
			ClusterName:      target.ClusterName,
			DeploymentID:     deploymentID,
			RegistryURL:      pipeline.EnvVars[envRegistryURL],
			RegistryUsername: pipeline.EnvVars[envRegistryUsername],
			RegistryPassword: pipeline.EnvVars[envRegistryPassword],
		})
		if err != nil {
			return fmt.Errorf("prepare image: %w", err)
		}
		imageRef = builtRef
	}

	kubeconfig, err := uc.kubeconfigProvider.GetKubeconfig(ctx, pipeline.ClusterID)
	if err != nil {
		return fmt.Errorf("get kubeconfig for cluster %s: %w", pipeline.ClusterID, err)
	}

	template := "go-web-api"
	switch pipeline.AppType {
	case domain.AppTypeWeb:
		template = "react-spa"
	case domain.AppTypeBatch:
		template = "python-fastapi"
	}

	var port int32
	if imageRef != "" {
		switch pipeline.AppType {
		case domain.AppTypeBackend:
			port = 8080
		case domain.AppTypeBatch:
			port = 8000
		}
	}

	generated, err := manifests.Generate(manifests.DeployAppRequest{
		AppName:      pipeline.Name,
		GitURL:       pipeline.GitRepoURL,
		Namespace:    namespace,
		Template:     template,
		ImageRef:     imageRef,
		Replicas:     1,
		Port:         port,
		EnvVars:      deployEnvVars(pipeline.EnvVars),
		OTLPEndpoint: OTLPEndpointFor(ctx, uc.stackReader, pipeline.StackID),
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "nullus-cicd",
			"app.kubernetes.io/name":       pipeline.Name,
			"nullus.io/pipeline-id":        pipeline.ID,
		},
		Resources: manifests.ResourceSpec{
			CPURequest: "100m",
			CPULimit:   "500m",
			MemRequest: "128Mi",
			MemLimit:   "512Mi",
		},
	})
	if err != nil {
		return fmt.Errorf("generate manifests: %w", err)
	}

	yamlDocs := []string{
		generated.Namespace,
		firstNonEmptyManifest(pipeline.EnvVars[envManifestDeployment], generated.Deployment),
	}
	if includesOptionalManifest(manifestTypes, "service") {
		yamlDocs = append(yamlDocs, firstNonEmptyManifest(pipeline.EnvVars[envManifestService], generated.Service))
	}
	if includesOptionalManifest(manifestTypes, "ingress") {
		yamlDocs = append(yamlDocs, firstNonEmptyManifest(pipeline.EnvVars[envManifestIngress], generated.Ingress))
	}

	return uc.applier.ApplyWithTracking(ctx, kubeconfig, yamlDocs, deploymentID, stepOffset)
}

// delegateBranch 는 러너에게 실행시킬 브랜치다.
//
// 스캐폴딩이 만드는 Jenkinsfile 은 main 에서만 빌드·배포한다. 다른 브랜치를
// 실행시키면 job 은 돌지만 모든 단계가 조건에서 걸러져 아무 일도 하지 않는다.
const delegateBranch = "main"

// delegateToRunner 는 배포 실행을 스택의 CI 러너에게 넘긴다.
//
// 플랫폼은 여기서 끝난다. 이어지는 일 — 빌드, 이미지 push, 매니페스트 되커밋,
// 클러스터 동기화 — 는 CI 와 Argo CD 가 하고, 그 결과는 SyncPipelineRuns 가
// 빌드 이력으로 들여와 화면에 보여 준다.
func (uc *DeployPipeline) delegateToRunner(ctx context.Context, pipeline *domain.Pipeline, deploymentID string) error {
	if uc.buildDelegate == nil {
		return fmt.Errorf(
			"파이프라인 %s 는 스택 %s 의 CI 러너가 실행합니다. 그런데 이 서버에는 러너 연결이 배선돼 있지 않습니다 — "+
				"스택의 CI 플랫폼(Jenkins) 연결을 확인하세요",
			pipeline.Name, pipeline.StackID)
	}

	runURL, err := uc.buildDelegate.DelegateBuild(ctx, port.DelegateBuildOpts{
		StackID:      pipeline.StackID,
		JobName:      pipeline.Name,
		Branch:       delegateBranch,
		DeploymentID: deploymentID,
	})
	if err != nil {
		return fmt.Errorf("delegate build to runner: %w", err)
	}

	slog.Info("배포 실행을 스택의 CI 러너에 넘겼습니다",
		"pipeline", pipeline.Name, "stack_id", pipeline.StackID, "run_url", runURL)
	return nil
}

func firstNonEmptyManifest(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func deployEnvVars(envVars map[string]string) map[string]string {
	filtered := make(map[string]string)
	maps.Copy(filtered, envVars)
	delete(filtered, envManifestDeployment)
	delete(filtered, envManifestService)
	delete(filtered, envManifestIngress)
	return filtered
}
