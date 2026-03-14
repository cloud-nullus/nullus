package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// CompatibilityRepository defines the interface for compatibility matrix persistence.
type CompatibilityRepository interface {
	GetAll(ctx context.Context) ([]*domain.CompatibilityMatrix, error)
	GetByID(ctx context.Context, id string) (*domain.CompatibilityMatrix, error)
	Validate(ctx context.Context, tools map[string]string) (*domain.CompatibilityMatrix, error)
}
