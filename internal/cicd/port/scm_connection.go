package port

import "context"

// SCMPlatform 은 스택이 쓰는 소스 저장소 플랫폼이다.
//
// 저장소를 만드는 방법도, 파이프라인 파일 형식도, 토큰을 얻는 경로도 플랫폼마다
// 다르다. 어댑터를 고른 뒤에도 렌더러까지 이 값이 따라가야 해서 포트로 올린다.
type SCMPlatform string

const (
	// SCMPlatformGitLab 은 Nullus 가 스택에 직접 설치한 GitLab 이다.
	SCMPlatformGitLab SCMPlatform = "gitlab"
	// SCMPlatformGitHub 은 외부 SaaS 인 GitHub(또는 GitHub Enterprise Server)다.
	SCMPlatformGitHub SCMPlatform = "github"
	// SCMPlatformGitea 는 스택 안에 설치되는 Gitea 다. GitHub 과 달리 주소가
	// 클러스터 내부 서비스 DNS 이고 조직도 우리가 만든다.
	SCMPlatformGitea SCMPlatform = "gitea"
)

// CIPlatform 은 파이프라인을 실행하는 CI 플랫폼이다.
//
// SCM 과 별개의 축이다. GitLab·GitHub 은 SCM 이 CI 를 겸하지만 Gitea 는 그렇지
// 않다 — 소스는 Gitea, 빌드는 Jenkins 처럼 갈린다. 하나로 묶어 두면 Gitea
// 스택에 .gitlab-ci.yml 이 깔리고 Jenkins 는 읽을 파일이 없어 영영 돌지 않는다.
type CIPlatform string

const (
	// CIPlatformGitLabCI / CIPlatformGitHubActions 는 SCM 이 겸하는 CI 다.
	CIPlatformGitLabCI      CIPlatform = "gitlab-ci"
	CIPlatformGitHubActions CIPlatform = "github-actions"
	// CIPlatformJenkins 는 SCM 과 독립적으로 도는 CI 다.
	CIPlatformJenkins CIPlatform = "jenkins"
)

// DefaultCIPlatformFor 는 CI 를 명시하지 않았을 때 SCM 이 함의하는 CI 다.
//
// Gitea 는 자체 CI 를 쓰지 않으므로 Jenkins 로 본다 — Gitea 스택에
// .gitlab-ci.yml 을 깔면 아무것도 그것을 읽지 않는다.
func DefaultCIPlatformFor(platform SCMPlatform) CIPlatform {
	switch platform {
	case SCMPlatformGitHub:
		return CIPlatformGitHubActions
	case SCMPlatformGitea:
		return CIPlatformJenkins
	default:
		return CIPlatformGitLabCI
	}
}

// SCMConnection 은 외부 SCM 에 붙기 위한 조직 단위 설정이다.
//
// 스택 안에 설치되는 GitLab 과 달리 GitHub 은 주소도 소유자도 우리가 정할 수
// 없다. 사용자가 등록한 값을 읽어야만 어느 org 에 리포를 만들지 알 수 있다.
type SCMConnection struct {
	Platform SCMPlatform
	// Owner 는 리포지토리가 만들어질 GitHub Organization 또는 사용자 계정이다.
	Owner string
	// APIBaseURL 은 GitHub Enterprise Server 의 API 주소다.
	// 비면 어댑터가 github.com 을 쓴다.
	APIBaseURL string
}

// SCMConnectionReader 는 조직에 등록된 SCM 연동 설정을 읽는다.
type SCMConnectionReader interface {
	// GetConnection 은 설정을 읽는다. 등록된 것이 없으면 nil, nil 을 돌려준다.
	GetConnection(ctx context.Context, orgID string, platform SCMPlatform) (*SCMConnection, error)
}
