package audit

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemorySink_LogAndSnapshot(t *testing.T) {
	sink := NewMemorySink()
	ctx := context.Background()

	require.NoError(t, sink.Log(ctx, AuditEntry{
		UserID:       "u1",
		Action:       "create",
		ResourceType: "stack",
		ResourceID:   "s1",
		Details:      map[string]any{"acknowledge_warnings": true},
	}))
	require.NoError(t, sink.Log(ctx, AuditEntry{
		UserID: "u2", Action: "deploy", ResourceType: "stack", ResourceID: "s2",
	}))

	snap := sink.Snapshot()
	require.Len(t, snap, 2)
	assert.Equal(t, "create", snap[0].Action)
	assert.Equal(t, true, snap[0].Details["acknowledge_warnings"])
	assert.Equal(t, "deploy", snap[1].Action)
}

func TestMemorySink_SnapshotIsDeepCopy(t *testing.T) {
	sink := NewMemorySink()
	require.NoError(t, sink.Log(context.Background(), AuditEntry{
		Action: "x", Details: map[string]any{"key": "before"},
	}))

	// Mutate the snapshot's Details map; the stored entry must not change.
	snap1 := sink.Snapshot()
	snap1[0].Details["key"] = "after"

	snap2 := sink.Snapshot()
	assert.Equal(t, "before", snap2[0].Details["key"])
}

func TestMemorySink_Reset(t *testing.T) {
	sink := NewMemorySink()
	require.NoError(t, sink.Log(context.Background(), AuditEntry{Action: "one"}))
	sink.Reset()
	assert.Empty(t, sink.Snapshot())
}

func TestMemorySink_ParallelLogIsSafe(t *testing.T) {
	sink := NewMemorySink()
	const n = 200
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = sink.Log(context.Background(), AuditEntry{Action: "parallel"})
		}()
	}
	wg.Wait()
	assert.Len(t, sink.Snapshot(), n)
}

// Compile-time check that *MemorySink satisfies Sink.
var _ Sink = (*MemorySink)(nil)
