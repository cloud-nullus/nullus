package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 스택 정책 변수를 읽는 단계는 넷이다 — 이미지 스캔·SAST·시크릿 탐지(#80)·의존성 스캔(#79).
// 시크릿 탐지와 의존성 스캔을 빠뜨리면 정책을 경고·끄기로 바꿔도 그 단계는 기본값(차단)으로 돈다.
func TestReadsScanPolicy_AllPolicyStages(t *testing.T) {
	for _, stages := range [][]string{
		{"Build", "ImageScan", "Deploy"},
		{"Build", "SAST", "Deploy"},
		{"SecretScan", "Build", "Deploy"},
		{"DependencyScan", "Build", "Deploy"},
		{"dependency-scan", "build", "deploy"}, // GitLab 잡 키 표기도 같은 단계다
	} {
		assert.True(t, readsScanPolicy(&domain.Pipeline{Stages: stages}), "%v", stages)
	}
	assert.False(t, readsScanPolicy(&domain.Pipeline{Stages: []string{"Build", "Deploy"}}),
		"단계 기록이 없는 옛 파이프라인에는 싣지 않는다")
}
