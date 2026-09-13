package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type UpgradeExecutorFactory func([]byte) port.ChartUpgradeExecutor
type UpgradeBackupVerifier interface {
	Ready(context.Context, string) (bool, string, error)
}
type UpgradeHealthVerifier interface {
	Verify(context.Context, []byte, *domain.Stack, domain.UpgradeBundle) error
}

type UpgradeStack struct {
	stacks      port.StackRepository
	repo        port.UpgradeRepository
	kubeconfigs port.KubeconfigProvider
	releases    ReleaseManagerFactory
	executors   UpgradeExecutorFactory
	backup      UpgradeBackupVerifier
	health      UpgradeHealthVerifier
}

func NewUpgradeStack(stacks port.StackRepository, repo port.UpgradeRepository,
	kubeconfigs port.KubeconfigProvider, releases ReleaseManagerFactory,
	executors UpgradeExecutorFactory) *UpgradeStack {
	return &UpgradeStack{stacks: stacks, repo: repo, kubeconfigs: kubeconfigs,
		releases: releases, executors: executors}
}

func (uc *UpgradeStack) WithSafetyChecks(backup UpgradeBackupVerifier, health UpgradeHealthVerifier) *UpgradeStack {
	uc.backup, uc.health = backup, health
	return uc
}

type resolvedUpgrade struct {
	stack      *domain.Stack
	kubeconfig []byte
	manager    port.HelmReleaseManager
	executor   port.ChartUpgradeExecutor
	namespace  string
}

func (uc *UpgradeStack) resolve(ctx context.Context, stackID string) (*resolvedUpgrade, error) {
	if uc == nil || uc.stacks == nil || uc.repo == nil || uc.kubeconfigs == nil || uc.releases == nil || uc.executors == nil {
		return nil, fmt.Errorf("upgrade 기능이 구성되지 않았다")
	}
	stack, err := uc.stacks.GetByID(ctx, strings.TrimSpace(stackID))
	if err != nil || stack == nil {
		return nil, ErrStackNotFound
	}
	kubeconfig, err := uc.kubeconfigs.GetKubeconfig(ctx, stack.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("클러스터 kubeconfig 가 등록되어 있지 않다")
	}
	ns := strings.TrimSpace(stack.Namespace)
	if ns == "" {
		ns = "nullus"
	}
	return &resolvedUpgrade{stack: stack, kubeconfig: kubeconfig,
		manager: uc.releases(kubeconfig), executor: uc.executors(kubeconfig), namespace: ns}, nil
}

func (uc *UpgradeStack) Candidates(ctx context.Context, stackID string) ([]domain.UpgradeCandidate, error) {
	r, err := uc.resolve(ctx, stackID)
	if err != nil {
		return nil, err
	}
	releases, err := r.manager.ListReleases(ctx, r.namespace)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var out []domain.UpgradeCandidate
	for _, rel := range releases {
		if rel.ReleaseName != "argo-cd" {
			continue
		}
		candidate := domain.UpgradeCandidate{Tool: "argocd", ReleaseName: rel.ReleaseName,
			CurrentChartVersion: rel.ChartVersion, CurrentAppVersion: rel.AppVersion,
			CurrentRevision: rel.Revision, ReleaseStatus: rel.Status, CheckedAt: now}
		bundles, err := uc.repo.ListVerifiedBundles(ctx, "argocd", rel.ChartVersion)
		if err != nil {
			return nil, err
		}
		if len(bundles) == 0 {
			candidate.UpToDate = true
		} else {
			sort.SliceStable(bundles, func(i, j int) bool {
				left, leftErr := semver.ParseTolerant(bundles[i].TargetChartVersion)
				right, rightErr := semver.ParseTolerant(bundles[j].TargetChartVersion)
				if leftErr == nil && rightErr == nil {
					return left.GT(right)
				}
				if leftErr == nil {
					return true
				}
				if rightErr == nil {
					return false
				}
				return bundles[i].TargetChartVersion > bundles[j].TargetChartVersion
			})
			candidate.Bundle = &bundles[0]
		}
		if strings.HasPrefix(rel.Status, "pending-") || rel.Status != "deployed" {
			candidate.BlockedReason = "Helm release 상태가 deployed 가 아니다"
		}
		out = append(out, candidate)
	}
	return out, nil
}

