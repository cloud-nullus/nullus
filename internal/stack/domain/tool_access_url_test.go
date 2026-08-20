package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolAccessURL_AlwaysHTTPS(t *testing.T) {
	assert.Equal(t, "https://grafana.nullus.local", ToolAccessURL("grafana", "nullus.local"))
}

func TestToolAccessURL_MapsProductToHost(t *testing.T) {
	cases := map[string]string{
		"gitlab":        "https://gitlab.nullus.local",
		"GitLab CE":     "https://gitlab.nullus.local",
		"gitlab-ci":     "https://gitlab.nullus.local",
		"gitea":         "https://gitea.nullus.local",
		"Jenkins":       "https://jenkins.nullus.local",
		"argocd":        "https://argocd.nullus.local",
		"Argo CD":       "https://argocd.nullus.local",
		"prometheus":    "https://prometheus.nullus.local",
		"harbor":        "https://harbor.nullus.local",
		"Nexus":         "https://nexus.nullus.local",
		"MinIO":         "https://minio.nullus.local",
		"opensearch":    "https://opensearch.nullus.local",
		"elasticsearch": "https://kibana.nullus.local",
		"jaeger":        "https://jaeger.nullus.local",
		"OpenBao":       "https://openbao.nullus.local",
	}

	for name, want := range cases {
		assert.Equal(t, want, ToolAccessURL(name, "nullus.local"), "tool %q", name)
	}
}

// 자체 UI 가 없는 도구는 그것을 보는 화면(Grafana)으로 보낸다.
// 스택 상세 화면이 이미 같은 규칙으로 안내하고 있어 여기서 갈라지면 두 화면이 어긋난다.
func TestToolAccessURL_ToolsWithoutOwnUIPointAtGrafana(t *testing.T) {
	for _, name := range []string{"tempo", "loki", "Grafana Loki", "opentelemetry-collector"} {
		assert.Equal(t, "https://grafana.nullus.local", ToolAccessURL(name, "nullus.local"), "tool %q", name)
	}
}

func TestToolAccessURL_BlankWhenAccessDomainMissing(t *testing.T) {
	assert.Equal(t, "", ToolAccessURL("grafana", ""))
	assert.Equal(t, "", ToolAccessURL("grafana", "   "))
}

// 주소를 모르는 도구에 그럴듯한 호스트를 지어내지 않는다 — 화면이 죽은 링크를
// 안내하느니 링크를 안 거는 편이 낫다.
func TestToolAccessURL_BlankWhenToolUnknown(t *testing.T) {
	assert.Equal(t, "", ToolAccessURL("some-unknown-tool", "nullus.local"))
	assert.Equal(t, "", ToolAccessURL("", "nullus.local"))
}

func TestToolAccessURL_TrimsAccessDomain(t *testing.T) {
	assert.Equal(t, "https://harbor.nullus.local", ToolAccessURL("harbor", "  nullus.local  "))
}

// 설치 목록에 오르는 OSS 는 전부 주소를 가져야 한다.
//
// InstalledToolWorkloads 가 모니터링이 무엇을 보여줄지 정하는 단일 관문이므로,
// 거기에 도구를 추가하면서 호스트 규칙을 빠뜨리면 화면에는 뜨는데 주소만 빈
// 항목이 생긴다. 이 테스트가 그 순간 알려준다.
func TestToolAccessURL_CoversEveryInstalledWorkload(t *testing.T) {
	cfg := StackConfig{
		AccessDomain: "nullus.local",
		Artifacts: ArtifactsConfig{
			SourceRepository:  ToolSelection{Name: "gitlab", Enabled: true},
			ContainerRegistry: ToolSelection{Name: "harbor", Enabled: true},
			PackageRegistry:   ToolSelection{Name: "nexus", Enabled: true},
			StorageBackend:    ToolSelection{Name: "minio", Enabled: true},
		},
		Pipeline: PipelineConfig{
			CIPlatform: ToolSelection{Name: "jenkins", Enabled: true},
			CDTool:     ToolSelection{Name: "argocd", Enabled: true},
		},
		Monitoring: MonitoringConfig{
			Collection:    ToolSelection{Name: "prometheus", Enabled: true},
			Visualization: ToolSelection{Name: "grafana", Enabled: true},
		},
		Logging: LoggingConfig{
			Collection:    ToolSelection{Name: "loki", Enabled: true},
			Search:        ToolSelection{Name: "opensearch", Enabled: true},
			TraceLayer:    ToolSelection{Name: "tempo", Enabled: true},
			TraceExporter: ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
		Authentication: &AuthenticationConfig{Provider: "openbao"},
	}

	workloads := InstalledToolWorkloads(cfg)
	require.NotEmpty(t, workloads)

	for _, w := range workloads {
		assert.NotEmpty(t, ToolAccessURL(w.Name, cfg.AccessDomain),
			"%s(%s) 의 접속 주소 규칙이 없습니다", w.Key, w.Name)
	}
}
