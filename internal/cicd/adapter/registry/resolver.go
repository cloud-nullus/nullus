// Package registry 는 스택이 선택한 이미지 레지스트리에 맞춰 이미지 저장 위치를
// 결정한다.
//
// GitLab 스택에서는 앱 프로젝트 자신의 Container Registry 를 쓰지만, Harbor 를
// 고른 스택에서는 Harbor 프로젝트를 써야 한다. CI 스크립트가 특정 레지스트리를
// 전제하지 않도록 이 결정을 한곳에 모은다.
package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	// GitLab CI 가 프로젝트 레지스트리용으로 자동 주입하는 변수들.
	scmUsernameVar = "CI_REGISTRY_USER"
	scmPasswordVar = "CI_REGISTRY_PASSWORD" // #nosec G101 -- CI 변수 이름

	// 외부 레지스트리는 내장 변수가 없어 파이프라인에 등록해야 한다.
	harborUsernameVar = "HARBOR_USERNAME"
	harborPasswordVar = "HARBOR_PASSWORD" // #nosec G101 -- CI 변수 이름

	nexusUsernameVar = "NEXUS_USERNAME"
	nexusPasswordVar = "NEXUS_PASSWORD" // #nosec G101 -- CI 변수 이름

	externalUsernameVar = "REGISTRY_USERNAME"
	externalPasswordVar = "REGISTRY_PASSWORD" // #nosec G101 -- CI 변수 이름

	// harborDefaultProject 는 조직을 알 수 없을 때 쓰는 Harbor 기본 프로젝트다.
	harborDefaultProject = "library"
)

// Config 는 스택 구성에서 온 레지스트리 선택 정보다.
type Config struct {
	// ToolName 은 스택이 고른 도구 이름이다 (예: "GitLab Registry", "Harbor").
	ToolName string
	// HarborHost 는 Harbor 를 고른 경우의 호스트다.
	HarborHost string
	// NexusDockerHost 는 Nexus 를 고른 경우의 Docker 커넥터 호스트다.
	//
	// Nexus 는 웹 UI 와 Docker 레지스트리를 다른 포트로 연다. UI 주소로 push
	// 하면 이미지 대신 HTML 을 받아 원인을 알기 어려운 실패가 나므로 별도 값이다.
	NexusDockerHost string
	// ExternalRepositoryPrefix 는 외부 레지스트리의 저장소 접두사다
	// (예: "ghcr.io/acme"). 알 수 없는 도구의 폴백으로도 쓴다.
	ExternalRepositoryPrefix string
}

// ResolverFor 는 스택 구성에 맞는 resolver 를 고른다.
//
// 어디에 올릴지 정할 수 없으면 실패한다 — 기본값으로 조용히 흘리면 CI 가
// 엉뚱한 곳에 이미지를 올리고, 배포는 존재하지 않는 이미지를 집게 된다.
func ResolverFor(cfg Config) (port.ImageRegistryResolver, error) {
	switch normalizeToolName(cfg.ToolName) {
	case "gitlab-registry", "gitlab-container-registry", "gitlab":
		return NewSCMProjectResolver(), nil
	case "harbor":
		return NewHarborResolver(cfg.HarborHost), nil
	case "nexus", "nexus3", "sonatype-nexus", "nexus-repository", "nexus-repository-manager":
		// 커넥터 호스트를 모르면 여기서 결정하지 않고 아래 외부 접두사 경로로
		// 넘긴다. 클러스터 밖 Nexus 를 가리키는 구성이 그 경우다.
		if strings.TrimSpace(cfg.NexusDockerHost) != "" {
			return NewNexusResolver(cfg.NexusDockerHost), nil
		}
	}

	if strings.TrimSpace(cfg.ExternalRepositoryPrefix) != "" {
		return NewExternalResolver(cfg.ExternalRepositoryPrefix), nil
	}
	return nil, fmt.Errorf(
		"이미지 레지스트리를 결정할 수 없습니다 (tool=%q): 지원 도구가 아니라면 외부 저장소 접두사를 지정하세요",
		cfg.ToolName)
}

// SCMProjectResolver 는 SCM 프로젝트에 딸린 레지스트리를 쓴다.
type SCMProjectResolver struct{}

// NewSCMProjectResolver 는 SCMProjectResolver 를 만든다.
func NewSCMProjectResolver() *SCMProjectResolver { return &SCMProjectResolver{} }

