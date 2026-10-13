// Package scheduler 는 스택 경보 수집 주기다(nullus-plan#87). 골격은 백업 · 토큰 회전 스케줄러를 따른다.
package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cloud-nullus/draft/internal/observability/usecase"
)

const (
	// DefaultStackAlertInterval 은 기본 수집 주기다. 경보 전달이 이만큼 늦을 수 있다.
	DefaultStackAlertInterval = time.Minute
	// minStackAlertInterval 은 스택 수 × 주기만큼 API 서버 프록시를 부르므로 너무 잦지 않게 막는 하한이다.
	minStackAlertInterval = 10 * time.Second
	// iterTimeout 은 한 번 도는 데 쓰는 시간이다(스택마다 조회 15초 · 발송은 채널마다 1분 안쪽).
	iterTimeout = 5 * time.Minute
)

// Collector 는 한 번의 수집이다(usecase.CollectStackAlerts).
type Collector interface {
	Run(ctx context.Context) (usecase.CollectResult, error)
}

type StackAlertScheduler struct {
	collector Collector
	interval  time.Duration
	logger    *slog.Logger
	inFlight  atomic.Bool
}

func NewStackAlertScheduler(collector Collector, interval time.Duration, logger *slog.Logger) *StackAlertScheduler {
	if interval <= 0 {
		interval = DefaultStackAlertInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &StackAlertScheduler{collector: collector, interval: interval, logger: logger}
}

// ParseStackAlertInterval 은 NULLUS_STACK_ALERT_INTERVAL 이다. 비면 1분, "0"·"off" 면 끈다(0), 읽을 수 없으면
// 기본값, 10초보다 짧으면 10초다.
func ParseStackAlertInterval(raw string) time.Duration {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "":
		return DefaultStackAlertInterval
	case "0", "off", "false", "disabled":
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return DefaultStackAlertInterval
	}
	if d < minStackAlertInterval {
		return minStackAlertInterval
	}
	return d
}

// Start 는 바로 한 번 돌고 주기마다 돈다. ctx 가 끝나면 멈춘다.
func (s *StackAlertScheduler) Start(ctx context.Context) {
	s.logger.Info("스택 경보 수집을 시작합니다", "interval", s.interval.String())
	s.runOnce(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("스택 경보 수집을 멈춥니다")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *StackAlertScheduler) runOnce(ctx context.Context) {
	if !s.inFlight.CompareAndSwap(false, true) {
		return
	}
	defer s.inFlight.Store(false)
	iterCtx, cancel := context.WithTimeout(ctx, iterTimeout)
	defer cancel()
	res, err := s.collector.Run(iterCtx)
	if err != nil {
		s.logger.Warn("스택 경보 수집 실패", "error", err)
		return
	}
	if res.Fired+res.Repeated+res.Resolved+res.Closed+res.Unreachable > 0 {
		s.logger.Info("스택 경보 수집", "stacks", res.Stacks, "fired", res.Fired, "repeated", res.Repeated,
			"resolved", res.Resolved, "closed", res.Closed, "unreachable", res.Unreachable)
	}
}
