package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// DeployPipelineInput holds the parameters for deploying a pipeline.
type DeployPipelineInput struct {
	PipelineID string
	Version    string
	DeployedBy string
}

// DeployPipelineOutput holds the result of deploying a pipeline.
type DeployPipelineOutput struct {
	Deployment *domain.Deployment
}

// DeployPipeline triggers a deployment for a pipeline, simulating K8s object creation.
type DeployPipeline struct {
	pipelineRepo   port.PipelineRepository
	deploymentRepo port.DeploymentRepository
}

// NewDeployPipeline constructs a DeployPipeline use case.
func NewDeployPipeline(pipelineRepo port.PipelineRepository, deploymentRepo port.DeploymentRepository) *DeployPipeline {
	return &DeployPipeline{
		pipelineRepo:   pipelineRepo,
		deploymentRepo: deploymentRepo,
	}
}

// Execute creates a new deployment record for the pipeline, simulating K8s object creation.
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

	// Simulate K8s object creation: Deployment, Service, ConfigMap
	// In production this would call the K8s API.
	_ = simulateK8sObjects(pipeline)

	now := time.Now()
	completed := now.Add(5 * time.Second) // simulated completion

	deployment := &domain.Deployment{
		ID:          generateID("dep"),
		PipelineID:  input.PipelineID,
		Version:     input.Version,
		Status:      domain.DeploymentStatusSuccess,
		StartedAt:   now,
		CompletedAt: &completed,
		DeployedBy:  input.DeployedBy,
	}

	if err := uc.deploymentRepo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}

	return &DeployPipelineOutput{Deployment: deployment}, nil
}

// simulateK8sObjects simulates the creation of K8s objects for a pipeline deployment.
func simulateK8sObjects(p *domain.Pipeline) []string {
	return []string{
		fmt.Sprintf("Deployment/%s-%s", p.Namespace, p.Name),
		fmt.Sprintf("Service/%s-%s", p.Namespace, p.Name),
		fmt.Sprintf("ConfigMap/%s-%s-config", p.Namespace, p.Name),
	}
}
