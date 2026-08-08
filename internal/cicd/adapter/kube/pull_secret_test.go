package kube

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// private 프로젝트 레지스트리는 익명 pull 을 거부한다. imagePullSecret 이 없으면
// kubelet 이 "failed to fetch anonymous token: 403" 으로 죽는다 (실제로 겪었다).
func TestRenderImagePullSecret_ProducesDockerConfigJSON(t *testing.T) {
	manifest, err := RenderImagePullSecret(ImagePullSecretSpec{
		Namespace:    "loopdemo",
		RegistryHost: "registry.test",
		Username:     "puller",
		Password:     "tok",
	})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &doc))

	assert.Equal(t, "Secret", doc["kind"])
	assert.Equal(t, "kubernetes.io/dockerconfigjson", doc["type"])

	meta := doc["metadata"].(map[string]any)
	assert.Equal(t, ImagePullSecretName, meta["name"])
	assert.Equal(t, "loopdemo", meta["namespace"])

	raw := doc["stringData"].(map[string]any)[".dockerconfigjson"].(string)
	var cfg struct {
		Auths map[string]struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Auth     string `json:"auth"`
		} `json:"auths"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &cfg))

	entry, ok := cfg.Auths["registry.test"]
	require.True(t, ok, "레지스트리 호스트 키가 있어야 kubelet 이 자격증명을 찾는다")
	assert.Equal(t, "puller", entry.Username)
	assert.Equal(t, "tok", entry.Password)

	decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
	require.NoError(t, err)
	assert.Equal(t, "puller:tok", string(decoded))
}

func TestRenderImagePullSecret_RequiresHostAndPassword(t *testing.T) {
	_, err := RenderImagePullSecret(ImagePullSecretSpec{Namespace: "ns", Password: "p"})
	require.Error(t, err)

	_, err = RenderImagePullSecret(ImagePullSecretSpec{Namespace: "ns", RegistryHost: "h"})
	require.Error(t, err)

	_, err = RenderImagePullSecret(ImagePullSecretSpec{RegistryHost: "h", Password: "p"})
	require.Error(t, err)
}

// Secret 을 적용하려면 네임스페이스가 먼저 있어야 한다. Argo CD 가
// CreateNamespace 로 만들기 전에 적용될 수 있으므로 함께 만든다.
func TestRenderNamespace_ProducesNamespace(t *testing.T) {
	manifest, err := RenderNamespace("loopdemo")
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &doc))
	assert.Equal(t, "Namespace", doc["kind"])
	assert.Equal(t, "loopdemo", doc["metadata"].(map[string]any)["name"])
}

func TestRenderNamespace_RequiresName(t *testing.T) {
	_, err := RenderNamespace("  ")
	require.Error(t, err)
}
