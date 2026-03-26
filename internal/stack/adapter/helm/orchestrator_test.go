package helm

import (
	"context"
	"fmt"
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
		{name: "installing_minio", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
	}

	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_1", step.name, step.phase))
	}

	assert.Equal(t, []string{
		"cert-manager",
		"minio",
		"gitlab",
		"argo-cd",
		"gitlab-runner",
		"kube-prometheus-stack",
		"grafana",
	}, installer.installed)
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

func TestOrchestrator_VerifyDeployment_Success(t *testing.T) {
	installer := &mockInstaller{}
	orch := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	steps := []struct {
		name  string
		phase string
	}{
		{name: "installing_cert_manager", phase: "A"},
		{name: "installing_minio", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
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
		{name: "installing_minio", phase: "A"},
		{name: "installing_gitlab", phase: "B"},
		{name: "installing_argocd", phase: "B"},
		{name: "installing_runner", phase: "B"},
		{name: "installing_prometheus", phase: "C"},
		{name: "installing_grafana", phase: "C"},
	}
	for _, step := range steps {
		require.NoError(t, orch.ExecuteStep(context.Background(), "stk_verify_fail", step.name, step.phase))
	}

	err := orch.VerifyDeployment(context.Background(), "stk_verify_fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not healthy")
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
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_override", "installing_minio", "A"))
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
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_access_domain", "installing_minio", "A"))
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
	require.NoError(t, orch.ExecuteStep(context.Background(), "stk_manifest", "installing_minio", "A"))
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

	manifest, ok := orch.monitoringManifestForStep("installing_prometheus")
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

	_, ok := orch.monitoringManifestForStep("installing_prometheus")
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
