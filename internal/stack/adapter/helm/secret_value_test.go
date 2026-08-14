package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 생성한 비밀번호는 CLI 인자로 그대로 넘어간다.
//
// base64.RawURLEncoding 알파벳(A-Za-z0-9-_)은 '-' 로 시작하는 값을 만들 수 있고,
// 그러면 mc·gitea 같은 CLI 가 이를 플래그로 파싱해 죽는다. 실제로 MinIO
// post-install 잡이 이렇게 실패했다:
//
//	mc: <ERROR> Invalid command usage, flag provided but not defined: -M-7HMgh...
//
// 확률적이라 어떤 설치는 통과하고 어떤 설치는 실패한다 — 재현이 어려운 형태다.
func TestGenerateSecretValue_IsSafeAsCLIArgument(t *testing.T) {
	// 확률적 결함이므로 여러 번 돌려 알파벳 자체를 검증한다.
	for i := 0; i < 200; i++ {
		v, err := generateSecretValue()
		require.NoError(t, err)
		require.NotEmpty(t, v)

		assert.NotEqual(t, "-", v[:1],
			"'-' 로 시작하면 CLI 가 플래그로 파싱한다")
		for _, r := range v {
			isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			assert.Truef(t, isAlnum,
				"영숫자 외 문자 %q 는 CLI·설정 파일 파싱에서 문제를 일으킨다 (값: %s)", r, v)
		}
	}
}

// 길이를 줄이면 엔트로피가 함께 줄어든다.
func TestGenerateSecretValue_KeepsLength(t *testing.T) {
	v, err := generateSecretValue()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(v), 40)
}

func TestGenerateSecretValue_IsRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		v, err := generateSecretValue()
		require.NoError(t, err)
		assert.False(t, seen[v], "같은 값이 두 번 나오면 안 된다")
		seen[v] = true
	}
	assert.Len(t, seen, 50)
}

func TestGenerateSecretValue_NoWhitespace(t *testing.T) {
	v, err := generateSecretValue()
	require.NoError(t, err)
	assert.Equal(t, v, strings.TrimSpace(v))
}
