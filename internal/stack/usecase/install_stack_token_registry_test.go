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

	require.NoError(t, uc.registerStackTokenSources(context.Background(), stack, SourceControlCredentials{}))
	// bootstrap 자격증명은 provisioning_secrets 로 이관되어 여기서는 도구
	// 토큰 경로만 등록한다.
	assert.Len(t, registry.inputs, 2)

	paths := make([]string, 0, len(registry.inputs))
	for _, in := range registry.inputs {
		paths = append(paths, in.Path)
	}
	// GitHub·GitHub Actions 는 외부 SaaS 라 회전 컨트롤러가 재발급할 수 없다.
	// 이 경로로 등록하면 소유자 정보가 없는 행이 남아 연동 조회를 오염시킨다.
	assert.NotContains(t, paths, "kv/nullus/dev/org-1/artifacts/github/token")
	assert.NotContains(t, paths, "kv/nullus/dev/org-1/pipeline/github-actions/token")
	assert.NotContains(t, paths, "kv/nullus/dev/org-1/storage/postgresql/access")
	assert.NotContains(t, paths, "kv/nullus/dev/org-1/artifacts/minio/access")
	assert.NotContains(t, paths, "kv/nullus/dev/org-1/pipeline/argocd/access")

	// 토큰 값은 실제 발급 시점에 회전 컨트롤러가 채운다.
	for _, in := range registry.inputs {
		assert.Empty(t, in.TokenValue)
	}
}

func TestInstallStack_RegisterStackTokenSources_SkipWhenNotOpenBao(t *testing.T) {
	t.Parallel()
	registry := &mockTokenRegistry{}
	uc := &InstallStack{tokenRegistry: registry, tokenRegistryEnv: "dev"}
	stack := &domain.Stack{OrgID: "org-1", Config: domain.StackConfig{}}
	require.NoError(t, uc.registerStackTokenSources(context.Background(), stack, SourceControlCredentials{}))
	assert.Len(t, registry.inputs, 0)
}
