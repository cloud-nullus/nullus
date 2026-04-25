package audit

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// MemorySink is a test-friendly Sink that appends every entry into an
// in-process slice instead of writing to Postgres. Safe for parallel tests:
// Log is mutex-protected and Snapshot returns a deep copy so callers can't
// race with subsequent writes.
type MemorySink struct {
	mu      sync.Mutex
	entries []AuditEntry
	// Recording metadata sidecar — kept in lockstep with `entries` so
	// Reader surfaces don't require touching AuditEntry itself.
	recorded []recordingMeta
	// now allows tests to inject a deterministic clock; nil falls back to
	// time.Now which is what production paths use.
	now   func() time.Time
	idSeq atomic.Uint64
}

type recordingMeta struct {
	id        string
	timestamp time.Time
}

// NewMemorySink constructs an empty MemorySink.
func NewMemorySink() *MemorySink {
	return &MemorySink{}
}

// WithClock overrides the timestamp source — tests use this to verify the
// sort order of ListByResource without relying on wall-clock spacing.
func (m *MemorySink) WithClock(now func() time.Time) *MemorySink {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
	return m
}

func (m *MemorySink) nextID() string {
	return fmt.Sprintf("mem-%d", m.idSeq.Add(1))
}

func (m *MemorySink) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

// Log implements Sink. Clones the AuditEntry's Details map so later mutations
// by the caller don't leak into the recorded snapshot.
func (m *MemorySink) Log(_ context.Context, entry AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := entry
	if entry.Details != nil {
		cp.Details = make(map[string]any, len(entry.Details))
		for k, v := range entry.Details {
			cp.Details[k] = v
		}
	}
	m.entries = append(m.entries, cp)
	m.recorded = append(m.recorded, recordingMeta{id: m.nextID(), timestamp: m.clock()})
	return nil
}

// Snapshot returns a shallow copy of all entries recorded so far. Each
// entry's Details map is copied so tests can mutate the result freely.
func (m *MemorySink) Snapshot() []AuditEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]AuditEntry, len(m.entries))
	for i, entry := range m.entries {
		cp := entry
		if entry.Details != nil {
			cp.Details = make(map[string]any, len(entry.Details))
			for k, v := range entry.Details {
				cp.Details[k] = v
			}
		}
		out[i] = cp
	}
	return out
}

// Reset clears all recorded entries — useful for re-using the same sink
// across multiple subtests.
func (m *MemorySink) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = nil
	m.recorded = nil
}

// ListByResource implements Reader. Returns every entry whose ResourceType
// and ResourceID match the arguments, deep-copied so callers can mutate the
// result freely, sorted by Timestamp DESC.
func (m *MemorySink) ListByResource(_ context.Context, resourceType, resourceID string) ([]TimedEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TimedEntry, 0, len(m.entries))
	for i, entry := range m.entries {
		if entry.ResourceType != resourceType || entry.ResourceID != resourceID {
			continue
		}
		cp := entry
		if entry.Details != nil {
			cp.Details = make(map[string]any, len(entry.Details))
			for k, v := range entry.Details {
				cp.Details[k] = v
			}
		}
		meta := m.recorded[i]
		out = append(out, TimedEntry{ID: meta.id, Timestamp: meta.timestamp, Entry: cp})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	return out, nil
}

// Compile-time proof.
var _ Sink = (*MemorySink)(nil)
var _ Reader = (*MemorySink)(nil)
