package port

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 플랫폼이 CI 에 싣는 값과 스크립트가 읽는 이름이 한 곳에서 나와야 한다.
func TestScanPolicyVariables(t *testing.T) {
	vars := ScanPolicyVariables(domain.ScanPolicy{
		BlockSeverity:        domain.SeverityHigh,
		IgnoreUnfixed:        false,
		OnScannerUnreachable: domain.UnreachableAllow,
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
	}, got)
}
