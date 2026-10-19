package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// WorkflowTrigger 는 port.CIBuildTrigger 의 GitHub Actions 구현체다.
//
// 스택에 묶인 파이프라인의 "실행" 은 플랫폼이 빌드하지 않고 CI 에 넘긴다. GitHub Actions 는
// push 가 있을 때만 돌므로, 커밋 없이 지금 실행하려면 workflow_dispatch 로 시작시켜야 한다
// — 스캐폴딩이 워크플로에 그 트리거를 둔다.
type WorkflowTrigger struct {
	client       *Client
	owner        string
	workflowFile string
	// webBaseURL 은 브라우저가 여는 GitHub 주소다. 비면 API 주소에서 유도한다.
	webBaseURL string
	// wait 는 dispatch 뒤 실행이 목록에 나타나길 기다리는 간격이다. 테스트가 바꾼다.
	wait func()
	// now · sleep 은 push 실행을 기다릴 때 쓴다. 테스트가 바꾼다.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

// NewWorkflowTrigger 는 owner 아래 리포의 workflowFile(.github/workflows/ 안의 파일 이름)을
// 시작시키는 트리거를 만든다.
func NewWorkflowTrigger(client *Client, owner, workflowFile string) *WorkflowTrigger {
	return &WorkflowTrigger{
		client:       client,
		owner:        strings.TrimSpace(owner),
		workflowFile: strings.TrimSpace(workflowFile),
		wait:         func() { time.Sleep(runLookupInterval) },
		now:          time.Now,
		sleep:        sleepContext,
	}
}

func (t *WorkflowTrigger) withClock(now func() time.Time, sleep func(context.Context, time.Duration) error) *WorkflowTrigger {
	t.now, t.sleep = now, sleep
	return t
}

// sleepContext 는 d 만큼 기다리되 요청이 끝나면 멈춘다.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// WithWebBaseURL 은 실행 링크에 쓸 웹 주소를 정한다.
func (t *WorkflowTrigger) WithWebBaseURL(base string) *WorkflowTrigger {
	t.webBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
	return t
}

func (t *WorkflowTrigger) withWait(wait func()) *WorkflowTrigger {
	t.wait = wait
	return t
}

const (
	// runLookupAttempts·runLookupInterval 은 dispatch 뒤 새 실행을 찾는 시도다. GitHub 은
	// dispatch 에 실행 id 를 돌려주지 않고 실행은 몇 초 뒤에 목록에 나타난다.
	runLookupAttempts = 3
	runLookupInterval = 2 * time.Second

	// push 실행을 기다리는 기준이다(GitLab 트리거와 같다). 머리 커밋이 pushRunGrace 안에 생겼고 그 커밋의
	// 실행이 아직 없으면 pushRunWait 동안 pushRunPoll 간격으로 다시 본다.
	pushRunGrace = 2 * time.Minute
	pushRunWait  = 20 * time.Second
	pushRunPoll  = 2 * time.Second
)

// inFlightRunStatuses 는 아직 끝나지 않은 실행 상태다(GitHub 상태값).
var inFlightRunStatuses = map[string]bool{
	"queued": true, "in_progress": true, "waiting": true, "pending": true, "requested": true,
}

type workflowRun struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Event   string `json:"event"`
	HTMLURL string `json:"html_url"`
}

