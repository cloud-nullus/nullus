package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

const (
	defaultSelfSignedBootstrapIssuer = "nullus-selfsigned-bootstrap"
	defaultInternalCAIssuer          = "nullus-internal-ca-issuer"
	defaultInternalCASecretName      = "nullus-internal-ca"
	defaultInternalCACertName        = "nullus-internal-ca-cert"
	defaultEnvoyDataPlaneTLSSecret   = "envoy"
	defaultEnvoyControlPlaneSecret   = "envoy-gateway"
	gatewayAPIStandardInstallURL     = "https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml"
	stepInstallingCertManager        = "installing_cert_manager"
	stepInstallingRunner             = "installing_runner"
)

var installGatewayOCIRelease = installOCIChartWithHelmCLI
var bootstrapInternalCAInstallation = func(ctx context.Context, o *Orchestrator, namespace string) error {
	return o.bootstrapInternalCA(ctx, namespace)
}
var waitForCertManagerInstallation = func(ctx context.Context, o *Orchestrator) error {
	return o.waitForCertManagerInstallation(ctx)
}
var verifyReleaseRuntimeReadiness = func(ctx context.Context, o *Orchestrator, step, releaseName, namespace string) error {
	return o.verifyReleaseRuntimeReadiness(ctx, step, releaseName, namespace)
}
var checkExistingCertManagerInstallation = func(ctx context.Context, o *Orchestrator) (bool, error) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return false, nil
	}

	requiredCRDs := []string{
		"certificaterequests.cert-manager.io",
		"certificates.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"issuers.cert-manager.io",
	}
	for _, crd := range requiredCRDs {
		if _, err := o.runKubectl(ctx, "get", "crd", crd); err != nil {
			return false, nil
		}
	}

	if _, err := o.detectCertManagerNamespace(ctx); err != nil {
		return false, nil
	}

	return true, nil
}

func (o *Orchestrator) certManagerNamespaceCandidates() []string {
	candidates := []string{"cert-manager", "nullus", "default"}
	if releaseNamespace, err := o.detectCertManagerReleaseNamespaceFromCRD(context.Background()); err == nil && strings.TrimSpace(releaseNamespace) != "" {
		trimmed := strings.TrimSpace(releaseNamespace)
		for _, candidate := range candidates {
			if candidate == trimmed {
				goto includeOrchestratorNamespace
			}
		}
		candidates = append([]string{trimmed}, candidates...)
	}

includeOrchestratorNamespace:
	if ns := strings.TrimSpace(o.namespace); ns != "" {
		for _, candidate := range candidates {
			if candidate == ns {
				return candidates
			}
		}
		candidates = append([]string{ns}, candidates...)
	}
	return candidates
}

func (o *Orchestrator) detectCertManagerNamespace(ctx context.Context) (string, error) {
	deployments := []string{
		"deployment/cert-manager",
		"deployment/cert-manager-webhook",
		"deployment/cert-manager-cainjector",
	}

	for _, namespace := range o.certManagerNamespaceCandidates() {
		allFound := true
		for _, deployment := range deployments {
			if _, err := o.runKubectl(ctx, "get", "-n", namespace, deployment); err != nil {
				allFound = false
				break
			}
		}
		if allFound {
			return namespace, nil
		}
	}

	return "", fmt.Errorf("cert-manager deployments not found in candidate namespaces")
}

