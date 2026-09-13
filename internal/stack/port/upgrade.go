package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type UpgradeRepository interface {
	ListVerifiedBundles(ctx context.Context, tool, sourceChartVersion string) ([]domain.UpgradeBundle, error)
	GetVerifiedBundle(ctx context.Context, id string) (*domain.UpgradeBundle, error)
	CreateRun(ctx context.Context, run *domain.UpgradeRun) error
	UpdateRun(ctx context.Context, run *domain.UpgradeRun) error
	GetRun(ctx context.Context, stackID, runID string) (*domain.UpgradeRun, error)
	FindRunByIdempotencyKey(ctx context.Context, stackID, key string) (*domain.UpgradeRun, error)
	HasActiveRun(ctx context.Context, stackID string) (bool, error)
}

type ChartUpgradeRequest struct {
	ReleaseName string
	ChartName   string
	RepoURL     string
	Version     string
	Namespace   string
	Values      map[string]any
	DryRun      bool
}

type ChartUpgradeResult struct {
	Revision int
	Status   string
	Manifest string
}

type ChartUpgradeExecutor interface {
	UpgradeChart(ctx context.Context, req ChartUpgradeRequest) (*ChartUpgradeResult, error)
	RollbackRelease(ctx context.Context, releaseName, namespace string, revision int) error
}
