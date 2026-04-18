package helm

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_ImplementsStepExecutor(t *testing.T) {
	t.Parallel()
	var _ port.StepExecutor = &Orchestrator{}
}

type mockInstaller struct {
	installed       []string
	namespaces      []string
	valuesByRelease map[string]map[string]any
	waitByRelease   map[string]bool
	uninstalled     []string
	failOn          string
	failDelete      map[string]error
	statusByRelease map[string]string
	strictStatus    bool
}

type mockResourceDefaultRepo struct {
	items []*domain.ResourceDefault
}

func (m *mockResourceDefaultRepo) List(_ context.Context) ([]*domain.ResourceDefault, error) {
	return m.items, nil
}

func (m *mockResourceDefaultRepo) Upsert(_ context.Context, _ *domain.ResourceDefault) error {
	return nil
}

func (m *mockInstaller) Install(_ context.Context, req port.HelmInstallRequest) (*port.HelmInstallResult, error) {
	if req.ReleaseName == m.failOn {
		return nil, fmt.Errorf("install %s failed", req.ReleaseName)
	}
	m.installed = append(m.installed, req.ReleaseName)
	m.namespaces = append(m.namespaces, req.Namespace)
	if m.valuesByRelease == nil {
		m.valuesByRelease = make(map[string]map[string]any)
	}
	m.valuesByRelease[req.ReleaseName] = req.Values
	if m.waitByRelease == nil {
		m.waitByRelease = make(map[string]bool)
	}
	if req.Wait == nil {
		m.waitByRelease[req.ReleaseName] = true
	} else {
		m.waitByRelease[req.ReleaseName] = *req.Wait
	}
	return &port.HelmInstallResult{ReleaseName: req.ReleaseName, Namespace: req.Namespace, Status: "deployed", Revision: 1}, nil
}

func (m *mockInstaller) Uninstall(_ context.Context, releaseName, _ string) error {
	m.uninstalled = append(m.uninstalled, releaseName)
	if m.failDelete == nil {
		return nil
	}
	if err, ok := m.failDelete[releaseName]; ok {
		return err
	}
	return nil
}

func (m *mockInstaller) Status(_ context.Context, releaseName, namespace string) (*port.HelmInstallResult, error) {
	if m.strictStatus {
		found := false
		for _, installed := range m.installed {
			if installed == releaseName {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("release: not found")
		}
	}

	status := "deployed"
	if m.statusByRelease != nil {
		if s, ok := m.statusByRelease[releaseName]; ok {
			status = s
		}
	}
	return &port.HelmInstallResult{ReleaseName: releaseName, Namespace: namespace, Status: status, Revision: 1}, nil
}

func TestOrchestrator_ExecuteStep_InExpectedOrder(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_metrics_server", phase: "A"},
		{name: "installing_postgresql", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_object_storage_secret", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
		{name: "installing_logging", phase: "C"},
		{name: "installing_log_search", phase: "C"},
		{name: "installing_opentelemetry", phase: "C"},
		{name: "installing_gateway", phase: "C"},
	}

	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_1", step.name, step.phase))
	}

	assert.Equal(t, []string{
		"cert-manager",
		"metrics-server",
		"nullus-postgresql",
		"nullus-minio",
		"gitlab",
		"argo-cd",
		"gitlab-runner",
		"kube-prometheus-stack",
		"grafana",
		"loki",
		"opensearch",
		"opentelemetry-collector",
		"eg",
	}, installer.installed)
}