func (o *Orchestrator) detectCertManagerReleaseNamespaceFromCRD(ctx context.Context) (string, error) {
	output, err := o.runKubectl(ctx, "get", "crd", "certificaterequests.cert-manager.io", "-o", "jsonpath={.metadata.annotations.meta\\.helm\\.sh/release-namespace}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

type Orchestrator struct {
	installer           port.HelmInstaller
	resourceDefaultRepo port.ResourceDefaultRepository
	rollback            *RollbackManager
	kubeconfig          []byte
	namespace           string
	chartConfig         map[string]ChartSpec
	stepOrder           map[string]int
	orderedStep         []string
	stackConfig         *domain.StackConfig
	stepConfigFieldPath map[string]string
	stepConfigEnabled   map[string]func(domain.StackConfig) bool
	mu                  sync.Mutex
	progress            map[string]int
	resourceDefaults    map[string]*domain.ResourceDefault
	defaultsLoaded      bool
	sharedClusterScoped bool
}

type OrchestratorOption func(*Orchestrator)

func WithResourceDefaultRepository(repo port.ResourceDefaultRepository) OrchestratorOption {
	return func(o *Orchestrator) {
		o.resourceDefaultRepo = repo
	}
}

func WithSharedClusterScopedComponents(enabled bool) OrchestratorOption {
	return func(o *Orchestrator) {
		o.sharedClusterScoped = enabled
	}
}

func (o *Orchestrator) VerifyDeployment(ctx context.Context, stackID string) error {
	_ = stackID

	for _, step := range o.orderedStep {
		if step == "integration_check" {
			continue
		}
		if !o.isStepEnabled(step) {
			continue
		}
		if _, ok := o.stepManifestForStep(step); ok {
			continue
		}

		spec, ok := o.chartConfig[step]
		if !ok {
			return fmt.Errorf("chart config not found for step %s", step)
		}
		spec = o.resolveChartSpecForStep(step, spec)
		if strings.TrimSpace(spec.ChartName) == "" {
			continue
		}

		namespace := o.namespace
		if spec.Namespace != "" {
			namespace = spec.Namespace
		}

		releaseName := o.releaseNameForSpec(spec)
		status, err := o.installer.Status(ctx, releaseName, namespace)
		if err != nil && step == "installing_gateway" && isReleaseNotFoundError(err) {
			if fallbackErr := installGatewayOCIRelease(ctx, o.kubeconfig, releaseName, spec.ChartName, namespace, spec.Version); fallbackErr == nil {
				status, err = o.installer.Status(ctx, releaseName, namespace)
			}
		}
		if err != nil {
			if step == stepInstallingCertManager && isReleaseNotFoundError(err) {
				if readinessErr := o.waitForCertManagerInstallation(ctx); readinessErr == nil {
					continue
				}
			}
			if step == stepInstallingRunner && isReleaseNotFoundError(err) {
				slog.Warn("skipping runner runtime health check because release is absent", "release", releaseName, "namespace", namespace)
				continue
			}
			return fmt.Errorf("status check failed for %s: %w", releaseName, err)
		}
		if status == nil {
			return fmt.Errorf("status check returned nil for %s", releaseName)
		}

		if !strings.EqualFold(status.Status, "deployed") {
			return fmt.Errorf("release %s is not healthy: status=%s", releaseName, status.Status)
		}
		if err := verifyReleaseRuntimeReadiness(ctx, o, step, releaseName, namespace); err != nil {
			return fmt.Errorf("runtime readiness failed for %s: %w", releaseName, err)
		}
	}

	return nil
}

func (o *Orchestrator) RollbackDeployment(ctx context.Context, stackID string) error {
	_ = stackID
	rbErr := o.rollback.RollbackAll(ctx, o.installer, o.namespace)
	cleanupErr := o.cleanupResidualReleaseResources(ctx)
	if rbErr != nil && cleanupErr != nil {
		return fmt.Errorf("rollback: %w; residual cleanup: %v", rbErr, cleanupErr)
	}
	if rbErr != nil {
		return rbErr
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	return nil
}

func (o *Orchestrator) cleanupResidualReleaseResources(ctx context.Context) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	resourceKinds := []string{"deploy", "sts", "ds", "job", "cronjob", "svc", "cm", "secret", "pvc"}
	seen := map[string]struct{}{}
	var errs []error

	for _, step := range o.orderedStep {
		spec, ok := o.chartConfig[step]
		if !ok {
			continue
		}
		spec = o.resolveChartSpecForStep(step, spec)
		releaseName := strings.TrimSpace(o.releaseNameForSpec(spec))
		if releaseName == "" {
			continue
		}
		namespace := strings.TrimSpace(o.namespace)
		if strings.TrimSpace(spec.Namespace) != "" {
			namespace = strings.TrimSpace(spec.Namespace)
		}
		if step == stepInstallingCertManager {
			if detectedNS, err := o.detectCertManagerReleaseNamespaceFromCRD(ctx); err == nil && strings.TrimSpace(detectedNS) != "" {
				namespace = strings.TrimSpace(detectedNS)
			}
		}
		if namespace == "" {
			continue
		}

		key := namespace + "::" + releaseName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		for _, kind := range resourceKinds {
			selector := "app.kubernetes.io/instance=" + releaseName
			if _, err := o.runKubectl(ctx, "delete", kind, "-n", namespace, "-l", selector, "--ignore-not-found"); err != nil {
				errs = append(errs, fmt.Errorf("delete %s for release %s in namespace %s: %w", kind, releaseName, namespace, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (o *Orchestrator) verifyReleaseRuntimeReadiness(ctx context.Context, step, releaseName, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	if err := o.waitForReleaseRollouts(ctx, releaseName, namespace); err != nil {
		return err
	}

	snapshot, err := o.releasePodSnapshot(ctx, releaseName, namespace)
	if err != nil {
		return err
	}
	if len(snapshot.Items) == 0 {
		return fmt.Errorf("no pods found for release %s in namespace %s", releaseName, namespace)
	}

	for _, pod := range snapshot.Items {
		phase := strings.TrimSpace(strings.ToLower(pod.Status.Phase))
		if phase == "succeeded" {
			continue
		}
		if phase != "running" {
			return fmt.Errorf("pod %s phase=%s", pod.Metadata.Name, strings.TrimSpace(pod.Status.Phase))
		}
		if len(pod.Status.ContainerStatuses) == 0 {
			return fmt.Errorf("pod %s has no container status yet", pod.Metadata.Name)
		}
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready {
				return fmt.Errorf("pod %s container %s not ready", pod.Metadata.Name, container.Name)
			}
		}
	}

	_ = step
	return nil
}

type ChartSpec struct {
	ReleaseName string
	ChartName   string
	RepoURL     string
	Version     string
	Namespace   string
	Values      map[string]any
	Wait        bool
}

func boolPtr(v bool) *bool {
	return &v
}

func NewOrchestrator(installer port.HelmInstaller, kubeconfig []byte, namespace string, opts ...OrchestratorOption) *Orchestrator {
	if namespace == "" {
		namespace = "nullus"
	}
	o := &Orchestrator{
		installer:  installer,
		rollback:   &RollbackManager{},
		kubeconfig: kubeconfig,
		namespace:  namespace,
		chartConfig: map[string]ChartSpec{
			stepInstallingCertManager: {
				ChartName: "cert-manager",
				RepoURL:   "https://charts.jetstack.io",
				Version:   "v1.16.3",
				Namespace: "cert-manager",
				Values:    DefaultValues(stepInstallingCertManager),
				Wait:      false,
			},
			"installing_metrics_server": {
				ChartName: "metrics-server",
				RepoURL:   "https://kubernetes-sigs.github.io/metrics-server/",
				Version:   "3.12.2",
				Values:    DefaultValues("installing_metrics_server"),
				Wait:      false,
			},
			"installing_postgresql": {
				ReleaseName: "nullus-postgresql",
				ChartName:   "postgresql",
				RepoURL:     "https://charts.bitnami.com/bitnami",
				Values:      DefaultValues("installing_postgresql"),
				Wait:        false,
			},
			"installing_minio": {
				ReleaseName: "nullus-minio",
				ChartName:   "minio",
				RepoURL:     "https://charts.min.io/",
				Version:     "5.4.0",
				Values:      DefaultValues("installing_minio"),
				Wait:        false,
			},
			"installing_object_storage_secret": {},
			"installing_openbao":               {},
			"installing_gitlab": {
				ChartName: "gitlab",
				RepoURL:   "https://charts.gitlab.io/",
				Version:   "8.7.2",
				Values:    DefaultValues("installing_gitlab"),
				Wait:      false,
			},
			"installing_argocd": {
				ChartName: "argo-cd",
				RepoURL:   "https://argoproj.github.io/argo-helm",
				Version:   "7.7.16",
				Values:    DefaultValues("installing_argocd"),
				Wait:      false,
			},
			stepInstallingRunner: {
				ChartName: "gitlab-runner",
				RepoURL:   "https://charts.gitlab.io/",
				Version:   "0.72.0",
				Values:    DefaultValues(stepInstallingRunner),
				Wait:      false,
			},
			"installing_prometheus": {
				ChartName: "kube-prometheus-stack",
				RepoURL:   "https://prometheus-community.github.io/helm-charts",
				Version:   "69.3.0",
				Values:    DefaultValues("installing_prometheus"),
				Wait:      false,
			},
			"installing_grafana": {
				ChartName: "grafana",
				RepoURL:   "https://grafana.github.io/helm-charts",
				Version:   "8.9.0",
				Values:    DefaultValues("installing_grafana"),
				Wait:      false,
			},
			"installing_logging": {
				ChartName: "loki",
				RepoURL:   "https://grafana.github.io/helm-charts",
				Version:   "2.10.3",
				Values:    DefaultValues("installing_logging"),
				Wait:      false,
			},
			"installing_log_search": {
				ChartName: "opensearch",
				RepoURL:   "https://opensearch-project.github.io/helm-charts",
				Version:   "2.22.0",
				Values:    DefaultValues("installing_logging_opensearch"),
				Wait:      false,
			},
			"installing_opentelemetry": {
				ChartName: "opentelemetry-collector",
				RepoURL:   "https://open-telemetry.github.io/opentelemetry-helm-charts",
				Version:   "0.75.0",
				Values:    DefaultValues("installing_opentelemetry"),
				Wait:      false,
			},
			"installing_gateway": {
				ReleaseName: "eg",
				ChartName:   "oci://docker.io/envoyproxy/gateway-helm",
				Wait:        false,
			},
			"integration_check": {},
		},
		stepOrder: map[string]int{
			stepInstallingCertManager:          0,
			"installing_metrics_server":        1,
			"installing_postgresql":            2,
			"installing_minio":                 3,
			"installing_object_storage_secret": 4,
			"installing_gitlab":                5,
			"installing_argocd":                6,
			stepInstallingRunner:               7,
			"installing_prometheus":            8,
			"installing_grafana":               9,
			"installing_logging":               10,
			"installing_log_search":            11,
			"installing_opentelemetry":         12,
			"installing_gateway":               13,
			"installing_openbao":               14,
			"integration_check":                15,
		},
		orderedStep: []string{
			stepInstallingCertManager,
			"installing_metrics_server",
			"installing_postgresql",
			"installing_minio",
			"installing_object_storage_secret",
			"installing_gitlab",
			"installing_argocd",
			stepInstallingRunner,
			"installing_prometheus",
			"installing_grafana",
			"installing_logging",
			"installing_log_search",
			"installing_opentelemetry",
			"installing_gateway",
			"installing_openbao",
			"integration_check",
		},
		stepConfigFieldPath: map[string]string{
			"installing_postgresql":            "config.storage.database",
			"installing_minio":                 "config.artifacts.storage_backend",
			"installing_object_storage_secret": "config.storage.object_storage",
			"installing_openbao":               "config.authentication.provider",
			"installing_gitlab":                "config.artifacts.source_repository",
			"installing_argocd":                "config.pipeline.cd_tool",
			stepInstallingRunner:               "config.pipeline.ci_platform",
			"installing_prometheus":            "config.monitoring.collection",
			"installing_grafana":               "config.monitoring.visualization",
			"installing_logging":               "config.logging.collection",
			"installing_log_search":            "config.logging.search",
			"installing_opentelemetry":         "config.logging.trace_layer",
		},
		stepConfigEnabled: map[string]func(domain.StackConfig) bool{
			"installing_postgresql": func(cfg domain.StackConfig) bool {
				if cfg.Storage == nil {
					return true
				}
				return strings.TrimSpace(cfg.Storage.Database.Mode) == "create"
			},
			"installing_minio": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.StorageBackend.Enabled
			},
			"installing_object_storage_secret": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.StorageBackend.Enabled && cfg.Artifacts.SourceRepository.Enabled
			},
			"installing_gitlab": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.SourceRepository.Enabled
			},
			"installing_openbao": func(cfg domain.StackConfig) bool {
				if cfg.Authentication == nil {
					return false
				}
				return strings.EqualFold(strings.TrimSpace(cfg.Authentication.Provider), "openbao")
			},
			"installing_argocd": func(cfg domain.StackConfig) bool {
				return cfg.Pipeline.CDTool.Enabled
			},
			stepInstallingRunner: func(cfg domain.StackConfig) bool {
				return cfg.Pipeline.CIPlatform.Enabled
			},
			"installing_prometheus": func(cfg domain.StackConfig) bool {
				return cfg.Monitoring.Collection.Enabled
			},
			"installing_grafana": func(cfg domain.StackConfig) bool {
				return cfg.Monitoring.Visualization.Enabled
			},
			"installing_logging": func(cfg domain.StackConfig) bool {
				return cfg.Logging.Collection.Enabled
			},
			"installing_log_search": func(cfg domain.StackConfig) bool {
				if !cfg.Logging.Search.Enabled {
					return false
				}
				search := strings.TrimSpace(cfg.Logging.Search.Name)
				collection := strings.TrimSpace(cfg.Logging.Collection.Name)
				if search != "" && collection != "" && search == collection {
					return false
				}
				return true
			},
			"installing_opentelemetry": func(cfg domain.StackConfig) bool {
				return cfg.Logging.TraceLayer.Enabled
			},
		},
		progress: make(map[string]int),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (o *Orchestrator) SetNamespace(namespace string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if namespace == "" {
		namespace = "nullus"
	}
	o.namespace = namespace
}

func (o *Orchestrator) IsStepEnabled(step string) bool {
	return o.isStepEnabled(step)
}

func (o *Orchestrator) SetStackConfig(config domain.StackConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	cfg := config
	o.stackConfig = &cfg
}

func (o *Orchestrator) ExecuteStep(ctx context.Context, stackID, step, phase string) error {
	_ = stackID
	_ = phase

	spec, ok := o.chartConfig[step]
	if !ok {
		return fmt.Errorf("unknown step %q", step)
	}

	order, ok := o.stepOrder[step]
	if !ok {
		return fmt.Errorf("step order not defined for %q", step)
	}
	if err := o.ensureOrder(stackID, step, order); err != nil {
		return err
	}

	if !o.isStepEnabled(step) {
		o.markCompleted(stackID, order)
		path := o.stepConfigFieldPath[step]
		slog.Info("skipping disabled stack install step", "stack_id", stackID, "step", step, "config_path", path)
		return nil
	}

	if step == "integration_check" {
		if looksLikeKubeconfig(o.kubeconfig) {
			targetNamespace := strings.TrimSpace(o.namespace)
			if targetNamespace == "" {
				targetNamespace = "nullus"
			}
			if err := o.tryReconcileGatewayDataPlaneTLSSecret(ctx, targetNamespace); err != nil {
				return fmt.Errorf("reconcile gateway data-plane tls secret: %w", err)
			}
		}
		o.markCompleted(stackID, order)
		return nil
	}

	namespace := o.namespace
	if spec.Namespace != "" {
		namespace = spec.Namespace
	}
	if step == stepInstallingCertManager && looksLikeKubeconfig(o.kubeconfig) {
		if releaseNamespace, err := o.detectCertManagerReleaseNamespaceFromCRD(ctx); err == nil && strings.TrimSpace(releaseNamespace) != "" {
			namespace = strings.TrimSpace(releaseNamespace)
		}
	}

	if step == "installing_object_storage_secret" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		manifest := o.sharedObjectStorageSecretManifest(namespace)
		if strings.TrimSpace(manifest) != "" {
			if err := o.applyManifest(ctx, namespace, manifest); err != nil {
				return fmt.Errorf("apply object storage secret manifest: %w", err)
			}
		}
		o.markCompleted(stackID, order)
		return nil
	}

	spec = o.resolveChartSpecForStep(step, spec)
	manifest, hasManifest := o.stepManifestForStep(step)
	if step == "installing_gateway" && !hasManifest && looksLikeKubeconfig(o.kubeconfig) {
		if generated := o.defaultGatewayBundleManifest(namespace); strings.TrimSpace(generated) != "" {
			manifest = generated
			hasManifest = true
		}
	}

	if hasManifest && step != "installing_gateway" {
		if err := o.applyManifest(ctx, namespace, manifest); err != nil {
			return fmt.Errorf("apply yaml manifest for step %s: %w", step, err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if strings.TrimSpace(spec.ChartName) == "" {
		o.markCompleted(stackID, order)
		return nil
	}
	if step == stepInstallingCertManager {
		installed, checkErr := checkExistingCertManagerInstallation(ctx, o)
		if checkErr != nil {
			return fmt.Errorf("detect existing cert-manager installation: %w", checkErr)
		}
		if installed {
			slog.Info("reusing existing cert-manager installation", "namespace", namespace)
			if err := waitForCertManagerInstallation(ctx, o); err != nil {
				return fmt.Errorf("wait for cert-manager readiness: %w", err)
			}
			if err := bootstrapInternalCAInstallation(ctx, o, namespace); err != nil {
				return fmt.Errorf("bootstrap internal ca: %w", err)
			}
			o.markCompleted(stackID, order)
			return nil
		}
	}
	if step == "installing_gateway" && looksLikeKubeconfig(o.kubeconfig) {
		if err := o.ensureGatewayAPICRDs(ctx); err != nil {
			return fmt.Errorf("ensure gateway api crds: %w", err)
		}
	}
	releaseName := o.releaseNameForSpec(spec)
	values := o.valuesForStep(step, spec)
	if step == stepInstallingRunner {
		if looksLikeKubeconfig(o.kubeconfig) {
			runnerToken, tokenErr := o.discoverGitLabRunnerRegistrationToken(ctx, namespace)
			if tokenErr != nil {
				slog.Warn("gitlab runner installation skipped: runner token discovery failed", "namespace", namespace, "error", tokenErr)
				o.markCompleted(stackID, order)
				return nil
			}
			values = mergeMaps(values, map[string]any{
				"runnerToken": runnerToken,
			})
		} else {
			slog.Warn("kubeconfig unavailable; skipping gitlab runner token discovery", "namespace", namespace)
		}
	}

	result, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: releaseName,
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   namespace,
		Values:      values,
		Wait:        boolPtr(spec.Wait),
	})
	if err != nil && step == "installing_gateway" && strings.Contains(strings.ToLower(err.Error()), "missing registry client") {
		if fallbackErr := installOCIChartWithHelmCLI(ctx, o.kubeconfig, releaseName, spec.ChartName, namespace, spec.Version); fallbackErr == nil {
			result = &port.HelmInstallResult{ReleaseName: releaseName, Namespace: namespace, Status: "deployed"}
			err = nil
		} else {
			err = fmt.Errorf("%w; fallback helm cli install failed: %v", err, fallbackErr)
		}
	}
	if err != nil {
		return fmt.Errorf("install step %s: %w", step, err)
	}
	if result != nil {
		o.rollback.Push(result.ReleaseName)
	}
	if step == stepInstallingCertManager {
		if err := waitForCertManagerInstallation(ctx, o); err != nil {
			return fmt.Errorf("wait for cert-manager readiness: %w", err)
		}
		if err := bootstrapInternalCAInstallation(ctx, o, namespace); err != nil {
			return fmt.Errorf("bootstrap internal ca: %w", err)
		}
	}
	if step == "installing_gateway" {
		if looksLikeKubeconfig(o.kubeconfig) {
			if err := o.reconcileGatewayDataPlaneTLSSecret(ctx, namespace); err != nil {
				return fmt.Errorf("reconcile gateway data-plane tls secret: %w", err)
			}
			if err := o.applyManifest(ctx, namespace, defaultEnvoyGatewayClassManifest()); err != nil {
				return fmt.Errorf("apply default gatewayclass manifest: %w", err)
			}
		}
	}
	if step == "installing_route" && looksLikeKubeconfig(o.kubeconfig) {
		if err := o.tryReconcileGatewayDataPlaneTLSSecret(ctx, namespace); err != nil {
			return fmt.Errorf("reconcile gateway data-plane tls secret: %w", err)
		}
	}
	if step == "installing_gateway" && hasManifest {
		manifestNamespace := o.namespace
		if strings.TrimSpace(manifestNamespace) == "" {
			manifestNamespace = namespace
		}
		if looksLikeKubeconfig(o.kubeconfig) {
			filteredManifest, skippedBackendTLSPolicy, filterErr := o.filterOptionalGatewayPolicies(ctx, manifest)
			if filterErr != nil {
				return fmt.Errorf("filter optional gateway policies: %w", filterErr)
			}
			if skippedBackendTLSPolicy {
				slog.Warn("skipping BackendTLSPolicy manifest because the CRD is unavailable", "namespace", manifestNamespace)
			}
			manifest = filteredManifest
			normalizedManifest, normalizedAny, normalizeErr := normalizeGatewayBackendServiceAliases(manifest)
			if normalizeErr != nil {
				return fmt.Errorf("normalize gateway backend aliases: %w", normalizeErr)
			}
			if normalizedAny {
				slog.Warn("normalized gateway backend service aliases to installed service names", "namespace", manifestNamespace)
			}
			manifest = normalizedManifest
		}
		if err := o.applyManifest(ctx, manifestNamespace, manifest); err != nil {
			return fmt.Errorf("apply yaml manifest for step %s: %w", step, err)
		}
	}
	o.markCompleted(stackID, order)
	return nil
}

type podListSnapshot struct {
	Items []podSnapshotItem `json:"items"`
}

type podSnapshotItem struct {
	Metadata podSnapshotMetadata `json:"metadata"`
	Status   podSnapshotStatus   `json:"status"`
}

type podSnapshotMetadata struct {
	Name string `json:"name"`
}

type podSnapshotStatus struct {
	Phase             string               `json:"phase"`
	PodIP             string               `json:"podIP"`
	ContainerStatuses []podContainerStatus `json:"containerStatuses"`
}

type podContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
}

func (o *Orchestrator) StartStepRuntimeTail(ctx context.Context, stackID, step string, emit func(level, message string)) (stop func()) {
	_ = stackID
	if emit == nil {
		return nil
	}
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	spec, ok := o.chartConfig[step]
	if !ok {
		return nil
	}
	spec = o.resolveChartSpecForStep(step, spec)
	if strings.TrimSpace(spec.ChartName) == "" {
		return nil
	}

	namespace := o.namespace
	if strings.TrimSpace(spec.Namespace) != "" {
		namespace = spec.Namespace
	}
	releaseName := o.releaseNameForSpec(spec)
	if strings.TrimSpace(releaseName) == "" {
		return nil
	}

	tailCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		seen := make(map[string]struct{})
		emitTail := func() {
			output, err := o.runKubectl(tailCtx,
				"logs",
				"-n", namespace,
				"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName),
				"--all-containers=true",
				"--tail=40",
				"--prefix=true",
			)
			if err != nil {
				return
			}
			for _, line := range strings.Split(string(output), "\n") {
				msg := strings.TrimSpace(line)
				if msg == "" {
					continue
				}
				if _, ok := seen[msg]; ok {
					continue
				}
				if len(seen) > 4000 {
					seen = map[string]struct{}{}
				}
				seen[msg] = struct{}{}
				emit("info", fmt.Sprintf("container stdout: %s", msg))
			}
		}

		emitTail()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-tailCtx.Done():
				return
			case <-ticker.C:
				emitTail()
			}
		}
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (o *Orchestrator) StepRuntimeLogs(ctx context.Context, stackID, step string) (infos []string, warns []string) {
	_ = stackID

	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil, nil
	}

	spec, ok := o.chartConfig[step]
	if !ok {
		return nil, nil
	}
	spec = o.resolveChartSpecForStep(step, spec)
	if strings.TrimSpace(spec.ChartName) == "" {
		return nil, nil
	}

	namespace := o.namespace
	if strings.TrimSpace(spec.Namespace) != "" {
		namespace = spec.Namespace
	}
	releaseName := o.releaseNameForSpec(spec)
	if strings.TrimSpace(releaseName) == "" {
		return nil, nil
	}

	snapshot, err := o.releasePodSnapshot(ctx, releaseName, namespace)
	if err != nil {
		return nil, []string{fmt.Sprintf("pod snapshot unavailable for release %s: %v", releaseName, err)}
	}

	if len(snapshot.Items) == 0 {
		return []string{fmt.Sprintf("pod snapshot: no pods found yet for release %s in namespace %s", releaseName, namespace)}, nil
	}

	const maxPodLines = 12
	infos = append(infos, fmt.Sprintf("pod snapshot for release %s in namespace %s (%d pods)", releaseName, namespace, len(snapshot.Items)))
	for idx, pod := range snapshot.Items {
		if idx >= maxPodLines {
			infos = append(infos, fmt.Sprintf("... %d additional pods omitted", len(snapshot.Items)-maxPodLines))
			break
		}
		readyCount := 0
		restartCount := 0
		for _, container := range pod.Status.ContainerStatuses {
			if container.Ready {
				readyCount++
			}
			restartCount += container.RestartCount
		}
		infos = append(infos, fmt.Sprintf(
			"pod=%s phase=%s ready=%d/%d restarts=%d ip=%s",
			pod.Metadata.Name,
			strings.TrimSpace(pod.Status.Phase),
			readyCount,
			len(pod.Status.ContainerStatuses),
			restartCount,
			strings.TrimSpace(pod.Status.PodIP),
		))
	}

	return infos, nil
}

func (o *Orchestrator) releasePodSnapshot(ctx context.Context, releaseName, namespace string) (*podListSnapshot, error) {
	selectors := releaseLabelSelectors(releaseName)
	for _, selector := range selectors {
		output, err := o.runKubectl(ctx,
			"get", "pods",
			"-n", namespace,
			"-l", selector,
			"-o", "json",
		)
		if err != nil {
			return nil, err
		}

		var snapshot podListSnapshot
		if err := json.Unmarshal(output, &snapshot); err != nil {
			return nil, err
		}
		if len(snapshot.Items) > 0 {
			return &snapshot, nil
		}
	}

	return &podListSnapshot{}, nil
}

func (o *Orchestrator) waitForReleaseRollouts(ctx context.Context, releaseName, namespace string) error {
	resources := []string{"deployments", "statefulsets", "daemonsets"}
	selectors := releaseLabelSelectors(releaseName)
	rolloutTimeout := "180s"
	if strings.TrimSpace(releaseName) == "gitlab" {
		rolloutTimeout = "600s"
	}
	for _, resourceType := range resources {
		for _, selector := range selectors {
			output, err := o.runKubectl(ctx,
				"get", resourceType,
				"-n", namespace,
				"-l", selector,
				"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`,
			)
			if err != nil {
				return err
			}
			for _, rawName := range strings.Split(string(output), "\n") {
				name := strings.TrimSpace(rawName)
				if name == "" {
					continue
				}
				resource := strings.TrimSuffix(resourceType, "s") + "/" + name
				if _, err := o.runKubectl(ctx, "rollout", "status", "-n", namespace, resource, "--timeout="+rolloutTimeout); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func releaseLabelSelectors(releaseName string) []string {
	name := strings.TrimSpace(releaseName)
	if name == "" {
		return []string{""}
	}
	return []string{
		fmt.Sprintf("app.kubernetes.io/instance=%s", name),
		fmt.Sprintf("release=%s", name),
	}
}

func looksLikeKubeconfig(kubeconfig []byte) bool {
	if len(kubeconfig) == 0 {
		return false
	}
	text := string(kubeconfig)
	return strings.Contains(text, "apiVersion:") && strings.Contains(text, "clusters:")
}

func isReleaseNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "release: not found") || strings.Contains(msg, "release not loaded")
}

func (o *Orchestrator) bootstrapInternalCA(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	manifest := o.internalCAManifest(namespace)
	if strings.TrimSpace(manifest) == "" {
		return nil
	}
	return o.applyManifest(ctx, namespace, manifest)
}

func (o *Orchestrator) waitForCertManagerInstallation(ctx context.Context) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	requiredCRDs := []string{
		"certificaterequests.cert-manager.io",
		"certificates.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"issuers.cert-manager.io",
	}
	for _, crd := range requiredCRDs {
		if err := o.waitForKubectlGet(ctx, "crd", crd); err != nil {
			return fmt.Errorf("cert-manager crd %s not ready: %w", crd, err)
		}
	}

	certManagerNamespace, err := o.detectCertManagerNamespace(ctx)
	if err != nil {
		return err
	}

	deployments := []string{
		"deployment/cert-manager",
		"deployment/cert-manager-webhook",
		"deployment/cert-manager-cainjector",
	}
	for _, deployment := range deployments {
		if err := o.waitForKubectlGet(ctx, "-n", certManagerNamespace, deployment); err != nil {
			return fmt.Errorf("cert-manager deployment %s not found: %w", deployment, err)
		}
		if _, err := o.runKubectl(ctx, "rollout", "status", "-n", certManagerNamespace, deployment, "--timeout=180s"); err != nil {
			return fmt.Errorf("cert-manager deployment %s not ready: %w", deployment, err)
		}
	}

	if err := o.waitForCertManagerWebhookTrust(ctx); err != nil {
		return fmt.Errorf("cert-manager webhook trust not stabilized: %w", err)
	}

	if err := o.waitForCertManagerStartupAPICheck(ctx, certManagerNamespace); err != nil {
		return fmt.Errorf("cert-manager startup API check not complete: %w", err)
	}

	return nil
}

func (o *Orchestrator) waitForCertManagerWebhookTrust(ctx context.Context) error {
	jsonpaths := []string{
		"{.webhooks[0].clientConfig.caBundle}",
		"{.webhooks[1].clientConfig.caBundle}",
	}
	resources := []string{
		"mutatingwebhookconfiguration/cert-manager-webhook",
		"validatingwebhookconfiguration/cert-manager-webhook",
	}

	for _, resource := range resources {
		ready := false
		for _, jsonpath := range jsonpaths {
			if err := o.waitForKubectlNonEmptyOutput(ctx, "get", resource, "-o", "jsonpath="+jsonpath); err == nil {
				ready = true
				break
			}
		}
		if !ready {
			return fmt.Errorf("cabundle not injected for %s", resource)
		}
	}

	return nil
}

func (o *Orchestrator) waitForCertManagerStartupAPICheck(ctx context.Context, namespace string) error {
	const resource = "job/cert-manager-startupapicheck"

	if err := o.waitForKubectlGet(ctx, "-n", namespace, resource); err != nil {
		if isKubectlNotFoundError(err) {
			slog.Info("cert-manager startup API check job not found; skipping wait", "namespace", namespace)
			return nil
		}
		return err
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace, "--for=condition=complete", "--timeout=180s", resource); err != nil {
		return err
	}
	return nil
}

func isKubectlNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "notfound") || strings.Contains(msg, "not found")
}

func (o *Orchestrator) waitForKubectlNonEmptyOutput(ctx context.Context, args ...string) error {
	const (
		maxAttempts = 30
		retryDelay  = 2 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, err := o.runKubectl(ctx, args...)
		if err == nil && strings.TrimSpace(string(output)) != "" {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("empty output")
		}

		if attempt == maxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("resource output not ready")
	}
	return lastErr
}

func (o *Orchestrator) internalCAManifest(namespace string) string {
	issuerName := defaultInternalCAIssuer
	secretName := defaultInternalCASecretName
	certName := defaultInternalCACertName

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	if cfg != nil && cfg.AccessDomainTLS != nil {
		if strings.TrimSpace(cfg.AccessDomainTLS.IssuerName) != "" {
			slog.Info("ignoring access-domain TLS issuer override for internal CA bootstrap", "issuer", cfg.AccessDomainTLS.IssuerName)
		}
	}

	return fmt.Sprintf(`apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: %s
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %s
  namespace: %s
spec:
  isCA: true
  commonName: nullus-internal-root
  secretName: %s
  duration: 87600h
  renewBefore: 720h
  issuerRef:
    name: %s
    kind: ClusterIssuer
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: %s
spec:
  ca:
    secretName: %s
`, defaultSelfSignedBootstrapIssuer, certName, namespace, secretName, defaultSelfSignedBootstrapIssuer, issuerName, secretName)
}

func (o *Orchestrator) stepManifestForStep(step string) (string, bool) {
	if step == "installing_openbao" {
		return o.openBaoManifest(o.namespace), true
	}
	if step != "installing_prometheus" && step != "installing_grafana" && step != "installing_logging" && step != "installing_log_search" && step != "installing_opentelemetry" && step != "installing_gateway" {
		return "", false
	}

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil || len(cfg.YAMLOverrides) == 0 {
		return "", false
	}

	keys := []string{step}
	if step == "installing_prometheus" {
		keys = append(keys, "prometheus", cfg.Monitoring.Collection.Name)
	}
	if step == "installing_grafana" {
		keys = append(keys, "grafana", cfg.Monitoring.Visualization.Name)
	}
	if step == "installing_logging" {
		keys = append(keys, "logging", cfg.Logging.Collection.Name)
	}
	if step == "installing_log_search" {
		keys = append(keys, "log_search", cfg.Logging.Search.Name)
	}
	if step == "installing_opentelemetry" {
		keys = append(keys, "opentelemetry", "opentelemetry-collector", cfg.Logging.TraceLayer.Name)
	}
	if step == "installing_gateway" {
		keys = append(keys, "gateway")
	}

	for _, key := range keys {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		raw, ok := cfg.YAMLOverrides[k]
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "apiVersion:") && strings.Contains(trimmed, "kind:") {
			return raw, true
		}
	}

	return "", false
}

func (o *Orchestrator) openBaoManifest(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: openbao
  namespace: %s
  labels:
    app.kubernetes.io/name: openbao
    app.kubernetes.io/instance: openbao
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: openbao
      app.kubernetes.io/instance: openbao
  template:
    metadata:
      labels:
        app.kubernetes.io/name: openbao
        app.kubernetes.io/instance: openbao
    spec:
      containers:
        - name: openbao
          image: openbao/openbao:latest
          imagePullPolicy: IfNotPresent
          args: ["server", "-dev", "-dev-root-token-id=root"]
          env:
            - name: VAULT_DEV_LISTEN_ADDRESS
              value: 0.0.0.0:8200
            - name: VAULT_UI
              value: "true"
          ports:
            - containerPort: 8200
              name: http
          readinessProbe:
            httpGet:
              path: /v1/sys/health?standbyok=true
              port: 8200
            initialDelaySeconds: 10
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: openbao
  namespace: %s
  labels:
    app.kubernetes.io/name: openbao
    app.kubernetes.io/instance: openbao
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: openbao
    app.kubernetes.io/instance: openbao
  ports:
    - name: http
      port: 8200
      targetPort: 8200
`, namespace, namespace)
}

func (o *Orchestrator) defaultGatewayBundleManifest(namespace string) string {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return ""
	}
	accessDomain := strings.TrimSpace(cfg.AccessDomain)
	if accessDomain == "" {
		return ""
	}
	stackLabel := strings.TrimSpace(strings.TrimSuffix(accessDomain, ".internal"))
	if stackLabel == "" {
		stackLabel = "nullus-stack"
	}

	gatewayName := fmt.Sprintf("%s-gateway", sanitizeK8sName(stackLabel))
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}

	manifests := []string{fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
spec:
  gatewayClassName: envoy
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      hostname: "*.%s"
      allowedRoutes:
        namespaces:
          from: Same
`, gatewayName, namespace, stackLabel, accessDomain)}

	type routeSpec struct {
		name    string
		host    string
		service string
		port    int
	}
	routes := make([]routeSpec, 0, 6)

	if cfg.Pipeline.CDTool.Enabled && (strings.EqualFold(cfg.Pipeline.CDTool.Name, "argocd") || strings.EqualFold(cfg.Pipeline.CDTool.Name, "argo-cd")) {
		routes = append(routes, routeSpec{name: "argocd-route", host: fmt.Sprintf("argocd.%s", accessDomain), service: "argo-cd-argocd-server", port: 80})
	}
	if cfg.Logging.Search.Enabled && strings.EqualFold(cfg.Logging.Search.Name, "opensearch") {
		routes = append(routes, routeSpec{name: "opensearch-route", host: fmt.Sprintf("opensearch.%s", accessDomain), service: "opensearch-cluster-master", port: 9200})
	}
	if cfg.Artifacts.SourceRepository.Enabled || cfg.Pipeline.CIPlatform.Enabled || cfg.Artifacts.PackageRegistry.Enabled || cfg.Artifacts.ContainerRegistry.Enabled {
		routes = append(routes, routeSpec{name: "gitlab-route", host: fmt.Sprintf("gitlab.%s", accessDomain), service: "gitlab-webservice-default", port: 8080})
	}
	if cfg.Monitoring.Visualization.Enabled && strings.EqualFold(cfg.Monitoring.Visualization.Name, "grafana") {
		routes = append(routes, routeSpec{name: "grafana-route", host: fmt.Sprintf("grafana.%s", accessDomain), service: "grafana", port: 80})
	}
	if cfg.Monitoring.Collection.Enabled && strings.EqualFold(cfg.Monitoring.Collection.Name, "prometheus") {
		routes = append(routes, routeSpec{name: "prometheus-route", host: fmt.Sprintf("prometheus.%s", accessDomain), service: "kube-prometheus-stack-prometheus", port: 9090})
	}
	if cfg.Artifacts.StorageBackend.Enabled && strings.EqualFold(cfg.Artifacts.StorageBackend.Name, "minio") {
		routes = append(routes, routeSpec{name: "minio-route", host: fmt.Sprintf("minio.%s", accessDomain), service: "nullus-minio-console", port: 9001})
	}
	if cfg.Authentication != nil && strings.EqualFold(strings.TrimSpace(cfg.Authentication.Provider), "openbao") {
		routes = append(routes, routeSpec{name: "openbao-route", host: fmt.Sprintf("openbao.%s", accessDomain), service: "openbao", port: 8200})
	}

	for _, route := range routes {
		manifests = append(manifests, fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
spec:
  parentRefs:
    - name: %s
  hostnames:
    - %s
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: %s
          port: %d
`, route.name, namespace, stackLabel, gatewayName, route.host, route.service, route.port))
	}

	return strings.Join(manifests, "\n---\n")
}

func sanitizeK8sName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, ".", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	parts := make([]rune, 0, len(normalized))
	lastDash := false
	for _, r := range normalized {
		isAlpha := r >= 'a' && r <= 'z'
		isNum := r >= '0' && r <= '9'
		if isAlpha || isNum {
			parts = append(parts, r)
			lastDash = false
			continue
		}
		if !lastDash {
			parts = append(parts, '-')
			lastDash = true
		}
	}
	out := strings.Trim(string(parts), "-")
	if out == "" {
		return "nullus-stack"
	}
	return out
}

func (o *Orchestrator) applyManifest(ctx context.Context, namespace, manifest string) error {
	if strings.TrimSpace(manifest) == "" {
		return nil
	}

	kubeconfigPath, err := o.writeKubeconfigTempFile()
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(kubeconfigPath)
	}()

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-n", namespace, "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (o *Orchestrator) writeKubeconfigTempFile() (string, error) {
	if len(o.kubeconfig) == 0 {
		return "", fmt.Errorf("kubeconfig is empty")
	}
	tmpFile, err := os.CreateTemp("", "nullus-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
	}()
	if _, err := tmpFile.Write(o.kubeconfig); err != nil {
		return "", fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	return tmpFile.Name(), nil
}

func (o *Orchestrator) runKubectl(ctx context.Context, args ...string) ([]byte, error) {
	kubeconfigPath, err := o.writeKubeconfigTempFile()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(kubeconfigPath)
	}()

	cmdArgs := append([]string{"--kubeconfig", kubeconfigPath}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("kubectl %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (o *Orchestrator) waitForKubectlGet(ctx context.Context, args ...string) error {
	const (
		maxAttempts = 60
		retryDelay  = 2 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if _, err := o.runKubectl(ctx, append([]string{"get"}, args...)...); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("kubectl get %s failed", strings.Join(args, " "))
	}
	return lastErr
}

func (o *Orchestrator) ensureGatewayAPICRDs(ctx context.Context) error {
	if _, err := o.runKubectl(ctx, "get", "crd", "gatewayclasses.gateway.networking.k8s.io"); err == nil {
		return nil
	}

	if _, err := o.runKubectl(ctx, "apply", "-f", gatewayAPIStandardInstallURL); err != nil {
		return err
	}
	if _, err := o.runKubectl(ctx, "get", "crd", "gatewayclasses.gateway.networking.k8s.io"); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) filterOptionalGatewayPolicies(ctx context.Context, manifest string) (string, bool, error) {
	if strings.TrimSpace(manifest) == "" {
		return manifest, false, nil
	}
	if _, err := o.runKubectl(ctx, "get", "crd", "backendtlspolicies.gateway.networking.k8s.io"); err == nil {
		return manifest, false, nil
	}
	return filterGatewayManifestDocuments(manifest, func(apiVersion, kind string) bool {
		return strings.HasPrefix(apiVersion, "gateway.networking.k8s.io/") && kind == "BackendTLSPolicy"
	})
}

func (o *Orchestrator) reconcileGatewayDataPlaneTLSSecret(ctx context.Context, namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	if err := o.waitForKubectlGet(ctx, "-n", namespace, "secret/"+defaultEnvoyControlPlaneSecret); err != nil {
		return err
	}

	caCRT, err := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "ca.crt")
	if err != nil {
		fallbackCA, fallbackErr := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "tls.crt")
		if fallbackErr != nil {
			return err
		}
		caCRT = fallbackCA
	}
	tlsCRT, err := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "tls.crt")
	if err != nil {
		return err
	}
	tlsKey, err := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "tls.key")
	if err != nil {
		return err
	}

	matches, err := o.secretDataMatches(ctx, namespace, defaultEnvoyDataPlaneTLSSecret, map[string]string{
		"ca.crt":  caCRT,
		"tls.crt": tlsCRT,
		"tls.key": tlsKey,
	})
	if err != nil {
		matches = false
	}

	secretManifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
  labels:
    control-plane: envoy-gateway
type: kubernetes.io/tls
data:
  ca.crt: %s
  tls.crt: %s
  tls.key: %s
`, defaultEnvoyDataPlaneTLSSecret, namespace, caCRT, tlsCRT, tlsKey)

	if err := o.applyManifest(ctx, namespace, secretManifest); err != nil {
		return err
	}

	if !matches {
		_, _ = o.runKubectl(ctx, "delete", "pod", "-n", namespace, "-l", "app.kubernetes.io/name=envoy", "--ignore-not-found=true")
	}

	return nil
}
func (o *Orchestrator) tryReconcileGatewayDataPlaneTLSSecret(ctx context.Context, namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	if _, err := o.runKubectl(ctx, "get", "secret", defaultEnvoyControlPlaneSecret, "-n", namespace, "-o", "name"); err != nil {
		return nil
	}
	return o.reconcileGatewayDataPlaneTLSSecret(ctx, namespace)
}
func (o *Orchestrator) secretDataField(ctx context.Context, namespace, secretName, key string) (string, error) {
	goTemplate := fmt.Sprintf("go-template={{ index .data %q }}", key)
	output, err := o.runKubectl(ctx, "get", "secret", secretName, "-n", namespace, "-o", goTemplate)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("secret %s/%s missing data key %s", namespace, secretName, key)
	}
	return value, nil
}

