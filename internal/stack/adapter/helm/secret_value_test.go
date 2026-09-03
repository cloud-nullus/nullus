package helm

import (
	"strings"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
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

// Harbor 의 secretKey 는 길이가 정해져 있다 — AES-128 키라 정확히 16자여야 하고,
// 기본 43자를 넣으면 Harbor core 가 기동 시 죽는다.
func TestGenerateSecretValueN_지정한_길이를_지킨다(t *testing.T) {
	for _, n := range []int{16, 32, 43} {
		v, err := generateSecretValueN(n)
		require.NoError(t, err)
		assert.Len(t, v, n)
	}
}

func TestGenerateSecretValueN_길이가_0_이하면_기본값(t *testing.T) {
	v, err := generateSecretValueN(0)
	require.NoError(t, err)
	assert.Len(t, v, secretValueLength)
}

func TestManagedSecrets_Harbor_암호화_키를_금고에서_받는다(t *testing.T) {
	// Harbor 는 이 키로 로봇 계정 자격증명과 레플리케이션 엔드포인트 비밀번호를
	// **DB 에 암호화해 넣는다.** 차트 기본값(not-a-secure-key)은 공개돼 있으므로,
	// 그 상태의 DB 덤프는 누구나 풀 수 있다 — 백업 산출물이 곧 평문이 된다.
	//
	// 금고를 SoT 로 두면 그 결함이 사라지고, 동시에 복구가 같은 키를 되돌려
	// 주므로 재설치해도 복원된 DB 를 읽을 수 있다.
	var harbor *ManagedSecret
	for i := range managedSecretsFixture() {
		if managedSecretsFixture()[i].TargetSecret == domain.HarborAdminSecret {
			harbor = &managedSecretsFixture()[i]
		}
	}
	require.NotNil(t, harbor, "Harbor 관리 시크릿이 없다")

	var key *SecretEntry
	for i := range harbor.Entries {
		if harbor.Entries[i].TargetKey == domain.HarborSecretKeyKey {
			key = &harbor.Entries[i]
		}
	}
	require.NotNil(t, key, "secretKey 엔트리가 없으면 차트 기본값이 그대로 쓰인다")
	assert.Equal(t, domain.HarborSecretKeyLength, key.Length,
		"Harbor 는 정확히 %d 자를 요구한다", domain.HarborSecretKeyLength)
	assert.Empty(t, key.Fixed, "고정값이면 모든 설치가 같은 키를 쓴다")
}

func managedSecretsFixture() []ManagedSecret { return managedSecrets("nullus-app") }

func TestDefaultValues_Harbor_가_금고의_암호화_키를_참조한다(t *testing.T) {
	// Secret 을 만들어 두기만 하고 values 를 주지 않으면 아무 일도 일어나지
	// 않는다 — 차트는 계속 기본값 not-a-secure-key 를 쓴다.
	v := DefaultValues("installing_harbor")
	require.NotNil(t, v, "installing_harbor 의 기본 values 가 없다")

	assert.Equal(t, domain.HarborAdminSecret, v["existingSecretSecretKey"],
		"existingSecretSecretKey 가 없으면 차트 기본 키가 그대로 쓰인다")
	assert.NotContains(t, v, "secretKey",
		"평문 secretKey 를 values 에 넣으면 Helm 릴리스 Secret 에 그대로 남는다")
}
