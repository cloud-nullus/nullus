// Package scheduler 는 주기 백업과 보존 정책 적용을 돌린다 (B3-1).
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §4.5·§8 (nullus-plan#75)
//
// 골격은 internal/admin/scheduler/token_rotation.go 를 따른다 — interval /
// iterTimeout / inFlight 가드 / slog. 새로 만들 이유가 없다.
package scheduler

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/usecase"
)

// SetRunner 는 세트 백업 유스케이스다(usecase.BackupSetUseCase). 인터페이스인 것은 테스트가 가로채기
// 위해서다 — 세트 유스케이스는 스택 목록과 스택별 어댑터가 있어야 돈다.
type SetRunner interface {
	Run(ctx context.Context, req usecase.RunBackupSetRequest) (*usecase.BackupSetResult, error)
}

type BackupScheduler struct {
	backup    *usecase.BackupUseCase
	retention *usecase.RetentionUseCase
	// set 이 있으면 주기마다 단독 백업 대신 세트(플랫폼 + 조직의 모든 스택)를 뜬다(schedule.all_stacks).
	set SetRunner

	interval    time.Duration
	iterTimeout time.Duration
	logger      *slog.Logger

	// inFlight 는 중복 실행을 막는다.
	//
	// 정지 백업에서 특히 치명적이다 — 정지 창이 겹치면 앞선 실행의 재개와
	// 뒤 실행의 정지가 엇갈려 워크로드가 못 뜬 채로 남을 수 있다.
	inFlight atomic.Bool

	orgID     string
	stackID   string
	namespace string
	mode      domain.Mode
}

type Config struct {
	Interval    time.Duration
	IterTimeout time.Duration
	OrgID       string
	StackID     string
	Namespace   string
	Mode        domain.Mode
	Logger      *slog.Logger
}

func New(backup *usecase.BackupUseCase, retention *usecase.RetentionUseCase, cfg Config) *BackupScheduler {
	if cfg.Interval <= 0 {
		cfg.Interval = 24 * time.Hour // RPO 24시간 (설계 §2)
	}
	if cfg.IterTimeout <= 0 {
		// 정지 백업은 데이터량이 지배한다. RTO 4시간과 같은 자릿수로 잡는다.
		cfg.IterTimeout = 4 * time.Hour
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Mode == "" {
		cfg.Mode = domain.ModeFull
	}
	return &BackupScheduler{
		backup: backup, retention: retention,
		interval: cfg.Interval, iterTimeout: cfg.IterTimeout, logger: cfg.Logger,
		orgID: cfg.OrgID, stackID: cfg.StackID, namespace: cfg.Namespace, mode: cfg.Mode,
	}
}

// WithBackupSet 은 주기 백업을 세트로 바꾼다. nil 이면 바뀌지 않는다.
func (s *BackupScheduler) WithBackupSet(set SetRunner) *BackupScheduler {
	if set != nil {
		s.set = set
	}
	return s
}

// configured 는 돌릴 것이 있는지다 — 단독 유스케이스든 세트든.
func (s *BackupScheduler) configured() bool {
	return s.backup != nil || s.set != nil
}

func (s *BackupScheduler) Start(ctx context.Context) {
	if !s.configured() {
		s.logger.Info("백업 스케줄러가 구성되지 않아 시작하지 않습니다")
		return
	}
	if s.set != nil {
		s.logger.Info("백업 스케줄러 시작 — 플랫폼 + 모든 스택을 한 세트로", "interval", s.interval)
	} else {
		s.logger.Info("백업 스케줄러 시작", "interval", s.interval, "mode", s.mode)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("백업 스케줄러 종료")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *BackupScheduler) tick(parent context.Context) {
	if !s.inFlight.CompareAndSwap(false, true) {
		// 앞선 백업이 아직 돌고 있다. 정지 창이 겹치면 안 된다.
		s.logger.Warn("이전 백업이 아직 진행 중이라 이번 주기를 건너뜁니다")
		return
	}
	defer s.inFlight.Store(false)

	ctx, cancel := context.WithTimeout(parent, s.iterTimeout)
	defer cancel()

	var status domain.Status
	if s.set != nil {
		res, err := s.set.Run(ctx, usecase.RunBackupSetRequest{OrgID: s.orgID, Trigger: domain.TriggerSchedule})
		if err != nil {
			s.logger.Error("주기 세트 백업 실패", "error", err)
			return
		}
		s.logger.Info("주기 세트 백업 완료", "set_id", res.SetID, "status", res.Status,
			"runs", len(res.Runs), "failed", len(res.Failed))
		status = res.Status
	} else {
		run, err := s.backup.Run(ctx, usecase.RunBackupRequest{
			OrgID: s.orgID, StackID: s.stackID, Namespace: s.namespace,
			Mode: s.mode, Trigger: domain.TriggerSchedule,
		})
		if err != nil {
			// 알림은 유스케이스가 이미 보냈다 (§9 F10). 여기서는 로그만 남긴다.
			s.logger.Error("주기 백업 실패", "error", err)
			return
		}
		s.logger.Info("주기 백업 완료", "run_id", run.ID, "status", run.Status, "bytes", run.TotalBytes)
		status = run.Status
	}

	// 보존 정책은 백업이 성공한 뒤에만 적용한다. 실패한 주기에 옛 백업을
	// 지우면 남은 것이 하나도 없는 상태로 갈 수 있다.
	if status == domain.StatusFailed || s.retention == nil {
		return
	}
	deleted, err := s.retention.Apply(ctx, s.orgID)
	if err != nil {
		s.logger.Error("보존 정책 적용 실패", "error", err)
		return
	}
	if len(deleted) > 0 {
		s.logger.Info("보존 정책으로 오래된 백업을 정리했습니다", "count", len(deleted))
	}
}
