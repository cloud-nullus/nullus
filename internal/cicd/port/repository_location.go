package port

import "context"

// RepositoryLocation 은 스택에 묶인 파이프라인의 앱 저장소가 만들어질 자리다.
//
// 저장소는 언제나 <GroupPath>/<앱 이름> 으로 만들어진다. 앱 배포 화면이 그 주소를 미리 보여 주는 데
// 쓴다 — 전에는 화면이 저장소 경로를 입력받았지만 서버가 그 값을 쓰지 않아, 입력한 주소와 실제
// 저장소가 달랐다.
type RepositoryLocation struct {
	Platform SCMPlatform `json:"platform"`
	// GroupPath 는 저장소가 들어갈 그룹(GitLab · Gitea) 또는 조직(GitHub)이다.
	GroupPath string `json:"group_path"`
	// WebBaseURL 은 브라우저가 여는 SCM 주소다. 접근 도메인이 없어 모르면 비어 있다.
	WebBaseURL string `json:"web_base_url"`
}

// RepositoryLocator 는 스택의 앱 저장소 자리를 알려 준다. 토큰을 발급하거나 SCM 에 붙지 않는다.
type RepositoryLocator interface {
	RepositoryLocation(ctx context.Context, stackID string) (*RepositoryLocation, error)
}
