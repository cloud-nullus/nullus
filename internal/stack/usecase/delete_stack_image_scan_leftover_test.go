package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 설치 이미지 스캔 Job 은 스택이 만든 것이지만 Helm 릴리스 소유가 아니다. 스캔 도중
// API 가 내려가면 끝난 Job 이 남는다(완료 1시간 뒤 TTL 로 사라지지만, 그 사이 스택을
// 지우면 사용자가 고른 네임스페이스에 남는다).
func TestIsInstallLeftoverArtifact_ImageScanJob(t *testing.T) {
	scanLabels := map[string]string{"app.kubernetes.io/managed-by": "nullus-image-scan"}

	assert.True(t, isInstallLeftoverArtifact(namespacedResource{Ref: "job/nullus-image-scan", Labels: scanLabels}))
	// 같은 이름이라도 표시가 없으면 사용자 것일 수 있다 — 지우지 않는다.
	assert.False(t, isInstallLeftoverArtifact(namespacedResource{Ref: "job/nullus-image-scan"}))
	assert.False(t, isInstallLeftoverArtifact(namespacedResource{Ref: "configmap/nullus-image-scan", Labels: scanLabels}))
}