// TriggerBuild 는 jobName(리포)의 branch 에서 워크플로를 시작시키고 실행 주소를 돌려준다.
//
// 그 브랜치의 최신 실행이 아직 돌고 있으면 새로 시작하지 않고 그 실행에 붙는다. 화면은
// 파이프라인을 만든 직후 "실행" 을 한 번 더 부르는데, 스캐폴딩 커밋의 실행과 겹치면 같은
// 커밋의 배포 잡 둘이 되커밋을 밀어 둘째가 실패한다(워크플로의 concurrency 는 취소하지
// 않고 직렬화하므로 둘 다 끝까지 돈다).
func (t *WorkflowTrigger) TriggerBuild(ctx context.Context, jobName, branch string) (string, error) {
	repo := strings.TrimSpace(jobName)
	if repo == "" || t.owner == "" {
		return "", fmt.Errorf("github: 리포 소유자와 이름이 필요합니다 (owner=%q, repo=%q)", t.owner, repo)
	}
	ref := strings.TrimSpace(branch)
	if ref == "" {
		return "", fmt.Errorf("github: 리포 %s/%s 를 실행할 브랜치가 필요합니다", t.owner, repo)
	}
	if t.workflowFile == "" {
		return "", fmt.Errorf("github: 시작시킬 워크플로 파일 이름이 필요합니다")
	}

	full := t.owner + "/" + repo
	base := "/repos/" + url.PathEscape(t.owner) + "/" + url.PathEscape(repo)

	latest := t.latestRun(ctx, base, full, ref, "")
	if latest != nil && inFlightRunStatuses[strings.ToLower(latest.Status)] {
		slog.Info("github actions 실행이 이미 돌고 있어 새로 시작하지 않습니다",
			"repo", full, "ref", ref, "run_id", latest.ID, "status", latest.Status)
		return latest.HTMLURL, nil
	}
	// 방금 push 된 커밋이면 그 push 실행이 곧 생긴다. 그보다 먼저 dispatch 하면 같은 커밋의 실행이 둘 생기고
	// 두 deploy 가 같은 브랜치에 되커밋한다.
	if pushed := t.awaitPushRun(ctx, base, full, ref); pushed != nil {
		slog.Info("github actions 머리 커밋의 push 실행에 붙습니다",
			"repo", full, "ref", ref, "run_id", pushed.ID, "status", pushed.Status)
		return pushed.HTMLURL, nil
	}
	var lastKnownID int64
	if latest != nil {
		lastKnownID = latest.ID
	}

	dispatchPath := base + "/actions/workflows/" + url.PathEscape(t.workflowFile) + "/dispatches"
	if err := t.client.send(ctx, "POST", dispatchPath, map[string]any{"ref": ref}, nil); err != nil {
		return "", t.explainDispatchError(err, full)
	}

	// 실행 id 는 dispatch 응답에 없다. 방금 생긴 실행을 찾아 그 주소를 준다.
	for attempt := 0; attempt < runLookupAttempts; attempt++ {
		if attempt > 0 && t.wait != nil {
			t.wait()
		}
		run := t.latestRun(ctx, base, full, ref, "workflow_dispatch")
		if run != nil && run.ID > lastKnownID && run.HTMLURL != "" {
			return run.HTMLURL, nil
		}
	}
	// 아직 목록에 없다. 지어낸 실행 주소 대신 워크플로 페이지를 준다.
	return t.workflowPageURL(repo), nil
}

