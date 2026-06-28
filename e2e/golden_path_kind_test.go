//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	helmadapter "github.com/cloud-nullus/draft/internal/stack/adapter/helm"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
)

// TestF8Task6_GoldenPath_KindDeploy exercises the three Narwhal-pinned Golden
// Path combinations end-to-end against a local Kind cluster. The test is
// gated by the `e2e` build tag and skipped gracefully when the Kind binary
// or the `nullus-platform` cluster are not available — this matches the
// runbook contract documented in docs/20_아키텍처/F8_Task6_Kind_Runbook.md
// and keeps the default `go test ./...` pipeline hermetic.
//
// Each subtest builds its own in-memory stack aggregate, wires a real
// helm.Orchestrator against the discovered Kind kubeconfig, kicks off the
// InstallStack usecase, and polls the stack state until either `completed`
// or a terminal failure state is reached.
//
// This is not a substitute for EKS / GKE validation (tracked separately as
// F8-F6-Cloud follow-up); it is a locally reproducible pre-merge smoke.
func TestF8Task6_GoldenPath_KindDeploy(t *testing.T) {
	clusterName, kubeconfig, ok := discoverKindCluster(t)
	if !ok {
		t.Skip("kind cluster 'nullus-platform' not available; see docs/20_아키텍처/F8_Task6_Kind_Runbook.md")
	}
	t.Logf("using kind cluster %q", clusterName)

	ts := time.Now().UnixNano()

	for _, tc := range goldenPathCases(ts) {
		tc := tc
		// Cases run sequentially: a single Kind node cannot host 3 full
		// stacks concurrently without flaky resource pressure.
		t.Run(tc.templateID, func(t *testing.T) {
			runGoldenPathDeploy(t, clusterName, kubeconfig, tc)
		})
	}
}

// goldenPathCase packages everything runGoldenPathDeploy needs to drive one
// matrix installation. toolOverrides are applied after templates are loaded
// into the Stack aggregate so the helm.Orchestrator sees the trimmed
// configuration (single replicas, persistence disabled, etc.).
type goldenPathCase struct {
	templateID string
	namespace  string
	timeout    time.Duration
	// toolOverrides mutates the in-memory StackConfig the Orchestrator
	// receives via SetStackConfig. Each entry replaces a ToolSelection
	// wholesale (Name/Version/Enabled/Values) and may toggle Enabled to
	// skip a step the matrix normally includes (e.g. drop monitoring on
	// a small kind node).
	toolOverrides func(cfg *domain.StackConfig)
}

func goldenPathCases(ts int64) []goldenPathCase {
	return []goldenPathCase{
		{
			templateID: "github-argocd-v1",
			namespace:  fmt.Sprintf("nullus-e2e-gh-argocd-%d", ts),
			timeout:    25 * time.Minute,
			toolOverrides: func(cfg *domain.StackConfig) {
				// GitHub + GitHub Actions are external SaaS — skip entirely.
				cfg.Artifacts.SourceRepository.Enabled = false
				cfg.Pipeline.CIPlatform.Enabled = false
				// Drop observability to keep the Kind footprint small; the
				// matrix's pass verdict only requires the clusterable bits
				// (container registry + CD + storage) to install cleanly.
				cfg.Monitoring.Collection.Enabled = false
				cfg.Monitoring.Visualization.Enabled = false
				cfg.Logging.Collection.Enabled = false
				cfg.Logging.Search.Enabled = false
			},
		},
		{
			templateID: "gitlab-argocd-v1",
			namespace:  fmt.Sprintf("nullus-e2e-glab-argocd-%d", ts),
			timeout:    12 * time.Minute,
			toolOverrides: func(cfg *domain.StackConfig) {
				// GitLab installs itself; drop monitoring/logging to keep
				// resources bounded on a single-node Kind.
				cfg.Monitoring.Collection.Enabled = false
				cfg.Monitoring.Visualization.Enabled = false
				cfg.Logging.Collection.Enabled = false
				cfg.Logging.Search.Enabled = false
			},
		},
		{
			templateID: "gitlab-allinone-v1",
			namespace:  fmt.Sprintf("nullus-e2e-glab-allinone-%d", ts),
			timeout:    15 * time.Minute,
			toolOverrides: func(cfg *domain.StackConfig) {
				cfg.Monitoring.Collection.Enabled = false
				cfg.Monitoring.Visualization.Enabled = false
				cfg.Logging.Collection.Enabled = false
				cfg.Logging.Search.Enabled = false
			},
		},
	}
}

