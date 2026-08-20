package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 2026-08-21 운영에서 minio·argocd 는 열리는데 harbor·gitea 만 안 열렸다.
//
// 설치 마법사가 모르는 도구의 백엔드를 "<도구>-svc:80" 으로 지어냈기 때문이다.
// 실제 이름과 포트는 도구마다 다르다 — gitea-http:3000, jenkins:8080, nexus:8081.
// 서버와 마법사가 각자 목록을 갖고 있던 것이 원인이라, 값이 갈라지면 여기서 걸린다.
func TestGatewayBackends_MatchInstallWizard(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..",
		"web", "src", "features", "stack", "utils", "install-manifest-builders.ts"))
	require.NoError(t, err)

	entry := regexp.MustCompile(`(?m)^\s{2}([a-z][a-z0-9-]*): \{ serviceName: '([^']+)', port: (\d+) \}`)
	matches := entry.FindAllStringSubmatch(string(source), -1)
	require.NotEmpty(t, matches, "마법사에서 백엔드 표를 찾지 못했다")

	for _, match := range matches {
		tool, service, rawPort := match[1], match[2], match[3]
		port, err := strconv.Atoi(rawPort)
		require.NoError(t, err)

		backend, ok := GatewayBackendForTool(tool)
		require.Truef(t, ok, "서버에 %s 백엔드가 없다 — 마법사만 알고 있으면 서버가 바로잡지 못한다", tool)
		assert.Equalf(t, backend.Service, service, "%s 의 서비스 이름이 서버와 다르다", tool)
		assert.Equalf(t, backend.Port, port, "%s 의 포트가 서버와 다르다", tool)
	}
}

// 마법사가 지어낸 이름에서 실제 백엔드를 되짚는다. 서버가 받아서 고치는 경로다.
func TestGatewayBackendForServiceAlias_RecoversRealService(t *testing.T) {
	backend, ok := GatewayBackendForServiceAlias("gitea-svc")
	require.True(t, ok)
	assert.Equal(t, GiteaHTTPServiceName, backend.Service)
	assert.Equal(t, GiteaServicePort, backend.Port)

	harbor, ok := GatewayBackendForServiceAlias("harbor-svc")
	require.True(t, ok)
	assert.Equal(t, HarborServiceName, harbor.Service)
	assert.Equal(t, HarborServicePort, harbor.Port)
}

func TestGatewayBackendForTool_AbsorbsNameSpellings(t *testing.T) {
	for _, name := range []string{"argocd", "argo-cd", "Argo CD", "ARGO_CD"} {
		backend, ok := GatewayBackendForTool(name)
		require.Truef(t, ok, "%q", name)
		assert.Equal(t, "argo-cd-argocd-server", backend.Service)
	}
}

func TestGatewayBackendForTool_UnknownToolIsNotGuessed(t *testing.T) {
	_, ok := GatewayBackendForTool("some-unknown-tool")
	assert.False(t, ok)
}
