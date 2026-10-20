package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

func TestNewMessage_FromEvent(t *testing.T) {
	at := time.Date(2026, 10, 13, 1, 2, 3, 0, time.UTC)
	msg := NewMessage(sharednotify.Event{
		Type: sharednotify.EventBackupFailed, OrgID: "org-1", Severity: sharednotify.SeverityCritical,
		Title: "백업 실패", Summary: "목적지에 쓸 수 없습니다",
		Fields: []sharednotify.Field{{Name: "실행", Value: "bkp_1"}}, OccurredAt: at,
	})
	assert.Equal(t, sharednotify.EventBackupFailed, msg.Event)
	assert.Equal(t, sharednotify.SeverityCritical, msg.Severity)
	assert.Equal(t, "백업 실패", msg.Title)
	assert.Equal(t, at, msg.OccurredAt)
	assert.Equal(t, []sharednotify.Field{{Name: "실행", Value: "bkp_1"}}, msg.Fields)
}

func TestNewMessage_DefaultsSeverityAndTime(t *testing.T) {
	msg := NewMessage(sharednotify.Event{Type: sharednotify.EventBackupSucceeded, Title: "백업 성공"})
	assert.Equal(t, sharednotify.SeverityInfo, msg.Severity)
	assert.False(t, msg.OccurredAt.IsZero())
}

func TestTestMessage_NamesTheChannel(t *testing.T) {
	msg := TestMessage(telegramChannel())
	assert.Equal(t, EventTest, msg.Event)
	assert.Contains(t, msg.Summary, "운영팀 텔레그램")
}

func TestEventCatalog_HasProducedEventsOnly(t *testing.T) {
	var types []sharednotify.EventType
	for _, spec := range EventCatalog() {
		types = append(types, spec.Type)
	}
	assert.Equal(t, []sharednotify.EventType{
		sharednotify.EventBackupFailed, sharednotify.EventBackupPartial, sharednotify.EventBackupSucceeded,
		sharednotify.EventAlertFired, sharednotify.EventAlertResolved, sharednotify.EventStackCVEFound,
	}, types)
	// 설치 OSS 의 새 CVE 는 대응 기한이 있는 고지다 — 새 채널에서 기본으로 받는다.
	for _, spec := range EventCatalog() {
		if spec.Type == sharednotify.EventStackCVEFound {
			assert.True(t, spec.DefaultOn)
		}
	}
	assert.True(t, IsKnownEvent(sharednotify.EventBackupFailed))
	assert.False(t, IsKnownEvent(EventTest), "테스트 전송은 구독하는 이벤트가 아니다")
}
