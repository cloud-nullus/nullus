package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock 은 기다림을 실제로 하지 않고 시간만 흘린다.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.now = c.now.Add(d)
	return nil
}

var triggerNow = time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)

// pushRaceStub 은 브랜치 머리 · 파이프라인 목록 · 생성에 응답한다. shaPipelines 는 머리 커밋으로 조회할 때
// 돌려줄 응답을 차례로 꺼낸다(push 파이프라인이 몇 초 늦게 생기는 것을 흉내).
func pushRaceStub(t *testing.T, committed time.Time, shaPipelines ...[]map[string]any) (string, *int32) {
	t.Helper()
	var posts int32
	var shaCalls int32
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		path := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/repository/branches/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "main", "commit": map[string]any{"id": "abc123", "committed_date": committed.Format(time.RFC3339)},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/pipelines") && r.URL.Query().Get("sha") != "":
			assert.Equal(t, "abc123", r.URL.Query().Get("sha"), "머리 커밋의 파이프라인을 찾는다")
			i := int(atomic.AddInt32(&shaCalls, 1)) - 1
			if i >= len(shaPipelines) {
				i = len(shaPipelines) - 1
			}
			var resp []map[string]any
			if i >= 0 {
				resp = shaPipelines[i]
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/pipelines"):
			// 브랜치의 최신 파이프라인 — 아직 없다(push 파이프라인이 늦게 생긴다).
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/pipeline"):
			atomic.AddInt32(&posts, 1)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 2, "web_url": "http://gl/acme/shop/-/pipelines/2"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	return srv.URL, &posts
}

func newRaceTrigger(baseURL string, clock *fakeClock) *PipelineTrigger {
	return NewPipelineTrigger(NewClient(baseURL, "tok"), "acme").
		WithWebBaseURL("https://gitlab.nullus.local").
		withClock(clock.Now, clock.Sleep)
}

// 파이프라인을 만든 직후의 "실행" 은 스캐폴딩 커밋의 push 파이프라인보다 먼저 목록을 본다. push 파이프라인은
// 커밋 API 응답보다 몇 초 늦게 생기므로, 그때 새로 만들면 파이프라인이 둘 생기고 둘째 deploy 가 되커밋 경쟁에서
// non-fast-forward 로 실패했다(2026-10-18 로컬 시연). 머리 커밋이 방금 생겼으면 그 파이프라인을 잠깐 기다려 붙는다.
func TestPipelineTrigger_WaitsForPushPipelineOfFreshCommit(t *testing.T) {
	clock := &fakeClock{now: triggerNow}
	base, posts := pushRaceStub(t, triggerNow.Add(-3*time.Second),
		nil, nil, []map[string]any{{"id": 1, "status": "pending", "web_url": "http://gl/acme/shop/-/pipelines/1"}})

	runURL, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, "https://gitlab.nullus.local/acme/shop/-/pipelines/1", runURL)
	assert.Equal(t, int32(0), atomic.LoadInt32(posts), "push 파이프라인에 붙고 새로 만들지 않는다")
}

// 오래된 커밋이면 기다리지 않는다 — push 파이프라인이 생길 일이 없고, 사용자는 지금 다시 돌리길 바란다.
func TestPipelineTrigger_DoesNotWaitForOldCommit(t *testing.T) {
	clock := &fakeClock{now: triggerNow}
	base, posts := pushRaceStub(t, triggerNow.Add(-10*time.Minute))

	_, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(posts))
	assert.Equal(t, triggerNow, clock.now, "기다리지 않았다")
}

// 머리 커밋의 파이프라인이 이미 끝났거나 [skip ci] 로 건너뛰었으면 붙을 것이 없다 — 새로 만든다.
func TestPipelineTrigger_CreatesWhenFreshCommitPipelineIsDone(t *testing.T) {
	for _, status := range []string{"success", "failed", "skipped", "canceled"} {
		t.Run(status, func(t *testing.T) {
			clock := &fakeClock{now: triggerNow}
			base, posts := pushRaceStub(t, triggerNow.Add(-5*time.Second),
				[]map[string]any{{"id": 1, "status": status}})

			_, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

			require.NoError(t, err)
			assert.Equal(t, int32(1), atomic.LoadInt32(posts))
		})
	}
}

// push 파이프라인이 끝내 생기지 않으면(CI 정의가 없는 등) 정해 둔 만큼만 기다리고 만든다.
func TestPipelineTrigger_CreatesAfterWaitingTooLong(t *testing.T) {
	clock := &fakeClock{now: triggerNow}
	base, posts := pushRaceStub(t, triggerNow.Add(-2*time.Second), nil)

	_, err := newRaceTrigger(base, clock).TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(posts))
	waited := clock.now.Sub(triggerNow)
	assert.GreaterOrEqual(t, waited, pushPipelineWait)
	assert.Less(t, waited, pushPipelineWait+pushPipelinePoll+time.Second, "정해 둔 만큼만 기다린다")
}
