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
	ctx := context.Background()

	deployment, err := uc.deploymentRepo.GetByID(ctx, deploymentID)
	if err != nil {
		slog.Error("apply: deployment not found", "id", deploymentID, "error", err)
		return
	}

	pipeline, err := uc.pipelineRepo.GetByID(ctx, deployment.PipelineID)
	if err != nil {
		slog.Error("apply: pipeline not found", "id", deployment.PipelineID, "error", err)
		uc.failDeployment(ctx, deployment, err)
		return
	}

	var selected []string
	if len(manifestTypes) > 0 {
		selected = manifestTypes[0]
	}
	if deployErr := uc.applyToCluster(ctx, pipeline, deploymentID, selected); deployErr != nil {
		slog.Error("apply: cluster deploy failed", "deployment", deploymentID, "error", deployErr)
		uc.failDeployment(ctx, deployment, deployErr)
		return
	}

	completed := time.Now()
	deployment.CompletedAt = &completed
	deployment.Status = domain.DeploymentStatusSuccess
	_ = uc.deploymentRepo.Update(ctx, deployment)
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
		AppName:   pipeline.Name,
		GitURL:    pipeline.GitRepoURL,
		Namespace: namespace,
		Template:  template,
		ImageRef:  imageRef,
		Replicas:  1,
		Port:      port,
		EnvVars:   deployEnvVars(pipeline.EnvVars),
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
