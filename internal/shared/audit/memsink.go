package audit

import (
	"context"
	"sync"
)

// MemorySink is a test-friendly Sink that appends every entry into an
// in-process slice instead of writing to Postgres. Safe for parallel tests:
// Log is mutex-protected and Snapshot returns a deep copy so callers can't
// race with subsequent writes.
type MemorySink struct {
	mu      sync.Mutex
	entries []AuditEntry
}

// NewMemorySink constructs an empty MemorySink.
func NewMemorySink() *MemorySink {
	return &MemorySink{}
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
}

// Compile-time proof.
var _ Sink = (*MemorySink)(nil)
