package port

import "context"

// StackSummary contains the minimal information the CI/CD module needs
// from the Stack context. This avoids importing stack/domain directly,
// keeping the Bounded Context boundary intact.
type StackSummary struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	ClusterID string `json:"cluster_id"`
	State     string `json:"state"` // "completed", "failed", etc.
}

// StackReader provides read-only access to Stack context data.
// This is a Port (in Hexagonal Architecture terms) that the CI/CD
// context uses to validate cross-context references without
// importing the Stack domain directly.
type StackReader interface {
	// GetStackSummary retrieves minimal stack information by ID.
	// Returns nil and no error if the stack does not exist.
	GetStackSummary(ctx context.Context, stackID string) (*StackSummary, error)
}
