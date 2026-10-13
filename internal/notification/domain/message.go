package domain

import (
	"fmt"
	"time"

	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// Message 는 채널로 보낼 내용이다. 채널마다 모양(Slack mrkdwn · Telegram HTML · 메일 본문)은
// 보내는 쪽 어댑터가 정한다.
type Message struct {
	Event      sharednotify.EventType
	Severity   sharednotify.Severity
	Title      string
	Summary    string
	Fields     []sharednotify.Field
	OccurredAt time.Time
}

// NewMessage 는 생산자의 이벤트를 메시지로 옮긴다.
func NewMessage(e sharednotify.Event) Message {
	severity := e.Severity
	if severity == "" {
		severity = sharednotify.SeverityInfo
	}
	at := e.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}
	return Message{
		Event: e.Type, Severity: severity, Title: e.Title, Summary: e.Summary,
		Fields: append([]sharednotify.Field(nil), e.Fields...), OccurredAt: at,
	}
}

// TestMessage 는 채널 설정을 확인하는 테스트 전송 내용이다.
func TestMessage(ch *Channel) Message {
	return Message{
		Event:    EventTest,
		Severity: sharednotify.SeverityInfo,
		Title:    "Nullus 알림 테스트",
		Summary:  fmt.Sprintf("'%s' 채널 설정이 맞습니다. 이 채널은 구독한 이벤트가 생기면 알림을 받습니다.", ch.Name),
		Fields: []sharednotify.Field{
			{Name: "채널", Value: string(ch.Type)},
		},
		OccurredAt: time.Now(),
	}
}
