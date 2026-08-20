package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 2026-08-20 운영 사고.
//
// 스택을 지웠는데 PVC 가 남았고(파드가 아직 물고 있어 삭제가 타임아웃), 경고 한
// 줄만 남긴 채 삭제가 성공으로 끝났다. 사용자는 깨끗이 지워진 줄 알고 같은
// 네임스페이스에 다시 설치했고, 옛 데이터베이스를 물려받은 PostgreSQL 의
// 비밀번호가 새 Secret 과 어긋나 Gitea 가 28P01 로 기동하지 못했다.
func TestRemainingPVCMessage_SaysWhatItBreaksAndHowToFix(t *testing.T) {
	message := remainingPVCMessage("nullus-demo", []string{"data-nullus-postgresql-0", "gitea-shared-storage"})

	assert.Contains(t, message, "nullus-demo")
	assert.Contains(t, message, "data-nullus-postgresql-0")
	assert.Contains(t, message, "gitea-shared-storage")
	// 무엇이 깨지는지
	assert.Contains(t, message, "옛 데이터베이스")
	// 어떻게 고치는지
	assert.Contains(t, message, "kubectl -n nullus-demo delete pvc data-nullus-postgresql-0 gitea-shared-storage")
}

func TestParseResourceNames_ReadsKubectlNameOutput(t *testing.T) {
	names := parseResourceNames("persistentvolumeclaim/data-nullus-postgresql-0\npersistentvolumeclaim/gitea-shared-storage\n")

	assert.Equal(t, []string{"data-nullus-postgresql-0", "gitea-shared-storage"}, names)
}

func TestParseResourceNames_EmptyOutputYieldsNothing(t *testing.T) {
	assert.Empty(t, parseResourceNames("   \n"))
}
