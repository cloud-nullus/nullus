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

// discoverGitLabRunnerRegistrationTokenOnce 가 내부 에러를 버리고 일반 메시지를
// 돌려주면, 그 메시지는 재시도 힌트에 걸리지 않아 24회 재시도 루프가 한 번도
// 돌지 않는다. GitLab 이 아직 기동 중인 정상 상황에서 즉시 실패하게 된다.
func TestRunnerTokenNotFoundError_PreservesRetryableCause(t *testing.T) {
	cases := []struct {
		name  string
		cause string
	}{
		{"toolbox 미생성", `deployments.apps "gitlab-toolbox" not found`},
		{"마이그레이션 미완", "PG::UndefinedTable: ERROR: relation \"application_settings\" does not exist"},
		{"컨테이너 준비 전", "container not found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runnerTokenNotFoundError(errors.New(tc.cause), errors.New(tc.cause))
			require.Error(t, err)
			assert.True(t, isRetryableRunnerTokenDiscoveryError(err),
				"원인이 보존되지 않으면 재시도 루프가 돌지 않는다: %v", err)
		})
	}
}

func TestRunnerTokenNotFoundError_NonRetryableStaysNonRetryable(t *testing.T) {
	err := runnerTokenNotFoundError(errors.New("permission denied"), errors.New("permission denied"))
	require.Error(t, err)
	assert.False(t, isRetryableRunnerTokenDiscoveryError(err))
}

func TestErrRunnerTokenDiscovery_IsRetryableSignal(t *testing.T) {
	// 스택 재시도(retry) 경로가 이 스텝부터 다시 시작할 수 있어야 하므로
	// 일반 설치 오류와 구분 가능한 sentinel 이어야 한다.
	err := wrapRunnerTokenDiscoveryError(errors.New("context deadline exceeded"))
	assert.True(t, errors.Is(err, ErrRunnerTokenDiscovery))

	// nil 은 nil 로 통과시킨다 — 호출부에서 분기를 단순하게 유지한다.
	assert.NoError(t, wrapRunnerTokenDiscoveryError(nil))
}
