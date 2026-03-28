package usecase

import (
	"context"
	"fmt"
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

func (uc *DeployPipeline) Execute(ctx context.Context, input DeployPipelineInput) (*DeployPipelineOutput, error) {
	if input.PipelineID == "" {
		return nil, fmt.Errorf("pipeline_id is required")
	}
	if input.Version == "" {
		return nil, fmt.Errorf("version is required")
	}

	pipeline, err := uc.pipelineRepo.GetByID(ctx, input.PipelineID)
	if err != nil {
		return nil, fmt.Errorf("pipeline not found: %w", err)
	}

	now := time.Now()
	deployment := &domain.Deployment{
		ID:         generateID("dep"),
		PipelineID: input.PipelineID,
		Version:    input.Version,
		Status:     domain.DeploymentStatusRunning,
		StartedAt:  now,
		DeployedBy: input.DeployedBy,
	}

	if err := uc.deploymentRepo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}

	deployErr := uc.applyToCluster(ctx, pipeline)

	completed := time.Now()
	deployment.CompletedAt = &completed

	if deployErr != nil {
		deployment.Status = domain.DeploymentStatusFailed
		_ = uc.deploymentRepo.Update(ctx, deployment)
		return nil, fmt.Errorf("apply to cluster: %w", deployErr)
	}

	deployment.Status = domain.DeploymentStatusSuccess
	_ = uc.deploymentRepo.Update(ctx, deployment)

	return &DeployPipelineOutput{Deployment: deployment}, nil
}

func (uc *DeployPipeline) applyToCluster(ctx context.Context, pipeline *domain.Pipeline) error {
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

	return uc.applier.Apply(ctx, kubeconfig, yamlDocs)
}
