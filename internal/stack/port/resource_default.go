package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type ResourceDefaultRepository interface {
	List(ctx context.Context) ([]*domain.ResourceDefault, error)
	Upsert(ctx context.Context, resource *domain.ResourceDefault) error
}