func TestOrchestrator_ApplyResourceDefaultsForArgoCDAndRunner(t *testing.T) {
	installer := &mockInstaller{}
	resourceRepo := &mockResourceDefaultRepo{items: []*domain.ResourceDefault{
		{
			ToolKey:         "argocd",
			CPURequest:      1,
			CPULimit:        2,
			MemoryRequestGi: 2,
			MemoryLimitGi:   4,
		},
		{
			ToolKey:         "gitlab-runner",
			CPURequest:      2,
			CPULimit:        4,
			MemoryRequestGi: 4,
			MemoryLimitGi:   8,
		},
	}}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus", WithResourceDefaultRepository(resourceRepo))

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_metrics_server", phase: "A"},
		{name: "installing_postgresql", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_object_storage_secret", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
	}

	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_resource_defaults", step.name, step.phase))
	}

	argocdValues := installer.valuesByRelease["argo-cd"]
	require.NotNil(t, argocdValues)

	server, ok := argocdValues["server"].(map[string]any)
	require.True(t, ok)
	serverResources, ok := server["resources"].(map[string]any)
	require.True(t, ok)

	requests, ok := serverResources["requests"].(map[string]any)
	require.True(t, ok)
	limits, ok := serverResources["limits"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "200m", requests["cpu"])
	assert.Equal(t, "0.4Gi", requests["memory"])
	assert.Equal(t, "400m", limits["cpu"])
	assert.Equal(t, "0.8Gi", limits["memory"])

	runnerValues := installer.valuesByRelease["gitlab-runner"]
	require.NotNil(t, runnerValues)

	runnerResources, ok := runnerValues["resources"].(map[string]any)
	require.True(t, ok)
	runnerRequests, ok := runnerResources["requests"].(map[string]any)
	require.True(t, ok)
	runnerLimits, ok := runnerResources["limits"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "2", runnerRequests["cpu"])
	assert.Equal(t, "4Gi", runnerRequests["memory"])
	assert.Equal(t, "4", runnerLimits["cpu"])
	assert.Equal(t, "8Gi", runnerLimits["memory"])
}

func TestOrchestrator_ApplyResourceDefaultsForGitLab_ClampsWebserviceAndSidekiqForStartup(t *testing.T) {
	installer := &mockInstaller{}
	resourceRepo := &mockResourceDefaultRepo{items: []*domain.ResourceDefault{{
		ToolKey:         "gitlab-ce",
		CPURequest:      8.8,
		CPULimit:        17.6,
		MemoryRequestGi: 19.2,
		MemoryLimitGi:   38.4,
	}}}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus", WithResourceDefaultRepository(resourceRepo))
	orch.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{SourceRepository: domain.ToolSelection{Enabled: true}},
	})

	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_gitlab_resource_clamp", "installing_cert_manager", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_gitlab_resource_clamp", "installing_metrics_server", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_gitlab_resource_clamp", "installing_postgresql", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_gitlab_resource_clamp", "installing_minio", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_gitlab_resource_clamp", "installing_object_storage_secret", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_gitlab_resource_clamp", "installing_gitlab", "B"))

	gitlabValues := installer.valuesByRelease["gitlab"]
	require.NotNil(t, gitlabValues)

	gitlabMap, ok := gitlabValues["gitlab"].(map[string]any)
	require.True(t, ok)

	webservice, ok := gitlabMap["webservice"].(map[string]any)
	require.True(t, ok)
	webResources, ok := webservice["resources"].(map[string]any)
	require.True(t, ok)
	webReq, ok := webResources["requests"].(map[string]any)
	require.True(t, ok)
	webLim, ok := webResources["limits"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1", webReq["cpu"])
	assert.Equal(t, "2Gi", webReq["memory"])
	assert.Equal(t, "2", webLim["cpu"])
	assert.Equal(t, "4Gi", webLim["memory"])

	sidekiq, ok := gitlabMap["sidekiq"].(map[string]any)
	require.True(t, ok)
	sidekiqResources, ok := sidekiq["resources"].(map[string]any)
	require.True(t, ok)
	sidekiqReq, ok := sidekiqResources["requests"].(map[string]any)
	require.True(t, ok)
	sidekiqLim, ok := sidekiqResources["limits"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "800m", sidekiqReq["cpu"])
	assert.Equal(t, "1.5Gi", sidekiqReq["memory"])
	assert.Equal(t, "1600m", sidekiqLim["cpu"])
	assert.Equal(t, "3Gi", sidekiqLim["memory"])

	redis, ok := gitlabValues["redis"].(map[string]any)
	require.True(t, ok)
	redisMaster, ok := redis["master"].(map[string]any)
	require.True(t, ok)
	redisResources, ok := redisMaster["resources"].(map[string]any)
	require.True(t, ok)
	redisReq, ok := redisResources["requests"].(map[string]any)
	require.True(t, ok)
	redisLim, ok := redisResources["limits"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "500m", redisReq["cpu"])
	assert.Equal(t, "1Gi", redisReq["memory"])
	assert.Equal(t, "1", redisLim["cpu"])
	assert.Equal(t, "2Gi", redisLim["memory"])
}