func (uc *UpgradeStack) Preflight(ctx context.Context, stackID, bundleID string) ([]domain.UpgradeCheck, error) {
	r, bundle, release, err := uc.resolveBundle(ctx, stackID, bundleID)
	if err != nil {
		return nil, err
	}
	checks := []domain.UpgradeCheck{
		{Name: "release_status", Status: checkStatus(release.Status == "deployed"), Message: "Helm release status: " + release.Status, Remediation: "pending 작업을 정리하고 deployed 상태로 복구하세요."},
		{Name: "version_path", Status: checkStatus(release.ChartVersion == bundle.SourceChartVersion), Message: fmt.Sprintf("chart %s → %s", release.ChartVersion, bundle.TargetChartVersion), Remediation: "현재 버전에 맞는 검증 번들을 선택하세요."},
	}
	if bundle.RequiresBackup {
		ready, message := false, "백업 검증기가 구성되지 않았다"
		if uc.backup != nil {
			var e error
			ready, message, e = uc.backup.Ready(ctx, stackID)
			if e != nil {
				message = e.Error()
			}
		}
		checks = append(checks, domain.UpgradeCheck{Name: "backup", Status: checkStatus(ready), Message: message, Remediation: "최신 백업과 복구 가능성을 먼저 확인하세요."})
	}
	if !hasBlocked(checks) {
		values, e := r.manager.GetValues(ctx, release.ReleaseName, r.namespace)
		if e != nil {
			checks = append(checks, domain.UpgradeCheck{Name: "values", Status: "blocked", Message: e.Error()})
		} else {
			_, e = r.executor.UpgradeChart(ctx, port.ChartUpgradeRequest{ReleaseName: release.ReleaseName,
				ChartName: bundle.ChartName, RepoURL: bundle.RepoURL, Version: bundle.TargetChartVersion,
				Namespace: r.namespace, Values: values, DryRun: true})
			checks = append(checks, domain.UpgradeCheck{Name: "helm_dry_run", Status: checkStatus(e == nil), Message: errorMessage(e, "Helm server-side dry-run 통과"), Remediation: "차트와 values 호환성을 확인하세요."})
		}
	}
	return checks, nil
}

type StartUpgradeInput struct{ StackID, BundleID, RequestedBy, Reason, IdempotencyKey string }

