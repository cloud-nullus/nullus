package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PipelineRepository defines the interface for pipeline persistence.
type PipelineRepository interface {
	Create(ctx context.Context, pipeline *domain.Pipeline) error
	GetByID(ctx context.Context, id string) (*domain.Pipeline, error)
	List(ctx context.Context, orgID string) ([]*domain.Pipeline, error)
	Update(ctx context.Context, pipeline *domain.Pipeline) error
}

// PipelineTemplateRepository defines the interface for pipeline template persistence.
type PipelineTemplateRepository interface {
	GetByID(ctx context.Context, id string) (*domain.PipelineTemplate, error)
	List(ctx context.Context) ([]*domain.PipelineTemplate, error)
}

// DeploymentRepository defines the interface for deployment persistence.
type DeploymentRepository interface {
	Create(ctx context.Context, deployment *domain.Deployment) error
	GetByID(ctx context.Context, id string) (*domain.Deployment, error)
	ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.Deployment, error)
	Update(ctx context.Context, deployment *domain.Deployment) error
}