func TestOrchestrator_ExecuteStep_UnknownStepReturnsError(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	err := orch.ExecuteStep(context.Background(), "stk_1", "installing_unknown", "A")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown step")
	assert.Empty(t, installer.installed)
}

func TestOrchestrator_ExecuteStep_IntegrationCheckNoOp(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	require.NoError(t, orch.ExecuteStep(context.Background(), "", "integration_check", "C"))
	assert.Empty(t, installer.installed)
}

func TestOrchestrator_ExecuteStep_OutOfOrderReturnsError(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	err := orch.ExecuteStep(context.Background(), "stk_1", "installing_gitlab", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of order")
	assert.Empty(t, installer.installed)
}

func TestOrchestrator_SetNamespace_OverridesDefaultNamespace(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetNamespace("production")

	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_1", "installing_cert_manager", "A"))

	assert.Equal(t, []string{"production"}, installer.namespaces)
}

func TestOrchestrator_ExecuteStep_ReusesExistingCertManagerInstallation(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("apiVersion: v1\nclusters:\n- name: test\n"), "nullus")

	originalCheck := checkExistingCertManagerInstallation
	originalBootstrap := bootstrapInternalCAInstallation
	checkExistingCertManagerInstallation = func(_ context.Context, _ *Orchestrator) (bool, error) {
		return true, nil
	}
	bootstrapInternalCAInstallation = func(_ context.Context, _ *Orchestrator, _ string) error {
		return nil
	}
	defer func() {
		checkExistingCertManagerInstallation = originalCheck
		bootstrapInternalCAInstallation = originalBootstrap
	}()

	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_reuse_cert_manager", "installing_cert_manager", "A"))
	assert.Empty(t, installer.installed)
}

func TestOrchestrator_VerifyDeployment_Success(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_metrics_server", phase: "A"},
		{name: "installing_postgresql", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_object_storage_secret", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
		{name: "installing_logging", phase: "C"},
		{name: "installing_log_search", phase: "C"},
		{name: "installing_opentelemetry", phase: "C"},
		{name: "installing_gateway", phase: "C"},
	}
	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_verify_ok", step.name, step.phase))
	}

	require.NoError(t, orch.VerifyDeployment(context.Background(), "stk_verify_ok"))
}

func TestOrchestrator_VerifyDeployment_FailsWhenReleaseNotHealthy(t *testing.T) {
	installer := &mockInstaller{statusByRelease: map[string]string{"gitlab": "pending-install"}}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_metrics_server", phase: "A"},
		{name: "installing_postgresql", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_object_storage_secret", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
		{name: "installing_logging", phase: "C"},
		{name: "installing_log_search", phase: "C"},
		{name: "installing_opentelemetry", phase: "C"},
		{name: "installing_gateway", phase: "C"},
	}
	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_verify_fail", step.name, step.phase))
	}

	err := orch.VerifyDeployment(context.Background(), "stk_verify_fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not healthy")
}

