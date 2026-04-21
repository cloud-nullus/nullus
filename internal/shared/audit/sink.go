package audit

import (
	"context"
	"time"
)

// Sink is the minimal interface handlers and use cases depend on to emit
// audit entries. The production implementation is *AuditLogger (writes to
// Postgres via pgxpool); tests inject a capturing in-memory sink so e2e
// suites can assert on the emitted fields without standing up a DB.
//
// This abstraction was added in Phase 2 of the F8 follow-up (2026-04-20)
// so Task 7-style E2Es could verify acknowledge_warnings / issue_codes
// propagation without touching the unexported auditQuerier machinery.
type Sink interface {
	Log(ctx context.Context, entry AuditEntry) error
}

// TimedEntry pairs an AuditEntry with the ID and Timestamp that the Reader
// surfaces externally. AuditEntry itself is intentionally kept lean —
// handlers and use cases should not need to know about recording metadata
// when emitting events.
type TimedEntry struct {
	ID        string
	Timestamp time.Time
	Entry     AuditEntry
}

// Reader exposes recorded audit events for per-resource queries. Added in
// the F8 follow-up so the retry-history surface can render emitted events
// without a separate storage path.
type Reader interface {
	ListByResource(ctx context.Context, resourceType, resourceID string) ([]TimedEntry, error)
}

// Compile-time proof that the production logger still satisfies Sink —
// tripped immediately if a future refactor changes the Log signature.
var _ Sink = (*AuditLogger)(nil)
