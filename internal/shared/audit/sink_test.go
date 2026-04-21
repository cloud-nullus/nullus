package audit

import (
	"context"
	"sync"
	"testing"
	"time"

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

// F8-UIUX-RetryAuditSurface-Backend — Reader contract tests.

func TestMemorySink_ListByResource_Empty(t *testing.T) {
	sink := NewMemorySink()
	out, err := sink.ListByResource(context.Background(), "stack", "missing")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestMemorySink_ListByResource_FiltersByResource(t *testing.T) {
	sink := NewMemorySink()
	ctx := context.Background()
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "s1"}))
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "other"}))
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "retry", ResourceType: "cluster", ResourceID: "s1"}))
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "deploy", ResourceType: "stack", ResourceID: "s1"}))

	out, err := sink.ListByResource(ctx, "stack", "s1")
	require.NoError(t, err)
	require.Len(t, out, 2)
	// ListByResource is action-agnostic — callers filter further.
	actions := []string{out[0].Entry.Action, out[1].Entry.Action}
	assert.ElementsMatch(t, []string{"retry", "deploy"}, actions)
}

func TestMemorySink_ListByResource_SortedNewestFirst(t *testing.T) {
	i := 0
	clock := func() time.Time {
		i++
		// Each call returns a timestamp 1 minute after the previous one, so the
		// recording order equals "oldest first". The Reader must flip this so
		// the most recent entry appears at index 0.
		return time.Date(2026, 4, 21, 9, i, 0, 0, time.UTC)
	}
	sink := NewMemorySink().WithClock(clock)
	ctx := context.Background()
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "s1", Details: map[string]any{"n": 1}}))
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "s1", Details: map[string]any{"n": 2}}))
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "s1", Details: map[string]any{"n": 3}}))

	out, err := sink.ListByResource(ctx, "stack", "s1")
	require.NoError(t, err)
	require.Len(t, out, 3)
	assert.Equal(t, 3, out[0].Entry.Details["n"])
	assert.Equal(t, 2, out[1].Entry.Details["n"])
	assert.Equal(t, 1, out[2].Entry.Details["n"])
	// The timestamps should be strictly decreasing.
	assert.True(t, out[0].Timestamp.After(out[1].Timestamp))
	assert.True(t, out[1].Timestamp.After(out[2].Timestamp))
}

func TestMemorySink_ListByResource_ReturnsAllMatchingActions(t *testing.T) {
	sink := NewMemorySink()
	ctx := context.Background()
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "s1"}))
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "deploy", ResourceType: "stack", ResourceID: "s1"}))
	require.NoError(t, sink.Log(ctx, AuditEntry{Action: "delete", ResourceType: "stack", ResourceID: "s1"}))

	out, err := sink.ListByResource(ctx, "stack", "s1")
	require.NoError(t, err)
	assert.Len(t, out, 3, "Reader must not filter by action — callers decide")
	for _, e := range out {
		assert.NotEmpty(t, e.ID)
	}
}