// Resolve 는 SCM 이 알려준 프로젝트 레지스트리 경로를 그대로 쓴다.
func (r *SCMProjectResolver) Resolve(_ context.Context, spec port.ImageTargetSpec) (*port.ImageTarget, error) {
	repository := strings.Trim(strings.TrimSpace(spec.SCMRegistryURL), "/")
	if repository == "" {
		return nil, fmt.Errorf("SCM 프로젝트 레지스트리 경로가 비어 있습니다 (app=%q)", spec.AppName)
	}
	return &port.ImageTarget{
		Kind:       port.RegistryKindSCMProject,
		Host:       hostOf(repository),
		Repository: repository,
		// GitLab CI 가 자동 주입하므로 별도 등록이 필요 없다.
		UsernameVar: scmUsernameVar,
		PasswordVar: scmPasswordVar,
	}, nil
}

// HarborResolver 는 독립 설치된 Harbor 를 쓴다.
type HarborResolver struct {
	host string
}

// NewHarborResolver 는 HarborResolver 를 만든다.
func NewHarborResolver(host string) *HarborResolver {
	return &HarborResolver{host: strings.Trim(strings.TrimSpace(host), "/")}
}

// Resolve 는 {harbor}/{org}/{app} 경로를 만든다.
func (r *HarborResolver) Resolve(_ context.Context, spec port.ImageTargetSpec) (*port.ImageTarget, error) {
	if r.host == "" {
		return nil, fmt.Errorf("harbor 호스트가 설정되지 않았습니다")
	}
	app := strings.TrimSpace(spec.AppName)
	if app == "" {
		return nil, fmt.Errorf("app_name is required")
	}

	project := strings.Trim(strings.TrimSpace(spec.OrgPath), "/")
	if project == "" {
		project = harborDefaultProject
	}

	return &port.ImageTarget{
		Kind:              port.RegistryKindHarbor,
		Host:              r.host,
		Repository:        fmt.Sprintf("%s/%s/%s", r.host, project, app),
		UsernameVar:       harborUsernameVar,
		PasswordVar:       harborPasswordVar,
		RequiredVariables: []string{harborUsernameVar, harborPasswordVar},
	}, nil
}

// NexusResolver 는 Nexus 의 Docker 커넥터를 쓴다.
//
// Harbor 와 달리 프로젝트 개념이 없다. 이미지는 커넥터 호스트 바로 아래에
// 놓이므로 조직 경로를 끼워 넣지 않는다 — 넣으면 존재하지 않는 저장소를
// 가리키게 되고 push 가 거부된다.
type NexusResolver struct {
	host string
}

// NewNexusResolver 는 Docker 커넥터 호스트(host:port)로 resolver 를 만든다.
func NewNexusResolver(host string) *NexusResolver {
	return &NexusResolver{host: strings.Trim(strings.TrimSpace(host), "/")}
}

// Resolve 는 {nexus-docker-host}/{app} 경로를 만든다.
func (r *NexusResolver) Resolve(_ context.Context, spec port.ImageTargetSpec) (*port.ImageTarget, error) {
	if r.host == "" {
		return nil, fmt.Errorf("nexus docker 커넥터 호스트가 설정되지 않았습니다")
	}
	app := strings.TrimSpace(spec.AppName)
	if app == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	return &port.ImageTarget{
		Kind:              port.RegistryKindNexus,
		Host:              r.host,
		Repository:        fmt.Sprintf("%s/%s", r.host, app),
		UsernameVar:       nexusUsernameVar,
		PasswordVar:       nexusPasswordVar,
		RequiredVariables: []string{nexusUsernameVar, nexusPasswordVar},
	}, nil
}

// ExternalResolver 는 클러스터 밖 레지스트리를 쓴다.
type ExternalResolver struct {
	prefix string
}

// NewExternalResolver 는 저장소 접두사(예: "ghcr.io/acme")로 resolver 를 만든다.
func NewExternalResolver(prefix string) *ExternalResolver {
	return &ExternalResolver{prefix: strings.Trim(strings.TrimSpace(prefix), "/")}
}

// Resolve 는 {prefix}/{app} 경로를 만든다.
func (r *ExternalResolver) Resolve(_ context.Context, spec port.ImageTargetSpec) (*port.ImageTarget, error) {
	if r.prefix == "" {
		return nil, fmt.Errorf("외부 레지스트리 저장소 접두사가 설정되지 않았습니다")
	}
	app := strings.TrimSpace(spec.AppName)
	if app == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	return &port.ImageTarget{
		Kind:              port.RegistryKindExternal,
		Host:              hostOf(r.prefix),
		Repository:        r.prefix + "/" + app,
		UsernameVar:       externalUsernameVar,
		PasswordVar:       externalPasswordVar,
		RequiredVariables: []string{externalUsernameVar, externalPasswordVar},
	}, nil
}

// hostOf 는 저장소 경로의 첫 구간(호스트)을 뽑는다.
func hostOf(repository string) string {
	if idx := strings.Index(repository, "/"); idx > 0 {
		return repository[:idx]
	}
	return repository
}

func normalizeToolName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return strings.NewReplacer(" ", "-", "_", "-").Replace(normalized)
}
