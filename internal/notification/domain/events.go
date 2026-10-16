package domain

import sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"

// EventTest 는 채널 설정을 확인하는 테스트 전송이다. 구독하는 이벤트가 아니다.
const EventTest sharednotify.EventType = "test"

// EventSpec 은 구독할 수 있는 이벤트 하나다.
type EventSpec struct {
	Type sharednotify.EventType `json:"type"`
	// DefaultOn 은 새 채널에서 미리 골라 둘지다. 성공 알림은 잡음이라 끈다.
	DefaultOn bool `json:"default_on"`
	// Description 은 화면이 번역이 없을 때 보여 줄 설명이다.
	Description string `json:"description"`
}

// EventCatalog 는 지금 생산자가 있는 이벤트다 — 백업 결과(B3-3)와 스택 심각 경보(#87). 생산자가
// 생길 때 여기에 더한다.
func EventCatalog() []EventSpec {
	return []EventSpec{
		{Type: sharednotify.EventBackupFailed, DefaultOn: true, Description: "백업이 실패로 끝났을 때"},
		{Type: sharednotify.EventBackupPartial, DefaultOn: true, Description: "백업이 일부 컴포넌트를 빠뜨린 채 끝났을 때"},
		{Type: sharednotify.EventBackupSucceeded, DefaultOn: false, Description: "백업이 성공했을 때"},
		{Type: sharednotify.EventAlertFired, DefaultOn: true, Description: "스택에서 심각한 경보가 발생했을 때(디스크 · 파드 재시작 · 도구 무응답 등)"},
		{Type: sharednotify.EventAlertResolved, DefaultOn: true, Description: "그 경보가 해제됐을 때"},
	}
}

// IsKnownEvent 는 구독할 수 있는 이벤트인지 본다.
func IsKnownEvent(t sharednotify.EventType) bool {
	for _, spec := range EventCatalog() {
		if spec.Type == t {
			return true
		}
	}
	return false
}
