// 차트가 백업 설정을 실제로 렌더하는지 본다.
//
// 이 테스트가 있는 이유: 백업 기능은 단위·통합 테스트와 축소 리허설을 전부
// 통과하고 CI 도 초록이었는데, **차트로는 켤 방법 자체가 없었다.**
// templates/configmap.yaml 이 명시적 allow-list 라 backup: 블록이 렌더되지
// 않았고, 그것은 코드가 아니라 배선의 결함이라 코드 테스트로는 원리적으로
// 잡히지 않는다. 인클러스터 리허설에서야 드러났다.
//
// helm 이 없는 환경에서는 건너뛴다. CI 에는 helm 을 설치해 실제로 돌린다 —
// 돌지 않는 회귀 테스트는 없는 것과 같다.
package tests

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const chartDir = ".."

func render(t *testing.T, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm 이 없어 건너뜁니다")
	}
	cmd := exec.Command("helm", append([]string{"template", "t", chartDir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "helm template 실패: %s", string(out))
	return string(out)
}

func renderExpectError(t *testing.T, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm 이 없어 건너뜁니다")
	}
	cmd := exec.Command("helm", append([]string{"template", "t", chartDir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "실패했어야 하는데 성공했다: %s", string(out))
	return string(out)
}

func enabled() []string {
	return []string{
		"--set", "config.backup.enabled=true",
		"--set", "secrets.backupSealKey=seal-key-32bytes-padding-value!",
		"--set", "secrets.backupDestinationSecretKey=dest-secret",
		"--set", "config.backup.destination.endpoint=minio.internal:9000",
		"--set", "config.backup.destination.accessKey=ak",
		"--set", "config.backup.keycloakDatabase.host=nullus-keycloak-postgresql",
	}
}

func TestChart_백업을_켜면_설정이_렌더된다(t *testing.T) {
	out := render(t, enabled()...)

	assert.Contains(t, out, "backup:", "ConfigMap 에 backup 블록이 있어야 한다")
	assert.Contains(t, out, "enabled: true")
	assert.Contains(t, out, "endpoint: \"minio.internal:9000\"")
	assert.Contains(t, out, "keycloak_database:")
	// 코드는 snake_case 로 읽는다 (internal/shared/config).
	assert.Contains(t, out, "seal_key_id:")
	assert.Contains(t, out, "access_key:")
	assert.Contains(t, out, "use_ssl:")
	assert.Contains(t, out, "max_total_bytes:")
}

// 설계 §5.2 — ConfigMap 은 RBAC 이 느슨하고 그대로 로그·백업본에 실려 나가기
// 쉽다. 봉인 키와 목적지 자격증명은 거기 두지 않는다.
func TestChart_비밀값은_ConfigMap_에_담지_않는다(t *testing.T) {
	out := render(t, enabled()...)

	cm := section(out, "kind: ConfigMap")
	require.NotEmpty(t, cm, "ConfigMap 을 찾지 못했다")
	assert.NotContains(t, cm, "seal-key-32bytes-padding-value!",
		"봉인 키가 ConfigMap 에 평문으로 들어갔다")
	assert.NotContains(t, cm, "dest-secret",
		"목적지 secret key 가 ConfigMap 에 평문으로 들어갔다")
	assert.NotContains(t, cm, "secret_key:")
}

func TestChart_비밀값은_Secret_에서_환경변수로_주입된다(t *testing.T) {
	out := render(t, enabled()...)

	assert.Contains(t, out, "backup-seal-key:", "Secret 에 봉인 키가 있어야 한다")
	assert.Contains(t, out, "backup-destination-secret-key:")
	// 코드는 viper BindEnv 로 이 이름들을 읽는다 (internal/shared/config).
	assert.Contains(t, out, "NULLUS_BACKUP_SEAL_KEY")
	assert.Contains(t, out, "NULLUS_BACKUP_DESTINATION_SECRET_KEY")
}

func TestChart_봉인_키가_없으면_렌더를_막는다(t *testing.T) {
	// 켜 놓고 키를 빠뜨리면 파드는 뜨는데 백업 모듈만 조용히 꺼진다.
	// 그 상태가 "백업이 돌고 있다" 는 착각을 만든다.
	out := renderExpectError(t,
		"--set", "config.backup.enabled=true",
		"--set", "config.backup.keycloakDatabase.host=h",
	)
	assert.Contains(t, out, "secrets.backupSealKey")
}

func TestChart_기본값은_꺼져_있고_비밀값을_요구하지_않는다(t *testing.T) {
	out := render(t)
	assert.Contains(t, out, "enabled: false")
	assert.NotContains(t, out, "backup-seal-key:")
	assert.NotContains(t, out, "NULLUS_BACKUP_SEAL_KEY")
}

// section 은 지정한 표식이 들어 있는 YAML 문서 하나를 돌려준다.
func section(manifest, marker string) string {
	for _, doc := range strings.Split(manifest, "\n---\n") {
		if strings.Contains(doc, marker) {
			return doc
		}
	}
	return ""
}
