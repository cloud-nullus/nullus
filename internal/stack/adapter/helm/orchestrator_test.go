package helm

import (
	"context"
	"fmt"
	"testing"

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
