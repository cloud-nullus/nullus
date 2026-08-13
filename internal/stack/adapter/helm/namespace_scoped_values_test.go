package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// nested 는 렌더된 values 에서 중첩 경로의 값을 꺼낸다.
func nested(t *testing.T, values map[string]any, path ...string) any {
	t.Helper()

	var current any = values
	for _, key := range path {
		m, ok := current.(map[string]any)
		require.Truef(t, ok, "경로 %v 중간이 매핑이 아닙니다", path)
		current, ok = m[key]
		require.Truef(t, ok, "경로 %v 가 없습니다", path)
	}
	return current
}

func orchestratorWithOverride(namespace, step, override string) *Orchestrator {
	o := NewOrchestrator(nil, nil, namespace)
	o.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			SourceRepository: domain.ToolSelection{Name: "gitlab", Enabled: true},
			StorageBackend:   domain.ToolSelection{Name: "minio", Enabled: true},
		},
		YAMLOverrides: map[string]string{step: override},
	})
	return o
}

// release values 의 live 편집은 플랫폼이 계산해 넣은 값까지 그대로 오버라이드에
// 얼려 담는다. 거기 담긴 DB 주소는 그 시점 네임스페이스에 묶여 있어, 설정을
// 다른 네임스페이스로 옮기면 삭제된 스택의 PostgreSQL 을 가리키게 된다.
func TestValuesForStep_GitLabDatabaseHostFollowsStackNamespace(t *testing.T) {
	o := orchestratorWithOverride("nullus-new", "installing_gitlab", `
global:
  psql:
    host: nullus-postgresql.nullus-old.svc.cluster.local
`)

	spec, ok := o.chartSpecForStep("installing_gitlab")
	require.True(t, ok)
	values := o.valuesForStep("installing_gitlab", spec)

	assert.Equal(t,
		"nullus-postgresql.nullus-new.svc.cluster.local",
		nested(t, values, "global", "psql", "host"),
		"오버라이드에 얼어붙은 옛 네임스페이스가 남으면 다른 스택의 DB 를 가리킨다")
}

func TestValuesForStep_MinioNamespaceFollowsStackNamespace(t *testing.T) {
	o := orchestratorWithOverride("nullus-new", "installing_minio", "namespace: nullus-old\n")

	spec, ok := o.chartSpecForStep("installing_minio")
	require.True(t, ok)
	values := o.valuesForStep("installing_minio", spec)

	assert.Equal(t, "nullus-new", values["namespace"])
}

func TestValuesForStep_RunnerGitLabURLFollowsStackNamespace(t *testing.T) {
	o := orchestratorWithOverride("nullus-new", stepInstallingRunner,
		"gitlabUrl: http://gitlab-webservice-default.nullus-old.svc:8181\n")

	spec, ok := o.chartSpecForStep(stepInstallingRunner)
	require.True(t, ok)
	values := o.valuesForStep(stepInstallingRunner, spec)

	assert.Equal(t, "http://gitlab-webservice-default.nullus-new.svc:8181", values["gitlabUrl"])
}

// 배선이 아닌 값은 사용자 의도다. 네임스페이스를 못박느라 이것까지 되돌리면
// 오버라이드 기능 자체가 무의미해진다.
func TestValuesForStep_KeepsUserOverridesThatAreNotNamespaceScoped(t *testing.T) {
	o := orchestratorWithOverride("nullus-new", "installing_gitlab", `
global:
  psql:
    host: nullus-postgresql.nullus-old.svc.cluster.local
gitlab:
  webservice:
    resources:
      limits:
        memory: 7Gi
`)

	spec, ok := o.chartSpecForStep("installing_gitlab")
	require.True(t, ok)
	values := o.valuesForStep("installing_gitlab", spec)

	assert.Equal(t, "7Gi", nested(t, values, "gitlab", "webservice", "resources", "limits", "memory"))
}

// GitLab 차트는 자체 Prometheus 를 함께 세운다. 스택은 이미 kube-prometheus-stack
// 을 설치하고 화면·게이트웨이·대시보드가 모두 그쪽을 보므로, 번들 쪽은 아무도
// 읽지 않으면서 메모리만 먹고 스택의 자원 계획 아래에서 OOMKilled 로 죽는다.
func TestValuesForStep_DisablesGitLabBundledPrometheus(t *testing.T) {
	o := NewOrchestrator(nil, nil, "nullus-new")
	o.SetStackConfig(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			SourceRepository: domain.ToolSelection{Name: "gitlab", Enabled: true},
		},
	})

	spec, ok := o.chartSpecForStep("installing_gitlab")
	require.True(t, ok)
	values := o.valuesForStep("installing_gitlab", spec)

	assert.Equal(t, false, nested(t, values, "prometheus", "install"))
}

// 오버라이드로도 되살아나면 안 된다 — 옛 스택에서 얼려 온 설정에는
// prometheus 블록이 그대로 들어 있다.
func TestValuesForStep_KeepsBundledPrometheusOffUnderStaleOverride(t *testing.T) {
	o := orchestratorWithOverride("nullus-new", "installing_gitlab", `
prometheus:
  install: true
  server:
    resources:
      limits:
        memory: 328Mi
`)

	spec, ok := o.chartSpecForStep("installing_gitlab")
	require.True(t, ok)
	values := o.valuesForStep("installing_gitlab", spec)

	assert.Equal(t, false, nested(t, values, "prometheus", "install"))
}
