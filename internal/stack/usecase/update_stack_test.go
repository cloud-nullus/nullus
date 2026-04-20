package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedUpdatableStack(t *testing.T, repo *repository.MemoryStackRepository, id string, state domain.DeploymentState) {
	t.Helper()
	stack := &domain.Stack{
		ID:        id,
		Name:      id,
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
		State:     state,
		Config:    domain.StackConfig{},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(context.Background(), stack))
}

func TestUpdateStack_FromPending_Succeeds(t *testing.T) {
	stackRepo := repository.NewMemoryStackRepository()
	historyRepo := repository.NewMemoryHistoryRepository()
	uc := NewUpdateStack(stackRepo, NewManageHistory(historyRepo))

	seedUpdatableStack(t, stackRepo, "s1", domain.StatePending)

	newName := "renamed"
	out, err := uc.Execute(context.Background(), UpdateStackInput{
		StackID: "s1",
		Name:    &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, "renamed", out.Stack.Name)

	versions, _ := historyRepo.ListVersions(context.Background(), "s1")
	assert.Len(t, versions, 1, "pre-update snapshot should have been recorded")
}

func TestUpdateStack_FromFailed_Succeeds(t *testing.T) {
	stackRepo := repository.NewMemoryStackRepository()
	uc := NewUpdateStack(stackRepo, nil) // nil history repo ⇒ no snapshot

	seedUpdatableStack(t, stackRepo, "s2", domain.StateFailed)

	ns := "new-ns"
	out, err := uc.Execute(context.Background(), UpdateStackInput{StackID: "s2", Namespace: &ns})
	require.NoError(t, err)
	assert.Equal(t, "new-ns", out.Stack.Namespace)
}

func TestUpdateStack_FromCompleted_Rejected(t *testing.T) {
	stackRepo := repository.NewMemoryStackRepository()
	uc := NewUpdateStack(stackRepo, nil)

	seedUpdatableStack(t, stackRepo, "s3", domain.StateCompleted)

	newName := "x"
	_, err := uc.Execute(context.Background(), UpdateStackInput{StackID: "s3", Name: &newName})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not updatable")
}

func TestUpdateStack_NotFound(t *testing.T) {
	stackRepo := repository.NewMemoryStackRepository()
	uc := NewUpdateStack(stackRepo, nil)

	_, err := uc.Execute(context.Background(), UpdateStackInput{StackID: "missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
