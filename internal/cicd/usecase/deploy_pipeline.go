package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/manifests"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type DeployPipelineInput struct {
	PipelineID string
	Version    string
	DeployedBy string
}

type DeployPipelineOutput struct {
	Deployment *domain.Deployment
}

type DeployPipeline struct {
	pipelineRepo       port.PipelineRepository
	deploymentRepo     port.DeploymentRepository
	kubeconfigProvider port.KubeconfigProvider
	applier            port.ManifestApplier
}

func NewDeployPipeline(
	pipelineRepo port.PipelineRepository,
	deploymentRepo port.DeploymentRepository,
	kubeconfigProvider port.KubeconfigProvider,
	applier port.ManifestApplier,
) *DeployPipeline {
	return &DeployPipeline{
		pipelineRepo:       pipelineRepo,
		deploymentRepo:     deploymentRepo,
		kubeconfigProvider: kubeconfigProvider,
		applier:            applier,
	}
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
func (uc *DeployPipeline) ApplyAsync(deploymentID string) {
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

	if deployErr := uc.applyToCluster(ctx, pipeline, deploymentID); deployErr != nil {
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

	uc.ApplyAsync(out.Deployment.ID)

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
	completed := time.Now()
	deployment.CompletedAt = &completed
	deployment.Status = domain.DeploymentStatusFailed
	_ = uc.deploymentRepo.Update(ctx, deployment)
}

func (uc *DeployPipeline) applyToCluster(ctx context.Context, pipeline *domain.Pipeline, deploymentID string) error {
	namespace := pipeline.Namespace
	if namespace == "" {
		namespace = "default"
	}

	kubeconfig, err := uc.kubeconfigProvider.GetKubeconfig(ctx, pipeline.ClusterID)
	if err != nil {
		return fmt.Errorf("get kubeconfig for cluster %s: %w", pipeline.ClusterID, err)
	}

	template := "go-web-api"
	if pipeline.AppType == domain.AppTypeWeb {
		template = "react-spa"
	} else if pipeline.AppType == domain.AppTypeBatch {
		template = "python-fastapi"
	}

	generated, err := manifests.Generate(manifests.DeployAppRequest{
		AppName:   pipeline.Name,
		GitURL:    pipeline.GitRepoURL,
		Namespace: namespace,
		Template:  template,
		Replicas:  1,
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
		generated.Service,
	}

	return uc.applier.ApplyWithTracking(ctx, kubeconfig, yamlDocs, deploymentID)
}
