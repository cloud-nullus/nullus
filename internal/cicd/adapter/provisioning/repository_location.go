package provisioning

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/github"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

var _ port.RepositoryLocator = (*BundleFactory)(nil)

// RepositoryLocation 은 스택의 앱 저장소가 만들어질 자리다. 번들과 같은 규칙으로 그룹을 정하되 토큰을
// 발급하거나 SCM 에 붙지 않는다 — 입력 화면을 여는 것만으로 OpenBao · GitLab 을 두드리지 않게.
func (f *BundleFactory) RepositoryLocation(ctx context.Context, stackID string) (*port.RepositoryLocation, error) {
	summary, err := f.stacks.GetStackSummary(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("read stack %s: %w", stackID, err)
	}
	if summary == nil {
		return nil, fmt.Errorf("%w: stack %s not found", port.ErrStackToolsUnavailable, stackID)
	}

	switch platformFor(summary.SourceRepository) {
	case port.SCMPlatformGitLab:
		return &port.RepositoryLocation{
			Platform:   port.SCMPlatformGitLab,
			GroupPath:  f.opts.GroupPath,
			WebBaseURL: toolWebBaseURL("gitlab", summary.AccessDomain),
		}, nil
	case port.SCMPlatformGitea:
		return &port.RepositoryLocation{
			Platform:   port.SCMPlatformGitea,
			GroupPath:  f.opts.GroupPath,
			WebBaseURL: toolWebBaseURL("gitea", summary.AccessDomain),
		}, nil
	case port.SCMPlatformGitHub:
		// 리포는 GitHub 조직 아래에 생긴다(gitHubBundle 과 같다).
		if f.githubConnections == nil {
			return nil, fmt.Errorf("GitHub 연동이 배선되지 않아 stack %s 의 저장소 자리를 알 수 없습니다", summary.ID)
		}
		conn, err := f.githubConnections.GetConnection(ctx, summary.OrgID, port.SCMPlatformGitHub)
		if err != nil {
			return nil, fmt.Errorf("read github connection for org %s: %w", summary.OrgID, err)
		}
		if conn == nil || strings.TrimSpace(conn.Owner) == "" {
			return nil, fmt.Errorf("조직 %s 에 GitHub 연동 설정이 없습니다 — organization 이름과 PAT 를 먼저 등록하세요", summary.OrgID)
		}
		return &port.RepositoryLocation{
			Platform:   port.SCMPlatformGitHub,
			GroupPath:  strings.TrimSpace(conn.Owner),
			WebBaseURL: github.WebBaseURLFor(conn.APIBaseURL),
		}, nil
	}
	return nil, fmt.Errorf("소스 저장소 %q 는 아직 지원하지 않습니다", summary.SourceRepository)
}