func (uc *UpgradeStack) Start(ctx context.Context, in StartUpgradeInput) (*domain.UpgradeRun, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, fmt.Errorf("upgrade reason is required")
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return nil, fmt.Errorf("idempotency key is required")
	}
	if prior, err := uc.repo.FindRunByIdempotencyKey(ctx, in.StackID, in.IdempotencyKey); err != nil {
		return nil, err
	} else if prior != nil {
		return prior, nil
	}
	active, err := uc.repo.HasActiveRun(ctx, in.StackID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, fmt.Errorf("another upgrade is already running")
	}
	r, bundle, release, err := uc.resolveBundle(ctx, in.StackID, in.BundleID)
	if err != nil {
		return nil, err
	}
	checks, err := uc.Preflight(ctx, in.StackID, in.BundleID)
	if err != nil {
		return nil, err
	}
	run := &domain.UpgradeRun{ID: uuid.NewString(), StackID: in.StackID, BundleID: bundle.ID,
		Tool: bundle.Tool, Status: domain.UpgradeStatusPreflighting, CurrentStep: "preflight",
		RequestedBy: in.RequestedBy, Reason: in.Reason, IdempotencyKey: in.IdempotencyKey,
		SourceChartVersion: release.ChartVersion, TargetChartVersion: bundle.TargetChartVersion,
		PreviousRevision: release.Revision, Checks: checks, StartedAt: time.Now().UTC()}
	if hasBlocked(checks) {
		run.Status = domain.UpgradeStatusBlocked
		run.Error = "preflight failed"
		finish(run)
		if err := uc.repo.CreateRun(ctx, run); err != nil {
			return nil, err
		}
		return run, nil
	}
	if err := uc.repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	values, err := r.manager.GetValues(ctx, release.ReleaseName, r.namespace)
	if err != nil {
		return uc.failAndRollback(ctx, r, bundle, run, err)
	}
	run.Status = domain.UpgradeStatusUpgrading
	run.CurrentStep = "helm_upgrade"
	if err := uc.repo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("persist upgrade state: %w", err)
	}
	result, err := r.executor.UpgradeChart(ctx, port.ChartUpgradeRequest{ReleaseName: release.ReleaseName,
		ChartName: bundle.ChartName, RepoURL: bundle.RepoURL, Version: bundle.TargetChartVersion,
		Namespace: r.namespace, Values: values})
	if err != nil {
		return uc.failAndRollback(ctx, r, bundle, run, err)
	}
	run.ResultRevision = result.Revision
	run.CurrentStep = "health_check"
	if result.Status != "deployed" {
		return uc.failAndRollback(ctx, r, bundle, run, fmt.Errorf("release status is %s", result.Status))
	}
	if uc.health != nil {
		if err := uc.health.Verify(ctx, r.kubeconfig, r.stack, *bundle); err != nil {
			return uc.failAndRollback(ctx, r, bundle, run, err)
		}
	}
	run.Status = domain.UpgradeStatusSucceeded
	run.CurrentStep = "complete"
	finish(run)
	return run, uc.repo.UpdateRun(ctx, run)
}

func (uc *UpgradeStack) GetRun(ctx context.Context, stackID, runID string) (*domain.UpgradeRun, error) {
	return uc.repo.GetRun(ctx, stackID, runID)
}

func (uc *UpgradeStack) resolveBundle(ctx context.Context, stackID, bundleID string) (*resolvedUpgrade, *domain.UpgradeBundle, *port.ReleaseInfo, error) {
	r, err := uc.resolve(ctx, stackID)
	if err != nil {
		return nil, nil, nil, err
	}
	b, err := uc.repo.GetVerifiedBundle(ctx, bundleID)
	if err != nil {
		return nil, nil, nil, err
	}
	rels, err := r.manager.ListReleases(ctx, r.namespace)
	if err != nil {
		return nil, nil, nil, err
	}
	for i := range rels {
		if rels[i].ReleaseName == b.ReleaseName {
			return r, b, &rels[i], nil
		}
	}
	return nil, nil, nil, fmt.Errorf("release %s not found", b.ReleaseName)
}
func (uc *UpgradeStack) failAndRollback(ctx context.Context, r *resolvedUpgrade, b *domain.UpgradeBundle, run *domain.UpgradeRun, cause error) (*domain.UpgradeRun, error) {
	run.Error = cause.Error()
	run.Status = domain.UpgradeStatusRollingBack
	run.CurrentStep = "rollback"
	_ = uc.repo.UpdateRun(ctx, run)
	err := r.executor.RollbackRelease(ctx, b.ReleaseName, r.namespace, run.PreviousRevision)
	finish(run)
	if err != nil {
		run.Status = domain.UpgradeStatusManual
		run.Error = errors.Join(cause, err).Error()
	} else {
		run.Status = domain.UpgradeStatusRolledBack
	}
	return run, uc.repo.UpdateRun(ctx, run)
}
func checkStatus(ok bool) string {
	if ok {
		return "pass"
	}
	return "blocked"
}
func hasBlocked(c []domain.UpgradeCheck) bool {
	for _, v := range c {
		if v.Status == "blocked" {
			return true
		}
	}
	return false
}
func errorMessage(err error, ok string) string {
	if err != nil {
		return err.Error()
	}
	return ok
}
func finish(r *domain.UpgradeRun) { now := time.Now().UTC(); r.FinishedAt = &now }
