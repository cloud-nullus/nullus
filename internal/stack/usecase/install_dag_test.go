package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonicalInstallOrder 는 설치 DAG 가 실행해야 하는 정식 순서다.
// helm.Orchestrator 의 orderedStep 과 반드시 일치해야 한다 —
// 오케스트레이터의 ensureOrder 가 이 순서를 강제하기 때문이다.
// (helm 패키지 쪽 대칭 테스트: TestOrderedStep_MatchesCanonicalInstallOrder)
var canonicalInstallOrder = []string{
	"installing_cert_manager",
	"installing_metrics_server",
	"installing_openbao",
	"installing_external_secrets",
	"provisioning_secrets",
	"installing_postgresql",
	"installing_minio",
	"installing_object_storage_secret",
	"installing_object_storage_buckets",
	"installing_database_connection_check",
	"provisioning_sso",
	"installing_gitlab",
	"installing_argocd",
	"installing_runner",
	"installing_prometheus",
	"installing_grafana",
	"installing_logging",
	"installing_log_search",
	"installing_opentelemetry",
	"installing_gateway",
	"integration_check",
}

func dagStepNames() []string {
	names := make([]string, 0, len(installDAG))
	for _, step := range installDAG {
		names = append(names, step.name)
	}
	return names
}

func dagStepByName(t *testing.T, name string) installStep {
	t.Helper()
	for _, step := range installDAG {
		if step.name == name {
			return step
		}
	}
	require.Failf(t, "step not found in installDAG", "step=%s", name)
	return installStep{}
}

func dagIndexOf(t *testing.T, name string) int {
	t.Helper()
	for i, step := range installDAG {
		if step.name == name {
			return i
		}
	}
	require.Failf(t, "step not found in installDAG", "step=%s", name)
	return -1
}

// 시크릿 평면(OpenBao → ESO → provisioning) 단계가 DAG 에 존재해야 한다.
// 빠져 있으면 차트가 참조하는 existingSecret 을 아무도 만들지 않아
// PostgreSQL/MinIO 파드가 FailedMount 로 영원히 기동하지 못한다.
func TestInstallDAG_ContainsSecretPlaneSteps(t *testing.T) {
	names := dagStepNames()

	assert.Contains(t, names, "installing_openbao")
	assert.Contains(t, names, "installing_external_secrets")
	assert.Contains(t, names, "provisioning_secrets")
}

func TestInstallDAG_MatchesCanonicalOrder(t *testing.T) {
	assert.Equal(t, canonicalInstallOrder, dagStepNames())
}

// provisioning_secrets 가 만든 Secret 을 PostgreSQL/MinIO 차트가
// existingSecret 으로 참조하므로 반드시 먼저 완료되어야 한다.
func TestInstallDAG_SecretProvisioningPrecedesStorage(t *testing.T) {
	provisioning := dagIndexOf(t, "provisioning_secrets")

	for _, consumer := range []string{"installing_postgresql", "installing_minio"} {
		assert.Less(t, provisioning, dagIndexOf(t, consumer),
			"provisioning_secrets must precede %s in list order", consumer)
	}

	postgres := dagStepByName(t, "installing_postgresql")
	assert.Contains(t, postgres.deps, "provisioning_secrets",
		"postgresql must depend on provisioning_secrets")
}

// OpenBao 는 file 스토리지 백엔드를 쓰므로 PostgreSQL/MinIO 에 의존하지 않는다.
// 의존하면 시크릿 평면을 스토리지 앞으로 옮길 수 없어 교착이 된다.
func TestInstallDAG_OpenBaoDoesNotDependOnStorage(t *testing.T) {
	openbao := dagStepByName(t, "installing_openbao")

	assert.NotContains(t, openbao.deps, "installing_postgresql")
	assert.NotContains(t, openbao.deps, "installing_minio")
}

// 시크릿 평면 내부 순서는 강제된다 — init 이 만든 root token 으로
// 부트스트랩이 인증을 구성하고, 그 role 이 있어야 ESO 가 로그인한다.
func TestInstallDAG_SecretPlaneInternalOrder(t *testing.T) {
	eso := dagStepByName(t, "installing_external_secrets")
	assert.Contains(t, eso.deps, "installing_openbao")

	provisioning := dagStepByName(t, "provisioning_secrets")
	assert.Contains(t, provisioning.deps, "installing_external_secrets")
}

// DAG 의 모든 의존은 DAG 안에 존재하는 단계여야 하고, 자기보다 앞서야 한다.
func TestInstallDAG_DependenciesResolveInOrder(t *testing.T) {
	for i, step := range installDAG {
		for _, dep := range step.deps {
			depIdx := dagIndexOf(t, dep)
			assert.Less(t, depIdx, i,
				"step %s depends on %s which must appear earlier", step.name, dep)
		}
	}
}
