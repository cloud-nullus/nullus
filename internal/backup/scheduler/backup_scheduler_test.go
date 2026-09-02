package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

func TestNew_기본값(t *testing.T) {
	s := New(nil, nil, Config{})
	assert.Equal(t, 24*time.Hour, s.interval, "RPO 24시간")
	assert.Equal(t, 4*time.Hour, s.iterTimeout, "RTO 와 같은 자릿수")
	assert.Equal(t, domain.ModeFull, s.mode)
}

func TestStart_백업이_없으면_시작하지_않는다(t *testing.T) {
	// 구성되지 않은 채로 도는 스케줄러는 조용히 아무것도 하지 않으면서
	// "백업이 돌고 있다" 는 착각만 만든다.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	New(nil, nil, Config{Interval: time.Millisecond}).Start(ctx) // 즉시 반환해야 한다
}

func TestTick_중복_실행을_막는다(t *testing.T) {
	// 정지 창이 겹치면 앞선 실행의 재개와 뒤 실행의 정지가 엇갈린다.
	s := New(nil, nil, Config{})
	assert.True(t, s.inFlight.CompareAndSwap(false, true))

	done := make(chan struct{})
	go func() {
		s.tick(context.Background()) // inFlight 가 true 라 즉시 빠져나와야 한다
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("inFlight 가드가 동작하지 않았다")
	}
}
