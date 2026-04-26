package port

import (
	"context"
	"errors"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// Sentinel errors exposed so handlers can map to HTTP status without
// string matching. F8-Phase5 (재개) introduces CRUD on compatibility
// matrices; admin endpoints distinguish 404 (not found) vs 409 (already
// exists) based on these values.
var (
	ErrCompatibilityMatrixNotFound = errors.New("compatibility matrix not found")
	ErrCompatibilityMatrixExists   = errors.New("compatibility matrix already exists")
)

// CompatibilityRepository defines the interface for compatibility matrix persistence.
type CompatibilityRepository interface {
	GetAll(ctx context.Context) ([]*domain.CompatibilityMatrix, error)
	GetByID(ctx context.Context, id string) (*domain.CompatibilityMatrix, error)
	Validate(ctx context.Context, tools map[string]string) (*domain.CompatibilityMatrix, error)

	// Create persists a new matrix. Returns ErrCompatibilityMatrixExists
	// when the id collides with an existing row.
	Create(ctx context.Context, m *domain.CompatibilityMatrix) error
	// Update replaces every mutable field on the matrix identified by
	// m.ID. Returns ErrCompatibilityMatrixNotFound when the row is
	// missing; no-op partial-update semantics — callers send the full
	// desired state.
	Update(ctx context.Context, m *domain.CompatibilityMatrix) error
	// Delete removes the matrix. Idempotent — missing row returns nil so
	// the admin UI can re-issue delete on an already-deleted id without
	// error. Handlers map post-facto 404s via GetByID when strict
	// semantics are desired.
	Delete(ctx context.Context, id string) error
}
