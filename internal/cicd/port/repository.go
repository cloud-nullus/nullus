package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PipelineRepository defines the interface for pipeline persistence.
type PipelineRepository interface {
	Create(ctx context.Context, pipeline *domain.Pipeline) error
	GetByID(ctx context.Context, id string) (*domain.Pipeline, error)
	// List returns pipelines for an organization.
	// An optional stackID filters results to pipelines linked to that stack.
	List(ctx context.Context, orgID string, stackID ...string) ([]*domain.Pipeline, error)
	// ListByStackID returns all pipelines linked to a specific stack.
	ListByStackID(ctx context.Context, stackID string) ([]*domain.Pipeline, error)
	Update(ctx context.Context, pipeline *domain.Pipeline) error
	Delete(ctx context.Context, id string) error
}

// PipelineTemplateRepository defines the interface for pipeline template persistence.
type PipelineTemplateRepository interface {
	GetByID(ctx context.Context, id string) (*domain.PipelineTemplate, error)
	List(ctx context.Context) ([]*domain.PipelineTemplate, error)
	Create(ctx context.Context, tmpl *domain.PipelineTemplate) error
	Update(ctx context.Context, tmpl *domain.PipelineTemplate) error
	Delete(ctx context.Context, id string) error
}

// DeploymentRepository defines the interface for deployment persistence.
type DeploymentRepository interface {
	Create(ctx context.Context, deployment *domain.Deployment) error
	GetByID(ctx context.Context, id string) (*domain.Deployment, error)
	ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.Deployment, error)
	Update(ctx context.Context, deployment *domain.Deployment) error
}
