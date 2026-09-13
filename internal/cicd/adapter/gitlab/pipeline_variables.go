package gitlab

import (
	"context"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// SetPipelineVariable 은 비밀이 아닌 프로젝트 CI/CD 변수를 등록하거나 갱신한다.
//
// 프로젝트 변수는 .gitlab-ci.yml 의 variables 보다 앞선다 — 이미 스캐폴딩한
// 파이프라인도 다시 커밋하지 않고 값이 바뀐다.
//
// 가리지도 보호하지도 않는다. GitLab 은 8자 미만 값(true·allow)의 마스킹 등록을
// 통째로 거부하고, 보호(protected)를 켜면 보호 브랜치 밖의 실행이 정책 없이 돈다.
func (c *Client) SetPipelineVariable(ctx context.Context, projectID, key, value string) error {
	return c.SetProjectVariable(ctx, projectID, port.ProjectVariable{Key: key, Value: value})
}