func runGoldenPathDeploy(t *testing.T, clusterName string, kubeconfig []byte, tc goldenPathCase) {
	t.Helper()

	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	streamer := logadapter.NewMemoryStreamer()

	// Pull the Golden Path template by id. If Task 2's seed changed names
	// this will surface immediately.
	tmpl, err := templateRepo.GetByID(context.Background(), tc.templateID)
	if err != nil || tmpl == nil {
		t.Fatalf("load template %q: %v", tc.templateID, err)
	}

	// Assemble a StackConfig from the template tools, mark all selections
	// enabled by default, and then let the test-specific override trim the
	// set. Any tool category missing from the template stays empty.
	cfg := domain.StackConfig{
		Resources: domain.ResourcesConfig{
			DevCount:          1,
			ConcurrentRunners: 1,
			CommitsPerWeek:    1,
			BuildFrequency:    "low",
		},
	}
	for _, tool := range tmpl.Tools {
		sel := domain.ToolSelection{
			Name:    tool.Name,
			Version: firstNonEmpty(tool.Version, tool.AppVersion, tool.HelmVersion),
			Enabled: true,
		}
		switch tool.Category {
		case "source_repository":
			cfg.Artifacts.SourceRepository = sel
		case "container_registry":
			cfg.Artifacts.ContainerRegistry = sel
		case "storage_backend":
			cfg.Artifacts.StorageBackend = sel
		case "ci_platform":
			cfg.Pipeline.CIPlatform = sel
		case "cd_tool":
			cfg.Pipeline.CDTool = sel
		case "monitoring_collection":
			cfg.Monitoring.Collection = sel
		case "monitoring_visualization":
			cfg.Monitoring.Visualization = sel
		}
	}
	if tc.toolOverrides != nil {
		tc.toolOverrides(&cfg)
	}

	// Persist the stack. The usecase uses GetByID/Update so a
	// MemoryStackRepository is sufficient; no DB needed.
	stack := &domain.Stack{
		ID:         fmt.Sprintf("stk-e2e-%s-%d", strings.ReplaceAll(tc.templateID, "-", ""), time.Now().UnixNano()),
		Name:       tc.templateID,
		TemplateID: tc.templateID,
		OrgID:      "org-e2e",
		ClusterID:  clusterName,
		Namespace:  tc.namespace,
		Tools:      tmpl.Tools,
		State:      domain.StatePending,
		Config:     cfg,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := stackRepo.Create(context.Background(), stack); err != nil {
		t.Fatalf("create stack: %v", err)
	}

	installer := helmadapter.NewHelmInstaller(kubeconfig)
	orch := helmadapter.NewOrchestrator(installer, kubeconfig, tc.namespace)
	orch.SetNamespace(tc.namespace)
	orch.SetStackConfig(cfg)

	installUC := stackuc.NewInstallStack(
		stackRepo,
		streamer,
		stackuc.WithExecutor(orch),
	)

	ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
	defer cancel()

	// Cleanup registered before Execute so any panic leaves the cluster
	// recoverable. kubectl delete ns is best-effort — if it fails we just
	// log so the operator can clean up manually.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()
		_ = deleteKindNamespace(cleanupCtx, kubeconfig, tc.namespace, t)
		// Cluster-scoped helm releases (cert-manager, metrics-server) own
		// CRDs that collide between subtests. Best-effort uninstall so the
		// next subtest can reinstall cleanly. Ignore errors — the next
		// subtest's `checkExistingCertManagerInstallation` path will handle
		// partial state.
		for _, rel := range []string{"cert-manager", "metrics-server"} {
			uninstallClusterRelease(cleanupCtx, kubeconfig, rel, t)
		}
	})

	if err := installUC.Execute(ctx, stackuc.InstallStackInput{StackID: stack.ID}); err != nil {
		t.Fatalf("install usecase execute: %v", err)
	}

	// InstallStack kicks off a goroutine; poll the persisted state until
	// completed/failed/rolled_back or ctx deadline.
	finalState := waitForTerminalState(ctx, t, stackRepo, stack.ID)

	switch finalState {
	case domain.StateCompleted:
		t.Logf("template %q reached completed state", tc.templateID)
	case domain.StateFailed, domain.StateRolledBack:
		dumpKindDiagnostics(t, kubeconfig, tc.namespace, stackRepo, stack.ID)
		t.Errorf("template %q did not complete: final state = %s", tc.templateID, finalState)
	default:
		dumpKindDiagnostics(t, kubeconfig, tc.namespace, stackRepo, stack.ID)
		t.Errorf("template %q timed out in state %s", tc.templateID, finalState)
	}
}

