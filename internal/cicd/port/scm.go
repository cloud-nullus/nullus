package port

import (
	"context"
	"errors"
)

// SCMGroup 은 소스 저장소 플랫폼의 그룹(네임스페이스)이다.
type SCMGroup struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
	WebURL   string `json:"web_url"`
}

// SCMProject 는 프로비저닝된 프로젝트다.
//
// RegistryURL 은 이 프로젝트에 종속된 컨테이너 레지스트리 경로다.
// GitLab 의 Container/Package Registry 는 프로젝트 단위로 존재하므로,
// 공용 베이스 이미지를 두려면 그것을 소유할 프로젝트가 반드시 필요하다.
type SCMProject struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	FullPath      string `json:"full_path"`
	WebURL        string `json:"web_url"`
	HTTPCloneURL  string `json:"http_clone_url"`
	RegistryURL   string `json:"registry_url"`
	DefaultBranch string `json:"default_branch"`
	// Created 는 이번 호출이 저장소를 새로 만들었는지다.
	//
	// 스캐폴딩을 덮어쓸지 판단하는 데 쓴다. CommitFiles 는 upsert 라 이미 쓰이던
	// 저장소에 다시 커밋하면 CI 가 갱신해 둔 이미지 태그가 초기값으로 되돌아가고,
	// 사용자가 고친 Dockerfile·워크플로·매니페스트도 함께 사라진다.
	Created bool `json:"created"`
}

// GroupSpec 은 그룹 생성 요청이다.
type GroupSpec struct {
	Name        string
	Path        string
	Description string
}

// ProjectSpec 은 프로젝트 생성 요청이다.
//
// GroupID 가 비면 인증 사용자의 개인 네임스페이스에 만들어진다.
type ProjectSpec struct {
	Name string
	Path string
	// GroupID 는 생성 시 네임스페이스 지정에 쓴다(숫자 ID).
	GroupID string
	// GroupPath 는 조회 시 경로 조합에 쓴다("{GroupPath}/{Path}").
	// 비면 개인 네임스페이스로 간주해 Path 만으로 조회한다.
	GroupPath   string
	Description string
	// Visibility 는 private | internal | public. 비면 private.
	Visibility string
	// InitReadme 가 true 면 기본 브랜치가 즉시 생성된다.
	// 브랜치가 없으면 파일 커밋이 실패하므로 첫 커밋 전에는 켜야 한다.
	InitReadme bool
}

// CommitFile 은 커밋에 포함할 파일 하나다.
type CommitFile struct {
	Path    string
	Content string
}

// CommitSpec 은 여러 파일을 한 커밋으로 올리는 요청이다.
type CommitSpec struct {
	Branch  string
	Message string
	Files   []CommitFile
}

// ErrSCMUserNotFound 는 그 이메일의 SCM 계정이 아직 없다는 뜻이다.
//
// 실패가 아니라 "아직" 이다 — OIDC 첫 로그인 전에는 계정이 존재하지 않는다.
// 호출부는 이것을 경고로 옮겨 담아야 한다.
var ErrSCMUserNotFound = errors.New("SCM 사용자 계정을 찾지 못했습니다")

// OrgMemberProvisioner 는 사람을 조직에 넣는다.
//
// 플랫폼이 만든 저장소는 자동화 계정 소유의 private 조직 안에 있어서, 그대로
// 두면 정작 그 저장소에 소스를 밀어야 할 사람이 보지도 못한다.
// 지원하지 않는 플랫폼에서는 nil 이며, 호출부는 nil 이면 건너뛴다.
type OrgMemberProvisioner interface {
	// EnsureOrgMember 는 이메일로 찾은 사용자를 조직에 넣는다(멱등).
	EnsureOrgMember(ctx context.Context, org, email string) error
}

// SCMProvisioner 는 소스 저장소 플랫폼에 그룹·프로젝트·파일을 만든다.
//
// Ensure* 는 멱등하다 — 이미 있으면 조회 결과를 그대로 돌려준다.
// 스택 설치는 여러 번 재시도될 수 있으므로 생성 API 를 그대로 노출하지 않는다.
type SCMProvisioner interface {
	EnsureGroup(ctx context.Context, spec GroupSpec) (*SCMGroup, error)
	EnsureProject(ctx context.Context, spec ProjectSpec) (*SCMProject, error)
	// CommitFiles 는 파일이 이미 있으면 갱신한다(upsert).
	CommitFiles(ctx context.Context, projectID string, spec CommitSpec) error
	// DeleteProject 는 저장소를 지운다. 이미 없으면 성공으로 본다.
	//
	// 되돌릴 수 없다. 사용자가 명시적으로 요청했을 때만 호출해야 하며,
	// 파이프라인 삭제의 기본 동작이어서는 안 된다 — 소스 코드는 사용자 자산이다.
	DeleteProject(ctx context.Context, projectID string) error
}