func TestOrchestrator_VerifyDeployment_RepairsMissingGatewayRelease(t *testing.T) {
	installer := &mockInstaller{strictStatus: true}
	installer.installed = []string{
		"cert-manager",
		"metrics-server",
		"nullus-postgresql",
		"nullus-minio",
		"gitlab",
		"argo-cd",
		"gitlab-runner",
		"kube-prometheus-stack",
		"grafana",
		"loki",
		"opensearch",
		"opentelemetry-collector",
	}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	originalInstallGatewayOCIRelease := installGatewayOCIRelease
	installGatewayOCIRelease = func(_ context.Context, _ []byte, releaseName, _, _, _ string) error {
		installer.installed = append(installer.installed, releaseName)
		return nil
	}
	t.Cleanup(func() {
		installGatewayOCIRelease = originalInstallGatewayOCIRelease
	})

	require.NoError(t, orch.VerifyDeployment(context.Background(), "stk_verify_gateway_repair"))
	assert.Contains(t, installer.installed, "eg")
}

func TestOrchestrator_ExecuteStep_AppliesYAMLOverrideByChartName(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{SourceRepository: domain.ToolSelection{Enabled: true}},
		YAMLOverrides: map[string]string{
			"gitlab": "global:\n  hosts:\n    domain: custom.internal\n",
		},
	})

	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_override", "installing_cert_manager", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_override", "installing_metrics_server", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_override", "installing_postgresql", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_override", "installing_minio", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_override", "installing_object_storage_secret", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_override", "installing_gitlab", "B"))

	gitlabValues := installer.valuesByRelease["gitlab"]
	require.NotNil(t, gitlabValues)
	global, ok := gitlabValues["global"].(map[string]any)
	require.True(t, ok)
	hosts, ok := global["hosts"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "custom.internal", hosts["domain"])
	assert.False(t, installer.waitByRelease["gitlab"])
}

func TestOrchestrator_ExecuteStep_AppliesAccessDomainToGitLabValues(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		AccessDomain: "template-domain.internal",
		Artifacts:    domain.ArtifactsConfig{SourceRepository: domain.ToolSelection{Enabled: true}},
	})

	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_access_domain", "installing_cert_manager", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_access_domain", "installing_metrics_server", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_access_domain", "installing_postgresql", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_access_domain", "installing_minio", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_access_domain", "installing_object_storage_secret", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_access_domain", "installing_gitlab", "B"))

	gitlabValues := installer.valuesByRelease["gitlab"]
	require.NotNil(t, gitlabValues)
	global, ok := gitlabValues["global"].(map[string]any)
	require.True(t, ok)
	hosts, ok := global["hosts"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "template-domain.internal", hosts["domain"])
}

func TestOrchestrator_ExecuteStep_SkipsManifestOverride(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{SourceRepository: domain.ToolSelection{Enabled: true}},
		YAMLOverrides: map[string]string{
			"gitlab": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: wrong-shape\n",
		},
	})

	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_manifest", "installing_cert_manager", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_manifest", "installing_metrics_server", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_manifest", "installing_postgresql", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_manifest", "installing_minio", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_manifest", "installing_object_storage_secret", "A"))
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_manifest", "installing_gitlab", "B"))

	gitlabValues := installer.valuesByRelease["gitlab"]
	require.NotNil(t, gitlabValues)
	global, ok := gitlabValues["global"].(map[string]any)
	require.True(t, ok)
	hosts, ok := global["hosts"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nullus.internal", hosts["domain"])
}

func TestOrchestrator_MonitoringManifestForStep_ReturnsPrometheusYAML(t *testing.T) {
	orch := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Name: "prometheus", Enabled: true},
			Visualization: domain.ToolSelection{Name: "grafana", Enabled: true},
		},
		YAMLOverrides: map[string]string{
			"prometheus": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: prom\n",
		},
	})

	manifest, ok := orch.stepManifestForStep("installing_prometheus")
	require.True(t, ok)
	assert.Contains(t, manifest, "kind: Deployment")
}

