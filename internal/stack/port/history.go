package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// HistoryRepository defines the interface for stack version history persistence.
type HistoryRepository interface {
	SaveVersion(ctx context.Context, version *domain.StackVersion) error
	ListVersions(ctx context.Context, stackID string) ([]*domain.StackVersion, error)
	GetVersion(ctx context.Context, stackID, versionID string) (*domain.StackVersion, error)
	GetDiff(ctx context.Context, stackID, versionID string) ([]domain.ConfigDiff, error)
}
