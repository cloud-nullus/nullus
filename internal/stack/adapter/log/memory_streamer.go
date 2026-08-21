package log

import (
	"context"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

const defaultChannelBuffer = 64

// maxHistoryEntries 는 배포 하나가 메모리에 남기는 로그 줄 수의 상한이다.
//
// 설치 하나가 남기는 양보다 넉넉하되 무한히 쌓이지는 않게 한다. 구독자마다
// 히스토리 크기만큼 채널을 잡으므로, 이 값은 구독 한 건의 메모리 상한이기도 하다.
const maxHistoryEntries = 5000

// MemoryStreamer is an in-memory implementation of port.LogStreamer.
// It fans out each published log entry to all active subscribers for a deployment.
type MemoryStreamer struct {
	mu          sync.RWMutex
	subscribers map[string][]chan port.LogEntry
	history     map[string][]port.LogEntry
}

// NewMemoryStreamer constructs a MemoryStreamer.
func NewMemoryStreamer() *MemoryStreamer {
	return &MemoryStreamer{
		subscribers: make(map[string][]chan port.LogEntry),
		history:     make(map[string][]port.LogEntry),
	}
}

// Subscribe registers a new channel to receive log entries for deploymentID.
// Any previously buffered entries are replayed to the new subscriber immediately.
func (s *MemoryStreamer) Subscribe(deploymentID string) <-chan port.LogEntry {
	s.mu.Lock()
	history := s.history[deploymentID]

	// 채널은 히스토리 전체가 들어갈 만큼 잡는다.
	//
	// 예전에는 고정 크기(64)에 default 로 흘려보내, 버퍼가 차는 순간 이후 재생이
	// 전부 조용히 버려졌다. 설치 로그는 그보다 훨씬 길어서 재접속한 화면에는
	// 초반까지만 남았고, 마지막으로 전달된 항목이 초반이라 진행률도 그 값에서
	// 굳었다 — 2026-08-21 운영에서 로그가 cert-manager 에서 끊기고 진행률이 5%
	// 로 고정된 것이 이것이다.
	//
	// 여유분(defaultChannelBuffer)은 재생 직후 들어오는 실시간 항목의 몫이다.
	ch := make(chan port.LogEntry, len(history)+defaultChannelBuffer)
	for _, entry := range history {
		ch <- entry
	}
	s.subscribers[deploymentID] = append(s.subscribers[deploymentID], ch)
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes ch from the subscriber list for deploymentID and closes it.
func (s *MemoryStreamer) Unsubscribe(deploymentID string, ch <-chan port.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := s.subscribers[deploymentID]
	for i, sub := range list {
		if sub == ch {
			s.subscribers[deploymentID] = append(list[:i], list[i+1:]...)
			close(sub)
			break
		}
	}
	if len(s.subscribers[deploymentID]) == 0 {
		delete(s.subscribers, deploymentID)
	}
}

// Stream publishes entry to all subscribers of deploymentID.
// Non-blocking: drops the entry for any subscriber whose buffer is full.
func (s *MemoryStreamer) Stream(ctx context.Context, deploymentID string, entry port.LogEntry) {
	s.mu.Lock()
	history := append(s.history[deploymentID], entry)
	// 상한을 넘으면 오래된 쪽을 버린다. 화면이 필요로 하는 것은 최근이고,
	// 진행률은 마지막 항목에서 복원되기 때문이다.
	if overflow := len(history) - maxHistoryEntries; overflow > 0 {
		history = append([]port.LogEntry(nil), history[overflow:]...)
	}
	s.history[deploymentID] = history
	list := make([]chan port.LogEntry, len(s.subscribers[deploymentID]))
	copy(list, s.subscribers[deploymentID])
	s.mu.Unlock()

	for _, ch := range list {
		select {
		case ch <- entry:
		default:
		}
	}
}
