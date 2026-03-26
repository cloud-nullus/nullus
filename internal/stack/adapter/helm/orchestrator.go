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

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"gopkg.in/yaml.v3"
)

const (
	defaultSelfSignedBootstrapIssuer = "nullus-selfsigned-bootstrap"
	defaultInternalCAIssuer          = "nullus-internal-ca-issuer"
	defaultInternalCASecretName      = "nullus-internal-ca"
	defaultInternalCACertName        = "nullus-internal-ca-cert"
)

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
		if _, ok := o.monitoringManifestForStep(step); ok {
			continue
		}

		spec, ok := o.chartConfig[step]
		if !ok {
			return fmt.Errorf("chart config not found for step %s", step)
		}

		namespace := o.namespace
		if spec.Namespace != "" {
			namespace = spec.Namespace
		}

		status, err := o.installer.Status(ctx, spec.ChartName, namespace)
		if err != nil {
			return fmt.Errorf("status check failed for %s: %w", spec.ChartName, err)
		}
		if status == nil {
			return fmt.Errorf("status check returned nil for %s", spec.ChartName)
		}

		if !strings.EqualFold(status.Status, "deployed") {
			return fmt.Errorf("release %s is not healthy: status=%s", spec.ChartName, status.Status)
		}
	}

	return nil
}

type ChartSpec struct {
	ChartName string
	RepoURL   string
	Version   string
	Namespace string
	Values    map[string]any
	Wait      bool
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
			"installing_minio": {
				ChartName: "minio",
				RepoURL:   "https://charts.min.io/",
				Version:   "5.4.0",
				Values:    DefaultValues("installing_minio"),
				Wait:      false,
			},
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
				Version:   "2.10.2",
				Values:    DefaultValues("installing_logging"),
				Wait:      false,
			},
			"installing_opentelemetry": {
				ChartName: "opentelemetry-collector",
				RepoURL:   "https://open-telemetry.github.io/opentelemetry-helm-charts",
				Version:   "0.75.0",
				Values:    DefaultValues("installing_opentelemetry"),
				Wait:      false,
			},
			"integration_check": {},
		},
		stepOrder: map[string]int{
			"installing_cert_manager":  0,
			"installing_minio":         1,
			"installing_gitlab":        2,
			"installing_argocd":        3,
			"installing_runner":        4,
			"installing_prometheus":    5,
			"installing_grafana":       6,
			"installing_logging":       7,
			"installing_opentelemetry": 8,
			"integration_check":        9,
		},
		orderedStep: []string{
			"installing_cert_manager",
			"installing_minio",
			"installing_gitlab",
			"installing_argocd",
			"installing_runner",
			"installing_prometheus",
			"installing_grafana",
			"installing_logging",
			"installing_opentelemetry",
			"integration_check",
		},
		stepConfigFieldPath: map[string]string{
			"installing_minio":         "config.artifacts.storage_backend",
			"installing_gitlab":        "config.artifacts.source_repository",
			"installing_argocd":        "config.pipeline.cd_tool",
			"installing_runner":        "config.pipeline.ci_platform",
			"installing_prometheus":    "config.monitoring.collection",
			"installing_grafana":       "config.monitoring.visualization",
			"installing_logging":       "config.logging.search",
			"installing_opentelemetry": "config.logging.trace_layer",
		},
		stepConfigEnabled: map[string]func(domain.StackConfig) bool{
			"installing_minio": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.StorageBackend.Enabled
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
				return cfg.Logging.Search.Enabled
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

	spec = o.resolveChartSpecForStep(step, spec)

	if manifest, ok := o.monitoringManifestForStep(step); ok {
		if err := o.applyManifest(ctx, namespace, manifest); err != nil {
			return fmt.Errorf("apply yaml manifest for step %s: %w", step, err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	result, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: spec.ChartName,
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   namespace,
		Values:      o.valuesForStep(step, spec),
		Wait:        boolPtr(spec.Wait),
	})
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

func (o *Orchestrator) monitoringManifestForStep(step string) (string, bool) {
	if step != "installing_prometheus" && step != "installing_grafana" && step != "installing_logging" && step != "installing_opentelemetry" {
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
		keys = append(keys, "logging", cfg.Logging.Search.Name)
	}
	if step == "installing_opentelemetry" {
		keys = append(keys, "opentelemetry", "opentelemetry-collector", cfg.Logging.TraceLayer.Name)
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

func (o *Orchestrator) valuesForStep(step string, spec ChartSpec) map[string]any {
	base := deepCopyMap(spec.Values)

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	if cfg == nil || len(cfg.YAMLOverrides) == 0 {
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

	keys := []string{step, spec.ChartName, strings.TrimPrefix(step, "installing_")}
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

	if step == "installing_logging" {
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
			spec.ChartName = "loki"
			spec.RepoURL = "https://grafana.github.io/helm-charts"
			spec.Version = "2.10.2"
			spec.Values = DefaultValues("installing_logging")
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
