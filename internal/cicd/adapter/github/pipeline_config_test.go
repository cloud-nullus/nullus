package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// GitHub 은 평문 시크릿을 받지 않는다. 등록한 값이 리포 공개키에 대응하는
// 비밀키로 실제 복호화되는지까지 확인해야 암호화가 맞다고 말할 수 있다.
func TestSetProjectVariable_EncryptsWithRepoPublicKey(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	var putBody map[string]any
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/myapp/actions/secrets/public-key":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"key_id": "key-1",
				"key":    base64.StdEncoding.EncodeToString(pub[:]),
			})
		case r.Method == http.MethodPut && r.URL.Path == "/repos/acme/myapp/actions/secrets/HARBOR_PASSWORD":
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	c := NewClient(srv.URL, "tok")
	err = c.SetProjectVariable(context.Background(), "acme/myapp", port.ProjectVariable{
		Key: "HARBOR_PASSWORD", Value: "s3cr3t-value", Masked: true,
	})
	require.NoError(t, err)

	require.NotNil(t, putBody)
	assert.Equal(t, "key-1", putBody["key_id"])

	sealed, err := base64.StdEncoding.DecodeString(putBody["encrypted_value"].(string))
	require.NoError(t, err)
	opened, ok := box.OpenAnonymous(nil, sealed, pub, priv)
	require.True(t, ok, "리포 공개키로 봉인된 값이어야 한다")
	assert.Equal(t, "s3cr3t-value", string(opened))
}

func TestSetProjectVariable_NormalizesName(t *testing.T) {
	pub, _, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	var putPath string
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key_id": "key-1", "key": base64.StdEncoding.EncodeToString(pub[:]),
		})
	})

	c := NewClient(srv.URL, "tok")
	require.NoError(t, c.SetProjectVariable(context.Background(), "acme/myapp",
		port.ProjectVariable{Key: "registry-username", Value: "robot"}))

	assert.Equal(t, "/repos/acme/myapp/actions/secrets/REGISTRY_USERNAME", putPath)
}

func TestSetProjectVariable_RejectsReservedNames(t *testing.T) {
	c := NewClient("https://api.github.test", "tok")

	for _, key := range []string{"GITHUB_TOKEN", "1INVALID", "bad$name"} {
		err := c.SetProjectVariable(context.Background(), "acme/myapp",
			port.ProjectVariable{Key: key, Value: "v"})
		assert.Error(t, err, "이름 %q 는 거부돼야 한다", key)
	}
}

// 토큰 발급을 조용히 빈 값으로 흘리면 Argo CD 가 자격증명 없이 구성돼
// 동기화가 실패한다. 호출자가 분기할 수 있도록 명시적 오류여야 한다.
func TestCreateProjectAccessToken_IsUnsupported(t *testing.T) {
	c := NewClient("https://api.github.test", "tok")

	token, err := c.CreateProjectAccessToken(context.Background(), "acme/myapp", port.AccessTokenSpec{
		Name: "nullus-deploy", Scopes: []string{"write_repository"},
	})

	assert.Empty(t, token)
	require.ErrorIs(t, err, ErrAccessTokenUnsupported)
}
