package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/observability/usecase"
)

type countingCollector struct {
	runs  atomic.Int32
	block chan struct{}
}

func (c *countingCollector) Run(ctx context.Context) (usecase.CollectResult, error) {
	c.runs.Add(1)
	if c.block != nil {
		<-c.block
	}
	return usecase.CollectResult{Stacks: 1}, nil
}

// 앞선 수집이 끝나지 않았으면 겹쳐 돌지 않는다.
func TestStackAlertScheduler_SkipsWhileInFlight(t *testing.T) {
	c := &countingCollector{block: make(chan struct{})}
	s := NewStackAlertScheduler(c, time.Hour, nil)
	go s.runOnce(context.Background())
	assert.Eventually(t, func() bool { return c.runs.Load() == 1 }, time.Second, 5*time.Millisecond)
	s.runOnce(context.Background())
	assert.EqualValues(t, 1, c.runs.Load())
	close(c.block)
}

// 시작하자마자 한 번 돌고, 주기마다 돌고, ctx 가 끝나면 멈춘다.
func TestStackAlertScheduler_RunsImmediatelyAndOnInterval(t *testing.T) {
	c := &countingCollector{}
	s := NewStackAlertScheduler(c, 20*time.Millisecond, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()
	assert.Eventually(t, func() bool { return c.runs.Load() >= 3 }, 2*time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ctx 가 끝났는데 멈추지 않았다")
	}
}

func TestParseStackAlertInterval(t *testing.T) {
	cases := map[string]time.Duration{"": time.Minute, "30s": 30 * time.Second, "2m": 2 * time.Minute, "0": 0, "off": 0, "nonsense": time.Minute, "1s": 10 * time.Second}
	for in, want := range cases {
		assert.Equal(t, want, ParseStackAlertInterval(in), in)
	}
}
