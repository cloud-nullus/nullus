package log

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

const defaultReplayLimit = 2000

// PostgresStreamer persists deployment logs and fans out to live subscribers.
type PostgresStreamer struct {
	db *pgxpool.Pool

	mu          sync.RWMutex
	subscribers map[string][]chan port.LogEntry
	history     map[string][]port.LogEntry
	loaded      map[string]bool
}

func NewPostgresStreamer(db *pgxpool.Pool) *PostgresStreamer {
	return &PostgresStreamer{
		db:          db,
		subscribers: make(map[string][]chan port.LogEntry),
		history:     make(map[string][]port.LogEntry),
		loaded:      make(map[string]bool),
	}
}

func (s *PostgresStreamer) Subscribe(deploymentID string) <-chan port.LogEntry {
	ch := make(chan port.LogEntry, defaultChannelBuffer)
	s.ensureLoaded(deploymentID)

	s.mu.Lock()
	for _, entry := range s.history[deploymentID] {
		select {
		case ch <- entry:
		default:
		}
	}
	s.subscribers[deploymentID] = append(s.subscribers[deploymentID], ch)
	s.mu.Unlock()

	return ch
}

func (s *PostgresStreamer) Unsubscribe(deploymentID string, ch <-chan port.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := s.subscribers[deploymentID]
	for i, sub := range list {
		if sub == ch {
			s.subscribers[deploymentID] = append(list[:i], list[i+1:]...)
			close(sub)
			break
		}
	}
	if len(s.subscribers[deploymentID]) == 0 {
		delete(s.subscribers, deploymentID)
	}
}

func (s *PostgresStreamer) Stream(ctx context.Context, deploymentID string, entry port.LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	if s.db != nil {
		_, _ = s.db.Exec(ctx, `
			INSERT INTO deployment_logs (deployment_id, timestamp, level, step, phase, message)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, deploymentID, entry.Timestamp, entry.Level, entry.Step, entry.Phase, entry.Message)
	}

	s.mu.Lock()
	s.history[deploymentID] = append(s.history[deploymentID], entry)
	list := make([]chan port.LogEntry, len(s.subscribers[deploymentID]))
	copy(list, s.subscribers[deploymentID])
	s.mu.Unlock()

	for _, sub := range list {
		select {
		case sub <- entry:
		default:
		}
	}
}

// ClearHistory clears replay history for a deployment in-memory and persistent store.
func (s *PostgresStreamer) ClearHistory(deploymentID string) {
	s.mu.Lock()
	delete(s.history, deploymentID)
	s.loaded[deploymentID] = true
	s.mu.Unlock()

	if s.db != nil {
		_, _ = s.db.Exec(context.Background(), `DELETE FROM deployment_logs WHERE deployment_id = $1`, deploymentID)
	}
}

func (s *PostgresStreamer) ensureLoaded(deploymentID string) {
	s.mu.RLock()
	loaded := s.loaded[deploymentID]
	s.mu.RUnlock()
	if loaded {
		return
	}

	entries := make([]port.LogEntry, 0)
	if s.db != nil {
		rows, err := s.db.Query(context.Background(), `
			SELECT timestamp, level, step, phase, message
			FROM deployment_logs
			WHERE deployment_id = $1
			ORDER BY id ASC
			LIMIT $2
		`, deploymentID, defaultReplayLimit)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var entry port.LogEntry
				if scanErr := rows.Scan(&entry.Timestamp, &entry.Level, &entry.Step, &entry.Phase, &entry.Message); scanErr == nil {
					entries = append(entries, entry)
				}
			}
		}
	}

	s.mu.Lock()
	if !s.loaded[deploymentID] {
		s.history[deploymentID] = entries
		s.loaded[deploymentID] = true
	}
	s.mu.Unlock()
}
