package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLister struct {
	ids []string
	err error
}

func (f *fakeLister) ListAllIDs(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ids, nil
}

type fakeRunner struct {
	mu       sync.Mutex
	called   []string
	failFor  map[string]error
	blockFor map[string]time.Duration
}

func (f *fakeRunner) RefreshDiscoveryByID(ctx context.Context, id string) error {
	f.mu.Lock()
	f.called = append(f.called, id)
	block := f.blockFor[id]
	fail := f.failFor[id]
	f.mu.Unlock()
	if block > 0 {
		select {
		case <-time.After(block):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fail
}

func (f *fakeRunner) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.called))
	copy(cp, f.called)
	return cp
}

// Scenario 1: first tick sweeps every cluster.
func TestRefreshDiscoveryScheduler_FirstIterationSweepsAll(t *testing.T) {
	lister := &fakeLister{ids: []string{"c1", "c2", "c3"}}
	runner := &fakeRunner{}
	s := NewRefreshDiscoveryScheduler(lister, runner, Options{
		Interval:    time.Hour, // don't tick again during the test
		IterTimeout: time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()

	// Wait briefly for the first iteration to complete.
	require.Eventually(t, func() bool { return len(runner.Calls()) == 3 }, time.Second, 10*time.Millisecond)
	cancel()
	<-done

	assert.Equal(t, []string{"c1", "c2", "c3"}, runner.Calls())
}

// Scenario 2: one cluster's refresh fails; subsequent clusters still run.
func TestRefreshDiscoveryScheduler_OneFailureDoesNotHaltSweep(t *testing.T) {
	lister := &fakeLister{ids: []string{"a", "b", "c"}}
	runner := &fakeRunner{failFor: map[string]error{"b": errors.New("boom")}}
	s := NewRefreshDiscoveryScheduler(lister, runner, Options{
		Interval:    time.Hour,
		IterTimeout: time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()

	require.Eventually(t, func() bool { return len(runner.Calls()) == 3 }, time.Second, 10*time.Millisecond)
	cancel()
	<-done

	assert.Equal(t, []string{"a", "b", "c"}, runner.Calls())
}

// Scenario 3: ctx cancellation stops the scheduler loop promptly.
func TestRefreshDiscoveryScheduler_ContextCancel_Stops(t *testing.T) {
	lister := &fakeLister{ids: []string{"a"}}
	runner := &fakeRunner{}
	s := NewRefreshDiscoveryScheduler(lister, runner, Options{
		Interval:    50 * time.Millisecond,
		IterTimeout: time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()

	// Let a couple of iterations run, then cancel.
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop within 1s of cancel")
	}
	assert.Greater(t, len(runner.Calls()), 0)
}

// Scenario 4: in-flight guard — if a previous tick is still running, the
// next tick is skipped. Simulate by making the runner block and checking
// that the second tick's iteration never starts.
func TestRefreshDiscoveryScheduler_InFlightGuard_SkipsOverlappingTick(t *testing.T) {
	var ticks atomic.Int32
	lister := &fakeLister{ids: []string{"slow"}}
	runner := &fakeRunner{
		blockFor: map[string]time.Duration{"slow": 120 * time.Millisecond},
	}
	// Wrap runner to count iterations more easily via ticks.
	wrapped := &countingRunner{inner: runner, count: &ticks}

	s := NewRefreshDiscoveryScheduler(lister, wrapped, Options{
		Interval:    30 * time.Millisecond,
		IterTimeout: time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()

	// In 90ms the scheduler would naively run ~3 iterations (first +
	// two ticks), but the in-flight guard should collapse them into 1.
	time.Sleep(90 * time.Millisecond)
	cancel()
	<-done

	final := int(ticks.Load())
	assert.Equal(t, 1, final, "overlapping iterations should have been skipped; saw %d", final)
}

type countingRunner struct {
	inner *fakeRunner
	count *atomic.Int32
}

func (c *countingRunner) RefreshDiscoveryByID(ctx context.Context, id string) error {
	c.count.Add(1)
	return c.inner.RefreshDiscoveryByID(ctx, id)
}
