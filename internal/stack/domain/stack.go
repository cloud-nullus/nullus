package domain

import (
	"fmt"
	"time"
)

// DeploymentState represents the state of a stack deployment.
type DeploymentState string

const (
	StatePending     DeploymentState = "pending"
	StateValidating  DeploymentState = "validating"
	StateInstalling  DeploymentState = "installing"
	StateConfiguring DeploymentState = "configuring"
	StateHealthCheck DeploymentState = "health_check"
	StateCompleted   DeploymentState = "completed"
	StateFailed      DeploymentState = "failed"
	StateRollingBack DeploymentState = "rolling_back"
	StateRolledBack  DeploymentState = "rolled_back"
)

// validTransitions defines which state transitions are allowed.
var validTransitions = map[DeploymentState][]DeploymentState{
	StatePending:     {StateValidating, StateFailed},
	StateValidating:  {StateInstalling, StateFailed},
	StateInstalling:  {StateConfiguring, StateFailed, StateRollingBack},
	StateConfiguring: {StateHealthCheck, StateFailed, StateRollingBack},
	StateHealthCheck: {StateCompleted, StateFailed, StateRollingBack},
	StateFailed:      {StateRollingBack, StatePending},
	StateRollingBack: {StateRolledBack, StateFailed},
	StateRolledBack:  {StatePending},
	StateCompleted:   {},
}

// Stack represents a deployed stack of DevOps tools.
type Stack struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	TemplateID string          `json:"template_id"`
	OrgID      string          `json:"org_id"`
	ClusterID  string          `json:"cluster_id"`
	State      DeploymentState `json:"state"`
	Config     interface{}     `json:"config"` // JSONB
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// TransitionTo attempts to transition the stack to a new state.
// Returns an error if the transition is not valid.
func (s *Stack) TransitionTo(newState DeploymentState) error {
	allowed, ok := validTransitions[s.State]
	if !ok {
		return fmt.Errorf("unknown current state %q", s.State)
	}
	for _, a := range allowed {
		if a == newState {
			s.State = newState
			s.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("invalid state transition from %q to %q", s.State, newState)
}
