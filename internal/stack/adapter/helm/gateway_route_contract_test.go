package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// registry.<도메인> 은 스택이 고른 레지스트리 하나만 가리켜야 한다.
//
// 두 라우트가 같은 호스트를 주장하면 게이트웨이가 어디로 보낼지 정해지지 않는다.
// 실제로 Nexus 라우트를 추가했을 때 기존 registry-route 와 겹쳤고, 그 registry-route
// 는 어떤 레지스트리를 골랐든 gitlab-registry 로 보내고 있었다 — Harbor 를 고른
// 스택이 비어 있는 GitLab 레지스트리로 push 하게 되는 상태였다.
func TestGatewayRoutes_RegistryHostHasSingleOwner(t *testing.T) {
	cases := []struct {
		registry  string
		wantRoute string
	}{
		{registry: "GitLab Registry", wantRoute: "registry-route"},
		{registry: "Harbor", wantRoute: "harbor-registry-route"},
		{registry: "Nexus", wantRoute: "nexus-docker-route"},
	}

	for _, tc := range cases {
		t.Run(tc.registry, func(t *testing.T) {
			manifest := gatewayManifestFor(tc.registry)

			// 호스트가 두 번 나오면 두 라우트가 주장하는 것이다.
			assert.Equalf(t, 1, strings.Count(manifest, "registry.example.test"),
				"%s: registry 호스트를 주장하는 라우트가 하나가 아니다", tc.registry)
			assert.Containsf(t, routeNames(manifest), tc.wantRoute,
				"%s: 기대한 라우트가 없다", tc.registry)
		})
	}
}

// Nexus 는 UI(8081)와 Docker 커넥터(8082)가 포트가 달라 호스트도 나뉘어야 한다.
// 합치면 docker push 가 UI 로 흘러들어 실패한다.
func TestGatewayRoutes_NexusSeparatesUIAndDockerHosts(t *testing.T) {
	names := routeNames(gatewayManifestFor("Nexus"))
	assert.Contains(t, names, "nexus-route")
	assert.Contains(t, names, "nexus-docker-route")
}

func gatewayManifestFor(registry string) string {
	cfg := domain.StackConfig{AccessDomain: "example.test"}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Name: "gitlab", Enabled: true}
	cfg.Artifacts.ContainerRegistry = domain.ToolSelection{Name: registry, Enabled: true}
	cfg.Pipeline.CDTool = domain.ToolSelection{Name: "argocd", Enabled: true}

	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus")
	o.SetStackConfig(cfg)
	return o.defaultGatewayBundleManifest("nullus")
}

func routeNames(manifest string) []string {
	var names []string
	for _, line := range strings.Split(manifest, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "-route") {
			names = append(names, strings.TrimPrefix(trimmed, "name: "))
		}
	}
	return names
}
