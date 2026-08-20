package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 2026-08-20 운영 사고 재현.
//
// Gitea 가 "password authentication failed for user gitlab (28P01)" 로 기동하지
// 못했다. 비밀번호의 출처(OpenBao → Secret)와 그것이 구워지는 곳(PostgreSQL 데이터
// 디렉터리)의 수명이 따로 논다 — 볼륨이 재사용되거나 금고가 새로 초기화되면 둘이
// 조용히 갈라지고, 설치는 여섯 단계쯤 더 간 뒤 Gitea 에서야 드러난다.
func TestPostgresRoleSync_AltersAppRoleToSecretValue(t *testing.T) {
	manifest := postgresRoleSyncManifest("nullus-demo", "demo-stack")

	assert.Contains(t, manifest, "kind: Job")
	assert.Contains(t, manifest, "namespace: nullus-demo")
	assert.Contains(t, manifest, `ALTER ROLE "`+domain.PostgresAppUser+`"`)
	assert.Contains(t, manifest, domain.PostgresServiceName+".nullus-demo.svc.cluster.local")
}

// 비밀번호를 매니페스트에 적으면 로그·이벤트·helm 히스토리에 그대로 남는다.
func TestPostgresRoleSync_TakesPasswordsFromSecretOnly(t *testing.T) {
	manifest := postgresRoleSyncManifest("nullus-demo", "demo-stack")

	assert.Contains(t, manifest, "secretKeyRef")
	assert.Contains(t, manifest, "name: "+domain.ProvisionedPostgresSecret)
	assert.Contains(t, manifest, "key: "+domain.PostgresPasswordKey)
	assert.Contains(t, manifest, "key: postgres-password")

	// psql 의 변수 인용을 쓴다 — 값을 SQL 문자열에 이어 붙이면 따옴표가 든
	// 비밀번호에서 구문이 깨지고, 명령줄에 남아 ps 로도 보인다.
	assert.Contains(t, manifest, `:'pw'`)
	assert.NotContains(t, manifest, "PASSWORD '$")
}

// 스택을 지울 때 함께 회수돼야 한다. 라벨이 없으면 Job 이 남는다.
func TestPostgresRoleSync_CarriesStackLabel(t *testing.T) {
	manifest := postgresRoleSyncManifest("nullus-demo", "demo-stack")

	assert.Contains(t, manifest, "nullus.io/stack-name: demo-stack")
}

// 차트가 쓰는 이미지와 갈라지면 에어갭 번들에 없는 이미지를 끌어오게 된다.
func TestPostgresRoleSync_UsesSameImageAsChart(t *testing.T) {
	manifest := postgresRoleSyncManifest("nullus-demo", "demo-stack")

	values, ok := chartSpecValuesForStep(t, "installing_postgresql")
	require.True(t, ok)
	image, ok := values["image"].(map[string]any)
	require.True(t, ok, "차트 values 에 image 가 없다")

	ref := image["registry"].(string) + "/" + image["repository"].(string) + ":" + image["tag"].(string)
	assert.Contains(t, manifest, "image: "+ref)
}

func chartSpecValuesForStep(t *testing.T, step string) (map[string]any, bool) {
	t.Helper()
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-demo")
	spec, ok := o.chartSpecForStep(step)
	if !ok {
		return nil, false
	}
	return spec.Values, true
}

// 이 단계는 기다렸다가 고친다. PostgreSQL 이 아직 안 떴는데 붙으면 실패한다.
func TestPostgresRoleSync_WaitsForDatabase(t *testing.T) {
	manifest := postgresRoleSyncManifest("nullus-demo", "demo-stack")

	assert.True(t, strings.Contains(manifest, "pg_isready"),
		"DB 가 준비될 때까지 기다려야 한다")
}
