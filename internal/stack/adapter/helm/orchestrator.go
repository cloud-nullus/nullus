package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"gopkg.in/yaml.v3"
)

const (
	defaultSelfSignedBootstrapIssuer = "nullus-selfsigned-bootstrap"
	defaultInternalCAIssuer          = "nullus-internal-ca-issuer"
	defaultInternalCASecretName      = "nullus-internal-ca"
	defaultInternalCACertName        = "nullus-internal-ca-cert"
	gatewayAPIStandardInstallURL     = "https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml"
)

var installGatewayOCIRelease = installOCIChartWithHelmCLI

type Orchestrator struct {
	installer           port.HelmInstaller
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
			return fmt.Errorf("status check failed for %s: %w", releaseName, err)
		}
		if status == nil {
			return fmt.Errorf("status check returned nil for %s", releaseName)
		}

		if !strings.EqualFold(status.Status, "deployed") {
			return fmt.Errorf("release %s is not healthy: status=%s", releaseName, status.Status)
		}
	}

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

func NewOrchestrator(installer port.HelmInstaller, kubeconfig []byte, namespace string) *Orchestrator {
	if namespace == "" {
		namespace = "nullus"
	}
	return &Orchestrator{
		installer:  installer,
		rollback:   &RollbackManager{},
		kubeconfig: kubeconfig,
		namespace:  namespace,
		chartConfig: map[string]ChartSpec{
			"installing_cert_manager": {
				ChartName: "cert-manager",
				RepoURL:   "https://charts.jetstack.io",
				Version:   "v1.16.3",
				Values:    DefaultValues("installing_cert_manager"),
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
			"installing_runner": {
				ChartName: "gitlab-runner",
				RepoURL:   "https://charts.gitlab.io/",
				Version:   "0.72.0",
				Values:    DefaultValues("installing_runner"),
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
			"installing_cert_manager":          0,
			"installing_postgresql":            1,
			"installing_minio":                 2,
			"installing_object_storage_secret": 3,
			"installing_gitlab":                4,
			"installing_argocd":                5,
			"installing_runner":                6,
			"installing_prometheus":            7,
			"installing_grafana":               8,
			"installing_logging":               9,
			"installing_log_search":            10,
			"installing_opentelemetry":         11,
			"installing_gateway":               12,
			"integration_check":                13,
		},
		orderedStep: []string{
			"installing_cert_manager",
			"installing_postgresql",
			"installing_minio",
			"installing_object_storage_secret",
			"installing_gitlab",
			"installing_argocd",
			"installing_runner",
			"installing_prometheus",
			"installing_grafana",
			"installing_logging",
			"installing_log_search",
			"installing_opentelemetry",
			"installing_gateway",
			"integration_check",
		},
		stepConfigFieldPath: map[string]string{
			"installing_postgresql":            "config.storage.database",
			"installing_minio":                 "config.artifacts.storage_backend",
			"installing_object_storage_secret": "config.storage.object_storage",
			"installing_gitlab":                "config.artifacts.source_repository",
			"installing_argocd":                "config.pipeline.cd_tool",
			"installing_runner":                "config.pipeline.ci_platform",
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
			"installing_argocd": func(cfg domain.StackConfig) bool {
				return cfg.Pipeline.CDTool.Enabled
			},
			"installing_runner": func(cfg domain.StackConfig) bool {
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
}

func (o *Orchestrator) SetNamespace(namespace string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if namespace == "" {
		namespace = "nullus"
	}
	o.namespace = namespace
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
		o.markCompleted(stackID, order)
		return nil
	}

	namespace := o.namespace
	if spec.Namespace != "" {
		namespace = spec.Namespace
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
	if step == "installing_gateway" && looksLikeKubeconfig(o.kubeconfig) {
		if err := o.ensureGatewayAPICRDs(ctx); err != nil {
			return fmt.Errorf("ensure gateway api crds: %w", err)
		}
	}
	releaseName := o.releaseNameForSpec(spec)
	values := o.valuesForStep(step, spec)
	if step == "installing_runner" {
		if looksLikeKubeconfig(o.kubeconfig) {
			runnerToken, tokenErr := o.discoverGitLabRunnerRegistrationToken(ctx, namespace)
			if tokenErr != nil {
				return fmt.Errorf("discover gitlab runner registration token: %w", tokenErr)
			}
			values = mergeMaps(values, map[string]any{
				"runnerRegistrationToken": runnerToken,
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
	if step == "installing_cert_manager" {
		if err := o.bootstrapInternalCA(ctx, namespace); err != nil {
			return fmt.Errorf("bootstrap internal ca: %w", err)
		}
	}
	if step == "installing_gateway" {
		if looksLikeKubeconfig(o.kubeconfig) {
			if err := o.applyManifest(ctx, namespace, defaultEnvoyGatewayClassManifest()); err != nil {
				return fmt.Errorf("apply default gatewayclass manifest: %w", err)
			}
		}
	}
	if step == "installing_gateway" && hasManifest {
		manifestNamespace := o.namespace
		if strings.TrimSpace(manifestNamespace) == "" {
			manifestNamespace = namespace
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

func (o *Orchestrator) internalCAManifest(namespace string) string {
	issuerName := defaultInternalCAIssuer
	secretName := defaultInternalCASecretName
	certName := defaultInternalCACertName

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	if cfg != nil && cfg.AccessDomainTLS != nil {
		if strings.TrimSpace(cfg.AccessDomainTLS.IssuerName) != "" {
			issuerName = cfg.AccessDomainTLS.IssuerName
		}
		if strings.TrimSpace(cfg.AccessDomainTLS.SecretName) != "" {
			secretName = cfg.AccessDomainTLS.SecretName
			certName = cfg.AccessDomainTLS.SecretName + "-cert"
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

func (o *Orchestrator) valuesForStep(step string, spec ChartSpec) map[string]any {
	base := deepCopyMap(spec.Values)

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

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
		if step == "installing_runner" {
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

	if step == "installing_runner" {
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
		base = mergeMaps(base, override)
		break
	}

	return base
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

func (o *Orchestrator) isStepEnabled(step string) bool {
	if step == "installing_cert_manager" || step == "integration_check" {
		return true
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
		lastErr = fmt.Errorf("runner registration token discovery failed")
	}
	return "", lastErr
}

func (o *Orchestrator) discoverGitLabRunnerRegistrationTokenOnce(ctx context.Context, namespace string) (string, error) {
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
		"gitlab-rails runner 'puts ApplicationSetting.current.runners_registration_token'",
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl exec failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	token := parseGitLabRunnerRegistrationTokenOutput(string(output))
	if token == "" {
		return "", fmt.Errorf("runner registration token not found in output")
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
