package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

type recordingPublisher struct {
	events []sharednotify.Event
	err    error
}

func (p *recordingPublisher) Publish(_ context.Context, e sharednotify.Event) error {
	p.events = append(p.events, e)
	return p.err
}

func finishedRun(status domain.Status) *domain.BackupRun {
	start := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)
	end := start.Add(95 * time.Second)
	return &domain.BackupRun{
		ID: "bkp_1", OrgID: "org-1", StackID: "stk_1", Mode: domain.ModeFull, Trigger: domain.TriggerSchedule,
		Status: status, TotalBytes: 1536 * 1024 * 1024, StartedAt: &start, FinishedAt: &end,
	}
}

func fieldValue(e sharednotify.Event, name string) string {
	for _, f := range e.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

func TestChannelNotifier_FailedBackupIsCritical(t *testing.T) {
	pub := &recordingPublisher{}
	run := finishedRun(domain.StatusFailed)
	run.Error = "목적지에 쓸 수 없습니다: connection refused"

	require.NoError(t, NewChannelNotifier(nil, pub).NotifyBackupResult(context.Background(), run))

	require.Len(t, pub.events, 1)
	e := pub.events[0]
	assert.Equal(t, sharednotify.EventBackupFailed, e.Type)
	assert.Equal(t, sharednotify.SeverityCritical, e.Severity)
	assert.Equal(t, "org-1", e.OrgID)
	assert.Contains(t, e.Title, "백업 실패")
	assert.Contains(t, e.Title, "stk_1")
	assert.Equal(t, "목적지에 쓸 수 없습니다: connection refused", e.Summary)
	assert.Equal(t, "bkp_1", fieldValue(e, "실행"))
	assert.Equal(t, "예약", fieldValue(e, "트리거"))
	assert.Equal(t, "1m35s", fieldValue(e, "소요"))
	assert.Equal(t, *run.FinishedAt, e.OccurredAt)
}

func TestChannelNotifier_PartialAndSucceeded(t *testing.T) {
	pub := &recordingPublisher{}
	n := NewChannelNotifier(nil, pub)

	partial := finishedRun(domain.StatusPartial)
	partial.Error = "volume data-gitlab-0: timeout"
	require.NoError(t, n.NotifyBackupResult(context.Background(), partial))

	ok := finishedRun(domain.StatusSucceeded)
	ok.StackID = ""
	ok.Mode = domain.ModePlatformOnly
	require.NoError(t, n.NotifyBackupResult(context.Background(), ok))

	require.Len(t, pub.events, 2)
	assert.Equal(t, sharednotify.EventBackupPartial, pub.events[0].Type)
	assert.Equal(t, sharednotify.SeverityWarning, pub.events[0].Severity)
	assert.Equal(t, sharednotify.EventBackupSucceeded, pub.events[1].Type)
	assert.Equal(t, sharednotify.SeverityInfo, pub.events[1].Severity)
	assert.Contains(t, pub.events[1].Title, "플랫폼")
	assert.Equal(t, "1.5 GiB", fieldValue(pub.events[1], "크기"))
}

// 세트 백업의 멤버면 세트를 알린다 — 같은 세트의 알림을 묶어 볼 수 있다.
func TestChannelNotifier_IncludesSet(t *testing.T) {
	pub := &recordingPublisher{}
	run := finishedRun(domain.StatusFailed)
	run.SetID = "set_9"
	require.NoError(t, NewChannelNotifier(nil, pub).NotifyBackupResult(context.Background(), run))
	assert.Equal(t, "set_9", fieldValue(pub.events[0], "세트"))
}

// 아직 끝나지 않은 실행은 알리지 않는다.
func TestChannelNotifier_IgnoresUnfinishedRuns(t *testing.T) {
	pub := &recordingPublisher{}
	n := NewChannelNotifier(nil, pub)
	require.NoError(t, n.NotifyBackupResult(context.Background(), finishedRun(domain.StatusRunning)))
	assert.Empty(t, pub.events)
}

// 채널 발송 실패는 돌려주되(백업 유스케이스가 경고로 남긴다) 로그 알림은 그대로 남는다.
func TestChannelNotifier_ReturnsPublishErrorAndWorksWithoutPublisher(t *testing.T) {
	pub := &recordingPublisher{err: errors.New("telegram: chat not found")}
	err := NewChannelNotifier(nil, pub).NotifyBackupResult(context.Background(), finishedRun(domain.StatusFailed))
	assert.ErrorContains(t, err, "chat not found")

	assert.NoError(t, NewChannelNotifier(nil, nil).NotifyBackupResult(context.Background(), finishedRun(domain.StatusFailed)))
}
