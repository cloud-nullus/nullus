package port

import (
	"context"
	"time"
)

// LogEntry represents a single log event emitted during a deployment.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // info, warn, error
	Step      string    `json:"step"`  // e.g. "installing_minio", "configuring_argocd"
	Message   string    `json:"message"`
	Phase     string    `json:"phase"` // A, B, C
}

// LogStreamer defines the interface for publishing and consuming deployment log entries.
type LogStreamer interface {
	// Stream publishes a log entry for the given deploymentID.
	Stream(ctx context.Context, deploymentID string, entry LogEntry)
	// Subscribe returns a channel that receives log entries for the given deploymentID.
	Subscribe(deploymentID string) <-chan LogEntry
	// Unsubscribe removes the given channel from the subscriber list for deploymentID.
	Unsubscribe(deploymentID string, ch <-chan LogEntry)
}