func TestOrchestrator_MonitoringManifestForStep_IgnoresValuesYAML(t *testing.T) {
	orch := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Monitoring: domain.MonitoringConfig{
			Collection: domain.ToolSelection{Name: "prometheus", Enabled: true},
		},
		YAMLOverrides: map[string]string{
			"prometheus": "global:\n  hosts:\n    domain: example.internal\n",
		},
	})

	_, ok := orch.stepManifestForStep("installing_prometheus")
	assert.False(t, ok)
}

func TestOrchestrator_VerifyDeployment_SkipsHelmStatusForYAMLMonitoring(t *testing.T) {
	installer := &mockInstaller{statusByRelease: map[string]string{"cert-manager": "deployed"}}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			StorageBackend:    domain.ToolSelection{Enabled: false},
			SourceRepository:  domain.ToolSelection{Enabled: false},
			ContainerRegistry: domain.ToolSelection{Enabled: false},
			PackageRegistry:   domain.ToolSelection{Enabled: false},
		},
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Enabled: false},
			CDTool:     domain.ToolSelection{Enabled: false},
		},
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Name: "prometheus", Enabled: true},
			Visualization: domain.ToolSelection{Name: "grafana", Enabled: true},
		},
		YAMLOverrides: map[string]string{
			"prometheus": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: prometheus-yaml\n",
			"grafana":    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: grafana-yaml\n",
		},
	})

	require.NoError(t, orch.VerifyDeployment(context.Background(), "stk_yaml_monitoring"))
}

func TestLooksLikeKubeconfig(t *testing.T) {
	assert.True(t, looksLikeKubeconfig([]byte("apiVersion: v1\nclusters:\n- name: kind\n")))
	assert.False(t, looksLikeKubeconfig([]byte("kubeconfig")))
	assert.False(t, looksLikeKubeconfig(nil))
}

func TestOrchestrator_InternalCAManifest_Defaults(t *testing.T) {
	orch := NewOrchestrator(&mockInstaller{}, []byte("apiVersion: v1\nclusters:\n- name: kind\n"), "nullus")
	manifest := orch.internalCAManifest("nullus")

	assert.Contains(t, manifest, "kind: ClusterIssuer")
	assert.Contains(t, manifest, "name: nullus-selfsigned-bootstrap")
	assert.Contains(t, manifest, "name: nullus-internal-ca-issuer")
	assert.Contains(t, manifest, "secretName: nullus-internal-ca")
	assert.Contains(t, manifest, "namespace: nullus")
}

func TestOrchestrator_InternalCAManifest_UsesTLSOverrides(t *testing.T) {
	orch := NewOrchestrator(&mockInstaller{}, []byte("apiVersion: v1\nclusters:\n- name: kind\n"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		AccessDomainTLS: &domain.AccessDomainTLSConfig{
			Enabled:    true,
			IssuerName: "corp-offline-ca",
			SecretName: "corp-ca-secret",
		},
	})

	manifest := orch.internalCAManifest("nullus")
	assert.Contains(t, manifest, "name: corp-offline-ca")
	assert.Contains(t, manifest, "secretName: corp-ca-secret")
	assert.Contains(t, manifest, "name: corp-ca-secret-cert")
}

func TestOrchestrator_ExecuteStep_UsesOpensearchForLoggingSearch(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			StorageBackend:   domain.ToolSelection{Enabled: true},
			SourceRepository: domain.ToolSelection{Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Enabled: true},
			CDTool:     domain.ToolSelection{Enabled: true},
		},
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Enabled: true},
			Visualization: domain.ToolSelection{Enabled: true},
		},
		Logging: domain.LoggingConfig{
			Search:     domain.ToolSelection{Name: "opensearch", Enabled: true},
			TraceLayer: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_metrics_server", phase: "A"},
		{name: "installing_postgresql", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_object_storage_secret", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
		{name: "installing_logging", phase: "C"},
		{name: "installing_log_search", phase: "C"},
		{name: "installing_opentelemetry", phase: "C"},
		{name: "installing_gateway", phase: "C"},
	}

	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_logging", step.name, step.phase))
	}

	assert.Contains(t, installer.installed, "opensearch")
	assert.Contains(t, installer.installed, "opentelemetry-collector")
}

