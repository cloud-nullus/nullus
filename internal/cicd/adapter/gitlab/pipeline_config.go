package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// defaultAccessTokenDays 는 프로젝트 토큰의 기본 만료 기간이다.
//
// 만료 없는 토큰은 회수 경로가 없어 사고 시 대응이 어렵다.
// GitLab 은 만료일이 "1년 뒤보다 이전" 이어야 한다고 요구하므로(경계 배타적)
// 정확히 365일을 쓰면 거부된다. 시간대 차이까지 감안해 여유를 둔다.
const defaultAccessTokenDays = 360

// SetProjectVariable 은 CI/CD 변수를 등록하거나 이미 있으면 갱신한다.
//
// GitLab 변수 API 에는 upsert 가 없다. POST 로 시도하고 키 충돌이면 PUT 한다 —
// 재프로비저닝에서 자격증명이 바뀌었는데 예전 값이 남으면 빌드가 계속 실패한다.
func (c *Client) SetProjectVariable(ctx context.Context, projectID string, v port.ProjectVariable) error {
	if strings.TrimSpace(projectID) == "" {
		return fmt.Errorf("project id is required")
	}
	key := strings.TrimSpace(v.Key)
	if key == "" {
		return fmt.Errorf("variable key is required")
	}

	body := map[string]any{
		"key":       key,
		"value":     v.Value,
		"masked":    v.Masked,
		"protected": v.Protected,
	}

	base := fmt.Sprintf("/api/v4/projects/%s/variables", url.PathEscape(projectID))
	err := c.post(ctx, base, body, nil)
	if err == nil {
		return nil
	}
	if !isVariableExistsError(err) {
		return fmt.Errorf("set variable %q on project %s: %w", key, projectID, err)
	}

	updatePath := base + "/" + url.PathEscape(key)
	if updateErr := c.put(ctx, updatePath, body, nil); updateErr != nil {
		return fmt.Errorf("update variable %q on project %s: %w", key, projectID, updateErr)
	}
	return nil
}

// CreateProjectAccessToken 은 프로젝트 범위 토큰을 발급한다.
//
// 값은 발급 응답에만 담기므로 호출자가 즉시 보관해야 한다.
func (c *Client) CreateProjectAccessToken(
	ctx context.Context,
	projectID string,
	spec port.AccessTokenSpec,
) (string, error) {
	if strings.TrimSpace(projectID) == "" {
		return "", fmt.Errorf("project id is required")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return "", fmt.Errorf("token name is required")
	}

	scopes := spec.Scopes
	if len(scopes) == 0 {
		scopes = []string{"write_repository"}
	}
	days := spec.ExpiresInDays
	if days <= 0 {
		days = defaultAccessTokenDays
	}

	body := map[string]any{
		"name":       name,
		"scopes":     scopes,
		"expires_at": time.Now().AddDate(0, 0, days).Format("2006-01-02"),
	}

	var out struct {
		Token string `json:"token"`
	}
	path := fmt.Sprintf("/api/v4/projects/%s/access_tokens", url.PathEscape(projectID))
	if err := c.post(ctx, path, body, &out); err != nil {
		return "", fmt.Errorf("create access token on project %s: %w", projectID, err)
	}
	if strings.TrimSpace(out.Token) == "" {
		return "", fmt.Errorf("gitlab 이 토큰 값을 돌려주지 않았습니다 (project %s)", projectID)
	}
	return out.Token, nil
}

// isVariableExistsError 는 변수 키 충돌인지 본다.
func isVariableExistsError(err error) bool {
	var apiErr *apiError
	if !asAPIError(err, &apiErr) {
		return false
	}
	msg := strings.ToLower(apiErr.Body)
	return strings.Contains(msg, "already been taken") || strings.Contains(msg, "already exists")
}
