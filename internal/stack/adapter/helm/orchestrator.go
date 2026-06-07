package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

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
			if fallbackErr := installGatewayOCIRelease(ctx, o.kubeconfig, releaseName, spec.ChartName, namespace, spec.Version, nil, "", false); fallbackErr == nil {
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
			"installing_object_storage_secret":     {},
			"installing_object_storage_buckets":    {},
			"installing_database_connection_check": {},
			"installing_openbao":                   {},
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
				Version:     "1.4.3",
				Wait:        false,
			},
			"integration_check": {},
		},
		stepOrder: map[string]int{
			stepInstallingCertManager:              0,
			"installing_metrics_server":            1,
			"installing_postgresql":                2,
			"installing_minio":                     3,
			"installing_object_storage_secret":     4,
			"installing_object_storage_buckets":    5,
			"installing_database_connection_check": 6,
			"installing_openbao":                   7,
			"installing_gitlab":                    8,
			"installing_argocd":                    9,
			stepInstallingRunner:                   10,
			"installing_prometheus":                11,
			"installing_grafana":                   12,
			"installing_logging":                   13,
			"installing_log_search":                14,
			"installing_opentelemetry":             15,
			"installing_gateway":                   16,
			"integration_check":                    17,
		},
		orderedStep: []string{
			stepInstallingCertManager,
			"installing_metrics_server",
			"installing_postgresql",
			"installing_minio",
			"installing_object_storage_secret",
			"installing_object_storage_buckets",
			"installing_database_connection_check",
			"installing_openbao",
			"installing_gitlab",
			"installing_argocd",
			stepInstallingRunner,
			"installing_prometheus",
			"installing_grafana",
			"installing_logging",
			"installing_log_search",
			"installing_opentelemetry",
			"installing_gateway",
			"integration_check",
		},
		stepConfigFieldPath: map[string]string{
			"installing_postgresql":                "config.storage.database",
			"installing_minio":                     "config.artifacts.storage_backend",
			"installing_object_storage_secret":     "config.storage.object_storage",
			"installing_object_storage_buckets":    "config.storage.object_storage",
			"installing_database_connection_check": "config.storage.database",
			"installing_openbao":                   "config.authentication.provider",
			"installing_gitlab":                    "config.artifacts.source_repository",
			"installing_argocd":                    "config.pipeline.cd_tool",
			stepInstallingRunner:                   "config.pipeline.ci_platform",
			"installing_prometheus":                "config.monitoring.collection",
			"installing_grafana":                   "config.monitoring.visualization",
			"installing_logging":                   "config.logging.collection",
			"installing_log_search":                "config.logging.search",
			"installing_opentelemetry":             "config.logging.trace_layer",
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
				return cfg.Artifacts.StorageBackend.Enabled && isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository)
			},
			"installing_object_storage_buckets": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.StorageBackend.Enabled && isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository)
			},
			"installing_database_connection_check": func(cfg domain.StackConfig) bool {
				if !isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository) || cfg.Storage == nil {
					return false
				}
				return strings.TrimSpace(cfg.Storage.Database.Mode) == "existing-connect"
			},
			"installing_gitlab": func(cfg domain.StackConfig) bool {
				return isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository)
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
				return isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository) || isGitLabCISelection(cfg.Pipeline.CIPlatform)
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

func isGitLabSourceRepositorySelection(sel domain.ToolSelection) bool {
	if !sel.Enabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(sel.Version), "external") {
		return false
	}
	name := normalizeToolName(sel.Name)
	if name == "" {
		return true
	}
	return name == "gitlab" || name == "gitlab-ce"
}

func isGitLabCISelection(sel domain.ToolSelection) bool {
	if !sel.Enabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(sel.Version), "external") {
		return false
	}
	name := normalizeToolName(sel.Name)
	if name == "" {
		return true
	}
	return name == "gitlab-ci" || name == "gitlab-runner"
}

func normalizeToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
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

// ResumeFromStep initializes ordering for a new executor created during a
// continued deployment, so the failed step can be reapplied directly.
func (o *Orchestrator) ResumeFromStep(stackID, step string) {
	if strings.TrimSpace(stackID) == "" {
		return
	}
	order, ok := o.stepOrder[step]
	if !ok {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.progress[stackID] = order - 1
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

	if step == "installing_object_storage_buckets" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		if err := o.ensureGitLabObjectStorageBuckets(ctx, namespace); err != nil {
			return fmt.Errorf("ensure gitlab object storage buckets: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "installing_database_connection_check" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		if err := o.ensureGitLabDatabaseConnectivity(ctx, namespace); err != nil {
			return fmt.Errorf("ensure gitlab database connectivity: %w", err)
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
		if fallbackErr := installOCIChartWithHelmCLI(ctx, o.kubeconfig, releaseName, spec.ChartName, namespace, spec.Version, boolPtr(spec.Wait), "", false); fallbackErr == nil {
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

func (o *Orchestrator) isStepEnabled(step string) bool {
	if step == "integration_check" {
		return true
	}
	if step == "installing_object_storage_buckets" && !looksLikeKubeconfig(o.kubeconfig) {
		return false
	}
	if o.sharedClusterScoped && isSharedClusterScopedStep(step) {
		return false
	}

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return step != "installing_openbao"
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
	if stackID == "" {
		return nil
	}

	o.mu.Lock()
	current := o.progress[stackID]
	if _, ok := o.progress[stackID]; !ok {
		current = -1
	}
	o.mu.Unlock()

	for current+1 < order {
		nextIdx := current + 1
		if nextIdx < 0 || nextIdx >= len(o.orderedStep) {
			break
		}
		nextStep := o.orderedStep[nextIdx]
		if o.isStepEnabled(nextStep) {
			break
		}
		current = nextIdx
	}

	if order != current+1 {
		expectedIdx := current + 1
		expected := ""
		if expectedIdx >= 0 && expectedIdx < len(o.orderedStep) {
			expected = o.orderedStep[expectedIdx]
		}
		return fmt.Errorf("out of order step %q for stack %s: expected %q", step, stackID, expected)
	}

	o.mu.Lock()
	o.progress[stackID] = current
	o.mu.Unlock()
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
