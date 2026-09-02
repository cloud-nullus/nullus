// Package notify 는 백업 결과 알림 어댑터다 (B3-3).
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §9 F10 (nullus-plan#75)
//
// "백업 실패보다 백업 실패를 모르는 것이 더 나쁘다." 채널 발송은 #63(Alert
// 채널 1차 확장)에 의존하므로, 그때까지는 최소한 구조화 로그로 남긴다 —
// 조용히 지나가는 것만은 막는다.
package notify

import (
	"context"
	"log/slog"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

type LogNotifier struct {
	logger *slog.Logger
}

func NewLogNotifier(logger *slog.Logger) *LogNotifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogNotifier{logger: logger}
}

func (n *LogNotifier) NotifyBackupResult(_ context.Context, run *domain.BackupRun) error {
	attrs := []any{
		"event", "backup_result",
		"run_id", run.ID,
		"org_id", run.OrgID,
		"mode", string(run.Mode),
		"trigger", string(run.Trigger),
		"status", string(run.Status),
		"total_bytes", run.TotalBytes,
	}
	if run.QuiesceStartedAt != nil && run.QuiesceEndedAt != nil {
		attrs = append(attrs, "quiesce_seconds", run.QuiesceEndedAt.Sub(*run.QuiesceStartedAt).Seconds())
	}

	switch run.Status {
	case domain.StatusSucceeded:
		n.logger.Info("백업 성공", attrs...)
	case domain.StatusPartial:
		// 부분 성공은 "쓸 수 있지만 완전하지 않은 백업" 이다. 성공으로
		// 묻으면 복구 시점에야 빠진 것을 알게 된다.
		n.logger.Warn("백업 부분 성공 — 일부 컴포넌트가 빠졌습니다",
			append(attrs, "error", run.Error)...)
	default:
		n.logger.Error("백업 실패", append(attrs, "error", run.Error)...)
	}
	return nil
}

var _ port.Notifier = (*LogNotifier)(nil)
