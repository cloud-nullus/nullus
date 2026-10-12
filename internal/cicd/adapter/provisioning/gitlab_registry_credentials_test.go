package provisioning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRegistrySecrets struct {
	values map[string]string
}

func (f *fakeRegistrySecrets) GetTokenForStack(_ context.Context, _, _, path string) (string, error) {
	return f.values[path], nil
}

// GitLab + Harbor 스택의 파이프라인도 Harbor 관리자 자격증명을 받아야 한다.
//
// 자격증명 배선이 Gitea 번들에만 있어, GitLab + Harbor 템플릿으로 만든 스택은
// HARBOR_USERNAME/HARBOR_PASSWORD 가 등록되지 않았다. CI build 가
// "Must provide --username with --password-stdin" 로 죽어 스캔도 배포도 건너뛰었다
// (kind harbor-e2e 실측).
func TestFor_GitLabHarborBundleResolvesRegistryCredentials(t *testing.T) {
	stack := gitlabStack()
	stack.ContainerRegistry = "Harbor"
	secrets := &fakeRegistrySecrets{values: map[string]string{
		"kv/nullus/dev/org-1/artifacts/harbor/admin-password": "harbor-admin-pw",
	}}

	bundle, err := newFactory(t, stack, &fakeTokenIssuer{token: "t"}).
		WithRegistrySecrets(secrets).
		For(context.Background(), "stk_1")
	require.NoError(t, err)
	require.NotNil(t, bundle.RegistryCredentials, "GitLab 번들에도 레지스트리 자격증명을 배선해야 한다")

	values, err := bundle.RegistryCredentials.Resolve(context.Background(),
		[]string{"HARBOR_USERNAME", "HARBOR_PASSWORD"})
	require.NoError(t, err)
	assert.Equal(t, "admin", values["HARBOR_USERNAME"])
	assert.Equal(t, "harbor-admin-pw", values["HARBOR_PASSWORD"])

	// 같은 자격증명으로 파이프라인 삭제 시 이미지 저장소도 지운다.
	assert.NotNil(t, bundle.Images)
}
