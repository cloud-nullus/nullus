package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 배포한 앱의 로그는 파드 여러 개에 흩어져 있다.
//
// 파드별로 나눠 보여주면 "요청이 어느 파드로 갔는지" 를 사람이 맞춰 봐야 한다.
// 시간순으로 섞어야 한 흐름으로 읽힌다 — kubectl logs -l 과 같은 이유다.
func TestMergeLogLines_SortsAcrossPodsByTimestamp(t *testing.T) {
	merged := mergeLogLines(map[string][]string{
		"demo-app-a": {
			"2026-08-12T10:20:31.000000000Z second",
			"2026-08-12T10:20:33.000000000Z fourth",
		},
		"demo-app-b": {
			"2026-08-12T10:20:30.000000000Z first",
			"2026-08-12T10:20:32.000000000Z third",
		},
	}, map[string]string{"demo-app-a": "demo-app", "demo-app-b": "demo-app"}, 10)

	messages := make([]string, 0, len(merged))
	for _, line := range merged {
		messages = append(messages, line.Message)
	}
	assert.Equal(t, []string{"first", "second", "third", "fourth"}, messages)
	assert.Equal(t, "demo-app-b", merged[0].Pod)
	assert.Equal(t, "demo-app", merged[0].App)
}

// 타임스탬프 없는 줄을 버리면 정작 필요한 스택트레이스가 사라진다.
// 앞줄의 시각을 물려받아 순서를 지킨다.
func TestMergeLogLines_KeepsLinesWithoutTimestamp(t *testing.T) {
	merged := mergeLogLines(map[string][]string{
		"demo-app-a": {
			"2026-08-12T10:20:30.000000000Z panic: boom",
			"\tgoroutine 1 [running]:",
		},
	}, map[string]string{"demo-app-a": "demo-app"}, 10)

	require.Len(t, merged, 2)
	assert.Equal(t, "panic: boom", merged[0].Message)
	assert.Equal(t, "\tgoroutine 1 [running]:", merged[1].Message)
	assert.Equal(t, merged[0].Timestamp, merged[1].Timestamp, "앞줄 시각을 물려받는다")
}

// 화면은 최근 것을 본다. 상한을 넘으면 오래된 것을 버린다.
func TestMergeLogLines_KeepsNewestWithinLimit(t *testing.T) {
	merged := mergeLogLines(map[string][]string{
		"demo-app-a": {
			"2026-08-12T10:20:30.000000000Z one",
			"2026-08-12T10:20:31.000000000Z two",
			"2026-08-12T10:20:32.000000000Z three",
		},
	}, map[string]string{"demo-app-a": "demo-app"}, 2)

	require.Len(t, merged, 2)
	assert.Equal(t, "two", merged[0].Message)
	assert.Equal(t, "three", merged[1].Message)
}

// 빈 줄은 버린다. 컨테이너가 개행만 뱉는 경우가 흔해 화면이 공백으로 찬다.
func TestMergeLogLines_DropsBlankLines(t *testing.T) {
	merged := mergeLogLines(map[string][]string{
		"demo-app-a": {
			"2026-08-12T10:20:30.000000000Z ",
			"",
			"2026-08-12T10:20:31.000000000Z real",
		},
	}, map[string]string{"demo-app-a": "demo-app"}, 10)

	require.Len(t, merged, 1)
	assert.Equal(t, "real", merged[0].Message)
}

// 로그를 못 읽는 파드가 있어도(방금 뜬 파드, ImagePullBackOff) 나머지는 보여야 한다.
func TestMergeLogLines_IgnoresPodsWithNoOutput(t *testing.T) {
	merged := mergeLogLines(map[string][]string{
		"demo-app-a": {"2026-08-12T10:20:30.000000000Z alive"},
		"demo-app-b": {},
	}, map[string]string{"demo-app-a": "demo-app", "demo-app-b": "demo-app"}, 10)

	require.Len(t, merged, 1)
	assert.Equal(t, "alive", merged[0].Message)
}