func (o *Orchestrator) secretDataMatches(ctx context.Context, namespace, secretName string, expected map[string]string) (bool, error) {
	if len(expected) == 0 {
		return true, nil
	}
	for key, expectedValue := range expected {
		actual, err := o.secretDataField(ctx, namespace, secretName, key)
		if err != nil {
			return false, err
		}
		if actual != strings.TrimSpace(expectedValue) {
			return false, nil
		}
	}
	return true, nil
}

func normalizeGatewayBackendServiceAliases(manifest string) (string, bool, error) {
	aliasByService := map[string]string{
		"grafana-svc":    "grafana",
		"prometheus-svc": "kube-prometheus-stack-prometheus",
	}

	decoder := yaml.NewDecoder(strings.NewReader(manifest))
	docs := make([]string, 0)
	normalizedAny := false

	for {
		var doc any
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return "", false, err
		}
		if doc == nil {
			continue
		}

		apiVersion := yamlDocumentStringField(doc, "apiVersion")
		kind := yamlDocumentStringField(doc, "kind")
		if strings.HasPrefix(apiVersion, "gateway.networking.k8s.io/") && kind == "HTTPRoute" {
			if normalizeHTTPRouteBackendRefs(doc, aliasByService) {
				normalizedAny = true
			}
		}

		encoded, err := yaml.Marshal(doc)
		if err != nil {
			return "", false, err
		}
		trimmed := strings.TrimSpace(string(encoded))
		if trimmed != "" {
			docs = append(docs, trimmed)
		}
	}

	return strings.Join(docs, "\n---\n"), normalizedAny, nil
}

