package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAddToolsRepo struct {
	stack      *domain.Stack
	findErr    error
	updateErr  error
	updated    *domain.Stack
	findCalled bool
}

func (f *fakeAddToolsRepo) Create(context.Context, *domain.Stack) error { return nil }
func (f *fakeAddToolsRepo) GetByID(context.Context, string) (*domain.Stack, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAddToolsRepo) List(context.Context, string) ([]*domain.Stack, error) { return nil, nil }
func (f *fakeAddToolsRepo) Update(context.Context, *domain.Stack) error           { return nil }
func (f *fakeAddToolsRepo) Delete(context.Context, string) error                  { return nil }

func (f *fakeAddToolsRepo) FindByID(_ context.Context, _ string) (*domain.Stack, error) {
	f.findCalled = true
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.stack == nil {
		return nil, errors.New("stack not found")
	}
	cp := *f.stack
	return &cp, nil
}

func (f *fakeAddToolsRepo) UpdateTools(_ context.Context, stack *domain.Stack) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	cp := *stack
	f.updated = &cp
	return nil
}

func TestAddToolsUseCase_Execute_Success(t *testing.T) {
	repo := &fakeAddToolsRepo{stack: &domain.Stack{
		ID:        "stk-1",
		Tools:     []domain.ToolConfig{{Category: "artifacts", Tool: "harbor", Version: "2.9.0"}},
		UpdatedAt: time.Now().Add(-time.Minute),
	}}
	uc := NewAddToolsUseCase(repo)

	out, err := uc.Execute(context.Background(), AddToolsInput{
		StackID: "stk-1",
		Tools:   []domain.ToolConfig{{Category: "pipeline", Tool: "argo-cd", Version: "2.11.0"}},
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, repo.findCalled)
	require.NotNil(t, repo.updated)
	assert.Len(t, out.Tools, 2)
	assert.Equal(t, "argo-cd", out.Tools[1].Tool)
}

func TestAddToolsUseCase_Execute_DuplicateTool(t *testing.T) {
	repo := &fakeAddToolsRepo{stack: &domain.Stack{
		ID:    "stk-1",
		Tools: []domain.ToolConfig{{Category: "pipeline", Tool: "argo-cd", Version: "2.10.0"}},
	}}
	uc := NewAddToolsUseCase(repo)

	out, err := uc.Execute(context.Background(), AddToolsInput{
		StackID: "stk-1",
		Tools:   []domain.ToolConfig{{Category: "pipeline", Tool: "argo-cd", Version: "2.11.0"}},
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "already exists")
	assert.Nil(t, repo.updated)
}

func TestAddToolsUseCase_Execute_StackNotFound(t *testing.T) {
	repo := &fakeAddToolsRepo{findErr: errors.New("missing")}
	uc := NewAddToolsUseCase(repo)

	out, err := uc.Execute(context.Background(), AddToolsInput{
		StackID: "missing",
		Tools:   []domain.ToolConfig{{Category: "logging", Tool: "loki", Version: "3.0.0"}},
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "stack not found")
}
