package log

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

func streamN(t *testing.T, s port.LogStreamer, deploymentID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		s.Stream(context.Background(), deploymentID, port.LogEntry{
			Level:   "info",
			Message: fmt.Sprintf("line-%d", i),
		})
	}
}

func TestMemoryStreamer_Tail_ReturnsRecentLines(t *testing.T) {
	s := NewMemoryStreamer()
	streamN(t, s, "dep-1", 5)

	got, err := s.Tail(context.Background(), "dep-1", 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// 최근 2줄을 기록 순서(오름차순)로 돌려준다 — WS 재생과 같은 순서.
	assert.Equal(t, "line-3", got[0].Message)
	assert.Equal(t, "line-4", got[1].Message)
}

func TestMemoryStreamer_Tail_LimitLargerThanHistory(t *testing.T) {
	s := NewMemoryStreamer()
	streamN(t, s, "dep-1", 3)

	got, err := s.Tail(context.Background(), "dep-1", 100)
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestMemoryStreamer_Tail_UnknownDeployment(t *testing.T) {
	s := NewMemoryStreamer()

	got, err := s.Tail(context.Background(), "no-such", 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPersistentStreamer_Tail_MemoryFirst(t *testing.T) {
	store := newFakeLogStore()
	s := NewPersistentStreamer(NewMemoryStreamer(), store)
	// Stream 은 메모리와 저장소 양쪽에 남는다. 이 프로세스가 스트리밍한
	// 배포는 메모리가 진실이다 — 저장소를 겹쳐 읽으면 같은 줄이 두 번 보인다.
	streamN(t, s, "dep-1", 4)

	got, err := s.Tail(context.Background(), "dep-1", 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "line-3", got[1].Message)
}

func TestPersistentStreamer_Tail_FallsBackToStoreAfterRestart(t *testing.T) {
	store := newFakeLogStore()
	require.NoError(t, store.Append(context.Background(), "dep-1", port.LogEntry{Message: "persisted"}))

	// 재시작 시나리오: 새 메모리 스트리머(이력 없음) + 기존 저장소.
	s := NewPersistentStreamer(NewMemoryStreamer(), store)

	got, err := s.Tail(context.Background(), "dep-1", 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "persisted", got[0].Message)
}

func TestPersistentStreamer_Tail_NilStoreUsesMemory(t *testing.T) {
	s := NewPersistentStreamer(NewMemoryStreamer(), nil)
	streamN(t, s, "dep-1", 1)

	got, err := s.Tail(context.Background(), "dep-1", 10)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