func normalizeHTTPRouteBackendRefs(doc any, aliasByService map[string]string) bool {
	root, ok := doc.(map[string]any)
	if !ok {
		return false
	}
	spec, ok := root["spec"].(map[string]any)
	if !ok {
		return false
	}
	rules, ok := spec["rules"].([]any)
	if !ok {
		return false
	}

	normalized := false
	for _, rawRule := range rules {
		rule, ok := rawRule.(map[string]any)
		if !ok {
			continue
		}
		backendRefs, ok := rule["backendRefs"].([]any)
		if !ok {
			continue
		}
		for _, rawBackendRef := range backendRefs {
			backendRef, ok := rawBackendRef.(map[string]any)
			if !ok {
				continue
			}
			name, ok := backendRef["name"].(string)
			if !ok {
				continue
			}
			replacement, ok := aliasByService[strings.TrimSpace(name)]
			if !ok {
				continue
			}
			backendRef["name"] = replacement
			normalized = true
		}
	}

	return normalized
}

func filterGatewayManifestDocuments(manifest string, skip func(apiVersion, kind string) bool) (string, bool, error) {
	decoder := yaml.NewDecoder(strings.NewReader(manifest))
	kept := make([]string, 0)
	skippedAny := false

	for {
		var doc any
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return "", false, err
		}
		if doc == nil {
			continue
		}

		apiVersion := yamlDocumentStringField(doc, "apiVersion")
		kind := yamlDocumentStringField(doc, "kind")
		if skip != nil && skip(apiVersion, kind) {
			skippedAny = true
			continue
		}

		encoded, err := yaml.Marshal(doc)
		if err != nil {
			return "", false, err
		}
		trimmed := strings.TrimSpace(string(encoded))
		if trimmed != "" {
			kept = append(kept, trimmed)
		}
	}

	var buffer bytes.Buffer
	for index, doc := range kept {
		if index > 0 {
			buffer.WriteString("\n---\n")
		}
		buffer.WriteString(doc)
	}
	return buffer.String(), skippedAny, nil
}

