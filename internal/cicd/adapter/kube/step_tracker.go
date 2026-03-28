package kube

import (
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// StepTracker stores deploy steps in memory for active deployments.
type StepTracker struct {
	mu    sync.RWMutex
	steps map[string][]domain.DeployStep
}

func NewStepTracker() *StepTracker {
	return &StepTracker{steps: make(map[string][]domain.DeployStep)}
}

func (t *StepTracker) Init(deploymentID string, stepNames []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	steps := make([]domain.DeployStep, len(stepNames))
	for i, name := range stepNames {
		steps[i] = domain.DeployStep{Name: name, Status: "pending"}
	}
	t.steps[deploymentID] = steps
}

func (t *StepTracker) MarkRunning(deploymentID string, index int, kind string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.steps[deploymentID]; ok && index < len(s) {
		s[index].Status = "running"
		s[index].Kind = kind
	}
}

func (t *StepTracker) MarkSuccess(deploymentID string, index int, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.steps[deploymentID]; ok && index < len(s) {
		s[index].Status = "success"
		s[index].Message = message
		s[index].AppliedAt = time.Now().Format(time.RFC3339)
	}
}

func (t *StepTracker) MarkFailed(deploymentID string, index int, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.steps[deploymentID]; ok && index < len(s) {
		s[index].Status = "failed"
		s[index].Message = message
		s[index].AppliedAt = time.Now().Format(time.RFC3339)
	}
}

func (t *StepTracker) Get(deploymentID string) []domain.DeployStep {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.steps[deploymentID]
	if !ok {
		return nil
	}
	out := make([]domain.DeployStep, len(s))
	copy(out, s)
	return out
}

// AppendLog appends a log line to the specified step.
func (t *StepTracker) AppendLog(deploymentID string, index int, line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.steps[deploymentID]; ok && index < len(s) {
		s[index].Logs = append(s[index].Logs, line)
	}
}

func (t *StepTracker) Remove(deploymentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.steps, deploymentID)
}
