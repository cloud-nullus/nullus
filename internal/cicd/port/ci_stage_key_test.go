package port

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// CI 마다 같은 단계를 다르게 적는다 — Jenkins 는 stage('ImageScan'),
// GitLab·GitHub 은 잡 키 image-scan. 대소문자만 맞추면 서로 다른 단계로 보여
// 스캔 판정이 기록되지 않고 화면의 단계도 "모름" 으로 남는다.
func TestStageKey_MatchesAcrossCIVocabularies(t *testing.T) {
	want := StageKey("ImageScan")
	for _, name := range []string{"image-scan", "image_scan", "Image Scan", " imagescan "} {
		assert.Equal(t, want, StageKey(name), name)
	}
	assert.NotEqual(t, StageKey("Build"), StageKey("Deploy"))
}
