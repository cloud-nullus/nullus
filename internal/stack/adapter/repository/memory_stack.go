package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// MemoryStackRepository is an in-memory implementation of port.StackRepository.
// It is intended for development and testing; it is not safe to use across
// process restarts.
type MemoryStackRepository struct {
	mu     sync.RWMutex
	stacks map[string]*domain.Stack
}

// NewMemoryStackRepository constructs an empty MemoryStackRepository.
func NewMemoryStackRepository() *MemoryStackRepository {
	return &MemoryStackRepository{
		stacks: make(map[string]*domain.Stack),
	}
}

// Create stores a new stack. Returns an error if the ID already exists.
func (r *MemoryStackRepository) Create(_ context.Context, stack *domain.Stack) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.stacks[stack.ID]; ok {
		return fmt.Errorf("stack %q already exists", stack.ID)
	}
	cp := *stack
	r.stacks[stack.ID] = &cp
	return nil
}

// GetByID retrieves a stack by its ID.
func (r *MemoryStackRepository) GetByID(_ context.Context, id string) (*domain.Stack, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stacks[id]
	if !ok {
		return nil, fmt.Errorf("stack %q not found", id)
	}
	cp := *s
	return &cp, nil
}

// List returns all stacks belonging to the given organization.
func (r *MemoryStackRepository) List(_ context.Context, orgID string) ([]*domain.Stack, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Stack, 0, len(r.stacks))
	for _, s := range r.stacks {
		if s.OrgID == orgID {
			cp := *s
			result = append(result, &cp)
		}
	}
	return result, nil
}

// Update replaces a stored stack with the given value.
func (r *MemoryStackRepository) Update(_ context.Context, stack *domain.Stack) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.stacks[stack.ID]; !ok {
		return fmt.Errorf("stack %q not found", stack.ID)
	}
	cp := *stack
	r.stacks[stack.ID] = &cp
	return nil
}

// Delete removes a stack by ID.
func (r *MemoryStackRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.stacks[id]; !ok {
		return fmt.Errorf("stack %q not found", id)
	}
	delete(r.stacks, id)
	return nil
}