func yamlDocumentStringField(doc any, key string) string {
	switch typed := doc.(type) {
	case map[string]any:
		if value, ok := typed[key].(string); ok {
			return strings.TrimSpace(value)
		}
	case map[any]any:
		for rawKey, rawValue := range typed {
			keyString, ok := rawKey.(string)
			if !ok || keyString != key {
				continue
			}
			if value, ok := rawValue.(string); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func (o *Orchestrator) valuesForStep(step string, spec ChartSpec) map[string]any {
	base := deepCopyMap(spec.Values)

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	base = mergeMaps(base, o.resourceDefaultValuesForStep(step, cfg))

	if cfg == nil || len(cfg.YAMLOverrides) == 0 {
		if step == "installing_minio" {
			namespace := strings.TrimSpace(o.namespace)
			if namespace == "" {
				namespace = "nullus"
			}
			base = mergeMaps(base, map[string]any{"namespace": namespace})
		}
		if step == "installing_postgresql" {
			base = mergeMaps(base, o.sharedPostgresValues(nil))
		}
		if step == "installing_gitlab" {
			base = mergeMaps(base, o.gitlabExternalSharedServiceValues(nil))
		}
		if step == stepInstallingRunner {
			namespace := strings.TrimSpace(o.namespace)
			if namespace == "" {
				namespace = "nullus"
			}
			base = mergeMaps(base, map[string]any{
				"gitlabUrl": fmt.Sprintf("http://gitlab-webservice-default.%s.svc:8181", namespace),
			})
		}
		if cfg != nil && step == "installing_gitlab" && strings.TrimSpace(cfg.AccessDomain) != "" {
			base = mergeMaps(base, map[string]any{
				"global": map[string]any{
					"hosts": map[string]any{
						"domain": cfg.AccessDomain,
					},
				},
			})
		}
		return base
	}

	if step == "installing_gitlab" && strings.TrimSpace(cfg.AccessDomain) != "" {
		base = mergeMaps(base, map[string]any{
			"global": map[string]any{
				"hosts": map[string]any{
					"domain": cfg.AccessDomain,
				},
			},
		})
	}

	if step == "installing_postgresql" {
		base = mergeMaps(base, o.sharedPostgresValues(cfg))
	}

	if step == "installing_minio" {
		namespace := strings.TrimSpace(o.namespace)
		if namespace == "" {
			namespace = "nullus"
		}
		base = mergeMaps(base, map[string]any{"namespace": namespace})
	}

	if step == "installing_gitlab" {
		base = mergeMaps(base, o.gitlabExternalSharedServiceValues(cfg))
	}

	if step == stepInstallingRunner {
		namespace := strings.TrimSpace(o.namespace)
		if namespace == "" {
			namespace = "nullus"
		}
		base = mergeMaps(base, map[string]any{
			"gitlabUrl": fmt.Sprintf("http://gitlab-webservice-default.%s.svc:8181", namespace),
		})
	}

	if step == "installing_gateway" {
		return base
	}

	keys := []string{step, o.releaseNameForSpec(spec), spec.ChartName, strings.TrimPrefix(step, "installing_")}
	for _, key := range keys {
		raw, ok := cfg.YAMLOverrides[key]
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}

		override, err := decodeValuesOverride(raw)
		if err != nil {
			slog.Warn("invalid yaml override skipped", "step", step, "key", key, "error", err)
			continue
		}
		override = normalizeLegacyResourceOverrideForStep(step, override)
		base = mergeMaps(base, override)
		break
	}

	return base
}

func (o *Orchestrator) resourceDefaultValuesForStep(step string, cfg *domain.StackConfig) map[string]any {
	resourceKey := o.resourceDefaultKeyForStep(step, cfg)
	if resourceKey == "" {
		return map[string]any{}
	}

	item := o.loadResourceDefault(resourceKey)
	if item == nil {
		return map[string]any{}
	}

	resources := toK8sResourceValues(item)
	if len(resources) == 0 {
		return map[string]any{}
	}

	scaled := func(ratio float64) map[string]any {
		return toK8sResourceValues(scaleResourceDefault(item, ratio))
	}
	webScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.12)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.4 {
			v.CPURequest = 0.4
		}
		if v.CPURequest > 1.0 {
			v.CPURequest = 1.0
		}
		if v.CPULimit < 0.8 {
			v.CPULimit = 0.8
		}
		if v.CPULimit > 2.0 {
			v.CPULimit = 2.0
		}
		if v.MemoryRequestGi < 1 {
			v.MemoryRequestGi = 1
		}
		if v.MemoryRequestGi > 2 {
			v.MemoryRequestGi = 2
		}
		if v.MemoryLimitGi < 2 {
			v.MemoryLimitGi = 2
		}
		if v.MemoryLimitGi > 4 {
			v.MemoryLimitGi = 4
		}
		return toK8sResourceValues(v)
	}
	sidekiqScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.10)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.35 {
			v.CPURequest = 0.35
		}
		if v.CPURequest > 0.8 {
			v.CPURequest = 0.8
		}
		if v.CPULimit < 0.7 {
			v.CPULimit = 0.7
		}
		if v.CPULimit > 1.6 {
			v.CPULimit = 1.6
		}
		if v.MemoryRequestGi < 1 {
			v.MemoryRequestGi = 1
		}
		if v.MemoryRequestGi > 1.5 {
			v.MemoryRequestGi = 1.5
		}
		if v.MemoryLimitGi < 2 {
			v.MemoryLimitGi = 2
		}
		if v.MemoryLimitGi > 3 {
			v.MemoryLimitGi = 3
		}
		return toK8sResourceValues(v)
	}
	redisMasterScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.06)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.2 {
			v.CPURequest = 0.2
		}
		if v.CPURequest > 0.5 {
			v.CPURequest = 0.5
		}
		if v.CPULimit < 0.4 {
			v.CPULimit = 0.4
		}
		if v.CPULimit > 1.0 {
			v.CPULimit = 1.0
		}
		if v.MemoryRequestGi < 0.5 {
			v.MemoryRequestGi = 0.5
		}
		if v.MemoryRequestGi > 1.0 {
			v.MemoryRequestGi = 1.0
		}
		if v.MemoryLimitGi < 1.0 {
			v.MemoryLimitGi = 1.0
		}
		if v.MemoryLimitGi > 2.0 {
			v.MemoryLimitGi = 2.0
		}
		return toK8sResourceValues(v)
	}
	toolboxScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.05)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.25 {
			v.CPURequest = 0.25
		}
		if v.CPURequest > 0.5 {
			v.CPURequest = 0.5
		}
		if v.CPULimit < 0.50 {
			v.CPULimit = 0.50
		}
		if v.CPULimit > 1.0 {
			v.CPULimit = 1.0
		}
		if v.MemoryRequestGi < 1 {
			v.MemoryRequestGi = 1
		}
		if v.MemoryRequestGi > 1.5 {
			v.MemoryRequestGi = 1.5
		}
		if v.MemoryLimitGi < 2 {
			v.MemoryLimitGi = 2
		}
		if v.MemoryLimitGi > 3 {
			v.MemoryLimitGi = 3
		}
		return toK8sResourceValues(v)
	}

	switch step {
	case stepInstallingCertManager:
		return map[string]any{
			"resources": resources,
			"webhook": map[string]any{
				"resources": resources,
			},
			"cainjector": map[string]any{
				"resources": resources,
			},
		}
	case "installing_minio":
		return map[string]any{
			"resources": resources,
		}
	case "installing_gitlab":
		return map[string]any{
			"gitlab": map[string]any{
				"webservice":      map[string]any{"resources": webScaled()},
				"sidekiq":         map[string]any{"resources": sidekiqScaled()},
				"toolbox":         map[string]any{"resources": toolboxScaled()},
				"gitaly":          map[string]any{"resources": scaled(0.20)},
				"kas":             map[string]any{"resources": scaled(0.12)},
				"gitlab-exporter": map[string]any{"resources": scaled(0.05)},
			},
			"registry": map[string]any{
				"resources": scaled(0.12),
			},
			"redis": map[string]any{
				"master": map[string]any{"resources": redisMasterScaled()},
			},
			"prometheus": map[string]any{
				"server": map[string]any{"resources": scaled(0.08)},
			},
		}
	case "installing_argocd":
		return map[string]any{
			"controller":     map[string]any{"resources": scaled(0.24)},
			"repoServer":     map[string]any{"resources": scaled(0.20)},
			"server":         map[string]any{"resources": scaled(0.20)},
			"redis":          map[string]any{"resources": scaled(0.12)},
			"dex":            map[string]any{"resources": scaled(0.10)},
			"applicationSet": map[string]any{"resources": scaled(0.07)},
			"notifications":  map[string]any{"resources": scaled(0.07)},
		}
	case stepInstallingRunner:
		return map[string]any{
			"resources": resources,
		}
	case "installing_prometheus":
		return map[string]any{
			"prometheus": map[string]any{
				"prometheusSpec": map[string]any{"resources": resources},
			},
			"alertmanager": map[string]any{
				"alertmanagerSpec": map[string]any{"resources": resources},
			},
			"kube-state-metrics":       map[string]any{"resources": resources},
			"prometheusOperator":       map[string]any{"resources": resources},
			"prometheus-node-exporter": map[string]any{"resources": resources},
		}
	case "installing_grafana":
		return map[string]any{
			"resources": resources,
		}
	case "installing_logging":
		return map[string]any{
			"resources":    resources,
			"loki":         map[string]any{"resources": resources},
			"singleBinary": map[string]any{"resources": resources},
			"read":         map[string]any{"resources": resources},
			"write":        map[string]any{"resources": resources},
			"backend":      map[string]any{"resources": resources},
			"promtail":     map[string]any{"resources": resources},
		}
	case "installing_log_search":
		return map[string]any{
			"resources": resources,
			"master":    map[string]any{"resources": resources},
		}
	case "installing_opentelemetry":
		traceName := ""
		if cfg != nil {
			traceName = strings.TrimSpace(strings.ToLower(cfg.Logging.TraceLayer.Name))
		}
		switch traceName {
		case "tempo":
			return map[string]any{
				"resources":  resources,
				"tempo":      map[string]any{"resources": resources},
				"tempoQuery": map[string]any{"resources": resources},
			}
		case "jaeger":
			return map[string]any{
				"resources": resources,
				"allInOne":  map[string]any{"resources": resources},
				"agent":     map[string]any{"resources": resources},
				"collector": map[string]any{"resources": resources},
				"query":     map[string]any{"resources": resources},
			}
		default:
			return map[string]any{
				"resources": resources,
			}
		}
	default:
		return map[string]any{}
	}
}

