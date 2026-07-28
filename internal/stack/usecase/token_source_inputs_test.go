package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestBuildStackTokenSourceInputs_OpenBao(t *testing.T) {
	t.Parallel()

	stack := &domain.Stack{
		OrgID:     "org-1",
		Namespace: "nullus-test",
		Config: domain.StackConfig{
			Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
			Artifacts: domain.ArtifactsConfig{
				SourceRepository:  domain.ToolSelection{Name: "GitLab CE", Enabled: true},
				ContainerRegistry: domain.ToolSelection{Name: "GitLab Registry", Enabled: true},
				StorageBackend:    domain.ToolSelection{Name: "MinIO", Enabled: true},
			},
			Pipeline: domain.PipelineConfig{
				CIPlatform: domain.ToolSelection{Name: "GitLab CI", Enabled: true},
				CDTool:     domain.ToolSelection{Name: "Argo CD", Enabled: true},
			},
			Storage: &domain.StorageConfig{Database: domain.StorageTarget{Mode: "create"}},
		},
	}

	inputs := BuildStackTokenSourceInputs(stack, "dev")
	// bootstrap 자격증명(postgresql/minio/argocd access)은 더 이상 여기서
	// 만들지 않는다. provisioning_secrets 가 생성해 OpenBao 에 기록하고
	// ESO 가 K8s Secret 으로 복제하므로 이중 관리를 없앴다.
	require.Len(t, inputs, 4)
	assert.Equal(t, "kv/nullus/dev/org-1/artifacts/gitlab-ce/token", inputs[0].Path)
	assert.Contains(t, inputs[0].Provider, "gitlab")
	assert.Contains(t, inputs, inputs[0])

	paths := map[string]struct{}{}
	for _, input := range inputs {
		paths[input.Path] = struct{}{}
	}
	assert.Contains(t, paths, "kv/nullus/dev/org-1/pipeline/argo-cd/token")
	assert.Contains(t, paths, "kv/nullus/dev/org-1/pipeline/gitlab-ci/token")
	assert.NotContains(t, paths, "kv/nullus/dev/org-1/storage/postgresql/access")
	assert.NotContains(t, paths, "kv/nullus/dev/org-1/artifacts/minio/access")
}

func TestBuildStackTokenSourceInputs_SkipsWhenNotOpenBao(t *testing.T) {
	t.Parallel()

	stack := &domain.Stack{OrgID: "org-1", Config: domain.StackConfig{}}
	assert.Nil(t, BuildStackTokenSourceInputs(stack, "dev"))
}
