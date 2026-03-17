package log

import (
	"context"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

const defaultChannelBuffer = 64

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
	ch := make(chan port.LogEntry, defaultChannelBuffer)
	s.mu.Lock()
	for _, entry := range s.history[deploymentID] {
		select {
		case ch <- entry:
		default:
		}
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
	s.history[deploymentID] = append(s.history[deploymentID], entry)
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
