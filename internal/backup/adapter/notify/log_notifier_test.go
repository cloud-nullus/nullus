package notify

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

func capture(t *testing.T) (*LogNotifier, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return NewLogNotifier(slog.New(h)), &buf
}

func TestNotify_성공(t *testing.T) {
	n, buf := capture(t)
	run := domain.NewBackupRun("o", domain.ModeFull, domain.TriggerSchedule, nil)
	run.ID = "b1"
	run.Status = domain.StatusSucceeded

	require.NoError(t, n.NotifyBackupResult(context.Background(), run))
	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "run_id=b1")
}

// partial 을 성공으로 묻으면 복구 시점에야 빠진 것을 알게 된다.
func TestNotify_부분성공은_경고다(t *testing.T) {
	n, buf := capture(t)
	run := domain.NewBackupRun("o", domain.ModeFull, domain.TriggerSchedule, nil)
	run.Status = domain.StatusPartial
	run.Error = "volume: tar 실패"

	require.NoError(t, n.NotifyBackupResult(context.Background(), run))
	assert.Contains(t, buf.String(), "level=WARN")
	assert.Contains(t, buf.String(), "tar")
}

func TestNotify_실패는_에러다(t *testing.T) {
	n, buf := capture(t)
	run := domain.NewBackupRun("o", domain.ModeFull, domain.TriggerManual, nil)
	run.Status = domain.StatusFailed
	run.Error = "목적지 연결 불가"

	require.NoError(t, n.NotifyBackupResult(context.Background(), run))
	assert.Contains(t, buf.String(), "level=ERROR")
}

func TestNotify_정지창_시간을_남긴다(t *testing.T) {
	n, buf := capture(t)
	run := domain.NewBackupRun("o", domain.ModeFull, domain.TriggerSchedule, nil)
	require.NoError(t, run.Start())
	run.BeginQuiesce()
	run.EndQuiesce()
	run.Status = domain.StatusSucceeded

	require.NoError(t, n.NotifyBackupResult(context.Background(), run))
	assert.Contains(t, buf.String(), "quiesce_seconds")
}
