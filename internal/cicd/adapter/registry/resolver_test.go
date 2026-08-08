package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestResolvers_ImplementPort(t *testing.T) {
	var _ port.ImageRegistryResolver = (*SCMProjectResolver)(nil)
	var _ port.ImageRegistryResolver = (*HarborResolver)(nil)
	var _ port.ImageRegistryResolver = (*ExternalResolver)(nil)
}

// GitLab 스택: 이미지는 앱 프로젝트 자신의 레지스트리에 들어간다.
// 인증은 GitLab CI 내장 변수로 해결되므로 추가 변수 등록이 필요 없다.
func TestSCMProjectResolver_UsesProjectRegistryAndBuiltinVars(t *testing.T) {
	r := NewSCMProjectResolver()

	target, err := r.Resolve(context.Background(), port.ImageTargetSpec{
		AppName:        "myapp",
		SCMProjectPath: "acme/myapp",
		SCMRegistryURL: "registry.nullus.local/acme/myapp",
	})
	require.NoError(t, err)

	assert.Equal(t, port.RegistryKindSCMProject, target.Kind)
	assert.Equal(t, "registry.nullus.local/acme/myapp", target.Repository)
	assert.Equal(t, "registry.nullus.local", target.Host)
	assert.Equal(t, "CI_REGISTRY_USER", target.UsernameVar)
	assert.Equal(t, "CI_REGISTRY_PASSWORD", target.PasswordVar)
	assert.Empty(t, target.RequiredVariables, "내장 변수만 쓰므로 등록할 변수가 없다")
}

func TestSCMProjectResolver_RequiresRegistryURL(t *testing.T) {
	r := NewSCMProjectResolver()

	_, err := r.Resolve(context.Background(), port.ImageTargetSpec{
		AppName: "myapp", SCMProjectPath: "acme/myapp",
	})
	require.Error(t, err, "레지스트리 경로를 모르면 CI 가 이미지를 올릴 곳을 정할 수 없다")
}

// Harbor 스택: 이미지는 조직 이름의 Harbor 프로젝트 아래로 들어간다.
// SCM 프로젝트 경로와 무관하다 — 이것이 추상화가 필요한 이유다.
func TestHarborResolver_UsesOrgProjectUnderHarborHost(t *testing.T) {
	r := NewHarborResolver("harbor.nullus.local")

	target, err := r.Resolve(context.Background(), port.ImageTargetSpec{
		AppName:        "myapp",
		OrgPath:        "acme",
		SCMProjectPath: "acme/myapp",
		SCMRegistryURL: "registry.nullus.local/acme/myapp",
	})
	require.NoError(t, err)

	assert.Equal(t, port.RegistryKindHarbor, target.Kind)
	assert.Equal(t, "harbor.nullus.local", target.Host)
	assert.Equal(t, "harbor.nullus.local/acme/myapp", target.Repository)
	assert.NotEqual(t, "registry.nullus.local/acme/myapp", target.Repository,
		"SCM 레지스트리가 아니라 Harbor 를 써야 한다")

	// 외부 레지스트리는 내장 변수가 없으므로 자격증명을 파이프라인에 등록해야 한다.
	assert.Equal(t, harborUsernameVar, target.UsernameVar)
	assert.Equal(t, harborPasswordVar, target.PasswordVar)
	assert.ElementsMatch(t, []string{harborUsernameVar, harborPasswordVar}, target.RequiredVariables)
}

func TestHarborResolver_FallsBackToAppNameWhenOrgMissing(t *testing.T) {
	r := NewHarborResolver("harbor.nullus.local")

	target, err := r.Resolve(context.Background(), port.ImageTargetSpec{AppName: "myapp"})
	require.NoError(t, err)
	assert.Equal(t, "harbor.nullus.local/library/myapp", target.Repository,
		"조직을 모르면 Harbor 기본 프로젝트로 보낸다")
}

func TestHarborResolver_RequiresHost(t *testing.T) {
	_, err := NewHarborResolver("").Resolve(context.Background(), port.ImageTargetSpec{AppName: "a"})
	require.Error(t, err)
}

func TestExternalResolver_UsesConfiguredRepositoryPrefix(t *testing.T) {
	r := NewExternalResolver("ghcr.io/acme")

	target, err := r.Resolve(context.Background(), port.ImageTargetSpec{AppName: "myapp"})
	require.NoError(t, err)

	assert.Equal(t, port.RegistryKindExternal, target.Kind)
	assert.Equal(t, "ghcr.io", target.Host)
	assert.Equal(t, "ghcr.io/acme/myapp", target.Repository)
	assert.ElementsMatch(t, []string{externalUsernameVar, externalPasswordVar}, target.RequiredVariables)
}

// 스택이 고른 도구 이름으로 구현을 선택한다. 이름 표기가 흔들려도
// (GitLab Registry / gitlab-registry / GitLab Container Registry) 같은 곳으로 가야 한다.
func TestResolverFor_SelectsByStackToolName(t *testing.T) {
	cases := []struct {
		toolName string
		want     port.RegistryKind
	}{
		{"GitLab Registry", port.RegistryKindSCMProject},
		{"gitlab-registry", port.RegistryKindSCMProject},
		{"GitLab Container Registry", port.RegistryKindSCMProject},
		{"Harbor", port.RegistryKindHarbor},
		{"harbor", port.RegistryKindHarbor},
	}

	for _, tc := range cases {
		t.Run(tc.toolName, func(t *testing.T) {
			r, err := ResolverFor(Config{ToolName: tc.toolName, HarborHost: "harbor.test"})
			require.NoError(t, err)

			target, err := r.Resolve(context.Background(), port.ImageTargetSpec{
				AppName: "app", OrgPath: "acme",
				SCMProjectPath: "acme/app", SCMRegistryURL: "reg.test/acme/app",
			})
			require.NoError(t, err)
			assert.Equal(t, tc.want, target.Kind)
		})
	}
}

func TestResolverFor_UnknownToolFallsBackToExternalWhenPrefixGiven(t *testing.T) {
	r, err := ResolverFor(Config{ToolName: "Nexus", ExternalRepositoryPrefix: "nexus.test/acme"})
	require.NoError(t, err)

	target, err := r.Resolve(context.Background(), port.ImageTargetSpec{AppName: "app"})
	require.NoError(t, err)
	assert.Equal(t, port.RegistryKindExternal, target.Kind)
	assert.Equal(t, "nexus.test/acme/app", target.Repository)
}

// 어디에 올릴지 정할 수 없으면 조용히 기본값으로 흘리지 않고 실패한다.
// 잘못된 곳에 이미지를 올리면 배포가 엉뚱한 이미지를 집는다.
func TestResolverFor_UnknownToolWithoutConfigFails(t *testing.T) {
	_, err := ResolverFor(Config{ToolName: "Nexus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nexus")
}
