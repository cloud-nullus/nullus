package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func seaweedFSConfig() domain.StackConfig {
	cfg := fullConfig()
	cfg.Artifacts.StorageBackend = domain.ToolSelection{Name: "SeaweedFS", Version: domain.SeaweedFSAppVersion, Enabled: true}
	return cfg
}

// 연결정보는 고른 구현체의 실제 이름을 안내한다. MinIO 이름을 그대로 두면
// 존재하지 않는 Secret 을 조회하라고 안내하게 된다.
func TestGetConnectionInfo_SeaweedFSObjectStorage(t *testing.T) {
	out, err := newConnUC(connStack(seaweedFSConfig())).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, domain.ObjectStorageToolSeaweedFS, out.ObjectStorage.Engine)
	assert.Equal(t, "http://nullus-seaweedfs-all-in-one:8333", out.ObjectStorage.Endpoint)
	assert.Equal(t, domain.ProvisionedSeaweedFSSecret, out.ObjectStorage.SecretRef)
	assert.Equal(t, domain.SeaweedFSSecretKeyKey, out.ObjectStorage.SecretKey)
	assert.Equal(t, domain.ObjectStorageAccessKey, out.ObjectStorage.AuthID)
	assert.Equal(t, domain.GitLabArtifactsBucket, out.ObjectStorage.ResourceName)

	// SeaweedFS 는 로그인 화면이 없다 — 도구 자격 카드로 안내하지 않는다.
	for _, tool := range out.Tools {
		assert.NotEqual(t, domain.ProvisionedMinIOSecret, tool.SecretRef)
		assert.NotEqual(t, domain.ProvisionedSeaweedFSSecret, tool.SecretRef)
	}
}

// 설치 DAG 는 두 구현체 중 무엇이 섰든 GitLab 연결 Secret 단계로 이어진다.
func TestInstallDAG_ObjectStorageConsumersWaitForEitherBackend(t *testing.T) {
	var deps []string
	for _, step := range installDAG {
		if step.name == "installing_object_storage_secret" {
			deps = step.deps
		}
	}
	assert.ElementsMatch(t, []string{"installing_minio", "installing_seaweedfs"}, deps)
}

// 스택 삭제는 SeaweedFS 가 남기는 것까지 회수한다. 차트가 keep 정책으로 만드는
// sftp Secret 은 helm uninstall 이 지우지 않는다.
func TestDeleteStack_ReclaimsSeaweedFSLeftovers(t *testing.T) {
	assert.Contains(t, legacyReleaseArtifactExactNames, domain.ProvisionedSeaweedFSSecret)
	assert.Contains(t, legacyReleaseArtifactExactNames, domain.SeaweedFSSFTPSecret)
	assert.Contains(t, domain.InstalledHelmReleaseNames, domain.SeaweedFSReleaseName)
}

func TestResumeStepForReadinessError_SeaweedFS(t *testing.T) {
	got := resumeStepForReadinessError(errString("runtime readiness failed for nullus-seaweedfs: rollout timeout"))
	assert.Equal(t, "installing_seaweedfs", got)
}

type errString string

func (e errString) Error() string { return string(e) }
