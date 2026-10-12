package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 스캐너의 취약점 DB PVC 는 5Gi 다(실측 사용량 1.3G).
//
// 릴리스만 지우고 PVC 를 남기면 다음 설치가 "이전 설치의 볼륨이 남아 있습니다"
// 검증에 걸려 막히고, 사용자는 스캐너를 고른 적도 없는데 그 이름의 볼륨을
// 손으로 지워야 한다.
func TestLegacyReleaseArtifacts_IncludeTrivyDataVolume(t *testing.T) {
	_, ok := legacyReleaseArtifactExactNames["data-trivy-0"]
	assert.True(t, ok,
		"data-trivy-0 이 삭제 목록에 없으면 스택을 지워도 취약점 DB 볼륨이 남는다")
}
