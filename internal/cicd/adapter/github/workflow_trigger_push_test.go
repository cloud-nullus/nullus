package github

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.now = c.now.Add(d)
	return nil
}

var triggerNow = time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)

// pushRaceStub 은 브랜치 머리 · 실행 목록 · dispatch 에 응답한다. headRuns 는 머리 커밋으로 조회할 때 돌려줄
// 응답을 차례로 꺼낸다(push 실행이 몇 초 늦게 생기는 것을 흉내).
func pushRaceStub(t *testing.T, committed time.Time, headRuns ...[]map[string]any) (string, *int32) {
	t.Helper()
	var dispatches, headCalls int32
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/shop/branches/main":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "main", "commit": map[string]any{
				"sha": "abc123", "commit": map[string]any{"committer": map[string]any{"date": committed.Format(time.RFC3339)}},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/shop/actions/runs" && r.URL.Query().Get("head_sha") != "":
			assert.Equal(t, "abc123", r.URL.Query().Get("head_sha"))
			i := int(atomic.AddInt32(&headCalls, 1)) - 1
			if i >= len(headRuns) {
				i = len(headRuns) - 1
			}
			var runs []map[string]any
			if i >= 0 {
				runs = headRuns[i]
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": runs})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/shop/actions/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/shop/actions/workflows/nullus-ci.yml/dispatches":
			atomic.AddInt32(&dispatches, 1)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	return srv.URL, &dispatches
}

func newRaceTrigger(baseURL string, clock *fakeClock) *WorkflowTrigger {
	return NewWorkflowTrigger(NewClient(baseURL, "ghp"), "acme", "nullus-ci.yml").withWait(noWait).
		withClock(clock.Now, clock.Sleep)
}

// GitLab 과 같은 경합이다 — 스캐폴딩 커밋의 push 실행이 목록에 늦게 나타나 dispatch 가 둘째 실행을 만들고,
// 두 deploy 가 같은 브랜치에 되커밋한다. 머리 커밋이 방금 생겼으면 그 실행을 잠깐 기다려 붙는다.
func TestWorkflowTrigger_WaitsForPushRunOfFreshCommit(t *testing.T) {
	clock := &fakeClock{now: triggerNow}
	base, dispatches := pushRaceStub(t, triggerNow.Add(-3*time.Second), nil,
		[]map[string]any{{"id": 9, "status": "queued", "event": "push", "html_url": "https://github.com/acme/shop/actions/runs/9"}})

	runURL, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/shop/actions/runs/9", runURL)
	assert.Equal(t, int32(0), atomic.LoadInt32(dispatches))
}

func TestWorkflowTrigger_DoesNotWaitForOldCommit(t *testing.T) {
	clock := &fakeClock{now: triggerNow}
	base, dispatches := pushRaceStub(t, triggerNow.Add(-10*time.Minute))

	_, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(dispatches))
	assert.Equal(t, triggerNow, clock.now)
}

func TestWorkflowTrigger_DispatchesWhenFreshCommitRunIsDone(t *testing.T) {
	clock := &fakeClock{now: triggerNow}
	base, dispatches := pushRaceStub(t, triggerNow.Add(-5*time.Second),
		[]map[string]any{{"id": 9, "status": "completed", "event": "push"}})

	_, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(dispatches))
}

// GitHub 은 [skip ci] 커밋에 실행을 만들지 않는다. 정해 둔 만큼만 기다리고 dispatch 한다.
func TestWorkflowTrigger_DispatchesAfterWaitingTooLong(t *testing.T) {
	clock := &fakeClock{now: triggerNow}
	base, dispatches := pushRaceStub(t, triggerNow.Add(-2*time.Second), nil)

	_, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(dispatches))
	waited := clock.now.Sub(triggerNow)
	assert.GreaterOrEqual(t, waited, pushRunWait)
	assert.Less(t, waited, pushRunWait+pushRunPoll+time.Second)
}
