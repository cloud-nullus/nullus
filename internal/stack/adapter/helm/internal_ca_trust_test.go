package helm

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const testCAPEM = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"

// Argo CD 가 스택 GitLab 의 인증서를 신뢰하지 못해 Application 이 Synced 에
// 도달하지 못했다 — "x509: certificate signed by unknown authority".
func TestArgoCDInternalCAValues(t *testing.T) {
	values := argoCDInternalCAValues(testCAPEM, "nullus.local")
	require.NotNil(t, values)

	configs := values["configs"].(map[string]any)
	tls := configs["tls"].(map[string]any)
	certs := tls["certificates"].(map[string]any)

	// Argo CD 는 호스트별로 CA 를 찾는다. 저장소 호스트가 키여야 한다.
	assert.Contains(t, certs, "gitlab.nullus.local")
	assert.Contains(t, certs["gitlab.nullus.local"].(string), "BEGIN CERTIFICATE")
}

// 넣을 것이 없으면 손대지 않는다. 사용자가 공인 인증서를 쓰도록 구성했을 수 있다.
func TestArgoCDInternalCAValues_NothingToTrust(t *testing.T) {
	assert.Nil(t, argoCDInternalCAValues("", "nullus.local"))
	assert.Nil(t, argoCDInternalCAValues(testCAPEM, ""))
	assert.Nil(t, argoCDInternalCAValues("   ", "nullus.local"))
}

// CI 잡의 git clone 이 "unable to get local issuer certificate" 로 멈췄다.
func TestGitLabRunnerValues_TrustsInternalCA(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.setNodeArchitectures([]string{domain.ArchARM64})
	o.stackConfig = &domain.StackConfig{AccessDomain: "nullus.local"}
	o.gatewayIP = "10.96.178.132"
	o.gatewayIPLoaded = true
	o.internalCAEncoded = base64.StdEncoding.EncodeToString([]byte(testCAPEM))
	o.internalCALoaded = true

	config := runnerConfigTOML(t, o.gitLabRunnerValues())
	// 잡 파드가 CA 를 볼륨으로 받는다.
	assert.Contains(t, config, "[[runners.kubernetes.volumes.secret]]")
	assert.Contains(t, config, `name = "nullus-internal-ca-bundle"`)
	assert.Contains(t, config, `mount_path = "/etc/nullus/ca"`)
	// clone 은 helper 가 build 보다 먼저 한다. 두 자리 모두에 넣어야 한다.
	assert.Contains(t, config, "pre_get_sources_script")
	assert.Contains(t, config, "pre_build_script")
	// 시스템 번들을 갈아치우지 않고 뒤에 잇는다 — 공인 인증서를 쓰는 곳도 살아야 한다.
	assert.Contains(t, config, "/etc/ssl/certs/ca-certificates.crt")
	assert.NotContains(t, config, "GIT_SSL_CAINFO")
	// 잡이 apk add 같은 것을 하면 번들이 통째로 다시 만들어진다. 배포판이 번들을
	// 만들 때 읽는 소스 디렉터리에도 넣어야 그때 살아남는다.
	assert.Contains(t, config, "/usr/local/share/ca-certificates/nullus-internal-ca.crt")
	assert.Contains(t, config, "/etc/pki/ca-trust/source/anchors/nullus-internal-ca.crt")
	// 앞선 수정들과 같은 TOML 에 함께 담긴다.
	assert.Contains(t, config, "helper_image")
	assert.Contains(t, config, "host_aliases")
	assert.Contains(t, config, "privileged = true")
}

// CA 를 읽지 못한 설치는 지금까지의 동작 그대로다.
func TestGitLabRunnerValues_NoCAKeepsDefault(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.setNodeArchitectures([]string{domain.ArchARM64})
	o.stackConfig = &domain.StackConfig{AccessDomain: "nullus.local"}

	config := runnerConfigTOML(t, o.gitLabRunnerValues())
	assert.NotContains(t, config, "volumes.secret")
	assert.NotContains(t, config, "pre_build_script")
}

func TestInternalCACertPEM_DecodesSecretValue(t *testing.T) {
	o := &Orchestrator{}
	o.internalCAEncoded = base64.StdEncoding.EncodeToString([]byte(testCAPEM))
	o.internalCALoaded = true
	assert.Equal(t, testCAPEM, o.internalCACertPEM())

	// 깨진 값은 조용히 빈 값이다 — 설치를 뒤집을 일은 아니다.
	broken := &Orchestrator{}
	broken.internalCAEncoded = "not base64!!"
	broken.internalCALoaded = true
	assert.Empty(t, broken.internalCACertPEM())
}

// 설치 경로까지 실려 가는지 본다.
func TestMergedValuesForStep_ArgoCDCarriesInternalCA(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_argocd")
	require.True(t, ok)

	o := &Orchestrator{namespace: "nullus"}
	o.stackConfig = &domain.StackConfig{AccessDomain: "nullus.local"}
	o.internalCAEncoded = base64.StdEncoding.EncodeToString([]byte(testCAPEM))
	o.internalCALoaded = true

	values := o.mergedValuesForStep("installing_argocd", spec)
	configs, ok := values["configs"].(map[string]any)
	require.True(t, ok, "configs 블록이 있어야 한다")
	tls, ok := configs["tls"].(map[string]any)
	require.True(t, ok, "configs.tls 가 있어야 한다")
	certs := tls["certificates"].(map[string]any)
	assert.Contains(t, certs, "gitlab.nullus.local")
}

// 이 스크립트가 잡을 죽이면 원래 하려던 일까지 못 하게 된다. 없는 파일에 >> 를
// 걸면 리디렉션이 실패하고, 잡 스크립트는 set -e 로 돌기 때문에 거기서 죽었다.
func TestInternalCATrustScript_CannotFailTheJob(t *testing.T) {
	script := internalCATrustScript()
	assert.True(t, strings.HasSuffix(script, "|| true"), "실패를 삼켜야 한다: %s", script)
	// 번들이 없는 이미지에서도 리디렉션이 일어나지 않아야 한다.
	assert.Contains(t, script, `[ -f "$b" ]`)
}
