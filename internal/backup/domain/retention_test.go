package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(n int) time.Time {
	base := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, -n)
}

func runsOverDays(n int) []RunSummary {
	out := make([]RunSummary, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, RunSummary{
			ID:         string(rune('a' + i)),
			CreatedAt:  day(i),
			TotalBytes: 10,
			Status:     StatusSucceeded,
		})
	}
	return out
}

func TestRetention_일간_보존수만큼_남긴다(t *testing.T) {
	policy := RetentionPolicy{Daily: 3, Weekly: 0, Monthly: 0}
	runs := runsOverDays(6)

	del := policy.SelectForDeletion(runs)

	// 최근 3일치는 남고 나머지 3건이 지워진다.
	assert.Len(t, del, 3)
	assert.NotContains(t, del, "a")
	assert.NotContains(t, del, "b")
	assert.NotContains(t, del, "c")
}

func TestRetention_주간_월간까지_고려한다(t *testing.T) {
	policy := RetentionPolicy{Daily: 2, Weekly: 2, Monthly: 1}
	runs := runsOverDays(60)

	del := policy.SelectForDeletion(runs)
	kept := len(runs) - len(del)

	assert.LessOrEqual(t, kept, 5, "일2 + 주2 + 월1 을 넘겨 남기지 않는다")
	assert.GreaterOrEqual(t, kept, 1)
}

func TestRetention_실패한_실행은_삭제대상이_아니다(t *testing.T) {
	// 실패한 실행에는 지울 산출물이 없다. 이력은 "언제 백업이 끊겼나" 의 근거라
	// 남겨야 한다.
	policy := RetentionPolicy{Daily: 1}
	runs := []RunSummary{
		{ID: "ok", CreatedAt: day(0), Status: StatusSucceeded, TotalBytes: 5},
		{ID: "bad", CreatedAt: day(1), Status: StatusFailed},
		{ID: "old", CreatedAt: day(2), Status: StatusSucceeded, TotalBytes: 5},
	}

	del := policy.SelectForDeletion(runs)

	assert.NotContains(t, del, "bad")
	assert.Contains(t, del, "old")
}

func TestRetention_partial_은_삭제대상이다(t *testing.T) {
	// partial 에는 실제 산출물이 있으므로 용량을 차지한다.
	policy := RetentionPolicy{Daily: 1}
	runs := []RunSummary{
		{ID: "new", CreatedAt: day(0), Status: StatusSucceeded, TotalBytes: 5},
		{ID: "part", CreatedAt: day(3), Status: StatusPartial, TotalBytes: 5},
	}

	assert.Contains(t, policy.SelectForDeletion(runs), "part")
}

func TestRetention_가장_최근_성공본은_절대_지우지_않는다(t *testing.T) {
	// 보존 정책이 백업을 0 개로 만들면 안 된다. 총량 상한이 아무리 낮아도
	// 마지막 하나는 남는다.
	policy := RetentionPolicy{Daily: 0, Weekly: 0, Monthly: 0, MaxTotalBytes: 1}
	runs := runsOverDays(3)

	del := policy.SelectForDeletion(runs)

	assert.NotContains(t, del, "a", "최신 성공본은 남아야 한다")
	assert.Len(t, del, 2)
}

func TestRetention_총량_상한을_넘으면_오래된_것부터_더_지운다(t *testing.T) {
	policy := RetentionPolicy{Daily: 5, MaxTotalBytes: 25}
	runs := runsOverDays(5) // 각 10 bytes = 50

	del := policy.SelectForDeletion(runs)

	require.NotEmpty(t, del, "상한을 넘으면 일간 보존수 안이라도 더 지운다")
	assert.Contains(t, del, "e", "가장 오래된 것부터")
	assert.NotContains(t, del, "a")
}

func TestRetention_빈_이력(t *testing.T) {
	assert.Empty(t, RetentionPolicy{Daily: 3}.SelectForDeletion(nil))
}

func TestDefaultRetentionPolicy(t *testing.T) {
	p := DefaultRetentionPolicy()
	assert.Equal(t, 7, p.Daily)
	assert.Equal(t, 4, p.Weekly)
	assert.Equal(t, 3, p.Monthly)
}
