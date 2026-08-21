package log

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

type fakeLogStore struct {
	mu       sync.Mutex
	entries  map[string][]port.LogEntry
	appendEr error
	listErr  error
	deleted  []string
}

func newFakeLogStore() *fakeLogStore {
	return &fakeLogStore{entries: make(map[string][]port.LogEntry)}
}

func (f *fakeLogStore) Append(_ context.Context, deploymentID string, entry port.LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appendEr != nil {
		return f.appendEr
	}
	f.entries[deploymentID] = append(f.entries[deploymentID], entry)
	return nil
}

func (f *fakeLogStore) List(_ context.Context, deploymentID string, _ int) ([]port.LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]port.LogEntry(nil), f.entries[deploymentID]...), nil
}

func (f *fakeLogStore) Delete(_ context.Context, deploymentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, deploymentID)
	delete(f.entries, deploymentID)
	return nil
}

func (f *fakeLogStore) count(deploymentID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.entries[deploymentID])
}

func drain(t *testing.T, ch <-chan port.LogEntry, n int) []string {
	t.Helper()
	got := make([]string, 0, n)
	for range n {
		select {
		case entry := <-ch:
			got = append(got, entry.Message)
		case <-time.After(time.Second):
			t.Fatalf("로그가 %d줄에서 끊겼습니다 (기대 %d줄)", len(got), n)
		}
	}
	return got
}

func TestPersistentStreamer_WritesThroughToStore(t *testing.T) {
	store := newFakeLogStore()
	s := NewPersistentStreamer(NewMemoryStreamer(), store)

	s.Stream(context.Background(), "stk-1", port.LogEntry{Message: "installing", Level: "info"})

	assert.Equal(t, 1, store.count("stk-1"))
}

// 파드가 재시작되면 메모리는 비어 있다. 그때도 그동안의 로그가 보여야 한다 —
// 설치는 20~30분짜리라 그 사이 재시작이 겹치면 사후 추적이 불가능했다.
func TestPersistentStreamer_ReplaysFromStoreAfterRestart(t *testing.T) {
	store := newFakeLogStore()
	for i := range 200 {
		require.NoError(t, store.Append(context.Background(), "stk-1",
			port.LogEntry{Message: fmt.Sprintf("line-%d", i)}))
	}

	// 새 프로세스: 메모리 스트리머는 비어 있다.
	s := NewPersistentStreamer(NewMemoryStreamer(), store)
	ch := s.Subscribe("stk-1")
	defer s.Unsubscribe("stk-1", ch)

	got := drain(t, ch, 200)
	assert.Equal(t, "line-0", got[0])
	assert.Equal(t, "line-199", got[199], "마지막 줄까지 와야 진행률이 최신 값으로 복원된다")
}

// 이 프로세스가 스트리밍 중이면 메모리가 진실이다. 저장소를 겹쳐 읽으면 같은
// 줄이 두 번 보인다.
func TestPersistentStreamer_PrefersMemoryWhileStreaming(t *testing.T) {
	store := newFakeLogStore()
	s := NewPersistentStreamer(NewMemoryStreamer(), store)

	s.Stream(context.Background(), "stk-1", port.LogEntry{Message: "only-once"})

	ch := s.Subscribe("stk-1")
	defer s.Unsubscribe("stk-1", ch)

	assert.Equal(t, []string{"only-once"}, drain(t, ch, 1))
	select {
	case extra := <-ch:
		t.Fatalf("중복 재생: %q", extra.Message)
	case <-time.After(50 * time.Millisecond):
	}
}

// 저장소가 흔들려도 설치는 계속돼야 한다. 로그를 남기지 못하는 것이 설치를
// 멈출 이유는 아니다.
func TestPersistentStreamer_KeepsStreamingWhenStoreFails(t *testing.T) {
	store := newFakeLogStore()
	store.appendEr = errors.New("db down")
	s := NewPersistentStreamer(NewMemoryStreamer(), store)

	ch := s.Subscribe("stk-1")
	defer s.Unsubscribe("stk-1", ch)
	s.Stream(context.Background(), "stk-1", port.LogEntry{Message: "still-live"})

	assert.Equal(t, []string{"still-live"}, drain(t, ch, 1))
}

// 새 설치는 이전 실행의 로그 위에 겹쳐 쌓이면 안 된다. 지울 때 저장소도 함께 지운다.
func TestPersistentStreamer_ClearHistoryClearsStore(t *testing.T) {
	store := newFakeLogStore()
	s := NewPersistentStreamer(NewMemoryStreamer(), store)
	s.Stream(context.Background(), "stk-1", port.LogEntry{Message: "old"})

	s.ClearHistory("stk-1")

	assert.Contains(t, store.deleted, "stk-1")
	assert.Equal(t, 0, store.count("stk-1"))
}
