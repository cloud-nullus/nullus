package notify

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// ChannelNotifier 는 백업 결과를 조직의 알림 채널(Telegram · 이메일 · Slack)로 보낸다(B3-3, nullus-plan#63).
//
// 구조화 로그는 그대로 남긴다 — 채널이 하나도 없거나 발송이 실패해도 결과가 어딘가에는 남아야 한다.
// 어느 채널로 보낼지는 알림 모듈이 조직의 구독으로 정한다. 백업은 이벤트만 낸다.
//
// 발송은 백업을 붙들지 않는다 — 결과를 기록한 뒤 따로 보낸다. 백업의 마무리 ctx(짧다)로 보내면 느린
// 채널 하나가 시간을 다 써서 다른 채널과 발송 이력까지 사라진다. 발송 결과는 알림 모듈의 이력에 남고,
// 실패는 여기서 경고 로그로도 남긴다.
type ChannelNotifier struct {
	log       *LogNotifier
	publisher sharednotify.Publisher
	wg        sync.WaitGroup
}

// publishTimeout 은 이벤트 하나를 모든 채널로 보내는 데 쓰는 시간이다. 채널은 동시에 가고, 채널
// 하나는 재시도까지 1분 안쪽이다.
const publishTimeout = 2 * time.Minute

func NewChannelNotifier(log *LogNotifier, publisher sharednotify.Publisher) *ChannelNotifier {
	if log == nil {
		log = NewLogNotifier(nil)
	}
	return &ChannelNotifier{log: log, publisher: publisher}
}

var _ port.Notifier = (*ChannelNotifier)(nil)

func (n *ChannelNotifier) NotifyBackupResult(ctx context.Context, run *domain.BackupRun) error {
	_ = n.log.NotifyBackupResult(ctx, run)
	if n.publisher == nil {
		return nil
	}
	event, ok := backupEvent(run)
	if !ok {
		return nil
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		defer cancel()
		if err := n.publisher.Publish(pubCtx, event); err != nil {
			n.log.logger.Warn("백업 결과 알림 발송 실패 — 알림 채널의 발송 이력을 확인하세요",
				"run_id", run.ID, "event", string(event.Type), "error", err)
		}
	}()
	return nil
}

// Wait 는 보내는 중인 알림이 끝나기를 기다린다(테스트 · 종료).
func (n *ChannelNotifier) Wait() { n.wg.Wait() }

// backupEvent 는 끝난 실행을 알림 이벤트로 옮긴다. 끝나지 않은 실행은 알리지 않는다.
func backupEvent(run *domain.BackupRun) (sharednotify.Event, bool) {
	var (
		typ      sharednotify.EventType
		severity sharednotify.Severity
		title    string
		summary  = run.Error
	)
	target := "플랫폼"
	if run.StackID != "" {
		target = "스택 " + run.StackID
	}
	switch run.Status {
	case domain.StatusFailed:
		typ, severity, title = sharednotify.EventBackupFailed, sharednotify.SeverityCritical, "백업 실패 — "+target
		if summary == "" {
			summary = "원인이 기록되지 않았습니다 — 백업 실행 기록과 API 로그를 확인하세요"
		}
	case domain.StatusPartial:
		typ, severity, title = sharednotify.EventBackupPartial, sharednotify.SeverityWarning, "백업 부분 성공 — "+target
		if summary == "" {
			summary = "일부 컴포넌트가 빠졌습니다 — 복구에 쓸 수 있지만 완전하지 않습니다"
		}
	case domain.StatusSucceeded:
		typ, severity, title = sharednotify.EventBackupSucceeded, sharednotify.SeverityInfo, "백업 성공 — "+target
		summary = "백업이 끝났습니다"
	default:
		return sharednotify.Event{}, false
	}

	fields := []sharednotify.Field{
		{Name: "실행", Value: run.ID},
		{Name: "모드", Value: string(run.Mode)},
		{Name: "트리거", Value: triggerLabel(run.Trigger)},
	}
	if run.SetID != "" {
		fields = append(fields, sharednotify.Field{Name: "세트", Value: run.SetID})
	}
	if run.TotalBytes > 0 {
		fields = append(fields, sharednotify.Field{Name: "크기", Value: humanBytes(run.TotalBytes)})
	}
	if run.StartedAt != nil && run.FinishedAt != nil {
		fields = append(fields, sharednotify.Field{Name: "소요", Value: run.FinishedAt.Sub(*run.StartedAt).Round(time.Second).String()})
	}
	at := time.Now()
	if run.FinishedAt != nil {
		at = *run.FinishedAt
	}
	return sharednotify.Event{
		Type: typ, OrgID: run.OrgID, Severity: severity, Title: title, Summary: summary,
		Fields: fields, OccurredAt: at,
	}, true
}

func triggerLabel(t domain.Trigger) string {
	switch t {
	case domain.TriggerSchedule:
		return "예약"
	case domain.TriggerManual:
		return "수동"
	default:
		return string(t)
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
