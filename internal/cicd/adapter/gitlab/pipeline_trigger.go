package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

// PipelineTrigger 는 port.CIBuildTrigger 의 GitLab CI 구현체다.
//
// 스택에 묶인 파이프라인의 "실행" 은 플랫폼이 빌드하지 않고 CI 에 넘긴다. GitLab 은
// 커밋이 있을 때만 파이프라인을 돌리므로, 커밋 없이 지금 실행하려면 API 로 만들어야 한다.
type PipelineTrigger struct {
	client    *Client
	groupPath string
	// webBaseURL 은 브라우저가 여는 GitLab 주소다. 비면 API 가 준 web_url 을 쓴다.
	webBaseURL string
}

// NewPipelineTrigger 는 groupPath 아래 앱 프로젝트의 파이프라인을 시작시키는 트리거를 만든다.
func NewPipelineTrigger(client *Client, groupPath string) *PipelineTrigger {
	return &PipelineTrigger{client: client, groupPath: groupPath}
}

// WithWebBaseURL 은 실행 링크에 쓸 외부 주소를 정한다.
func (t *PipelineTrigger) WithWebBaseURL(base string) *PipelineTrigger {
	t.webBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
	return t
}

// inFlightPipelineStatuses 는 아직 끝나지 않은 파이프라인 상태다(GitLab 상태값).
var inFlightPipelineStatuses = map[string]bool{
	"created": true, "waiting_for_resource": true, "preparing": true, "pending": true, "running": true,
}

type pipelineRef struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
}

// TriggerBuild 는 jobName(앱 프로젝트)의 branch 에 파이프라인을 만들고 그 주소를 돌려준다.
//
// 그 브랜치의 최신 파이프라인이 아직 돌고 있으면 새로 만들지 않고 그 실행에 붙는다.
// 화면은 파이프라인을 만든 직후 "실행" 을 한 번 더 부르는데, 스캐폴딩 커밋이 이미
// 파이프라인을 시작한 뒤라 또 만들면 같은 커밋의 배포 잡 둘이 같은 브랜치에 되커밋을
// 밀어 둘째가 non-fast-forward 로 실패한다. 최신 것이 돌고 있다는 것은 곧 최신 코드가
// 실행 중이라는 뜻이므로 거기에 붙는 것이 사용자가 바란 일과 같다.
func (t *PipelineTrigger) TriggerBuild(ctx context.Context, jobName, branch string) (string, error) {
	app := strings.TrimSpace(jobName)
	if app == "" {
		return "", fmt.Errorf("gitlab: 프로젝트 이름이 필요합니다")
	}
	ref := strings.TrimSpace(branch)
	if ref == "" {
		return "", fmt.Errorf("gitlab: 프로젝트 %q 를 실행할 브랜치가 필요합니다", app)
	}

	project := appProjectPath(t.groupPath, app)
	base := appProjectAPIBase(t.groupPath, app)

	if running := t.latestInFlight(ctx, base, project, ref); running != nil {
		slog.Info("gitlab 파이프라인이 이미 돌고 있어 새로 만들지 않습니다",
			"project", project, "ref", ref, "pipeline_id", running.ID, "status", running.Status)
		return t.runURL(project, running), nil
	}

	var created pipelineRef
	err := t.client.post(ctx, base+"/pipeline", map[string]any{"ref": ref}, &created)
	if err != nil {
		var apiErr *apiError
		if asAPIError(err, &apiErr) && apiErr.StatusCode == 404 {
			// 상태 코드만 옮기면 사용자는 무엇을 해야 하는지 알 수 없다.
			return "", fmt.Errorf(
				"GitLab 에 프로젝트 %s 가 없습니다. 파이프라인 프로비저닝이 끝나지 않았거나 프로젝트가 지워졌습니다 — 파이프라인을 다시 프로비저닝하세요",
				project)
		}
		return "", fmt.Errorf("gitlab 파이프라인 생성 (%s@%s): %w", project, ref, err)
	}
	return t.runURL(project, &created), nil
}

// latestInFlight 는 그 브랜치의 최신 파이프라인이 아직 돌고 있으면 그것을 돌려준다.
//
// 조회 실패는 실행을 막지 않는다 — 이 조회는 중복을 피하기 위한 것이지 실행의 전제가 아니다.
func (t *PipelineTrigger) latestInFlight(ctx context.Context, base, project, ref string) *pipelineRef {
	q := url.Values{}
	q.Set("ref", ref)
	q.Set("order_by", "id")
	q.Set("sort", "desc")
	q.Set("per_page", "1")

	var pipelines []pipelineRef
	found, err := t.client.get(ctx, base+"/pipelines?"+q.Encode(), &pipelines)
	if err != nil {
		slog.Warn("gitlab 파이프라인 이력을 읽지 못해 중복 여부를 보지 않고 실행합니다",
			"project", project, "ref", ref, "error", err)
		return nil
	}
	if !found || len(pipelines) == 0 || !inFlightPipelineStatuses[pipelines[0].Status] {
		return nil
	}
	return &pipelines[0]
}

// runURL 은 사람이 열어 볼 파이프라인 주소다. 외부 주소를 알면 그것으로 만들고,
// 모르면 API 가 준 web_url 을 쓴다 — 그 값은 클러스터 안 주소일 수 있지만 지어낸 것보다 낫다.
func (t *PipelineTrigger) runURL(project string, p *pipelineRef) string {
	if t.webBaseURL != "" && p.ID > 0 {
		return t.webBaseURL + "/" + project + "/-/pipelines/" + strconv.FormatInt(p.ID, 10)
	}
	return strings.TrimSpace(p.WebURL)
}
