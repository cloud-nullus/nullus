package gitea

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// teamName 은 파이프라인을 만든 사람을 넣는 팀이다.
//
// Owners 에 넣지 않는다. 저장소를 보고 밀 수 있으면 충분하고, 조직 소유권까지
// 주면 되돌리기 어려운 권한이 조용히 퍼진다.
const teamName = "developers"

// teamPermission 은 그 팀의 권한이다. 개발자는 앱 소스를 밀어야 한다.
const teamPermission = "write"

// EnsureOrgMember 는 이메일로 찾은 사용자를 조직의 write 팀에 넣는다.
//
// 플랫폼이 만든 저장소는 자동화 계정(gitea_admin) 소유의 private 조직 안에 있다.
// SSO 로 들어온 사람은 그 조직의 멤버가 아니라, 로그인은 되는데 화면이 텅 비어
// 보인다 — 그런데 개발자는 그 저장소에 앱 소스를 밀어야 한다.
//
// 멱등하다. 이미 멤버면 Gitea 가 성공으로 답한다.
func (c *Client) EnsureOrgMember(ctx context.Context, org, email string) error {
	orgName := strings.TrimSpace(org)
	target := strings.TrimSpace(email)
	if orgName == "" || target == "" {
		// 사용자를 특정할 수 없는 경로(자동화 호출 등)까지 실패로 만들 이유가 없다.
		return nil
	}

	login, err := c.findUserByEmail(ctx, target)
	if err != nil {
		return err
	}

	teamID, err := c.ensureWriteTeam(ctx, orgName)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/teams/%d/members/%s", teamID, url.PathEscape(login))
	if err := c.send(ctx, http.MethodPut, path, nil, nil); err != nil {
		return fmt.Errorf("add %s to team %d: %w", login, teamID, err)
	}
	return nil
}

// findUserByEmail 은 이메일로 Gitea 계정을 찾는다.
//
// 못 찾는 것은 실패가 아니라 "아직" 이다 — OIDC 첫 로그인 전에는 계정이 존재하지
// 않는다. 호출부가 경고로 옮겨 담아 무엇을 하면 되는지 알린다.
func (c *Client) findUserByEmail(ctx context.Context, email string) (string, error) {
	var payload struct {
		Data []struct {
			Login string `json:"login"`
			Email string `json:"email"`
		} `json:"data"`
	}

	path := "/users/search?q=" + url.QueryEscape(email)
	found, err := c.get(ctx, path, &payload)
	if err != nil {
		return "", fmt.Errorf("search gitea user %s: %w", email, err)
	}
	if !found || len(payload.Data) == 0 {
		return "", fmt.Errorf("%w: %s", port.ErrSCMUserNotFound, email)
	}

	// 검색은 부분 일치도 돌려준다. 이메일이 정확히 같은 것만 쓴다 —
	// 엉뚱한 사람을 조직에 넣는 것이 못 넣는 것보다 나쁘다.
	for _, user := range payload.Data {
		if strings.EqualFold(strings.TrimSpace(user.Email), email) {
			return user.Login, nil
		}
	}
	return "", fmt.Errorf("%w: %s", port.ErrSCMUserNotFound, email)
}

// ensureWriteTeam 은 조직의 write 팀을 찾거나 만든다.
//
// 새 조직에는 Owners 밖에 없다.
func (c *Client) ensureWriteTeam(ctx context.Context, org string) (int64, error) {
	var teams []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	path := "/orgs/" + url.PathEscape(org) + "/teams"
	if _, err := c.get(ctx, path, &teams); err != nil {
		return 0, fmt.Errorf("list teams of %s: %w", org, err)
	}
	for _, team := range teams {
		if strings.EqualFold(strings.TrimSpace(team.Name), teamName) {
			return team.ID, nil
		}
	}

	var created struct {
		ID int64 `json:"id"`
	}
	body := map[string]any{
		"name":       teamName,
		"permission": teamPermission,
		// units 를 주지 않으면 Gitea 가 코드 접근이 없는 팀을 만든다 —
		// 멤버는 조직에 들어가지만 저장소는 여전히 보이지 않는다.
		"units": []string{
			"repo.code", "repo.issues", "repo.pulls", "repo.releases", "repo.wiki",
		},
		"includes_all_repositories": true,
	}
	if err := c.send(ctx, http.MethodPost, path, body, &created); err != nil {
		return 0, fmt.Errorf("create team %s in %s: %w", teamName, org, err)
	}
	return created.ID, nil
}
