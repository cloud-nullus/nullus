package port

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 플랫폼이 CI 에 싣는 값과 스크립트가 읽는 이름이 한 곳에서 나와야 한다.
func TestScanPolicyVariables(t *testing.T) {
	vars := ScanPolicyVariables(domain.ScanPolicy{
		BlockSeverity:        domain.SeverityHigh,
		IgnoreUnfixed:        false,
		OnScannerUnreachable: domain.UnreachableAllow,
		SASTOnGateFailure:    domain.SASTGateWarn,
	})

	got := map[string]string{}
	for _, v := range vars {
		got[v.Key] = v.Value
		// 정책은 비밀이 아니다. GitLab 은 8자 미만 값(true·allow)의 마스킹 등록을
		// 통째로 거부하므로 가리면 변수가 아예 안 생긴다.
		assert.False(t, v.Masked, v.Key)
	}
	assert.Equal(t, map[string]string{
		ScanSeverityVariable:      "HIGH,CRITICAL",
		ScanIgnoreUnfixedVariable: "false",
		ScanOnUnreachableVariable: "allow",
		SASTOnGateFailureVariable: "warn",
		// 이미지도 정책과 함께 민다 — 플랫폼이 이미지를 올리면(예: 멀티아치 스캐너) 파이프라인
		// 정의를 다시 쓰지 않아도 다음 실행부터 따라온다. GitLab 은 프로젝트 변수가 YAML 전역
		// 변수보다 앞서고, GitHub 워크플로는 vars 를 먼저 본다.
		ScanImageVariable: shareddomain.TrivyCLIImage,
		SASTImageVariable: shareddomain.SonarScannerImage,
	}, got)
}
