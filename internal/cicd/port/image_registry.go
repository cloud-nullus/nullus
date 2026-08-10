package port

import (
	"context"
	"errors"
)

// ErrImageDeletionUnsupported 는 이 레지스트리에서 이미지 저장소를 지울 수단이
// 없음을 알린다.
//
// 조용히 건너뛰지 않는 이유는, 사용자가 "이미지도 삭제" 를 골랐는데 아무 일도
// 일어나지 않으면 지워진 줄 알고 넘어가기 때문이다. Harbor·Nexus 는 삭제에
// 관리자 자격증명이 따로 필요해 파이프라인 삭제 경로에서는 아직 다루지 않는다.
var ErrImageDeletionUnsupported = errors.New("이 레지스트리는 이미지 저장소 삭제를 지원하지 않습니다")

// ImageRepositoryDeleter 는 레지스트리에서 이미지 저장소를 통째로 지운다.
//
// 태그 하나가 아니라 저장소 전체가 대상이다. 파이프라인을 지우는 맥락에서는
// 그 앱의 이미지가 전부 필요 없어지기 때문이다.
type ImageRepositoryDeleter interface {
	DeleteImageRepository(ctx context.Context, target *ImageTarget) error
}

// RegistryKind 는 이미지 레지스트리 백엔드의 종류다.
//
// 스택 구성에 따라 이미지가 저장될 곳이 달라진다. GitLab 스택이면 SCM 프로젝트에
// 딸린 레지스트리를, Harbor 를 고른 스택이면 Harbor 프로젝트를 쓴다.
// CI 스크립트를 특정 레지스트리에 맞춰 쓰지 않기 위해 이 구분을 포트로 올린다.
type RegistryKind string

const (
	// RegistryKindSCMProject 는 소스 저장소 플랫폼이 프로젝트마다 제공하는
	// 레지스트리다 (GitLab Container Registry). 저장소와 수명이 같다.
	RegistryKindSCMProject RegistryKind = "scm_project"
	// RegistryKindHarbor 는 독립 설치된 Harbor 다.
	RegistryKindHarbor RegistryKind = "harbor"
	// RegistryKindNexus 는 독립 설치된 Nexus 의 Docker 커넥터다.
	RegistryKindNexus RegistryKind = "nexus"
	// RegistryKindGHCR 은 GitHub Container Registry 다.
	//
	// 다른 종류와 달리 push 자격증명을 등록할 필요가 없다 — GitHub Actions 의
	// 내장 GITHUB_TOKEN 이 packages:write 권한을 가질 수 있다.
	RegistryKindGHCR RegistryKind = "ghcr"
	// RegistryKindExternal 은 그 밖의 클러스터 외부 레지스트리다 (ECR 등).
	RegistryKindExternal RegistryKind = "external"
)

// ImageTargetSpec 은 이미지 저장 위치를 묻는 요청이다.
type ImageTargetSpec struct {
	// AppName 은 애플리케이션 이름이다.
	AppName string
	// SCMProjectPath 는 앱의 SCM 프로젝트 전체 경로다 (예: "acme/myapp").
	// SCM 프로젝트 레지스트리를 쓰는 구성에서만 의미가 있다.
	SCMProjectPath string
	// SCMRegistryURL 은 SCM 이 알려준 프로젝트 레지스트리 경로다.
	SCMRegistryURL string
	// OrgPath 는 조직 그룹 경로다. Harbor 프로젝트 이름 등에 쓴다.
	OrgPath string
}

// ImageTarget 은 CI 가 이미지를 올릴 위치와 인증 방법이다.
//
// 로그인 명령을 문자열로 담지 않는다 — 셸 조각이 포트에 새면 CI 플랫폼을
// 바꿀 때 포트까지 흔들린다. 렌더러가 이 구조를 보고 스크립트를 만든다.
type ImageTarget struct {
	Kind RegistryKind
	// Host 는 레지스트리 호스트다 (예: "registry.nullus.local").
	Host string
	// Repository 는 태그를 뺀 완전한 이미지 경로다
	// (예: "registry.nullus.local/acme/myapp").
	Repository string
	// UsernameVar / PasswordVar 는 CI 가 로그인에 쓸 변수 이름이다.
	// GitLab 프로젝트 레지스트리는 내장 변수를, 외부 레지스트리는
	// 파이프라인에 등록된 변수를 가리킨다.
	UsernameVar string
	PasswordVar string
	// RequiredVariables 는 파이프라인에 미리 등록돼야 하는 변수들이다.
	// 내장 변수만 쓰는 구성에서는 비어 있다.
	RequiredVariables []string
}

// ImageRegistryResolver 는 스택 구성에 맞는 이미지 저장 위치를 결정한다.
type ImageRegistryResolver interface {
	Resolve(ctx context.Context, spec ImageTargetSpec) (*ImageTarget, error)
}
