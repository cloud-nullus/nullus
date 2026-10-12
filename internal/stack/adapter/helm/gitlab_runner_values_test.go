package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// arm64 클러스터에서 모든 CI 잡이 runner_system_failure 로 끝난 원인이다 —
// 러너가 helper 이미지로 x86_64 태그를 고르고, init 컨테이너가 실행되지 못했다.
func TestGitLabRunnerArchValues_ARM64SetsHelperImage(t *testing.T) {
	values := gitLabRunnerArchValues([]string{domain.ArchARM64})
	require.NotNil(t, values, "arm64 노드에서는 helper 이미지를 못박아야 한다")

	config := runnerConfigTOML(t, values)
	assert.Contains(t, config,
		`helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:arm64-v17.7.0"`)
	// dind 가 기동하지 못하면 이미지 빌드 단계가 통째로 실패한다. 함께 남아야 한다.
	assert.Contains(t, config, "privileged = true")
	assert.Contains(t, config, `namespace = "{{.Release.Namespace}}"`)
}

func TestGitLabRunnerArchValues_AMD64SetsHelperImage(t *testing.T) {
	values := gitLabRunnerArchValues([]string{domain.ArchAMD64})
	require.NotNil(t, values)
	assert.Contains(t, runnerConfigTOML(t, values),
		`helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v17.7.0"`)
}

// 노드를 모르거나 섞여 있으면 손대지 않는다 — 태그 하나로 양쪽을 덮을 수 없고,
// 잘못 박으면 지금 도는 클러스터를 우리가 깨뜨린다.
func TestGitLabRunnerArchValues_UnknownOrMixedKeepsDefault(t *testing.T) {
	assert.Nil(t, gitLabRunnerArchValues(nil))
	assert.Nil(t, gitLabRunnerArchValues([]string{}))
	assert.Nil(t, gitLabRunnerArchValues([]string{domain.ArchAMD64, domain.ArchARM64}))
	assert.Nil(t, gitLabRunnerArchValues([]string{"riscv64"}))
}

// 기본값에는 helper_image 가 없다. 노드를 보기 전에 박으면 amd64 클러스터를 깨뜨린다.
func TestDefaultValues_RunnerHasNoHelperImage(t *testing.T) {
	config := runnerConfigTOML(t, DefaultValues(stepInstallingRunner))
	assert.NotContains(t, config, "helper_image")
	assert.Contains(t, config, "privileged = true")
}

// 기본값과 아키텍처 보정이 각자 TOML 을 들고 있으면 한쪽만 고쳐져 갈라진다.
func TestGitLabRunnerConfigTOML_SharedBetweenDefaultAndArchValues(t *testing.T) {
	base := runnerConfigTOML(t, DefaultValues(stepInstallingRunner))
	arch := runnerConfigTOML(t, gitLabRunnerArchValues([]string{domain.ArchARM64}))
	assert.True(t, strings.HasPrefix(arch, base),
		"아키텍처 보정은 기본 TOML 에 helper_image 한 줄만 더한다")
}

func runnerConfigTOML(t *testing.T, values map[string]any) string {
	t.Helper()
	runners, ok := values["runners"].(map[string]any)
	require.True(t, ok, "runners 블록이 있어야 한다")
	config, ok := runners["config"].(string)
	require.True(t, ok, "runners.config 는 TOML 문자열이다")
	return config
}

// 값이 실제 설치 경로까지 실려 가는지 본다. 단위로만 맞으면 배선이 빠져도
// 테스트는 통과하고 클러스터에서는 그대로 x86_64 helper 를 받는다.
func TestMergedValuesForStep_RunnerCarriesArchHelperImage(t *testing.T) {
	spec, ok := DefaultChartSpecForStep(stepInstallingRunner)
	require.True(t, ok)

	o := &Orchestrator{namespace: "nullus"}
	o.setNodeArchitectures([]string{domain.ArchARM64})
	assert.Contains(t, runnerConfigTOML(t, o.mergedValuesForStep(stepInstallingRunner, spec)),
		`helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:arm64-v17.7.0"`)

	// 노드를 읽지 못한 설치는 지금까지의 동작 그대로다.
	unknown := &Orchestrator{namespace: "nullus"}
	assert.NotContains(t, runnerConfigTOML(t, unknown.mergedValuesForStep(stepInstallingRunner, spec)),
		"helper_image")
}

// CI 잡은 스택 자신의 GitLab 에서 소스를 받고 스택 레지스트리에 이미지를 올린다.
// 둘 다 접속 도메인 이름이고, 클러스터 안에서는 아무도 그 이름을 풀어 주지 않아
// 잡이 "Could not resolve host" 로 끝났다.
func TestGitLabRunnerValues_HostAliasesForStackTools(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.setNodeArchitectures([]string{domain.ArchARM64})
	o.stackConfig = &domain.StackConfig{AccessDomain: "nullus.local"}
	o.gatewayIP = "10.96.178.132"
	o.gatewayIPLoaded = true

	config := runnerConfigTOML(t, o.gitLabRunnerValues())
	assert.Contains(t, config, "[[runners.kubernetes.host_aliases]]")
	assert.Contains(t, config, `ip = "10.96.178.132"`)
	assert.Contains(t, config, `hostnames = ["gitlab.nullus.local", "registry.nullus.local"]`)
	// 아키텍처 보정과 같은 키를 쓴다. 한쪽이 다른 쪽을 지우면 안 된다.
	assert.Contains(t, config,
		`helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:arm64-v17.7.0"`)
	assert.Contains(t, config, "privileged = true")
}

// 게이트웨이 주소를 모르거나 접속 도메인이 없으면 손대지 않는다.
func TestGitLabRunnerValues_NoGatewayKeepsArchOnly(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.setNodeArchitectures([]string{domain.ArchARM64})
	o.stackConfig = &domain.StackConfig{AccessDomain: "nullus.local"}

	config := runnerConfigTOML(t, o.gitLabRunnerValues())
	assert.NotContains(t, config, "host_aliases")
	assert.Contains(t, config, "helper_image")

	noDomain := &Orchestrator{namespace: "nullus"}
	noDomain.setNodeArchitectures([]string{domain.ArchARM64})
	noDomain.gatewayIP = "10.96.178.132"
	noDomain.gatewayIPLoaded = true
	assert.NotContains(t, runnerConfigTOML(t, noDomain.gitLabRunnerValues()), "host_aliases")
}

// 아무것도 정할 수 없으면 values 를 내지 않는다 — 기본값이 그대로 남아야 한다.
func TestGitLabRunnerValues_NothingKnownKeepsDefault(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	assert.Nil(t, o.gitLabRunnerValues())
}

// 차트를 올릴 때 helper 이미지 태그가 같이 따라가야 한다. 어긋나면 그 태그가
// 없어 helper 를 받지 못하고 그 스택의 CI 가 한 건도 돌지 않는다.
func TestRunnerChartSpec_UsesDeclaredChartVersion(t *testing.T) {
	spec, ok := DefaultChartSpecForStep(stepInstallingRunner)
	require.True(t, ok)
	assert.Equal(t, domain.GitLabRunnerChartVersion, spec.Version)
}
