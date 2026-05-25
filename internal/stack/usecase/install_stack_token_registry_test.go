package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type mockTokenRegistry struct {
	inputs []port.TokenSourceInput
}

func (m *mockTokenRegistry) Upsert(_ context.Context, input port.TokenSourceInput) error {
	m.inputs = append(m.inputs, input)
	return nil
}

func TestInstallStack_RegisterStackTokenSources_OpenBao(t *testing.T) {
	t.Parallel()
	registry := &mockTokenRegistry{}
	uc := &InstallStack{tokenRegistry: registry, tokenRegistryEnv: "dev"}
	stack := &domain.Stack{
		OrgID:     "org-1",
		Namespace: "nullus-test",
		Config: domain.StackConfig{
			Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
			Artifacts: domain.ArtifactsConfig{
				SourceRepository:  domain.ToolSelection{Name: "GitHub", Enabled: true, Version: "external"},
				ContainerRegistry: domain.ToolSelection{Name: "Harbor", Enabled: true},
				StorageBackend:    domain.ToolSelection{Name: "MinIO", Enabled: true},
			},
			Pipeline: domain.PipelineConfig{
				CIPlatform: domain.ToolSelection{Name: "GitHub Actions", Enabled: true, Version: "external"},
				CDTool:     domain.ToolSelection{Name: "Argo CD", Enabled: true},
			},
			Storage: &domain.StorageConfig{
				Database: domain.StorageTarget{Mode: "create"},
			},
		},
	}

	require.NoError(t, uc.registerStackTokenSources(context.Background(), stack))
	assert.Len(t, registry.inputs, 7)
	assert.Equal(t, "kv/nullus/dev/org-1/artifacts/github/token", registry.inputs[0].Path)

	paths := make([]string, 0, len(registry.inputs))
	for _, in := range registry.inputs {
		paths = append(paths, in.Path)
	}
	assert.Contains(t, paths, "kv/nullus/dev/org-1/storage/postgresql/access")
	assert.Contains(t, paths, "kv/nullus/dev/org-1/artifacts/minio/access")
	assert.Contains(t, paths, "kv/nullus/dev/org-1/pipeline/argocd/access")

	var hasArgocdAccess bool
	for _, in := range registry.inputs {
		if in.Path == "kv/nullus/dev/org-1/pipeline/argocd/access" {
			hasArgocdAccess = strings.Contains(in.TokenValue, "argocd-initial-admin-secret")
		}
	}
	assert.True(t, hasArgocdAccess)
}

func TestInstallStack_RegisterStackTokenSources_SkipWhenNotOpenBao(t *testing.T) {
	t.Parallel()
	registry := &mockTokenRegistry{}
	uc := &InstallStack{tokenRegistry: registry, tokenRegistryEnv: "dev"}
	stack := &domain.Stack{OrgID: "org-1", Config: domain.StackConfig{}}
	require.NoError(t, uc.registerStackTokenSources(context.Background(), stack))
	assert.Len(t, registry.inputs, 0)
}
