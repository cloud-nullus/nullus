package scheduler

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/cloud-nullus/draft/internal/admin/usecase"
)

// RefreshRunner is the subset of ClusterUseCase the scheduler calls per
// cluster. Mirrors `ClusterUseCase.RefreshDiscovery(ctx, id)`.
type RefreshRunner interface {
	RefreshDiscoveryByID(ctx context.Context, id string) error
}

// clusterUseCaseAdapter wraps the real *usecase.ClusterUseCase so tests
// can stub out both the lister + runner halves without importing the
// admin domain directly.
type clusterUseCaseAdapter struct {
	uc *usecase.ClusterUseCase
}

// NewClusterAdapter wires the production ClusterUseCase for scheduler use.
func NewClusterAdapter(uc *usecase.ClusterUseCase) *clusterUseCaseAdapter {
	return &clusterUseCaseAdapter{uc: uc}
}

// ListAllIDs returns every known cluster id across all organizations.
func (a *clusterUseCaseAdapter) ListAllIDs(ctx context.Context) ([]string, error) {
	clusters, err := a.uc.ListClusters(ctx, "")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(clusters))
	for _, c := range clusters {
		ids = append(ids, c.ID)
	}
	return ids, nil
}

// RefreshDiscoveryByID delegates to ClusterUseCase.RefreshDiscovery and
// drops the returned *domain.Cluster — the scheduler only cares about
// error propagation for logging.
func (a *clusterUseCaseAdapter) RefreshDiscoveryByID(ctx context.Context, id string) error {
	_, err := a.uc.RefreshDiscovery(ctx, id)
	return err
}

// IDLister is the scheduler's read-side dependency. Adapter above is the
// production wiring; tests pass an in-memory stub.
type IDLister interface {
	ListAllIDs(ctx context.Context) ([]string, error)
}

// RefreshDiscoveryScheduler runs ClusterUseCase.RefreshDiscovery on every
// registered cluster at a fixed interval. Phase 7 of F8 follow-up: catches
// node_architectures drift that would otherwise accumulate between manual
// Refresh Discovery clicks in the admin UI.
type RefreshDiscoveryScheduler struct {
	lister      IDLister
	runner      RefreshRunner
	interval    time.Duration
	iterTimeout time.Duration
	logger      *slog.Logger
	inFlight    atomic.Bool
	clock       func() time.Time
}

// Options configures the scheduler; all optional.
type Options struct {
	Interval    time.Duration
	IterTimeout time.Duration
	Logger      *slog.Logger
	Clock       func() time.Time
}

// NewRefreshDiscoveryScheduler constructs a scheduler. interval defaults to
// 24h; iterTimeout defaults to 5m; clock defaults to time.Now.
func NewRefreshDiscoveryScheduler(lister IDLister, runner RefreshRunner, opts Options) *RefreshDiscoveryScheduler {
	interval := opts.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	iterTimeout := opts.IterTimeout
	if iterTimeout <= 0 {
		iterTimeout = 5 * time.Minute
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	return &RefreshDiscoveryScheduler{
		lister:      lister,
		runner:      runner,
		interval:    interval,
		iterTimeout: iterTimeout,
		logger:      logger,
		clock:       clock,
	}
}

// Start blocks until ctx is canceled, running an iteration every
// interval. The first iteration runs immediately on Start so fresh
// deployments don't wait a full interval for initial discovery.
func (s *RefreshDiscoveryScheduler) Start(ctx context.Context) {
	s.runIteration(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("refresh discovery scheduler stopping")
			return
		case <-ticker.C:
			s.runIteration(ctx)
		}
	}
}

// runIteration lists clusters and refreshes each one. If a prior iteration
// is still running (shouldn't normally happen, but possible with a
// misconfigured short interval + slow network), the tick is skipped.
func (s *RefreshDiscoveryScheduler) runIteration(ctx context.Context) {
	if !s.inFlight.CompareAndSwap(false, true) {
		s.logger.Warn("refresh discovery previous iteration still running; skipping tick")
		return
	}
	defer s.inFlight.Store(false)

	iterCtx, cancel := context.WithTimeout(ctx, s.iterTimeout)
	defer cancel()

	ids, err := s.lister.ListAllIDs(iterCtx)
	if err != nil {
		s.logger.Error("refresh discovery: list clusters failed", "error", err)
		return
	}
	for _, id := range ids {
		if iterCtx.Err() != nil {
			return
		}
		if err := s.runner.RefreshDiscoveryByID(iterCtx, id); err != nil {
			s.logger.Warn("refresh discovery: cluster failed", "cluster_id", id, "error", err)
			// Continue; one failed cluster shouldn't halt the fleet sweep.
		}
	}
}
