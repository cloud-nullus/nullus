package audit

import "context"

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

// Compile-time proof that the production logger still satisfies Sink —
// tripped immediately if a future refactor changes the Log signature.
var _ Sink = (*AuditLogger)(nil)
