package helm

import (
	"strings"
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

// kubeconfig 가 없으면(로컬 단위 테스트 경로) 검사를 건너뛴다.
func TestPreflightNamespace_SkipsWithoutKubeconfig(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")

	assert.NoError(t, o.PreflightNamespace(t.Context(), "nullus-demo"))
}

// 복구가 놓아둔 볼륨은 막지 않는다 — 복구는 볼륨과 Secret·금고를 같은 시점으로
// 함께 되돌리므로, preflight 가 막으려는 "비밀번호 어긋남" 이 생기지 않는다.
func TestClassifyLeftovers_복구된_볼륨은_통과시킨다(t *testing.T) {
	out := classifyLeftovers(pvcListJSON(
		pvcJSON("gitea-shared-storage", "bk-1"),
		pvcJSON("jenkins", "bk-1"),
	))
	assert.ElementsMatch(t, []string{"gitea-shared-storage", "jenkins"}, out.Names)
	assert.True(t, out.AllRestoredFromSameBackup, "같은 백업에서 온 볼륨만 있으면 설치를 막을 이유가 없다")
	assert.Equal(t, "bk-1", out.BackupRunID)
}

func TestClassifyLeftovers_표시가_없으면_막는다(t *testing.T) {
	out := classifyLeftovers(pvcListJSON(
		pvcJSON("gitea-shared-storage", "bk-1"),
		pvcJSON("jenkins", ""), // 실패한 설치가 남긴 것
	))
	assert.False(t, out.AllRestoredFromSameBackup,
		"하나라도 출처가 없으면 어느 시점의 상태인지 말할 수 없다")
}

func TestClassifyLeftovers_서로_다른_백업이_섞이면_막는다(t *testing.T) {
	out := classifyLeftovers(pvcListJSON(
		pvcJSON("gitea-shared-storage", "bk-1"),
		pvcJSON("jenkins", "bk-2"),
	))
	assert.False(t, out.AllRestoredFromSameBackup,
		"두 백업이 섞인 볼륨은 일관된 시점이 아니다")
}

func TestClassifyLeftovers_볼륨이_없으면_통과(t *testing.T) {
	out := classifyLeftovers(pvcListJSON())
	assert.Empty(t, out.Names)
}

func pvcListJSON(items ...string) string {
	return `{"items":[` + strings.Join(items, ",") + `]}`
}

func pvcJSON(name, restoredFrom string) string {
	ann := ""
	if restoredFrom != "" {
		ann = `,"annotations":{"backup.nullus.io/restored-from":"` + restoredFrom + `"}`
	}
	return `{"metadata":{"name":"` + name + `"` + ann + `}}`
}
