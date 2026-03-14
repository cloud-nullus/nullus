package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// StackRepository defines the interface for stack persistence.
type StackRepository interface {
	Create(ctx context.Context, stack *domain.Stack) error
	GetByID(ctx context.Context, id string) (*domain.Stack, error)
	List(ctx context.Context, orgID string) ([]*domain.Stack, error)
	Update(ctx context.Context, stack *domain.Stack) error
	Delete(ctx context.Context, id string) error
}

// TemplateRepository defines the interface for template persistence.
type TemplateRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Template, error)
	List(ctx context.Context) ([]*domain.Template, error)
}
