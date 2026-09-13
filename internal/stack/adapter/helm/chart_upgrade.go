package helm

import (
	"context"
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// UpgradeChart is the version-changing path. It is separate from the values
// editor, whose contract deliberately reuses the currently installed chart.
func (h *HelmInstaller) UpgradeChart(ctx context.Context, req port.ChartUpgradeRequest) (*port.ChartUpgradeResult, error) {
	cfg, err := h.newActionConfig(nil, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("init action config: %w", err)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = req.Namespace
	upgrade.Timeout = helmOperationTimeout
	upgrade.Version = req.Version
	upgrade.ChartPathOptions.RepoURL = req.RepoURL
	upgrade.ChartPathOptions.Version = req.Version
	upgrade.DryRun = req.DryRun
	if req.DryRun {
		upgrade.DryRunOption = "server"
	}
	upgrade.Wait = !req.DryRun
	upgrade.Atomic = !req.DryRun
	upgrade.ResetValues = true

	settings := cli.New()
	path, err := locateChartWithRetry(ctx, func() (string, error) {
		return upgrade.ChartPathOptions.LocateChart(req.ChartName, settings)
	})
	if err != nil {
		return nil, fmt.Errorf("locate chart %s: %w", req.ChartName, err)
	}
	chart, err := loader.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load chart %s: %w", path, err)
	}
	values := req.Values
	if values == nil {
		values = map[string]any{}
	}
	rel, err := upgrade.RunWithContext(ctx, req.ReleaseName, chart, values)
	if err != nil {
		return nil, fmt.Errorf("upgrade release %s: %w", req.ReleaseName, err)
	}
	out := &port.ChartUpgradeResult{Revision: rel.Version, Manifest: rel.Manifest}
	if rel.Info != nil {
		out.Status = rel.Info.Status.String()
	}
	return out, nil
}

func (h *HelmInstaller) RollbackRelease(ctx context.Context, releaseName, namespace string, revision int) error {
	if revision < 1 {
		return fmt.Errorf("rollback revision must be positive")
	}
	cfg, err := h.newActionConfig(nil, namespace)
	if err != nil {
		return fmt.Errorf("init action config: %w", err)
	}
	rb := action.NewRollback(cfg)
	rb.Version = revision
	rb.Timeout = helmOperationTimeout
	rb.Wait = true
	rb.CleanupOnFail = true
	if err := rb.Run(releaseName); err != nil {
		return fmt.Errorf("rollback release %s: %w", releaseName, err)
	}
	return ctx.Err()
}