// waitForTerminalState polls stackRepo.GetByID until the stack reaches a
// terminal deployment state or ctx expires. Returns the last-observed state.
func waitForTerminalState(
	ctx context.Context,
	t *testing.T,
	stackRepo *stackrepo.MemoryStackRepository,
	id string,
) domain.DeploymentState {
	t.Helper()
	pollInterval := 5 * time.Second
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()

	var last domain.DeploymentState
	for {
		s, err := stackRepo.GetByID(context.Background(), id)
		if err != nil {
			t.Logf("poll stack: %v", err)
		} else if s != nil {
			if s.State != last {
				t.Logf("stack %s state: %s", id, s.State)
				last = s.State
			}
			switch s.State {
			case domain.StateCompleted, domain.StateFailed, domain.StateRolledBack, domain.StateCancelled:
				return s.State
			}
		}
		select {
		case <-ctx.Done():
			return last
		case <-tick.C:
		}
	}
}

// dumpKindDiagnostics surfaces cluster-side evidence when a subtest fails so
// the operator can diagnose whether the failure was a chart version drift, a
// resource shortage, or something else.
func dumpKindDiagnostics(
	t *testing.T,
	kubeconfig []byte,
	namespace string,
	stackRepo *stackrepo.MemoryStackRepository,
	stackID string,
) {
	t.Helper()

	stack, err := stackRepo.GetByID(context.Background(), stackID)
	if err == nil && stack != nil {
		cfgJSON, _ := json.MarshalIndent(stack.Config, "", "  ")
		t.Logf("stack=%s state=%s namespace=%s config=%s", stack.ID, stack.State, stack.Namespace, cfgJSON)
	}

	// Write kubeconfig to a temp file so kubectl can consume it.
	path, cleanup, err := writeKubeconfigTempFile(kubeconfig)
	if err != nil {
		t.Logf("write kubeconfig temp file: %v", err)
		return
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	for _, args := range [][]string{
		{"get", "pods", "-n", namespace, "-o", "wide"},
		{"get", "events", "-n", namespace, "--sort-by=.lastTimestamp"},
	} {
		cmd := exec.CommandContext(ctx, "kubectl", append([]string{"--kubeconfig", path}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("kubectl %v: %v\n%s", args, err, string(out))
			continue
		}
		t.Logf("kubectl %v:\n%s", args, tailLines(string(out), 50))
	}
}

// uninstallClusterRelease best-effort removes a helm release that lives in
// its eponymous namespace (cert-manager → ns=cert-manager, etc.) plus any
// CRDs the release annotated as its own. Ignores errors because the release
// may not exist — the next subtest then installs fresh.
//
// cert-manager's CRDs are cluster-scoped and leak across subtests; without
// this cleanup the next subtest fails with "CRD exists in ns=<prev> and
// cannot be imported".
func uninstallClusterRelease(ctx context.Context, kubeconfig []byte, release string, t *testing.T) {
	path, cleanup, err := writeKubeconfigTempFile(kubeconfig)
	if err != nil {
		t.Logf("uninstall %s: write kubeconfig: %v", release, err)
		return
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, "helm", "uninstall", release, "--namespace", release, "--kubeconfig", path, "--wait")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("helm uninstall %s: %v\n%s", release, err, tailLines(string(out), 20))
	}
	nsCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", path, "delete", "namespace", release, "--ignore-not-found=true", "--wait=false")
	if out, err := nsCmd.CombinedOutput(); err != nil {
		t.Logf("kubectl delete ns %s: %v\n%s", release, err, string(out))
	}
	// Purge CRDs the release owns. Cluster-scoped resources aren't cleaned
	// up by `helm uninstall` so we explicitly delete anything labeled with
	// the release name. Best-effort.
	crdCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", path,
		"delete", "crd",
		"-l", "app.kubernetes.io/instance="+release,
		"--ignore-not-found=true")
	if out, err := crdCmd.CombinedOutput(); err != nil {
		t.Logf("kubectl delete crds labeled %s: %v\n%s", release, err, string(out))
	}
}

func deleteKindNamespace(ctx context.Context, kubeconfig []byte, namespace string, t *testing.T) error {
	path, cleanup, err := writeKubeconfigTempFile(kubeconfig)
	if err != nil {
		return err
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", path, "delete", "namespace", namespace, "--wait=false", "--ignore-not-found=true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("cleanup ns %s: %v\n%s", namespace, err, string(out))
	}
	return err
}

func writeKubeconfigTempFile(kubeconfig []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "nullus-e2e-kubeconfig-*.yaml")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.Write(kubeconfig); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

func firstNonEmpty(candidates ...string) string {
	for _, c := range candidates {
		if strings.TrimSpace(c) != "" {
			return c
		}
	}
	return ""
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return "… (truncated " + fmt.Sprint(len(lines)-n) + " lines)\n" + strings.Join(lines[len(lines)-n:], "\n")
}
