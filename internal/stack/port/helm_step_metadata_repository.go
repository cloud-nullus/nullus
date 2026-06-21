package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// HelmStepMetadataRepository defines persistence for DB-backed Helm step metadata.
type HelmStepMetadataRepository interface {
	Create(ctx context.Context, item *domain.HelmStepMetadata) error
	Update(ctx context.Context, item *domain.HelmStepMetadata) error
	Delete(ctx context.Context, stepName string) error
	GetByStep(ctx context.Context, stepName string) (*domain.HelmStepMetadata, error)
	List(ctx context.Context) ([]*domain.HelmStepMetadata, error)
}
