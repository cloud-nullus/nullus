package log

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

func TestMemoryStreamer_SubscribeAndReceive(t *testing.T) {
	s := NewMemoryStreamer()

	ch := s.Subscribe("dep-1")

	entry := port.LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Step:      "installing_minio",
		Message:   "MinIO install started",
		Phase:     "A",
	}
	s.Stream(context.Background(), "dep-1", entry)

	select {
	case received := <-ch:
		assert.Equal(t, entry.Step, received.Step)
		assert.Equal(t, entry.Message, received.Message)
		assert.Equal(t, entry.Phase, received.Phase)
	case <-time.After(time.Second):
		t.Fatal("expected log entry but received nothing")
	}
}

func TestMemoryStreamer_UnsubscribeStopsDelivery(t *testing.T) {
	s := NewMemoryStreamer()

	ch := s.Subscribe("dep-2")
	s.Unsubscribe("dep-2", ch)

	entry := port.LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Step:      "configuring_argocd",
		Message:   "ArgoCD configuration complete",
		Phase:     "B",
	}
	s.Stream(context.Background(), "dep-2", entry)

	// Channel should be closed and empty after unsubscribe.
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after unsubscribe")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected closed channel signal")
	}
}

func TestMemoryStreamer_MultipleSubscribers(t *testing.T) {
	s := NewMemoryStreamer()

	ch1 := s.Subscribe("dep-3")
	ch2 := s.Subscribe("dep-3")

	entry := port.LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Step:      "health_check",
		Message:   "Health check passed",
		Phase:     "C",
	}
	s.Stream(context.Background(), "dep-3", entry)

	for i, ch := range []<-chan port.LogEntry{ch1, ch2} {
		select {
		case received := <-ch:
			assert.Equal(t, entry.Message, received.Message, "subscriber %d should receive the entry", i+1)
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive entry", i+1)
		}
	}
}

func TestMemoryStreamer_IsolatedDeployments(t *testing.T) {
	s := NewMemoryStreamer()

	ch1 := s.Subscribe("dep-A")
	ch2 := s.Subscribe("dep-B")

	entryA := port.LogEntry{Level: "info", Step: "step-a", Message: "for A", Phase: "A"}
	s.Stream(context.Background(), "dep-A", entryA)

	select {
	case received := <-ch1:
		assert.Equal(t, "for A", received.Message)
	case <-time.After(time.Second):
		t.Fatal("dep-A subscriber did not receive entry")
	}

	// dep-B channel should be empty.
	select {
	case received := <-ch2:
		t.Fatalf("dep-B should not have received an entry, got: %v", received)
	case <-time.After(100 * time.Millisecond):
		// expected: no entry
	}

	require.NoError(t, nil) // explicit pass
}

func TestMemoryStreamer_ReplaysHistoryToLateSubscriber(t *testing.T) {
	s := NewMemoryStreamer()

	first := port.LogEntry{Level: "info", Step: "installing_gitlab", Message: "before subscribe 1", Phase: "B"}
	second := port.LogEntry{Level: "info", Step: "installing_argocd", Message: "before subscribe 2", Phase: "B"}
	s.Stream(context.Background(), "dep-history", first)
	s.Stream(context.Background(), "dep-history", second)

	ch := s.Subscribe("dep-history")

	select {
	case got := <-ch:
		assert.Equal(t, first.Message, got.Message)
	case <-time.After(time.Second):
		t.Fatal("expected first replayed log entry")
	}

	select {
	case got := <-ch:
		assert.Equal(t, second.Message, got.Message)
	case <-time.After(time.Second):
		t.Fatal("expected second replayed log entry")
	}
}
