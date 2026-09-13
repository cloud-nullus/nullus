package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// BuildReader 는 GitHub Actions 워크플로 실행 이력을 읽는다.
//
// 스택의 파이프라인은 organization 아래 앱 이름의 리포에서 돈다.
// port.CIBuildReader 는 job 이름으로 묻기 때문에 소유자를 쥐고 리포를 찾는다.
type BuildReader struct {
	client *Client
	owner  string
}

// NewBuildReader 는 owner 아래 리포의 실행 이력을 읽는 BuildReader 를 만든다.
func NewBuildReader(client *Client, owner string) *BuildReader {
	return &BuildReader{client: client, owner: owner}
}

// maxPerPage 는 GitHub 목록 API 의 한 쪽 최대 크기다.
const maxPerPage = 100

// ListBuilds 는 최근 워크플로 실행과 그 잡을 읽어 정규화한다.
//
// 리포가 없으면 빈 목록이다 — 실행 기록이 없는 것이지 오류가 아니다.
func (r *BuildReader) ListBuilds(ctx context.Context, jobName, branch string, limit int) ([]port.CIBuild, error) {
	repo, owner := strings.TrimSpace(jobName), strings.TrimSpace(r.owner)
	if repo == "" || owner == "" {
		return nil, fmt.Errorf("github: 리포 소유자와 이름이 필요합니다 (owner=%q, repo=%q)", owner, repo)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > maxPerPage {
		limit = maxPerPage
	}
	base := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)

	q := url.Values{}
	if b := strings.TrimSpace(branch); b != "" {
		q.Set("branch", b)
	}
	q.Set("per_page", strconv.Itoa(limit))

	var payload struct {
		WorkflowRuns []struct {
			ID           int64     `json:"id"`
			RunNumber    int       `json:"run_number"`
			Status       string    `json:"status"`
			Conclusion   *string   `json:"conclusion"`
			RunStartedAt time.Time `json:"run_started_at"`
			CreatedAt    time.Time `json:"created_at"`
			UpdatedAt    time.Time `json:"updated_at"`
		} `json:"workflow_runs"`
	}
	found, err := r.client.get(ctx, base+"/actions/runs?"+q.Encode(), &payload)
	if err != nil {
		return nil, fmt.Errorf("github actions 실행 이력 조회 (%s/%s): %w", owner, repo, err)
	}
	if !found {
		return nil, nil
	}

	out := make([]port.CIBuild, 0, len(payload.WorkflowRuns))
	for _, run := range payload.WorkflowRuns {
		started := run.RunStartedAt
		if started.IsZero() {
			started = run.CreatedAt
		}
		result, building := runResult(run.Status, deref(run.Conclusion))
		build := port.CIBuild{
			Number:    run.RunNumber,
			ID:        strconv.FormatInt(run.ID, 10),
			Result:    result,
			Building:  building,
			StartedAt: started,
			Stages:    r.listJobs(ctx, base, run.ID),
		}
		if !building && run.UpdatedAt.After(started) {
			build.Duration = run.UpdatedAt.Sub(started)
		}
		out = append(out, build)
	}
	return out, nil
}

// listJobs 는 실행 하나의 잡을 단계로 읽는다.
//
// 실패해도 오류를 올리지 않는다 — 단계 정보 때문에 실행 기록 전체를 잃지 않는다.
func (r *BuildReader) listJobs(ctx context.Context, base string, runID int64) []port.CIStage {
	var payload struct {
		Jobs []struct {
			ID          int64      `json:"id"`
			Name        string     `json:"name"`
			Status      string     `json:"status"`
			Conclusion  *string    `json:"conclusion"`
			StartedAt   *time.Time `json:"started_at"`
			CompletedAt *time.Time `json:"completed_at"`
		} `json:"jobs"`
	}
	path := fmt.Sprintf("%s/actions/runs/%d/jobs?per_page=%d", base, runID, maxPerPage)
	found, err := r.client.get(ctx, path, &payload)
	if err != nil || !found {
		return nil
	}

	out := make([]port.CIStage, 0, len(payload.Jobs))
	for _, j := range payload.Jobs {
		stage := port.CIStage{
			ID:     strconv.FormatInt(j.ID, 10),
			Name:   strings.TrimSpace(j.Name),
			Status: jobStatus(j.Status, deref(j.Conclusion)),
		}
		if j.StartedAt != nil {
			stage.StartedAt = *j.StartedAt
			if j.CompletedAt != nil && j.CompletedAt.After(*j.StartedAt) {
				stage.Duration = j.CompletedAt.Sub(*j.StartedAt)
			}
		}
		out = append(out, stage)
	}
	return out
}

// runResult 는 GitHub 의 status·conclusion 두 필드를 CIBuild 결과 어휘로 옮긴다.
//
// 끝나기 전에는 conclusion 이 없다. 끝난 뒤의 모르는 결론은 실패로 본다 —
// 성공으로 넘겨짚으면 막힌 실행이 초록불로 보인다.
func runResult(status, conclusion string) (result string, building bool) {
	if strings.ToLower(strings.TrimSpace(status)) != "completed" {
		return "", true
	}
	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "success", "neutral":
		return "SUCCESS", false
	case "cancelled", "skipped":
		return "ABORTED", false
	default: // failure, timed_out, startup_failure, action_required, stale
		return "FAILURE", false
	}
}

// jobStatus 는 잡의 status·conclusion 을 정규화한다.
func jobStatus(status, conclusion string) port.CIStageStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
	case "in_progress":
		return port.CIStageRunning
	case "queued", "waiting", "pending", "requested":
		return port.CIStageQueued
	default:
		return port.CIStageUnknown
	}
	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "success", "neutral":
		return port.CIStageSuccess
	case "skipped":
		return port.CIStageSkipped
	case "cancelled":
		// 취소는 실패가 아니다 — 새 커밋이 앞선 실행을 밀어낸 것이다.
		return port.CIStageCanceled
	case "":
		return port.CIStageUnknown
	default: // failure, timed_out, startup_failure, action_required, stale
		return port.CIStageFailed
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
