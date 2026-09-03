package helm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Runner 토큰 발견이 실패하면 예전에는 스텝을 completed 로 마킹하고 조용히
// 넘어갔다. 그 결과 CI 실행기가 없는데도 스택이 completed 로 끝나
// "설치는 성공했는데 파이프라인이 하나도 안 도는" 상태가 만들어졌다.
// 실패는 실패로 드러나야 재시도 경로를 탈 수 있다.

func TestRunnerTokenFailure_IsWrappedAsStepFailure(t *testing.T) {
	err := wrapRunnerTokenDiscoveryError(errors.New("kubectl exec failed: pods \"gitlab-toolbox\" not found"))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunnerTokenDiscovery)

	msg := err.Error()
	// 운영자가 무엇을 봐야 하는지 메시지에 남긴다.
	assert.Contains(t, msg, "gitlab-toolbox")
	assert.Contains(t, msg, "runner")
}

func TestErrRunnerTokenDiscovery_IsRetryableSignal(t *testing.T) {
	// 스택 재시도(retry) 경로가 이 스텝부터 다시 시작할 수 있어야 하므로
	// 일반 설치 오류와 구분 가능한 sentinel 이어야 한다.
	err := wrapRunnerTokenDiscoveryError(errors.New("context deadline exceeded"))
	assert.True(t, errors.Is(err, ErrRunnerTokenDiscovery))

	// nil 은 nil 로 통과시킨다 — 호출부에서 분기를 단순하게 유지한다.
	assert.NoError(t, wrapRunnerTokenDiscoveryError(nil))
}

// 토큰에는 두 종류가 있고 차트에서 쓰는 자리가 다르다.
//
//   - 등록 토큰  → runnerRegistrationToken. 러너가 **스스로 등록**한다.
//   - 인증 토큰  → runnerToken. 이미 등록된 러너의 자격증명이다.
//
// 인증 토큰을 등록 토큰 자리에 넣거나 그 반대로 넣으면 러너가 기동하지
// 못한다. 실환경에서 인증 토큰이 runnerToken 으로 들어간 뒤 그 러너가
// GitLab 에서 사라지자, 러너는 등록이 아니라 검증을 시도하며
// "Verifying runner... is removed" 로 30회 재시도 후 죽었다.
//
// **인증 토큰은 복구 경로가 없다.** 가리키는 러너가 사라지면 영구 실패다.
// 등록 토큰은 러너가 다시 등록하므로 스스로 회복한다 — 그래서 등록 토큰을
// 쓸 수 있으면 그쪽을 고른다.
func TestRunnerTokenValues_등록토큰은_등록_자리에_넣는다(t *testing.T) {
	v := runnerTokenValues(runnerToken{Value: "GWUDD4J6j2lfjI", Kind: runnerTokenRegistration})

	assert.Equal(t, "GWUDD4J6j2lfjI", v["runnerRegistrationToken"])
	assert.NotContains(t, v, "runnerToken",
		"등록 토큰을 인증 토큰 자리에 넣으면 러너가 검증을 시도하다 죽는다")
}

func TestRunnerTokenValues_인증토큰은_인증_자리에_넣는다(t *testing.T) {
	v := runnerTokenValues(runnerToken{Value: "t1_4xGyLBokNu", Kind: runnerTokenAuthentication})

	assert.Equal(t, "t1_4xGyLBokNu", v["runnerToken"])
	assert.NotContains(t, v, "runnerRegistrationToken")
}

func TestRunnerTokenValues_빈_토큰은_아무것도_넣지_않는다(t *testing.T) {
	// 빈 값을 넣으면 차트가 빈 문자열로 Secret 을 만들고, 러너는 그것으로
	// 등록을 시도하다 죽는다. 값이 없으면 호출부가 실패로 처리해야 한다.
	assert.Empty(t, runnerTokenValues(runnerToken{}))
}

// GitLab 은 rails 한 번으로 두 정보를 함께 준다: 등록 토큰이 허용되는지와
// 그 값. 출력 형식을 고정해 두지 않으면 파서가 엉뚱한 줄을 토큰으로 집는다 —
// 실제로 파서가 마지막 공백 없는 줄을 집는 구조라, 뒤에 무엇이 찍히면
// 그것이 토큰이 된다.
func TestParseRunnerTokenProbe_등록토큰이_허용되면_그것을_쓴다(t *testing.T) {
	got := parseRunnerTokenProbe("registration_allowed=true\nregistration_token=GWUDD4J6j2lfjI\nauth_token=t1_4xGyLBokNu\n")

	assert.Equal(t, runnerTokenRegistration, got.Kind,
		"등록 토큰은 러너가 스스로 재등록할 수 있어 인증 토큰보다 낫다")
	assert.Equal(t, "GWUDD4J6j2lfjI", got.Value)
}

func TestParseRunnerTokenProbe_등록토큰이_막히면_인증토큰(t *testing.T) {
	got := parseRunnerTokenProbe("registration_allowed=false\nregistration_token=\nauth_token=t1_4xGyLBokNu\n")

	assert.Equal(t, runnerTokenAuthentication, got.Kind)
	assert.Equal(t, "t1_4xGyLBokNu", got.Value)
}

func TestParseRunnerTokenProbe_등록토큰이_허용인데_비어있으면_인증토큰(t *testing.T) {
	// 설정은 켜져 있는데 값이 비는 경우가 있다. 빈 값을 넘기면 러너가
	// 빈 토큰으로 등록을 시도하다 죽으므로, 있는 쪽을 쓴다.
	got := parseRunnerTokenProbe("registration_allowed=true\nregistration_token=\nauth_token=t1_abc\n")

	assert.Equal(t, runnerTokenAuthentication, got.Kind)
	assert.Equal(t, "t1_abc", got.Value)
}

func TestParseRunnerTokenProbe_아무것도_없으면_none(t *testing.T) {
	// 호출부가 실패로 다뤄야 한다 — 빈 토큰으로 설치하면 completed 인데
	// CI 가 한 건도 돌지 않는 스택이 만들어진다.
	assert.Equal(t, runnerTokenNone, parseRunnerTokenProbe("registration_allowed=false\nregistration_token=\nauth_token=\n").Kind)
	assert.Equal(t, runnerTokenNone, parseRunnerTokenProbe("Defaulted container \"toolbox\" out of: toolbox\n").Kind)
}

func TestParseRunnerTokenProbe_잡음이_섞여도_키로_찾는다(t *testing.T) {
	// rails 는 경고를 함께 찍는다. 마지막 줄을 집는 방식이면 경고가 토큰이 된다.
	noisy := "Defaulted container \"toolbox\" out of: toolbox, certificates (init)\n" +
		"registration_allowed=true\nregistration_token=REG123\nauth_token=t1_x\n" +
		"DEPRECATION WARNING: something\n"
	got := parseRunnerTokenProbe(noisy)

	assert.Equal(t, runnerTokenRegistration, got.Kind)
	assert.Equal(t, "REG123", got.Value)
}
