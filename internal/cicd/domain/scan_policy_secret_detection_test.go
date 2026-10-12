package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 기본은 차단이다 — 시크릿이 든 소스로 이미지를 만들어 올리는 것부터 막는다.
func TestDefaultScanPolicy_SecretDetectionBlocks(t *testing.T) {
	assert.Equal(t, SecretDetectionBlock, DefaultScanPolicy().SecretDetection)
}

// 이 필드를 모르는 옛 클라이언트·저장 행은 빈 값이다. 빈 값을 경고나 끄기로 읽으면 조용히 게이트가 풀린다.
func TestScanPolicy_SecretDetectionOrDefault(t *testing.T) {
	assert.Equal(t, SecretDetectionBlock, ScanPolicy{}.SecretDetectionOrDefault())
	assert.Equal(t, SecretDetectionWarn, ScanPolicy{SecretDetection: SecretDetectionWarn}.SecretDetectionOrDefault())
	assert.Equal(t, SecretDetectionOff, ScanPolicy{SecretDetection: SecretDetectionOff}.SecretDetectionOrDefault())
}

// 모르는 값을 기본값으로 바꿔 받지 않는다 — 오타(disable)가 조용히 차단으로 바뀌면 운영자는 꺼진 줄 안다.
func TestScanPolicy_ValidateSecretDetection(t *testing.T) {
	for _, v := range []SecretDetectionAction{"", SecretDetectionBlock, SecretDetectionWarn, SecretDetectionOff} {
		p := DefaultScanPolicy()
		p.SecretDetection = v
		require.NoError(t, p.Validate(), string(v))
	}
	p := DefaultScanPolicy()
	p.SecretDetection = "disable"
	err := p.Validate()
	require.ErrorIs(t, err, ErrInvalidScanPolicy)
	assert.Contains(t, err.Error(), "block·warn·off")
}