func (o *Orchestrator) resourceDefaultKeyForStep(step string, cfg *domain.StackConfig) string {
	switch step {
	case stepInstallingCertManager:
		return "cert-manager"
	case "installing_minio":
		return "minio"
	case "installing_gitlab":
		return "gitlab-ce"
	case "installing_argocd":
		return "argocd"
	case stepInstallingRunner:
		return "gitlab-runner"
	case "installing_prometheus":
		return "prometheus"
	case "installing_grafana":
		return "grafana"
	case "installing_logging":
		return "loki"
	case "installing_log_search":
		if cfg != nil {
			switch strings.TrimSpace(strings.ToLower(cfg.Logging.Search.Name)) {
			case "elasticsearch":
				return "elasticsearch"
			case "opensearch", "":
				return "opensearch"
			}
		}
		return "opensearch"
	case "installing_opentelemetry":
		if cfg != nil {
			switch strings.TrimSpace(strings.ToLower(cfg.Logging.TraceLayer.Name)) {
			case "tempo":
				return "tempo"
			case "jaeger":
				return "jaeger"
			}
		}
		return "opentelemetry"
	default:
		return ""
	}
}

func (o *Orchestrator) loadResourceDefault(key string) *domain.ResourceDefault {
	if strings.TrimSpace(key) == "" || o.resourceDefaultRepo == nil {
		return nil
	}

	o.mu.Lock()
	loaded := o.defaultsLoaded
	o.mu.Unlock()

	if !loaded {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		items, err := o.resourceDefaultRepo.List(ctx)
		if err != nil {
			slog.Warn("resource default load failed", "error", err)
		} else {
			loadedMap := make(map[string]*domain.ResourceDefault, len(items))
			for _, item := range items {
				if item == nil {
					continue
				}
				loadedMap[strings.ToLower(strings.TrimSpace(item.ToolKey))] = item
			}
			o.mu.Lock()
			o.resourceDefaults = loadedMap
			o.defaultsLoaded = true
			o.mu.Unlock()
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.resourceDefaults == nil {
		return nil
	}
	return o.resourceDefaults[strings.ToLower(strings.TrimSpace(key))]
}

func toK8sResourceValues(item *domain.ResourceDefault) map[string]any {
	if item == nil {
		return map[string]any{}
	}

	requests := map[string]any{}
	limits := map[string]any{}

	if v := cpuQuantity(item.CPURequest); v != "" {
		requests["cpu"] = v
	}
	if v := cpuQuantity(item.CPULimit); v != "" {
		limits["cpu"] = v
	}
	if v := memoryGiQuantity(item.MemoryRequestGi); v != "" {
		requests["memory"] = v
	}
	if v := memoryGiQuantity(item.MemoryLimitGi); v != "" {
		limits["memory"] = v
	}

	out := map[string]any{}
	if len(requests) > 0 {
		out["requests"] = requests
	}
	if len(limits) > 0 {
		out["limits"] = limits
	}
	return out
}

func scaleResourceDefault(item *domain.ResourceDefault, ratio float64) *domain.ResourceDefault {
	if item == nil {
		return nil
	}
	if ratio <= 0 {
		ratio = 1
	}
	round2 := func(v float64) float64 {
		return math.Round(v*100) / 100
	}
	scaled := *item
	scaled.CPURequest = round2(math.Max(0.05, item.CPURequest*ratio))
	scaled.CPULimit = round2(math.Max(0.10, item.CPULimit*ratio))
	scaled.MemoryRequestGi = round2(math.Max(0.08, item.MemoryRequestGi*ratio))
	scaled.MemoryLimitGi = round2(math.Max(0.16, item.MemoryLimitGi*ratio))
	scaled.StorageRequestGi = round2(math.Max(0, item.StorageRequestGi*ratio))
	scaled.StorageLimitGi = round2(math.Max(0, item.StorageLimitGi*ratio))
	return &scaled
}

func cpuQuantity(cores float64) string {
	if cores <= 0 {
		return ""
	}
	milli := int64(math.Round(cores * 1000))
	if milli <= 0 {
		return ""
	}
	if milli%1000 == 0 {
		return fmt.Sprintf("%d", milli/1000)
	}
	return fmt.Sprintf("%dm", milli)
}

func memoryGiQuantity(gi float64) string {
	if gi <= 0 {
		return ""
	}
	if math.Mod(gi, 1.0) == 0 {
		return fmt.Sprintf("%dGi", int64(gi))
	}
	return fmt.Sprintf("%gGi", gi)
}

func (o *Orchestrator) resolveChartSpecForStep(step string, spec ChartSpec) ChartSpec {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return spec
	}

	if step == "installing_log_search" {
		switch strings.TrimSpace(cfg.Logging.Search.Name) {
		case "opensearch":
			spec.ChartName = "opensearch"
			spec.RepoURL = "https://opensearch-project.github.io/helm-charts"
			spec.Version = "2.22.0"
			spec.Values = DefaultValues("installing_logging_opensearch")
		case "elasticsearch":
			spec.ChartName = "elasticsearch"
			spec.RepoURL = "https://helm.elastic.co"
			spec.Version = "8.5.1"
			spec.Values = DefaultValues("installing_logging_elasticsearch")
		default:
			spec.ChartName = "opensearch"
			spec.RepoURL = "https://opensearch-project.github.io/helm-charts"
			spec.Version = "2.22.0"
			spec.Values = DefaultValues("installing_logging_opensearch")
		}
	}

	if step == "installing_opentelemetry" {
		switch strings.TrimSpace(cfg.Logging.TraceLayer.Name) {
		case "tempo":
			spec.ChartName = "tempo"
			spec.RepoURL = "https://grafana.github.io/helm-charts"
			spec.Version = "1.18.1"
			spec.Values = DefaultValues("installing_tempo")
		case "jaeger":
			spec.ChartName = "jaeger"
			spec.RepoURL = "https://jaegertracing.github.io/helm-charts"
			spec.Version = "3.3.0"
			spec.Values = DefaultValues("installing_jaeger")
		default:
			spec.ChartName = "opentelemetry-collector"
			spec.RepoURL = "https://open-telemetry.github.io/opentelemetry-helm-charts"
			spec.Version = "0.75.0"
			spec.Values = DefaultValues("installing_opentelemetry")
		}
	}

	return spec
}

func (o *Orchestrator) releaseNameForSpec(spec ChartSpec) string {
	if strings.TrimSpace(spec.ReleaseName) != "" {
		return spec.ReleaseName
	}
	return spec.ChartName
}

func (o *Orchestrator) sharedPostgresValues(cfg *domain.StackConfig) map[string]any {
	storageGi := 20.0
	if cfg != nil && cfg.Storage != nil && cfg.Storage.Database.Size > 0 {
		storageGi = cfg.Storage.Database.Size
	}

	return map[string]any{
		"auth": map[string]any{
			"username":         "gitlab",
			"password":         "nullus-gitlab-password",
			"database":         "gitlabhq_production",
			"postgresPassword": "nullus-postgres-admin",
		},
		"primary": map[string]any{
			"persistence": map[string]any{
				"enabled": true,
				"size":    fmt.Sprintf("%gGi", storageGi),
			},
		},
	}
}

func (o *Orchestrator) gitlabExternalSharedServiceValues(_ *domain.StackConfig) map[string]any {
	namespace := strings.TrimSpace(o.namespace)
	if namespace == "" {
		namespace = "nullus"
	}

	return map[string]any{
		"postgresql": map[string]any{
			"install": false,
		},
		"global": map[string]any{
			"minio": map[string]any{
				"enabled": false,
			},
			"psql": map[string]any{
				"host":     fmt.Sprintf("nullus-postgresql.%s.svc.cluster.local", namespace),
				"port":     5432,
				"database": "gitlabhq_production",
				"username": "gitlab",
				"password": map[string]any{
					"useSecret": true,
					"secret":    "nullus-postgresql",
					"key":       "password",
				},
			},
			"appConfig": map[string]any{
				"object_store": map[string]any{
					"enabled": true,
					"connection": map[string]any{
						"secret": "nullus-object-storage",
						"key":    "connection",
					},
				},
			},
		},
		"gitlab": map[string]any{
			"toolbox": map[string]any{
				"backups": map[string]any{
					"objectStorage": map[string]any{
						"config": map[string]any{
							"secret": "nullus-object-storage",
							"key":    "config",
						},
					},
				},
			},
		},
	}
}

func (o *Orchestrator) sharedObjectStorageSecretManifest(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}

	endpoint := fmt.Sprintf("http://nullus-minio.%s.svc.cluster.local:9000", namespace)
	connection := fmt.Sprintf("provider: AWS\nregion: us-east-1\naws_access_key_id: nullus-admin\naws_secret_access_key: nullus-minio-secret\nendpoint: %s\npath_style: true\n", endpoint)

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: nullus-object-storage
  namespace: %s
type: Opaque
stringData:
  connection: |
%s
  config: |
%s
`, namespace, indentYAML(connection, 4), indentYAML(connection, 4))
}

func indentYAML(value string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	trimmed := strings.TrimRight(value, "\n")
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

func deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(src)
	if err != nil {
		return map[string]any{}
	}
	var copied map[string]any
	if err := json.Unmarshal(b, &copied); err != nil {
		return map[string]any{}
	}
	return copied
}

func decodeValuesOverride(raw string) (map[string]any, error) {
	var parsed any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	b, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("normalize yaml: %w", err)
	}

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("expected mapping yaml for helm values: %w", err)
	}

	if _, hasAPIVersion := out["apiVersion"]; hasAPIVersion {
		if _, hasKind := out["kind"]; hasKind {
			if converted, ok := resourceOverrideFromManifest(out); ok {
				return converted, nil
			}
			return nil, fmt.Errorf("manifest yaml is not supported for helm values override")
		}
	}

	return out, nil
}

func mergeMaps(base, override map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range override {
		subOverride, ok := value.(map[string]any)
		if !ok {
			base[key] = value
			continue
		}

		subBase, _ := base[key].(map[string]any)
		base[key] = mergeMaps(subBase, subOverride)
	}
	return base
}

func normalizeLegacyResourceOverrideForStep(step string, override map[string]any) map[string]any {
	if len(override) == 0 {
		return override
	}
	resources, ok := override["resources"].(map[string]any)
	if (!ok || len(resources) == 0) && step == "installing_logging" {
		resources = firstResourcesFromNestedLoggingOverride(override)
		if len(resources) > 0 {
			override = mergeMaps(map[string]any{"resources": resources}, override)
			ok = true
		}
	}
	if !ok || len(resources) == 0 {
		return override
	}

	switch step {
	case "installing_gitlab":
		return mergeMaps(map[string]any{
			"gitlab": map[string]any{
				"webservice":      map[string]any{"resources": resources},
				"sidekiq":         map[string]any{"resources": resources},
				"toolbox":         map[string]any{"resources": resources},
				"gitaly":          map[string]any{"resources": resources},
				"kas":             map[string]any{"resources": resources},
				"gitlab-exporter": map[string]any{"resources": resources},
			},
			"registry": map[string]any{"resources": resources},
			"redis":    map[string]any{"master": map[string]any{"resources": resources}},
			"prometheus": map[string]any{
				"server": map[string]any{"resources": resources},
			},
		}, override)
	case "installing_argocd":
		return mergeMaps(map[string]any{
			"controller":     map[string]any{"resources": resources},
			"repoServer":     map[string]any{"resources": resources},
			"server":         map[string]any{"resources": resources},
			"redis":          map[string]any{"resources": resources},
			"dex":            map[string]any{"resources": resources},
			"applicationSet": map[string]any{"resources": resources},
			"notifications":  map[string]any{"resources": resources},
		}, override)
	case "installing_prometheus":
		return mergeMaps(map[string]any{
			"prometheus":               map[string]any{"prometheusSpec": map[string]any{"resources": resources}},
			"alertmanager":             map[string]any{"alertmanagerSpec": map[string]any{"resources": resources}},
			"kube-state-metrics":       map[string]any{"resources": resources},
			"prometheusOperator":       map[string]any{"resources": resources},
			"prometheus-node-exporter": map[string]any{"resources": resources},
		}, override)
	case "installing_logging":
		return mergeMaps(map[string]any{
			"resources":    resources,
			"loki":         map[string]any{"resources": resources},
			"singleBinary": map[string]any{"resources": resources},
			"read":         map[string]any{"resources": resources},
			"write":        map[string]any{"resources": resources},
			"backend":      map[string]any{"resources": resources},
			"promtail":     map[string]any{"resources": resources},
		}, override)
	case "installing_log_search":
		return mergeMaps(map[string]any{
			"master": map[string]any{"resources": resources},
		}, override)
	default:
		return override
	}
}

func firstResourcesFromNestedLoggingOverride(override map[string]any) map[string]any {
	candidates := []string{"loki", "singleBinary", "read", "write", "backend", "promtail"}
	for _, key := range candidates {
		node, ok := override[key].(map[string]any)
		if !ok {
			continue
		}
		resources, ok := node["resources"].(map[string]any)
		if !ok || len(resources) == 0 {
			continue
		}
		return resources
	}
	return map[string]any{}
}

func resourceOverrideFromManifest(doc map[string]any) (map[string]any, bool) {
	if len(doc) == 0 {
		return nil, false
	}
	spec, ok := doc["spec"].(map[string]any)
	if !ok {
		return nil, false
	}

	if template, ok := spec["template"].(map[string]any); ok {
		if templateSpec, ok := template["spec"].(map[string]any); ok {
			spec = templateSpec
		}
	}

	containers, ok := spec["containers"].([]any)
	if !ok || len(containers) == 0 {
		return nil, false
	}

	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		resources, ok := containerMap["resources"].(map[string]any)
		if !ok || len(resources) == 0 {
			continue
		}
		return map[string]any{"resources": resources}, true
	}

	return nil, false
}

func (o *Orchestrator) isStepEnabled(step string) bool {
	if step == "integration_check" {
		return true
	}
	if o.sharedClusterScoped && isSharedClusterScopedStep(step) {
		return false
	}

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return true
	}

	enabledFn, ok := o.stepConfigEnabled[step]
	if !ok {
		return true
	}

	return enabledFn(*cfg)
}

func isSharedClusterScopedStep(step string) bool {
	return step == stepInstallingCertManager || step == "installing_metrics_server"
}

func (o *Orchestrator) ensureOrder(stackID, step string, order int) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if stackID == "" {
		return nil
	}
	current := o.progress[stackID]
	if _, ok := o.progress[stackID]; !ok {
		current = -1
	}
	if order != current+1 {
		expectedIdx := current + 1
		expected := ""
		if expectedIdx >= 0 && expectedIdx < len(o.orderedStep) {
			expected = o.orderedStep[expectedIdx]
		}
		return fmt.Errorf("out of order step %q for stack %s: expected %q", step, stackID, expected)
	}
	return nil
}

func (o *Orchestrator) markCompleted(stackID string, order int) {
	if stackID == "" {
		return
	}
	o.mu.Lock()
	o.progress[stackID] = order
	o.mu.Unlock()
}

func installOCIChartWithHelmCLI(ctx context.Context, kubeconfig []byte, releaseName, chartName, namespace, version string) error {
	if strings.TrimSpace(releaseName) == "" || strings.TrimSpace(chartName) == "" || strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("invalid helm cli install arguments")
	}
	if len(kubeconfig) == 0 {
		return fmt.Errorf("kubeconfig is empty")
	}
	tmpFile, err := os.CreateTemp("", "nullus-helm-kubeconfig-*.yaml")
	if err != nil {
		return fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err := tmpFile.Write(kubeconfig); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close kubeconfig temp file: %w", err)
	}

	args := []string{"upgrade", "--install", releaseName, chartName, "--namespace", namespace, "--create-namespace", "--skip-crds"}
	if strings.TrimSpace(version) != "" {
		args = append(args, "--version", version)
	}
	args = append(args, "--kubeconfig", tmpFile.Name())
	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm %s failed: %w (output=%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (o *Orchestrator) discoverGitLabRunnerRegistrationToken(ctx context.Context, namespace string) (string, error) {
	const (
		maxAttempts = 24
		retryDelay  = 10 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		token, err := o.discoverGitLabRunnerRegistrationTokenOnce(ctx, namespace)
		if err == nil {
			return token, nil
		}
		lastErr = err

		retryable := isRetryableRunnerTokenDiscoveryError(err)
		if !retryable || attempt == maxAttempts {
			return "", err
		}

		slog.Warn("gitlab runner token not ready yet; retrying",
			"namespace", namespace,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("runner token discovery failed")
	}
	return "", lastErr
}

func (o *Orchestrator) discoverGitLabRunnerRegistrationTokenOnce(ctx context.Context, namespace string) (string, error) {
	authTokenScript := `runner = Ci::Runner.where(description: "nullus-shared-runner", runner_type: :instance_type).order(id: :desc).first; runner ||= Ci::Runner.create!(description: "nullus-shared-runner", runner_type: :instance_type, run_untagged: true, locked: false); puts runner.token.to_s`
	if token, err := o.discoverGitLabRunnerTokenFromRailsRunner(ctx, namespace, authTokenScript); err == nil {
		return token, nil
	}

	legacyRegistrationTokenScript := `puts ApplicationSetting.current.runners_registration_token`
	if token, err := o.discoverGitLabRunnerTokenFromRailsRunner(ctx, namespace, legacyRegistrationTokenScript); err == nil {
		return token, nil
	}

	return "", fmt.Errorf("runner token not found in rails output")
}

func (o *Orchestrator) discoverGitLabRunnerTokenFromRailsRunner(ctx context.Context, namespace, script string) (string, error) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return "", fmt.Errorf("kubeconfig unavailable")
	}
	kubeconfigPath, err := o.writeKubeconfigTempFile()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(kubeconfigPath)
	}()

	args := []string{
		"--kubeconfig", kubeconfigPath,
		"-n", namespace,
		"exec", "deploy/gitlab-toolbox",
		"-c", "toolbox",
		"--", "bash", "-lc",
		fmt.Sprintf("gitlab-rails runner '%s'", script),
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl exec failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	token := parseGitLabRunnerRegistrationTokenOutput(string(output))
	if token == "" {
		return "", fmt.Errorf("runner token not found in output")
	}

	return token, nil
}

func isRetryableRunnerTokenDiscoveryError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	retryHints := []string{
		"container not found",
		"unable to upgrade connection",
		"does not have a host assigned",
		"pods \"gitlab-toolbox\" not found",
		"deployments.apps \"gitlab-toolbox\" not found",
		"no such host",
		"i/o timeout",
		"connection refused",
		"context deadline exceeded",
		"application_settings",
		"pg::undefinedtable",
	}

	for _, hint := range retryHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}

	return false
}

func parseGitLabRunnerRegistrationTokenOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	token := ""
	for _, line := range lines {
		candidate := strings.TrimSpace(line)
		if candidate == "" || strings.HasPrefix(candidate, "Defaulted container") || strings.Contains(candidate, " ") {
			continue
		}
		token = candidate
	}
	return token
}

func defaultEnvoyGatewayClassManifest() string {
	return `apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: envoy
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
`
}
