package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// kubectl rollout status 는 RollingUpdate 전략에서만 동작한다. OpenBao 차트의
// StatefulSet 은 OnDelete 를 쓰기 때문에, 이를 걸러내지 않으면 준비 검사가
//
//	error: rollout status is only available for RollingUpdate strategy type
//
// 로 실패해 배포 전체가 health_check 에서 무너진다.
func TestIsRolloutStatusUnsupportedStrategy(t *testing.T) {
	cases := []struct {
		name     string
		strategy string
		want     bool
	}{
		{name: "OnDelete 는 지원되지 않음", strategy: "OnDelete", want: true},
		{name: "RollingUpdate 는 지원됨", strategy: "RollingUpdate", want: false},
		{name: "빈 값은 기본값(RollingUpdate)으로 간주", strategy: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isRolloutStatusUnsupportedStrategy(tc.strategy))
		})
	}
}

// kubectl 이 돌려주는 에러 문구로도 같은 상황을 인식할 수 있어야 한다.
// 전략 조회가 실패하거나 경쟁 상태로 값이 바뀐 경우의 방어선이다.
func TestIsRolloutStatusUnsupportedError(t *testing.T) {
	assert.True(t, isRolloutStatusUnsupportedError(
		assertError("error: rollout status is only available for RollingUpdate strategy type")))
	assert.True(t, isRolloutStatusUnsupportedError(
		assertError("Error: ROLLOUT STATUS IS ONLY AVAILABLE FOR ROLLINGUPDATE STRATEGY TYPE")))

	assert.False(t, isRolloutStatusUnsupportedError(assertError("timed out waiting for the condition")))
	assert.False(t, isRolloutStatusUnsupportedError(nil))
}

type stringError string

func (e stringError) Error() string { return string(e) }

func assertError(msg string) error { return stringError(msg) }
