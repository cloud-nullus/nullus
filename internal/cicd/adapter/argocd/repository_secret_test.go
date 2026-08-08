package argocd

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func parseSecret(t *testing.T, manifest string) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &doc))
	return doc
}

// Application 만 만들고 자격증명을 주지 않으면 private 저장소에서
// "authentication required" 로 동기화가 실패한다 (실제로 겪었다).
func TestRenderRepositorySecret_ProducesArgoCDRepositorySecret(t *testing.T) {
	manifest, err := RenderRepositorySecret(RepositorySecretSpec{
		AppName:       "myapp",
		ArgoNamespace: "devsecops",
		RepoURL:       "http://gitlab.test/nullus/myapp.git",
		Username:      "nullus-argocd",
		Password:      "glpat-read",
	})
	require.NoError(t, err)

	doc := parseSecret(t, manifest)
	assert.Equal(t, "v1", doc["apiVersion"])
	assert.Equal(t, "Secret", doc["kind"])

	meta := doc["metadata"].(map[string]any)
	assert.Equal(t, "devsecops", meta["namespace"])

	// 이 라벨이 없으면 Argo CD 가 자격증명으로 인식하지 않는다.
	labels := meta["labels"].(map[string]any)
	assert.Equal(t, "repository", labels["argocd.argoproj.io/secret-type"])

	data := doc["stringData"].(map[string]any)
	assert.Equal(t, "http://gitlab.test/nullus/myapp.git", data["url"])
	assert.Equal(t, "nullus-argocd", data["username"])
	assert.Equal(t, "glpat-read", data["password"])
	assert.Equal(t, "git", data["type"])
}

// 이름이 앱마다 달라야 여러 앱의 자격증명이 서로 덮어쓰지 않는다.
func TestRenderRepositorySecret_NameIsScopedToApp(t *testing.T) {
	first, err := RenderRepositorySecret(RepositorySecretSpec{
		AppName: "alpha", ArgoNamespace: "ns", RepoURL: "http://x/a.git", Password: "p",
	})
	require.NoError(t, err)
	second, err := RenderRepositorySecret(RepositorySecretSpec{
		AppName: "beta", ArgoNamespace: "ns", RepoURL: "http://x/b.git", Password: "p",
	})
	require.NoError(t, err)

	assert.NotEqual(t,
		parseSecret(t, first)["metadata"].(map[string]any)["name"],
		parseSecret(t, second)["metadata"].(map[string]any)["name"])
}

func TestRenderRepositorySecret_RequiresRepoAndPassword(t *testing.T) {
	_, err := RenderRepositorySecret(RepositorySecretSpec{
		AppName: "a", ArgoNamespace: "ns", Password: "p",
	})
	require.Error(t, err)

	_, err = RenderRepositorySecret(RepositorySecretSpec{
		AppName: "a", ArgoNamespace: "ns", RepoURL: "http://x/a.git",
	})
	require.Error(t, err)
}

func TestRenderRepositorySecret_DefaultsUsername(t *testing.T) {
	manifest, err := RenderRepositorySecret(RepositorySecretSpec{
		AppName: "a", ArgoNamespace: "ns", RepoURL: "http://x/a.git", Password: "p",
	})
	require.NoError(t, err)

	data := parseSecret(t, manifest)["stringData"].(map[string]any)
	assert.NotEmpty(t, data["username"])
}

// stringData 를 쓰므로 base64 인코딩은 쿠버네티스가 한다.
// 직접 인코딩하면 이중 인코딩이 되어 인증이 실패한다.
func TestRenderRepositorySecret_UsesStringDataNotBase64(t *testing.T) {
	manifest, err := RenderRepositorySecret(RepositorySecretSpec{
		AppName: "a", ArgoNamespace: "ns", RepoURL: "http://x/a.git", Password: "secret-value",
	})
	require.NoError(t, err)

	assert.Contains(t, manifest, "stringData:")
	assert.NotContains(t, manifest, base64.StdEncoding.EncodeToString([]byte("secret-value")))
}
