package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 2026-08-20~21 운영에서 같은 원인이 두 번 다른 얼굴로 나왔다. 이전 설치의 볼륨이
// 남아 옛 데이터베이스를 물려받았고, PostgreSQL 은 Gitea 의 28P01 로, Harbor 는
// 프로비저닝 401 로 드러났다 — 둘 다 20분을 태운 뒤였다.
func TestLeftoverVolumeError_SaysWhatBreaksAndHowToFix(t *testing.T) {
	err := leftoverVolumeError("nullus-demo", []string{"data-nullus-postgresql-0", "database-data-harbor-database-0"})
	require.Error(t, err)

	message := err.Error()
	assert.Contains(t, message, "nullus-demo")
	assert.Contains(t, message, "data-nullus-postgresql-0")
	assert.Contains(t, message, "database-data-harbor-database-0")
	// 무엇이 깨지는지
	assert.Contains(t, message, "옛 데이터베이스")
	// 어떻게 고치는지 — 두 가지 길을 다 준다
	assert.Contains(t, message, "kubectl delete namespace nullus-demo")
	assert.Contains(t, message, "kubectl -n nullus-demo delete pvc data-nullus-postgresql-0 database-data-harbor-database-0")
}

func TestParsePVCNames_ReadsKubectlNameOutput(t *testing.T) {
	names := parsePVCNames("persistentvolumeclaim/data-nullus-postgresql-0\npersistentvolumeclaim/gitea-shared-storage\n")

	assert.Equal(t, []string{"data-nullus-postgresql-0", "gitea-shared-storage"}, names)
}

func TestParsePVCNames_EmptyOutputYieldsNothing(t *testing.T) {
	assert.Empty(t, parsePVCNames("  \n "))
}

// kubeconfig 가 없으면(로컬 단위 테스트 경로) 검사를 건너뛴다.
func TestPreflightNamespace_SkipsWithoutKubeconfig(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")

	assert.NoError(t, o.PreflightNamespace(t.Context(), "nullus-demo"))
}
