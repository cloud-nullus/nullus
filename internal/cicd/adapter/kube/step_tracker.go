package kube

import (
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

type LogEvent struct {
	DeploymentID string
	StepIndex    int
	Level        string
	Message      string
	Progress     int
	Status       string
	Timestamp    time.Time
}

// StepTracker stores deploy steps in memory for active deployments.
type StepTracker struct {
	mu          sync.RWMutex
	steps       map[string][]domain.DeployStep
	subscribers map[string][]chan LogEvent
}

func NewStepTracker() *StepTracker {
	return &StepTracker{
		steps:       make(map[string][]domain.DeployStep),
		subscribers: make(map[string][]chan LogEvent),
	}
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
	now := time.Now()
	var event *LogEvent

	t.mu.Lock()
	if s, ok := t.steps[deploymentID]; ok && index < len(s) {
		s[index].Status = "success"
		s[index].Message = message
		s[index].AppliedAt = now.Format(time.RFC3339)

		event = &LogEvent{
			DeploymentID: deploymentID,
			StepIndex:    index,
			Level:        "success",
			Message:      message,
			Progress:     (index + 1) * 100 / len(s),
			Timestamp:    now,
		}
		if index == len(s)-1 {
			event.Status = "success"
		}
	}
	t.mu.Unlock()

	if event != nil {
		t.publish(deploymentID, *event)
	}
}

func (t *StepTracker) MarkFailed(deploymentID string, index int, message string) {
	now := time.Now()
	var event *LogEvent

	t.mu.Lock()
	if s, ok := t.steps[deploymentID]; ok && index < len(s) {
		s[index].Status = "failed"
		s[index].Message = message
		s[index].AppliedAt = now.Format(time.RFC3339)

		event = &LogEvent{
			DeploymentID: deploymentID,
			StepIndex:    index,
			Level:        "error",
			Message:      message,
			Progress:     (index + 1) * 100 / len(s),
			Status:       "failed",
			Timestamp:    now,
		}
	}
	t.mu.Unlock()

	if event != nil {
		t.publish(deploymentID, *event)
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
	now := time.Now()
	var event *LogEvent

	t.mu.Lock()
	if s, ok := t.steps[deploymentID]; ok && index < len(s) {
		s[index].Logs = append(s[index].Logs, line)
		event = &LogEvent{
			DeploymentID: deploymentID,
			StepIndex:    index,
			Level:        "info",
			Message:      line,
			Progress:     (index + 1) * 100 / len(s),
			Timestamp:    now,
		}
	}
	t.mu.Unlock()

	if event != nil {
		t.publish(deploymentID, *event)
	}
}

func (t *StepTracker) Subscribe(deploymentID string) chan LogEvent {
	ch := make(chan LogEvent, 64)

	t.mu.Lock()
	t.subscribers[deploymentID] = append(t.subscribers[deploymentID], ch)
	t.mu.Unlock()

	return ch
}

func (t *StepTracker) Unsubscribe(deploymentID string, ch chan LogEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	subs := t.subscribers[deploymentID]
	for i := range subs {
		if subs[i] == ch {
			t.subscribers[deploymentID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	if len(t.subscribers[deploymentID]) == 0 {
		delete(t.subscribers, deploymentID)
	}
}

func (t *StepTracker) publish(deploymentID string, event LogEvent) {
	t.mu.RLock()
	subs := append([]chan LogEvent(nil), t.subscribers[deploymentID]...)
	t.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (t *StepTracker) Remove(deploymentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, ch := range t.subscribers[deploymentID] {
		close(ch)
	}
	delete(t.subscribers, deploymentID)
	delete(t.steps, deploymentID)
}
