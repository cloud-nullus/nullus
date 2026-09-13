package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// BuildReader 는 GitLab CI 파이프라인 실행 이력을 읽는다.
//
// 스택의 파이프라인은 그룹 아래 앱 이름의 프로젝트에서 돈다(ProvisionAppProject).
// port.CIBuildReader 는 job 이름으로 묻기 때문에 그룹 경로를 쥐고 프로젝트를 찾는다.
type BuildReader struct {
	client    *Client
	groupPath string
	// webBaseURL 은 브라우저가 여는 GitLab 주소다(리포트 링크용). 비면 링크를 만들지 않는다.
	webBaseURL string
}

// WithWebBaseURL 은 리포트 링크에 쓸 외부 주소를 정한다.
func (r *BuildReader) WithWebBaseURL(base string) *BuildReader {
	r.webBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
	return r
}

// NewBuildReader 는 groupPath 아래 프로젝트의 실행 이력을 읽는 BuildReader 를 만든다.
func NewBuildReader(client *Client, groupPath string) *BuildReader {
	return &BuildReader{client: client, groupPath: groupPath}
}

// projectPath 는 앱 프로젝트의 전체 경로다(그룹/앱).
func (r *BuildReader) projectPath(app string) string {
	if group := strings.Trim(strings.TrimSpace(r.groupPath), "/"); group != "" {
		return group + "/" + app
	}
	return app
}

// projectBase 는 앱 프로젝트 API 경로의 접두사다. GitLab 은 경로를 한 조각으로
// 인코딩해 받는다(그룹/앱 → 그룹%2F앱).
func (r *BuildReader) projectBase(app string) string {
	return "/api/v4/projects/" + url.PathEscape(r.projectPath(app))
}

// maxPerPage 는 GitLab 목록 API 의 한 쪽 최대 크기다.
const maxPerPage = 100

// ListBuilds 는 최근 파이프라인과 그 잡을 읽어 정규화한다.
//
// 프로젝트가 없으면(프로비저닝 전) 빈 목록이다 — 실행 기록이 없는 것이지 오류가 아니다.
func (r *BuildReader) ListBuilds(ctx context.Context, jobName, branch string, limit int) ([]port.CIBuild, error) {
	app := strings.TrimSpace(jobName)
	if app == "" {
		return nil, fmt.Errorf("gitlab: 프로젝트 이름이 필요합니다")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > maxPerPage {
		limit = maxPerPage
	}

	project := r.projectPath(app)
	base := r.projectBase(app)

	q := url.Values{}
	if b := strings.TrimSpace(branch); b != "" {
		q.Set("ref", b)
	}
	q.Set("per_page", strconv.Itoa(limit))
	q.Set("order_by", "id")
	q.Set("sort", "desc")

	var pipelines []struct {
		ID        int64     `json:"id"`
		IID       int       `json:"iid"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	found, err := r.client.get(ctx, base+"/pipelines?"+q.Encode(), &pipelines)
	if err != nil {
		return nil, fmt.Errorf("gitlab 파이프라인 이력 조회 (%s): %w", project, err)
	}
	if !found {
		return nil, nil
	}

	out := make([]port.CIBuild, 0, len(pipelines))
	for _, p := range pipelines {
		result, building := pipelineResult(p.Status)
		build := port.CIBuild{
			Number:    p.IID,
			ID:        strconv.FormatInt(p.ID, 10),
			Result:    result,
			Building:  building,
			StartedAt: p.CreatedAt,
			Stages:    r.listJobs(ctx, base, p.ID),
		}
		// 목록 API 는 걸린 시간을 주지 않는다. 끝난 파이프라인은 마지막 갱신이
		// 끝난 시각이다. 실행 중이면 0 으로 둔다 — 화면이 완료 시각을 지어내지 않게.
		if !building && p.UpdatedAt.After(p.CreatedAt) {
			build.Duration = p.UpdatedAt.Sub(p.CreatedAt)
		}
		out = append(out, build)
	}
	return out, nil
}

// listJobs 는 파이프라인 하나의 잡을 단계로 읽는다.
//
// 실패해도 오류를 올리지 않는다 — 단계 정보는 부가 정보이고, 이것 때문에
// 실행 기록 전체를 잃는 편이 나쁘다(Jenkins 어댑터와 같은 규약).
func (r *BuildReader) listJobs(ctx context.Context, base string, pipelineID int64) []port.CIStage {
	var jobs []struct {
		ID        int64      `json:"id"`
		Name      string     `json:"name"`
		Status    string     `json:"status"`
		StartedAt *time.Time `json:"started_at"`
		Duration  *float64   `json:"duration"`
	}
	path := fmt.Sprintf("%s/pipelines/%d/jobs?per_page=%d", base, pipelineID, maxPerPage)
	found, err := r.client.get(ctx, path, &jobs)
	if err != nil || !found {
		return nil
	}

	// GitLab 은 최근 잡을 먼저 준다. 잡은 파이프라인 생성 때 단계 순서대로
	// 만들어지므로 id 오름차순이 실행 순서다.
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })

	out := make([]port.CIStage, 0, len(jobs))
	for _, j := range jobs {
		stage := port.CIStage{
			ID:     strconv.FormatInt(j.ID, 10),
			Name:   strings.TrimSpace(j.Name),
			Status: jobStatus(j.Status),
		}
		if j.StartedAt != nil {
			stage.StartedAt = *j.StartedAt
		}
		if j.Duration != nil {
			stage.Duration = time.Duration(*j.Duration * float64(time.Second))
		}
		out = append(out, stage)
	}
	return out
}

// pipelineResult 는 GitLab 파이프라인 상태를 CIBuild 의 결과 어휘로 옮긴다.
//
// 끝나지 않은 상태(created·pending·running·manual 등)는 실행 중으로 본다 —
// 모르는 값을 성공이나 실패로 넘겨짚지 않는다.
func pipelineResult(status string) (result string, building bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return "SUCCESS", false
	case "failed":
		return "FAILURE", false
	case "canceled", "cancelled", "skipped":
		return "ABORTED", false
	default:
		return "", true
	}
}

// jobStatus 는 GitLab 잡 상태를 정규화한다. 공통 어휘에 없는 대기 상태를 채운다.
func jobStatus(status string) port.CIStageStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "preparing", "waiting_for_resource", "scheduled":
		return port.CIStageQueued
	case "canceling":
		return port.CIStageRunning
	default:
		return port.NormalizeStageStatus(status)
	}
}
