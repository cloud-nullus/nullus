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
	return s.SubscribeWithHistory(deploymentID, nil)
}

// SubscribeWithHistory 는 메모리에 없는 이력을 앞에 붙여 구독한다.
//
// 프로세스가 재시작되면 메모리 이력은 비어 있다. 그때 저장소에서 읽어 온 것을
// 이 자리로 넘기면, 재생과 구독 등록이 같은 잠금 안에서 일어나 그 사이에 들어온
// 실시간 항목을 놓치지 않는다.
func (s *MemoryStreamer) SubscribeWithHistory(deploymentID string, seed []port.LogEntry) <-chan port.LogEntry {
	s.mu.Lock()
	history := s.history[deploymentID]
	if len(seed) > 0 {
		combined := make([]port.LogEntry, 0, len(seed)+len(history))
		combined = append(combined, seed...)
		history = append(combined, history...)
	}

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

// HasHistory 는 이 배포의 이력이 메모리에 있는지다.
//
// 있으면 이 프로세스가 그 배포를 스트리밍했다는 뜻이라, 저장소를 겹쳐 읽으면
// 같은 줄이 두 번 보인다.
func (s *MemoryStreamer) HasHistory(deploymentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.history[deploymentID]) > 0
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

// ClearHistory 는 이 배포의 이력을 지운다.
//
// 새 실행이 이전 실행의 로그 위에 겹쳐 쌓이지 않게 한다. 구독은 건드리지
// 않는다 — 보고 있는 화면을 끊을 이유가 없다.
func (s *MemoryStreamer) ClearHistory(deploymentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.history, deploymentID)
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
