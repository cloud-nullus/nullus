package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuditQuerier struct {
	execFn     func(ctx context.Context, sql string, args ...any) error
	queryFn    func(ctx context.Context, sql string, args ...any) (auditRows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) auditRow
}

func (m *mockAuditQuerier) Exec(ctx context.Context, sql string, args ...any) error {
	if m.execFn == nil {
		return nil
	}
	return m.execFn(ctx, sql, args...)
}

func (m *mockAuditQuerier) Query(ctx context.Context, sql string, args ...any) (auditRows, error) {
	if m.queryFn == nil {
		return &mockRows{}, nil
	}
	return m.queryFn(ctx, sql, args...)
}

func (m *mockAuditQuerier) QueryRow(ctx context.Context, sql string, args ...any) auditRow {
	if m.queryRowFn == nil {
		return &mockRow{}
	}
	return m.queryRowFn(ctx, sql, args...)
}

type mockRows struct {
	idx  int
	rows []mockScanRow
	err  error
}

func (r *mockRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *mockRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("scan called without current row")
	}
	return r.rows[r.idx-1].scan(dest...)
}

func (r *mockRows) Close() {}

func (r *mockRows) Err() error { return r.err }

type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.scanFn == nil {
		return nil
	}
	return r.scanFn(dest...)
}

type mockScanRow struct {
	userID       string
	action       string
	resourceType string
	resourceID   string
	details      []byte
	ipAddress    string
	createdAt    time.Time
}

func (r mockScanRow) scan(dest ...any) error {
	if len(dest) != 7 {
		return errors.New("unexpected destination count")
	}
	*(dest[0].(*string)) = r.userID
	*(dest[1].(*string)) = r.action
	*(dest[2].(*string)) = r.resourceType
	*(dest[3].(*string)) = r.resourceID
	*(dest[4].(*[]byte)) = r.details
	*(dest[5].(*string)) = r.ipAddress
	*(dest[6].(*time.Time)) = r.createdAt
	return nil
}

func TestAuditLogger_Log_InsertsEntry(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any

	mockQ := &mockAuditQuerier{
		execFn: func(_ context.Context, sql string, args ...any) error {
			capturedSQL = sql
			capturedArgs = args
			return nil
		},
	}

	logger := NewAuditLoggerWithQuerier(nil, mockQ)

	err := logger.Log(context.Background(), AuditEntry{
		UserID:       "user-1",
		Action:       "deploy",
		ResourceType: "stack",
		ResourceID:   "stack-1",
		Details:      map[string]any{"status": "started"},
		IPAddress:    "127.0.0.1",
	})

	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT INTO audit_logs")
	require.Len(t, capturedArgs, 6)
	assert.Equal(t, "user-1", capturedArgs[0])
	assert.Equal(t, "deploy", capturedArgs[1])
	assert.Equal(t, "stack", capturedArgs[2])
	assert.Equal(t, "stack-1", capturedArgs[3])

	detailsJSON, ok := capturedArgs[4].([]byte)
	require.True(t, ok)
	var details map[string]any
	require.NoError(t, json.Unmarshal(detailsJSON, &details))
	assert.Equal(t, "started", details["status"])
	assert.Equal(t, "127.0.0.1", capturedArgs[5])
}

func TestAuditLogger_List_ReturnsPaginatedResults(t *testing.T) {
	now := time.Now().UTC()

	mockQ := &mockAuditQuerier{
		queryFn: func(_ context.Context, sql string, args ...any) (auditRows, error) {
			assert.Contains(t, sql, "FROM audit_logs")
			require.Len(t, args, 2)
			assert.Equal(t, 2, args[0])
			assert.Equal(t, 1, args[1])

			return &mockRows{rows: []mockScanRow{
				{
					userID:       "user-1",
					action:       "create",
					resourceType: "organization",
					resourceID:   "org-1",
					details:      []byte(`{"name":"acme"}`),
					ipAddress:    "10.0.0.1",
					createdAt:    now,
				},
				{
					userID:       "user-2",
					action:       "delete",
					resourceType: "cluster",
					resourceID:   "cluster-1",
					details:      []byte(`{"reason":"cleanup"}`),
					ipAddress:    "10.0.0.2",
					createdAt:    now,
				},
			}}, nil
		},
		queryRowFn: func(_ context.Context, sql string, _ ...any) auditRow {
			assert.Contains(t, sql, "SELECT COUNT(*)")
			return &mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int)) = 7
				return nil
			}}
		},
	}

	logger := NewAuditLoggerWithQuerier(nil, mockQ)
	items, total, err := logger.List(context.Background(), 2, 1)

	require.NoError(t, err)
	assert.Equal(t, 7, total)
	require.Len(t, items, 2)
	assert.Equal(t, "user-1", items[0].UserID)
	assert.Equal(t, "create", items[0].Action)
	assert.Equal(t, "acme", items[0].Details["name"])
	assert.Equal(t, "user-2", items[1].UserID)
	assert.Equal(t, "cleanup", items[1].Details["reason"])
}
