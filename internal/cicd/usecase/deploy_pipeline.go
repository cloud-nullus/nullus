package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/manifests"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
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

func BuildStepPlan(pipeline *domain.Pipeline, manifestTypes ...[]string) []string {
	var selected []string
	if len(manifestTypes) > 0 {
		selected = manifestTypes[0]
	}
	steps := []string{"Namespace 생성", "Deployment 생성"}
	if pipeline != nil && pipeline.DockerfilePath != "" {
		steps = append([]string{"Git Clone", "Docker Build", "Image Load"}, steps...)
	}
	if includesOptionalManifest(selected, "service") {
		steps = append(steps, "Service 생성")
	}
	if includesOptionalManifest(selected, "ingress") {
		steps = append(steps, "Ingress 생성")
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
	stepOffset := 0

	if pipeline.DockerfilePath != "" && uc.imagePreparer != nil && uc.clusterTargetProvider != nil {
		target, err := uc.clusterTargetProvider.GetTarget(ctx, pipeline.ClusterID)
		if err != nil {
			return fmt.Errorf("get cluster target: %w", err)
		}

		suffixStart := max(len(deploymentID)-8, 0)
		imageName := fmt.Sprintf("%s:%s", pipeline.Name, deploymentID[suffixStart:])

		builtRef, err := uc.imagePreparer.PrepareImage(ctx, port.PrepareImageOpts{
			GitRepoURL:     pipeline.GitRepoURL,
			DockerfilePath: pipeline.DockerfilePath,
			DockerContext:  pipeline.DockerContext,
			ImageName:      imageName,
			ClusterName:    target.ClusterName,
			DeploymentID:   deploymentID,
		})
		if err != nil {
			return fmt.Errorf("prepare image: %w", err)
		}
		imageRef = builtRef
		stepOffset = 3
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
		EnvVars:   pipeline.EnvVars,
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
		generated.Deployment,
	}
	if includesOptionalManifest(manifestTypes, "service") {
		yamlDocs = append(yamlDocs, generated.Service)
	}
	if includesOptionalManifest(manifestTypes, "ingress") {
		yamlDocs = append(yamlDocs, generated.Ingress)
	}

	return uc.applier.ApplyWithTracking(ctx, kubeconfig, yamlDocs, deploymentID, stepOffset)
}
