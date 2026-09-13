package port

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// CI 마다 어휘가 다르다. 변환을 어댑터 경계에서 끝내지 않으면 OSS 를 하나
// 늘릴 때마다 도메인과 화면이 모두 바뀐다.
func TestNormalizeStageStatus_AcrossVocabularies(t *testing.T) {
	cases := map[string]CIStageStatus{
		// Jenkins
		"SUCCESS": CIStageSuccess, "FAILED": CIStageFailed,
		"IN_PROGRESS": CIStageRunning, "NOT_EXECUTED": CIStageSkipped,
		"ABORTED": CIStageCanceled, "UNSTABLE": CIStageFailed,
		// GitLab CI
		"success": CIStageSuccess, "failed": CIStageFailed,
		"running": CIStageRunning, "pending": CIStageQueued,
		"skipped": CIStageSkipped, "manual": CIStageSkipped,
		"canceled": CIStageCanceled,
		// GitHub Actions
		"completed": CIStageSuccess, "queued": CIStageQueued,
		"in_progress": CIStageRunning, "failure": CIStageFailed,
		"cancelled": CIStageCanceled,
	}
	for raw, want := range cases {
		assert.Equalf(t, want, NormalizeStageStatus(raw), "raw=%q", raw)
	}
}

// 취소는 실패가 아니다 — 새 커밋이 앞선 실행을 밀어낸 것이다. 실패로 옮기면
// 스캔 판정이 error 로 남고 대시보드가 스캐너 장애로 센다.
func TestNormalizeStageStatus_CanceledIsNotFailure(t *testing.T) {
	for _, raw := range []string{"ABORTED", "canceled", "cancelled"} {
		assert.NotEqual(t, CIStageFailed, NormalizeStageStatus(raw), raw)
	}
}

// 모르는 값을 성공으로 넘겨짚으면 돌지 않은 단계가 성공으로 보인다.
func TestNormalizeStageStatus_UnknownIsNotSuccess(t *testing.T) {
	assert.Equal(t, CIStageUnknown, NormalizeStageStatus("something-new"))
	assert.Equal(t, CIStageUnknown, NormalizeStageStatus(""))
}

// 건너뛴 것은 잘못된 것이 아니다 — 실패와 구분해야 한다.
func TestNormalizeStageStatus_SkippedIsNotFailure(t *testing.T) {
	assert.NotEqual(t, CIStageFailed, NormalizeStageStatus("skipped"))
	assert.NotEqual(t, CIStageFailed, NormalizeStageStatus("NOT_EXECUTED"))
}