// awaitPushRun 은 브랜치 머리 커밋이 방금 생겼을 때 그 커밋의 실행을 잠깐 기다린다.
//
// 돌고 있는 것이 나타나면 그것을 돌려준다. 이미 끝났으면 붙을 것이 없으므로 nil 이다. 커밋이 오래됐거나
// 조회가 실패하면 기다리지 않는다.
func (t *WorkflowTrigger) awaitPushRun(ctx context.Context, base, full, ref string) *workflowRun {
	var branch struct {
		Commit struct {
			SHA    string `json:"sha"`
			Commit struct {
				Committer struct {
					Date time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
		} `json:"commit"`
	}
	found, err := t.client.get(ctx, base+"/branches/"+url.PathEscape(ref), &branch)
	committed := branch.Commit.Commit.Committer.Date
	if err != nil || !found || branch.Commit.SHA == "" || committed.IsZero() {
		return nil
	}
	if t.now().Sub(committed) > pushRunGrace {
		return nil
	}

	q := url.Values{}
	q.Set("head_sha", branch.Commit.SHA)
	q.Set("per_page", "1")
	deadline := t.now().Add(pushRunWait)
	for {
		var payload struct {
			WorkflowRuns []workflowRun `json:"workflow_runs"`
		}
		found, err := t.client.get(ctx, base+"/actions/runs?"+q.Encode(), &payload)
		if err != nil {
			slog.Warn("github actions 머리 커밋의 실행을 읽지 못해 기다리지 않습니다", "repo", full, "ref", ref, "error", err)
			return nil
		}
		if found && len(payload.WorkflowRuns) > 0 {
			run := payload.WorkflowRuns[0]
			if inFlightRunStatuses[strings.ToLower(run.Status)] && run.HTMLURL != "" {
				return &run
			}
			return nil
		}
		if !t.now().Before(deadline) {
			slog.Info("github actions 머리 커밋의 push 실행이 나타나지 않아 새로 시작합니다",
				"repo", full, "ref", ref, "waited", pushRunWait)
			return nil
		}
		if err := t.sleep(ctx, pushRunPoll); err != nil {
			return nil
		}
	}
}

// latestRun 은 그 브랜치의 최신 실행이다. event 가 비어 있지 않으면 그 종류만 본다.
//
// 조회 실패는 실행을 막지 않는다 — 이 조회는 중복을 피하고 링크를 찾기 위한 것이다.
func (t *WorkflowTrigger) latestRun(ctx context.Context, base, full, ref, event string) *workflowRun {
	q := url.Values{}
	q.Set("branch", ref)
	q.Set("per_page", "1")
	if event != "" {
		q.Set("event", event)
	}
	var payload struct {
		WorkflowRuns []workflowRun `json:"workflow_runs"`
	}
	found, err := t.client.get(ctx, base+"/actions/runs?"+q.Encode(), &payload)
	if err != nil {
		slog.Warn("github actions 실행 이력을 읽지 못했습니다", "repo", full, "ref", ref, "error", err)
		return nil
	}
	if !found || len(payload.WorkflowRuns) == 0 {
		return nil
	}
	return &payload.WorkflowRuns[0]
}

// explainDispatchError 는 GitHub 의 거절을 무엇을 해야 하는지로 옮긴다.
func (t *WorkflowTrigger) explainDispatchError(err error, full string) error {
	var apiErr *apiError
	if asAPIError(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 404:
			return fmt.Errorf(
				"GitHub 에 리포 %s 또는 워크플로 %s 가 없습니다. 파이프라인 프로비저닝이 끝나지 않았거나 리포가 지워졌습니다 — 파이프라인을 다시 프로비저닝하세요",
				full, t.workflowPath())
		case apiErr.StatusCode == 422 && strings.Contains(strings.ToLower(apiErr.Body), "workflow_dispatch"):
			return fmt.Errorf(
				"리포 %s 의 워크플로 %s 에 workflow_dispatch 트리거가 없어 GitHub 이 실행 요청을 거절했습니다. 이 트리거를 두기 전에 만들어진 파이프라인입니다 — 워크플로의 on: 에 workflow_dispatch 를 더하거나 파이프라인을 다시 프로비저닝하세요",
				full, t.workflowPath())
		}
	}
	return fmt.Errorf("github actions 실행 요청 (%s, %s): %w", full, t.workflowPath(), err)
}

func (t *WorkflowTrigger) workflowPath() string {
	return ".github/workflows/" + t.workflowFile
}

// workflowPageURL 은 워크플로의 실행 목록 페이지다.
func (t *WorkflowTrigger) workflowPageURL(repo string) string {
	base := t.webBaseURL
	if base == "" {
		base = WebBaseURLFor(t.client.BaseURL())
	}
	return base + "/" + t.owner + "/" + repo + "/actions/workflows/" + t.workflowFile
}

func asAPIError(err error, target **apiError) bool {
	for err != nil {
		if e, ok := err.(*apiError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
