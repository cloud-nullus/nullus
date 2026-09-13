package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SetPipelineVariable 은 리포 Actions 변수(vars)를 만들거나 갱신한다.
//
// 시크릿이 아니라 변수다. 워크플로가 ${{ vars.X }} 로 읽으므로 시크릿에 넣으면
// 빈 값이 되고, 정책 같은 비밀 아닌 값이 로그에서 *** 로 가려진다.
// 변수 API 에는 upsert 가 없다. POST 로 만들고 이미 있으면(409) PATCH 한다.
func (c *Client) SetPipelineVariable(ctx context.Context, projectID, key, value string) error {
	repo := strings.Trim(strings.TrimSpace(projectID), "/")
	if repo == "" {
		return fmt.Errorf("repository (owner/name) is required")
	}
	// 변수 이름 규칙은 시크릿과 같다(영숫자·밑줄, 숫자 시작 불가, GITHUB_ 접두 금지).
	name, err := normalizeSecretName(key)
	if err != nil {
		return err
	}
	body := map[string]any{"name": name, "value": value}

	err = c.send(ctx, http.MethodPost, repoPath(repo, "/actions/variables"), body, nil)
	if err == nil {
		return nil
	}
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict {
		return fmt.Errorf("create variable %q on %s: %w", name, repo, err)
	}
	if err := c.send(ctx, http.MethodPatch,
		repoPath(repo, "/actions/variables/"+url.PathEscape(name)), body, nil); err != nil {
		return fmt.Errorf("update variable %q on %s: %w", name, repo, err)
	}
	return nil
}
