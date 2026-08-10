package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestResolverFor_PicksGHCR(t *testing.T) {
	for _, name := range []string{"GHCR", "GitHub Container Registry", "github-packages"} {
		resolver, err := ResolverFor(Config{ToolName: name, GitHubOwner: "acme"})
		require.NoError(t, err, "tool=%q", name)
		assert.IsType(t, &GHCRResolver{}, resolver, "tool=%q", name)
	}
}

// GitHub 어댑터가 알려준 경로를 우선한다. 이미 있는 리포를 재사용하면
// 리포 이름과 앱 이름이 달라, 앱 이름으로 조립하면 없는 패키지를 가리킨다.
func TestGHCRResolver_PrefersSCMRegistryURL(t *testing.T) {
	target, err := NewGHCRResolver("acme").Resolve(context.Background(), port.ImageTargetSpec{
		AppName:        "myapp",
		SCMRegistryURL: "ghcr.io/acme/legacy-repo-name",
	})

	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/legacy-repo-name", target.Repository)
	assert.Equal(t, "ghcr.io", target.Host)
}

func TestGHCRResolver_BuildsFromOwnerAndApp(t *testing.T) {
	target, err := NewGHCRResolver("acme").Resolve(context.Background(), port.ImageTargetSpec{
		AppName: "myapp",
	})

	require.NoError(t, err)
	assert.Equal(t, port.RegistryKindGHCR, target.Kind)
	assert.Equal(t, "ghcr.io/acme/myapp", target.Repository)
}

// GHCR 은 소문자 경로만 받는다. 대문자를 그대로 밀면 push 가 거부되는데
// 오류가 권한 문제처럼 보여 원인을 찾기 어렵다.
func TestGHCRResolver_LowercasesRepository(t *testing.T) {
	target, err := NewGHCRResolver("Acme").Resolve(context.Background(), port.ImageTargetSpec{
		AppName: "MyApp",
	})

	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/myapp", target.Repository)
}

// 내장 GITHUB_TOKEN 으로 push 하므로 사람이 등록할 시크릿이 없어야 한다.
// 여기에 값이 생기면 프로비저닝이 "변수를 채우라"는 경고를 잘못 띄운다.
func TestGHCRResolver_RequiresNoUserSuppliedSecrets(t *testing.T) {
	target, err := NewGHCRResolver("acme").Resolve(context.Background(), port.ImageTargetSpec{
		AppName: "myapp",
	})

	require.NoError(t, err)
	assert.Empty(t, target.RequiredVariables)
	assert.Equal(t, "GITHUB_ACTOR", target.UsernameVar)
	assert.Equal(t, "GITHUB_TOKEN", target.PasswordVar)
}

func TestGHCRResolver_FallsBackToOrgPath(t *testing.T) {
	target, err := NewGHCRResolver("").Resolve(context.Background(), port.ImageTargetSpec{
		AppName: "myapp", OrgPath: "fallback-org",
	})

	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/fallback-org/myapp", target.Repository)
}

func TestGHCRResolver_ErrorsWithoutOwner(t *testing.T) {
	_, err := NewGHCRResolver("").Resolve(context.Background(), port.ImageTargetSpec{AppName: "myapp"})
	require.Error(t, err)
}
