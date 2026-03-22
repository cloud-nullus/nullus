package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStack_AddTools_AppendsNewToolsAndUpdatesTimestamp(t *testing.T) {
	before := time.Now().Add(-1 * time.Minute)
	s := &Stack{
		ID: "s1",
		Tools: []ToolConfig{
			{Category: "artifacts", Tool: "harbor", Version: "2.9.0"},
		},
		UpdatedAt: before,
	}

	err := s.AddTools([]ToolConfig{
		{Category: "pipeline", Tool: "argo-cd", Version: "2.11.0"},
		{Category: "monitoring", Tool: "prometheus", Version: "2.53.0"},
	})
	require.NoError(t, err)
	assert.Len(t, s.Tools, 3)
	assert.Equal(t, "harbor", s.Tools[0].Tool)
	assert.Equal(t, "argo-cd", s.Tools[1].Tool)
	assert.Equal(t, "prometheus", s.Tools[2].Tool)
	assert.True(t, s.UpdatedAt.After(before))
}

func TestStack_AddTools_DuplicateReturnsErrorAndNoMutation(t *testing.T) {
	before := time.Now().Add(-1 * time.Minute)
	s := &Stack{
		ID: "s1",
		Tools: []ToolConfig{
			{Category: "pipeline", Tool: "argo-cd", Version: "2.10.0"},
		},
		UpdatedAt: before,
	}

	err := s.AddTools([]ToolConfig{{Category: "pipeline", Tool: "argo-cd", Version: "2.11.0"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Len(t, s.Tools, 1)
	assert.Equal(t, before, s.UpdatedAt)
}

func TestStack_TransitionTo_ValidSequence(t *testing.T) {
	s := &Stack{ID: "s1", State: StatePending}

	transitions := []DeploymentState{
		StateValidating,
		StateInstalling,
		StateConfiguring,
		StateHealthCheck,
		StateCompleted,
	}

	for _, next := range transitions {
		prev := s.State
		err := s.TransitionTo(next)
		require.NoError(t, err, "transition from %s to %s should succeed", prev, next)
		assert.Equal(t, next, s.State)
	}
}

func TestStack_TransitionTo_InvalidDirectToCompleted(t *testing.T) {
	s := &Stack{ID: "s1", State: StatePending}

	err := s.TransitionTo(StateCompleted)
	assert.Error(t, err)
	assert.Equal(t, StatePending, s.State, "state should not change on invalid transition")
}

func TestStack_TransitionTo_PendingToFailed(t *testing.T) {
	s := &Stack{ID: "s1", State: StatePending}

	err := s.TransitionTo(StateFailed)
	require.NoError(t, err)
	assert.Equal(t, StateFailed, s.State)
}

func TestStack_TransitionTo_FailedToRollingBack(t *testing.T) {
	s := &Stack{ID: "s1", State: StateFailed}

	err := s.TransitionTo(StateRollingBack)
	require.NoError(t, err)
	assert.Equal(t, StateRollingBack, s.State)
}

func TestStack_TransitionTo_RollingBackToRolledBack(t *testing.T) {
	s := &Stack{ID: "s1", State: StateRollingBack}

	err := s.TransitionTo(StateRolledBack)
	require.NoError(t, err)
	assert.Equal(t, StateRolledBack, s.State)
}

func TestStack_TransitionTo_CompletedIsTerminal(t *testing.T) {
	s := &Stack{ID: "s1", State: StateCompleted}

	err := s.TransitionTo(StatePending)
	assert.Error(t, err)
	assert.Equal(t, StateCompleted, s.State)
}