func TestOrchestrator_VerifyDeployment_UsesResolvedChartsForLoggingAndTrace(t *testing.T) {
	installer := &mockInstaller{strictStatus: true}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{SourceRepository: domain.ToolSelection{Enabled: true}},
		Pipeline:  domain.PipelineConfig{CDTool: domain.ToolSelection{Enabled: true}},
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Enabled: true},
			Visualization: domain.ToolSelection{Enabled: true},
		},
		Logging: domain.LoggingConfig{
			Search:     domain.ToolSelection{Name: "opensearch", Enabled: true},
			TraceLayer: domain.ToolSelection{Name: "tempo", Enabled: true},
		},
	})

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_metrics_server", phase: "A"},
		{name: "installing_postgresql", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_object_storage_secret", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
		{name: "installing_logging", phase: "C"},
		{name: "installing_log_search", phase: "C"},
		{name: "installing_opentelemetry", phase: "C"},
		{name: "installing_gateway", phase: "C"},
	}

	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_verify_logging_trace", step.name, step.phase))
	}

	require.NoError(t, orch.VerifyDeployment(context.Background(), "stk_verify_logging_trace"))
}

func TestOrchestrator_ExecuteStep_AppliesRunnerGitlabURL(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			StorageBackend:   domain.ToolSelection{Enabled: true},
			SourceRepository: domain.ToolSelection{Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Enabled: true},
			CDTool:     domain.ToolSelection{Enabled: true},
		},
	})

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_metrics_server", phase: "A"},
		{name: "installing_postgresql", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_object_storage_secret", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
	}

	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_runner_url", step.name, step.phase))
	}

	runnerValues := installer.valuesByRelease["gitlab-runner"]
	require.NotNil(t, runnerValues)
	assert.Equal(t, "http://gitlab-webservice-default.nullus.svc:8181", runnerValues["gitlabUrl"])
}

func TestOrchestrator_DefaultGatewayBundleManifest_IncludesEnabledOSSRoutes(t *testing.T) {
	orch := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus")
	orch.SetStackConfig(domain.StackConfig{
		AccessDomain: "nullus-devsecops-stack.internal",
		Artifacts: domain.ArtifactsConfig{
			SourceRepository: domain.ToolSelection{Name: "gitlab", Enabled: true},
			StorageBackend:   domain.ToolSelection{Name: "minio", Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CDTool: domain.ToolSelection{Name: "argocd", Enabled: true},
		},
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Name: "prometheus", Enabled: true},
			Visualization: domain.ToolSelection{Name: "grafana", Enabled: true},
		},
		Logging: domain.LoggingConfig{
			Search: domain.ToolSelection{Name: "opensearch", Enabled: true},
		},
	})

	manifest := orch.defaultGatewayBundleManifest("nullus")
	require.NotEmpty(t, manifest)
	assert.Contains(t, manifest, "kind: Gateway")
	assert.Contains(t, manifest, "name: nullus-devsecops-stack-gateway")
	assert.Contains(t, manifest, "argocd.nullus-devsecops-stack.internal")
	assert.Contains(t, manifest, "opensearch.nullus-devsecops-stack.internal")
	assert.Contains(t, manifest, "gitlab.nullus-devsecops-stack.internal")
	assert.Contains(t, manifest, "grafana.nullus-devsecops-stack.internal")
	assert.Contains(t, manifest, "prometheus.nullus-devsecops-stack.internal")
	assert.Contains(t, manifest, "minio.nullus-devsecops-stack.internal")
	assert.Contains(t, manifest, "name: argo-cd-argocd-server")
	assert.Contains(t, manifest, "name: opensearch-cluster-master")
	assert.Contains(t, manifest, "name: gitlab-webservice-default")
	assert.Contains(t, manifest, "name: kube-prometheus-stack-prometheus")
	assert.Contains(t, manifest, "name: nullus-minio-console")
}

