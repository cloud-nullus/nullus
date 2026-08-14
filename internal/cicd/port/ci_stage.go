package port

import (
	"strings"
	"time"
)

// CIStageStatus 는 단계 상태의 정규화된 어휘다.
//
// CI 마다 어휘가 다르다 — Jenkins 는 SUCCESS/FAILED/IN_PROGRESS/NOT_EXECUTED,
// GitLab 은 success/failed/running/pending/skipped, GitHub Actions 는
// status 와 conclusion 두 필드로 나눠 표현한다.
//
// 이 변환을 어댑터 경계에서 끝낸다. 도메인과 화면이 CI 별 어휘를 알게 되면
// OSS 를 하나 늘릴 때마다 위쪽 계층이 모두 바뀐다.
type CIStageStatus string

const (
	CIStageSuccess CIStageStatus = "success"
	CIStageFailed  CIStageStatus = "failed"
	CIStageRunning CIStageStatus = "running"
	CIStageQueued  CIStageStatus = "queued"
	// CIStageSkipped 는 조건 때문에 실행되지 않은 단계다.
	// 실패와 구분해야 한다 — 건너뛴 것은 잘못된 것이 아니다.
	CIStageSkipped CIStageStatus = "skipped"
	// CIStageUnknown 은 CI 가 상태를 알려주지 않은 경우다.
	// 성공으로 넘겨짚지 않는다 — 실행되지 않은 일을 성공이라 말하면 안 된다.
	CIStageUnknown CIStageStatus = "unknown"
)

// CIStage 는 실행 하나 안의 단계다.
type CIStage struct {
	Name      string
	Status    CIStageStatus
	StartedAt time.Time
	Duration  time.Duration
}

// NormalizeStageStatus 는 CI 가 쓰는 표현을 정규화된 어휘로 옮긴다.
//
// 어댑터가 자기 CI 의 표현을 여기 넘긴다. 모르는 값은 unknown 이다 —
// 넘겨짚으면 돌지 않은 단계가 성공으로 보인다.
func NormalizeStageStatus(raw string) CIStageStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "success", "successful", "completed", "passed":
		return CIStageSuccess
	case "failed", "failure", "error", "unstable", "aborted", "canceled", "cancelled":
		return CIStageFailed
	case "running", "in_progress", "in progress", "started":
		return CIStageRunning
	case "queued", "pending", "waiting", "created", "not_built":
		return CIStageQueued
	case "skipped", "not_executed", "manual":
		return CIStageSkipped
	default:
		return CIStageUnknown
	}
}
