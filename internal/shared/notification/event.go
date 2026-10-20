// Package notification 은 모듈 사이에서 알림 이벤트를 주고받는 공개 계약이다(nullus-plan#63).
//
// 이벤트를 만드는 모듈(백업 등)은 Event 를 만들어 Publisher 에 넘긴다. 어느 채널(Telegram ·
// 이메일 · Slack)로 어떻게 보낼지는 알림 모듈(internal/notification)이 정한다 — 생산자는 채널을
// 몰라도 되고, 채널이 늘어도 생산자를 고치지 않는다.
package notification

import (
	"context"
	"time"
)

// EventType 은 채널이 구독하는 이벤트 종류다. 생산자가 실제로 내는 것만 둔다 — 아무도 내지 않는
// 이벤트를 구독 목록에 보여 주면 설정은 저장되는데 알림은 영영 오지 않는다.
type EventType string

const (
	// EventBackupFailed 는 백업 실행이 실패로 끝났다는 뜻이다(백업복구 설계 B3-3).
	EventBackupFailed EventType = "backup_failed"
	// EventBackupPartial 은 일부 컴포넌트가 빠진 채 끝난 백업이다 — 쓸 수 있지만 완전하지 않다.
	EventBackupPartial EventType = "backup_partial"
	// EventBackupSucceeded 는 백업 성공이다. 기본으로는 구독하지 않는다(잡음).
	EventBackupSucceeded EventType = "backup_succeeded"
	// EventAlertFired 는 스택의 심각(critical) 경보가 발생했다는 뜻이다(nullus-plan#87). 스택 Prometheus 의
	// 기본 규칙이 평가하고, 플랫폼이 Alertmanager 에서 가져온다. 계속 발생 중이면 주기적으로 다시 낸다.
	EventAlertFired EventType = "alert_fired"
	// EventAlertResolved 는 그 경보가 해제됐다는 뜻이다.
	EventAlertResolved EventType = "alert_resolved"
	// EventStackCVEFound 는 스택이 설치한 OSS 이미지에서 수정판 있는 CRITICAL 취약점이 새로
	// 발견됐다는 뜻이다(stack 모듈의 설치 이미지 스캔).
	EventStackCVEFound EventType = "stack_cve_found"
)

// Severity 는 알림의 심각도다.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Field 는 알림 본문에 붙는 이름·값 한 줄이다(실행 ID, 대상 등).
type Field struct {
	Name  string
	Value string
}

// Event 는 생산자가 알리는 일 하나다. 문구는 사람이 읽을 한국어로 채운다.
type Event struct {
	Type       EventType
	OrgID      string
	Severity   Severity
	Title      string
	Summary    string
	Fields     []Field
	OccurredAt time.Time
}

// Publisher 는 이벤트를 그 조직의 구독 채널로 보낸다. 실패한 채널이 있어도 나머지는 보내고,
// 실패를 모아 돌려준다 — 생산자는 기록만 하고 자기 일을 계속한다.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}
