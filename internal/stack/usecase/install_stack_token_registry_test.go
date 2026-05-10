package usecase

import (
	"context"
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
		OrgID: "org-1",
		Config: domain.StackConfig{
			Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
			Artifacts: domain.ArtifactsConfig{
				SourceRepository:  domain.ToolSelection{Name: "GitHub"},
				ContainerRegistry: domain.ToolSelection{Name: "Harbor"},
			},
			Pipeline: domain.PipelineConfig{
				CIPlatform: domain.ToolSelection{Name: "GitHub Actions"},
				CDTool:     domain.ToolSelection{Name: "Argo CD"},
			},
		},
	}

	require.NoError(t, uc.registerStackTokenSources(context.Background(), stack))
	assert.Len(t, registry.inputs, 4)
	assert.Equal(t, "kv/nullus/dev/org-1/artifacts/github/token", registry.inputs[0].Path)
}

func TestInstallStack_RegisterStackTokenSources_SkipWhenNotOpenBao(t *testing.T) {
	t.Parallel()
	registry := &mockTokenRegistry{}
	uc := &InstallStack{tokenRegistry: registry, tokenRegistryEnv: "dev"}
	stack := &domain.Stack{OrgID: "org-1", Config: domain.StackConfig{}}
	require.NoError(t, uc.registerStackTokenSources(context.Background(), stack))
	assert.Len(t, registry.inputs, 0)
}