func TestFilterGatewayManifestDocuments_SkipsBackendTLSPolicy(t *testing.T) {
	manifest := `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: sample-gateway
---
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: opensearch-backend-tls
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: sample-route
`

	filtered, skipped, err := filterGatewayManifestDocuments(manifest, func(apiVersion, kind string) bool {
		return strings.HasPrefix(apiVersion, "gateway.networking.k8s.io/") && kind == "BackendTLSPolicy"
	})
	require.NoError(t, err)
	assert.True(t, skipped)
	assert.NotContains(t, filtered, "kind: BackendTLSPolicy")
	assert.Contains(t, filtered, "kind: Gateway")
	assert.Contains(t, filtered, "kind: HTTPRoute")
}

func TestFilterGatewayManifestDocuments_LeavesManifestUntouchedWhenNoOptionalPolicyExists(t *testing.T) {
	manifest := `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: sample-gateway
`

	filtered, skipped, err := filterGatewayManifestDocuments(manifest, func(apiVersion, kind string) bool {
		return strings.HasPrefix(apiVersion, "gateway.networking.k8s.io/") && kind == "BackendTLSPolicy"
	})
	require.NoError(t, err)
	assert.False(t, skipped)
	assert.Contains(t, filtered, "kind: Gateway")
	assert.True(t, bytes.Contains([]byte(filtered), []byte("sample-gateway")))
}

func TestParseGitLabRunnerRegistrationTokenOutput(t *testing.T) {
	output := "Defaulted container \"toolbox\" out of: toolbox, certificates (init), configure (init)\n2QEqK1k4dNXwreEMDL9JhXTTNeyG6VK6M2g1U10jRAhMBHwI5HqaCFTeEzby0r0C\n"
	token := parseGitLabRunnerRegistrationTokenOutput(output)
	assert.Equal(t, "2QEqK1k4dNXwreEMDL9JhXTTNeyG6VK6M2g1U10jRAhMBHwI5HqaCFTeEzby0r0C", token)
}

func TestParseGitLabRunnerRegistrationTokenOutput_Empty(t *testing.T) {
	assert.Equal(t, "", parseGitLabRunnerRegistrationTokenOutput("\nDefaulted container \"toolbox\"\n"))
}

func TestIsRetryableRunnerTokenDiscoveryError_TrueCases(t *testing.T) {
	cases := []error{
		fmt.Errorf("kubectl exec failed: unable to upgrade connection: container not found (\"toolbox\")"),
		fmt.Errorf("PG::UndefinedTable: ERROR: relation \"application_settings\" does not exist"),
		fmt.Errorf("connect: connection refused"),
		fmt.Errorf("kubectl exec failed: Error from server (BadRequest): pod gitlab-toolbox-abc123 does not have a host assigned"),
	}

	for _, tc := range cases {
		assert.True(t, isRetryableRunnerTokenDiscoveryError(tc))
	}
}

func TestIsRetryableRunnerTokenDiscoveryError_FalseCase(t *testing.T) {
	err := fmt.Errorf("runner registration token not found in output")
	assert.False(t, isRetryableRunnerTokenDiscoveryError(err))
}

func TestNormalizeLegacyResourceOverrideForStep_LoggingAddsRootResourcesFromNested(t *testing.T) {
	override := map[string]any{
		"loki": map[string]any{
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "350m", "memory": "0.7Gi"},
				"limits":   map[string]any{"cpu": "700m", "memory": "1.4Gi"},
			},
		},
	}

	normalized := normalizeLegacyResourceOverrideForStep("installing_logging", override)
	resources, ok := normalized["resources"].(map[string]any)
	assert.True(t, ok)
	requests, ok := resources["requests"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "350m", requests["cpu"])
	assert.Equal(t, "0.7Gi", requests["memory"])
}
