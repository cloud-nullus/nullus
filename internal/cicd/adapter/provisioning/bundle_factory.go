// Package provisioning 은 스택 정보를 보고 프로비저닝 도구 묶음을 조립한다.
//
// GitLab 주소·토큰·레지스트리 종류가 모두 스택마다 다르므로 기동 시점에 하나로
// 만들 수 없다. 요청 시점에 스택을 읽어 조립한다.
package provisioning

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/gitea"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/github"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/gitlab"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/jenkins"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/registry"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// gitLabWebservicePort 는 GitLab 웹서비스의 클러스터 내부 포트다.
const gitLabWebservicePort = 8181

// giteaHTTPPort 는 Gitea HTTP Service 의 클러스터 내부 포트다.
const giteaHTTPPort = 3000

// jenkinsPort 는 Jenkins 컨트롤러의 클러스터 내부 포트다.
const jenkinsPort = 8080

// giteaCredentialID 는 Jenkins 에 등록된 Gitea 자격증명 식별자다.
// JCasC 가 ESO 로 동기화한 Secret 을 읽어 이 이름으로 만든다.
const giteaCredentialID = "nullus-gitea"

// Options 는 팩토리 기동 설정이다.
type Options struct {
	// Env 는 시크릿 경로에 쓰는 환경 이름이다 (dev/staging/prod).
	Env string
	// GroupPath 는 프로젝트가 만들어질 조직 그룹 경로다.
	GroupPath string
	// ExternalRegistryPrefix 는 알 수 없는 레지스트리 도구의 폴백이다.
	ExternalRegistryPrefix string
	// GitLabBaseURLOverride 는 클러스터 내부 서비스 DNS 대신 쓸 주소다.
	//
	// 기본 경로는 gitlab-webservice-default.{ns}.svc 인데, 이는 API 서버가
	// 클러스터 안에서 돌 때만 해석된다. 로컬 실행이나 외부 GitLab 을 붙일 때
	// 이 값으로 대체한다.
	GitLabBaseURLOverride string
	// GiteaBaseURLOverride 는 Gitea 의 클러스터 내부 서비스 DNS 대신 쓸 주소다.
	//
	// 기본 경로는 gitea-http.{ns}.svc 인데, 이는 API 서버가 클러스터 안에서 돌
	// 때만 해석된다. 로컬 실행에서는 이 값이 없으면 인증 확인(Ping)이 DNS 오류로
	// 실패하고, 그러면 토큰을 강제 재발급하는 경로로 잘못 흘러간다.
	GiteaBaseURLOverride string
	// JenkinsBaseURLOverride 는 Jenkins 의 클러스터 내부 서비스 DNS 대신 쓸
	// 주소다. Gitea 와 같은 이유로 필요하다 — 기본 경로는 API 서버가 클러스터
	// 안에서 돌 때만 해석된다.
	JenkinsBaseURLOverride string
}

// BundleFactory 는 port.SCMBundleFactory 구현체다.
type BundleFactory struct {
	stacks port.StackReader
	tokens port.SCMTokenIssuer
	opts   Options

	// GitHub 경로는 선택적으로 배선된다. 없으면 GitHub 스택 요청이 명확한
	// 안내와 함께 실패한다 — 조용히 GitLab 으로 흘리면 엉뚱한 곳에 리포가 생긴다.
	githubTokens      port.SCMTokenIssuer
	githubConnections port.SCMConnectionReader

	// Gitea 경로도 선택적으로 배선된다. 같은 이유로 없으면 조용히 흘리지 않는다.
	giteaTokens port.SCMTokenIssuer
	// giteaSecrets 는 파이프라인 자격증명을 OpenBao 에 기록한다.
	giteaSecrets gitea.SecretWriter
	// jenkinsCreds 는 Jenkins job 생성에 쓰는 관리자 자격증명을 스택별로 푼다.
	// 기동 시점에 고정할 수 없다 — CI 서버가 스택마다 따로 서고 비밀번호도
	// 스택마다 다르게 생성된다.
	jenkinsCreds port.CICredentialResolver
}

// NewBundleFactory 는 팩토리를 만든다.
func NewBundleFactory(stacks port.StackReader, tokens port.SCMTokenIssuer, opts Options) *BundleFactory {
	if strings.TrimSpace(opts.GroupPath) == "" {
		opts.GroupPath = "nullus"
	}
	return &BundleFactory{stacks: stacks, tokens: tokens, opts: opts}
}

// WithGitHub 는 GitHub 스택을 프로비저닝할 수 있게 한다.
func (f *BundleFactory) WithGitHub(
	tokens port.SCMTokenIssuer,
	connections port.SCMConnectionReader,
) *BundleFactory {
	f.githubTokens = tokens
	f.githubConnections = connections
	return f
}

// WithGitea 는 Gitea 스택을 프로비저닝할 수 있게 한다.
func (f *BundleFactory) WithGitea(tokens port.SCMTokenIssuer, secrets gitea.SecretWriter) *BundleFactory {
	f.giteaTokens = tokens
	f.giteaSecrets = secrets
	return f
}

// WithJenkins 는 Gitea 스택의 CI job 을 만들 수 있게 한다.
//
// 배선되지 않으면 CIJobs 가 nil 로 남고, 호출부는 job 생성을 건너뛴다 —
// 리포와 스캐폴딩은 만들어지되 빌드는 돌지 않는다.
func (f *BundleFactory) WithJenkins(creds port.CICredentialResolver) *BundleFactory {
	f.jenkinsCreds = creds
	return f
}

// For 는 스택에 맞는 도구 묶음을 조립한다.
func (f *BundleFactory) For(ctx context.Context, stackID string) (*port.SCMBundle, error) {
	summary, err := f.stacks.GetStackSummary(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("read stack %s: %w", stackID, err)
	}
	if summary == nil {
		return nil, fmt.Errorf("stack %s not found", stackID)
	}
	if !strings.EqualFold(strings.TrimSpace(summary.State), "completed") {
		// 설치 중인 스택은 SCM 이 아직 응답하지 않을 수 있다.
		return nil, fmt.Errorf("stack %s 가 아직 준비되지 않았습니다 (state=%q)", stackID, summary.State)
	}

	switch platformFor(summary.SourceRepository) {
	case port.SCMPlatformGitLab:
		return f.gitLabBundle(ctx, summary)
	case port.SCMPlatformGitHub:
		return f.gitHubBundle(ctx, summary)
	case port.SCMPlatformGitea:
		return f.giteaBundle(ctx, summary)
	default:
		return nil, fmt.Errorf(
			"소스 저장소 %q 는 아직 지원하지 않습니다 (GitLab·GitHub·Gitea 스택에서만 프로젝트를 만들 수 있습니다)",
			summary.SourceRepository)
	}
}

// gitLabBundle 은 스택 안에 설치된 GitLab 을 향하는 묶음을 만든다.
func (f *BundleFactory) gitLabBundle(
	ctx context.Context,
	summary *port.StackSummary,
) (*port.SCMBundle, error) {
	namespace := strings.TrimSpace(summary.Namespace)
	if namespace == "" {
		return nil, fmt.Errorf("stack %s 의 네임스페이스를 알 수 없습니다", summary.ID)
	}

	baseURL := strings.TrimSpace(f.opts.GitLabBaseURLOverride)
	if baseURL == "" {
		baseURL = gitLabBaseURL(namespace)
	}

	client, err := f.authenticatedClient(ctx, summary, namespace, baseURL)
	if err != nil {
		return nil, err
	}

	resolver, err := registry.ResolverFor(registry.Config{
		ToolName:   summary.ContainerRegistry,
		HarborHost: harborHostFor(summary.AccessDomain),
		// Nexus 의 Docker 커넥터는 스택 모듈이 registry.<domain> 으로 라우팅한다.
		// 웹 UI(nexus.<domain>)로 push 하면 이미지 대신 HTML 을 받는다.
		NexusDockerHost:          registryHostFor(summary.AccessDomain),
		ExternalRepositoryPrefix: f.opts.ExternalRegistryPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve image registry for stack %s: %w", summary.ID, err)
	}

	return &port.SCMBundle{
		Provisioner:   client,
		Pipeline:      client,
		Registry:      resolver,
		Platform:      port.SCMPlatformGitLab,
		GroupPath:     f.opts.GroupPath,
		ArgoNamespace: namespace,
		ClusterID:     summary.ClusterID,
		AccessDomain:  summary.AccessDomain,
		GatewayName:   gatewayNameFor(summary.AccessDomain),
	}, nil
}

// gitHubBundle 은 외부 GitHub 을 향하는 묶음을 만든다.
//
// GitLab 과 달리 주소도 소유자도 우리가 정할 수 없다. 조직에 등록된 연동
// 설정과 PAT 가 모두 있어야 하며, 하나라도 없으면 무엇을 등록해야 하는지
// 알려주고 멈춘다 — 여기서 흘리면 리포 생성이 401/404 로 죽는다.
func (f *BundleFactory) gitHubBundle(
	ctx context.Context,
	summary *port.StackSummary,
) (*port.SCMBundle, error) {
	if f.githubTokens == nil || f.githubConnections == nil {
		return nil, fmt.Errorf("GitHub 연동이 배선되지 않아 stack %s 를 프로비저닝할 수 없습니다", summary.ID)
	}

	conn, err := f.githubConnections.GetConnection(ctx, summary.OrgID, port.SCMPlatformGitHub)
	if err != nil {
		return nil, fmt.Errorf("read github connection for org %s: %w", summary.OrgID, err)
	}
	if conn == nil || strings.TrimSpace(conn.Owner) == "" {
		return nil, fmt.Errorf(
			"조직 %s 에 GitHub 연동 설정이 없습니다 — organization 이름과 PAT 를 먼저 등록하세요",
			summary.OrgID)
	}

	token, err := f.githubTokens.EnsureToken(ctx, port.SCMTokenSpec{
		StackID:   summary.ID,
		ClusterID: summary.ClusterID,
		Namespace: strings.TrimSpace(summary.Namespace),
		OrgID:     summary.OrgID,
		Env:       f.opts.Env,
	})
	if err != nil {
		return nil, err
	}

	client := github.NewClient(conn.APIBaseURL, token)
	// 저장된 PAT 는 사용자가 폐기하거나 만료될 수 있다. GitLab 과 달리 재발급
	// 경로가 없으므로, 여기서 끊고 다시 등록하라고 알리는 것이 최선이다.
	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf(
			"GitHub 인증에 실패했습니다 (stack %s) — 등록된 PAT 가 만료·폐기되었는지 확인하세요: %w",
			summary.ID, err)
	}

	resolver, err := registry.ResolverFor(registry.Config{
		ToolName:                 summary.ContainerRegistry,
		GitHubOwner:              conn.Owner,
		HarborHost:               harborHostFor(summary.AccessDomain),
		NexusDockerHost:          registryHostFor(summary.AccessDomain),
		ExternalRepositoryPrefix: f.opts.ExternalRegistryPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve image registry for stack %s: %w", summary.ID, err)
	}

	return &port.SCMBundle{
		Provisioner: client,
		Pipeline:    client,
		Registry:    resolver,
		// GHCR 패키지는 같은 PAT 로 지운다(delete:packages 스코프 필요).
		Images:   client,
		Platform: port.SCMPlatformGitHub,
		// 리포는 GitHub organization 아래에 만들어진다. 스택의 nullus 그룹
		// 경로를 쓰면 존재하지 않는 네임스페이스를 가리킨다.
		GroupPath:       conn.Owner,
		RepoAccessToken: token,
		// Argo CD 는 여전히 클러스터 안에 있다.
		ArgoNamespace: strings.TrimSpace(summary.Namespace),
		ClusterID:     summary.ClusterID,
		AccessDomain:  summary.AccessDomain,
		GatewayName:   gatewayNameFor(summary.AccessDomain),
	}, nil
}

// giteaBundle 은 스택 안에 설치된 Gitea 를 향하는 묶음을 만든다.
//
// GitLab 과 마찬가지로 주소도 소유자도 우리가 정한다 — 다른 점은 Gitea 가
// 소스 저장소만 담당한다는 것이다. CI 는 Jenkins, 레지스트리는 Harbor 가 맡으므로
// 이미지 대상은 레지스트리 해석기가 따로 정한다.
func (f *BundleFactory) giteaBundle(
	ctx context.Context,
	summary *port.StackSummary,
) (*port.SCMBundle, error) {
	if f.giteaTokens == nil {
		return nil, fmt.Errorf("Gitea 연동이 배선되지 않아 stack %s 를 프로비저닝할 수 없습니다", summary.ID)
	}
	namespace := strings.TrimSpace(summary.Namespace)
	if namespace == "" {
		return nil, fmt.Errorf("stack %s 의 네임스페이스를 알 수 없습니다", summary.ID)
	}

	spec := port.SCMTokenSpec{
		StackID:   summary.ID,
		ClusterID: summary.ClusterID,
		Namespace: namespace,
		OrgID:     summary.OrgID,
		Env:       f.opts.Env,
	}

	client, token, err := f.authenticatedGiteaClient(ctx, summary, spec, namespace)
	if err != nil {
		return nil, err
	}

	resolver, err := registry.ResolverFor(registry.Config{
		ToolName:                 summary.ContainerRegistry,
		HarborHost:               harborHostFor(summary.AccessDomain),
		NexusDockerHost:          registryHostFor(summary.AccessDomain),
		ExternalRepositoryPrefix: f.opts.ExternalRegistryPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve image registry for stack %s: %w", summary.ID, err)
	}

	bundle := &port.SCMBundle{
		Provisioner: client,
		// Gitea 에는 GitLab 같은 프로젝트 CI 변수 저장소가 없다. 파이프라인
		// 자격증명은 OpenBao → ESO → K8s Secret 평면이 나른다.
		Registry: resolver,
		Webhooks: client,
		Credentials: gitea.NewCredentialPlane(
			f.giteaSecrets, f.opts.Env, summary.OrgID, summary.ID, namespace),
		Platform:        port.SCMPlatformGitea,
		GroupPath:       f.opts.GroupPath,
		RepoAccessToken: token,
		// 우회 주소를 쓰지 않는다. Jenkins 가 이 주소로 Gitea 를 스캔하므로
		// 클러스터 안에서 해석되는 값이어야 하고, JCasC 가 등록한 서버 주소와
		// 정확히 같은 형식이어야 한다(스택 모듈의 jenkinsGiteaServerValues).
		SCMInClusterURL: giteaBaseURL(namespace),
		ArgoNamespace:   namespace,
		ClusterID:       summary.ClusterID,
		AccessDomain:    summary.AccessDomain,
		GatewayName:     gatewayNameFor(summary.AccessDomain),
	}

	// Jenkins 가 배선돼 있어야 job 을 만들 수 있다. 없으면 CIJobs 를 비워 두고
	// 리포·스캐폴딩까지만 진행한다 — 조용히 성공한 것처럼 보이지 않도록
	// 호출부가 job 생성 생략을 결과에 남긴다.
	//
	// 자격증명 조회 실패도 같게 다룬다. 여기서 끊으면 리포조차 만들어지지
	// 않는데, 그 실패는 되돌리기 어려운 job 생성보다 앞 단계의 문제다.
	if f.jenkinsCreds != nil {
		user, password, credErr := f.jenkinsCreds.ResolveCICredential(ctx, spec)
		if credErr != nil {
			slog.Warn("jenkins 자격증명을 읽지 못해 CI job 생성을 건너뜁니다",
				"stack_id", summary.ID, "error", credErr)
		} else {
			ciBaseURL := strings.TrimSpace(f.opts.JenkinsBaseURLOverride)
			if ciBaseURL == "" {
				ciBaseURL = jenkinsBaseURL(namespace)
			}
			bundle.CIJobs = jenkins.NewClient(ciBaseURL, user, password)
			bundle.CIBaseURL = ciBaseURL
		}
	}
	return bundle, nil
}

// jenkinsBaseURL 은 클러스터 내부 Jenkins 주소다.
func jenkinsBaseURL(namespace string) string {
	return fmt.Sprintf("http://jenkins.%s.svc:%d", namespace, jenkinsPort)
}

// authenticatedGiteaClient 는 실제로 인증되는 Gitea 클라이언트를 돌려준다.
//
// 보관된 토큰은 폐기·만료될 수 있다. 그대로 쓰면 이후 모든 호출이 401 로 죽고
// 복구 경로가 없으므로, 한 번 확인하고 안 되면 재발급한다(GitLab 과 같은 규약).
func (f *BundleFactory) authenticatedGiteaClient(
	ctx context.Context,
	summary *port.StackSummary,
	spec port.SCMTokenSpec,
	namespace string,
) (*gitea.Client, string, error) {
	baseURL := strings.TrimSpace(f.opts.GiteaBaseURLOverride)
	if baseURL == "" {
		baseURL = giteaBaseURL(namespace)
	}

	// 토큰을 함께 돌려준다. 여기서 재발급이 일어날 수 있으므로 호출부가 다시
	// EnsureToken 을 부르면 방금 발급한 것과 다른 값을 쥘 위험이 있다.
	token, err := f.giteaTokens.EnsureToken(ctx, spec)
	if err != nil {
		return nil, "", fmt.Errorf("ensure gitea token for stack %s: %w", summary.ID, err)
	}
	client := gitea.NewClient(baseURL, token)
	if err := client.Ping(ctx); err == nil {
		return client, token, nil
	}

	spec.Force = true
	token, err = f.giteaTokens.EnsureToken(ctx, spec)
	if err != nil {
		return nil, "", fmt.Errorf("reissue gitea token for stack %s: %w", summary.ID, err)
	}
	client = gitea.NewClient(baseURL, token)
	if err := client.Ping(ctx); err != nil {
		return nil, "", fmt.Errorf("gitea 인증 실패 (stack %s): %w", summary.ID, err)
	}
	return client, token, nil
}

// authenticatedClient 는 실제로 인증되는 클라이언트를 돌려준다.
//
// 보관된 토큰은 폐기·만료될 수 있다. 그대로 쓰면 이후 모든 호출이 401 로 죽고
// 복구 경로가 없으므로, 여기서 한 번 확인하고 안 되면 재발급한다.
func (f *BundleFactory) authenticatedClient(
	ctx context.Context,
	summary *port.StackSummary,
	namespace, baseURL string,
) (*gitlab.Client, error) {
	spec := port.SCMTokenSpec{
		StackID:   summary.ID,
		ClusterID: summary.ClusterID,
		Namespace: namespace,
		OrgID:     summary.OrgID,
		Env:       f.opts.Env,
	}

	token, err := f.tokens.EnsureToken(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("ensure gitlab token for stack %s: %w", summary.ID, err)
	}

	client := gitlab.NewClient(baseURL, token).
		WithRegistryHost(registryHostFor(summary.AccessDomain))
	if err := client.Ping(ctx); err == nil {
		return client, nil
	}

	// 보관된 토큰이 더 이상 유효하지 않다 — 강제로 재발급한다.
	spec.Force = true
	token, err = f.tokens.EnsureToken(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("reissue gitlab token for stack %s: %w", summary.ID, err)
	}

	client = gitlab.NewClient(baseURL, token).
		WithRegistryHost(registryHostFor(summary.AccessDomain))
	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("gitlab 인증 실패 (stack %s): %w", summary.ID, err)
	}
	return client, nil
}

// platformFor 는 스택이 고른 소스 저장소 도구를 플랫폼으로 매핑한다.
//
// 알 수 없는 값은 빈 문자열이다 — 기본값으로 흘리지 않는다. 어느 쪽으로든
// 잘못 추측하면 엉뚱한 곳에 리포를 만들거나 닿지 않는 주소로 호출한다.
func platformFor(name string) port.SCMPlatform {
	switch normalize(name) {
	case "gitlab", "gitlab-ce", "gitlab-ee":
		return port.SCMPlatformGitLab
	case "github", "github-enterprise", "github-enterprise-server", "github-com":
		return port.SCMPlatformGitHub
	case "gitea":
		return port.SCMPlatformGitea
	}
	return ""
}

// giteaBaseURL 은 클러스터 내부 Gitea 주소다.
// 차트가 Service 이름을 {release}-http 로 유도한다.
func giteaBaseURL(namespace string) string {
	return fmt.Sprintf("http://gitea-http.%s.svc:%d", namespace, giteaHTTPPort)
}

// gitLabBaseURL 은 클러스터 내부 GitLab 주소다.
// 외부 노출 없이 동작해야 하므로 서비스 DNS 를 쓴다.
func gitLabBaseURL(namespace string) string {
	return fmt.Sprintf("http://gitlab-webservice-default.%s.svc:%d", namespace, gitLabWebservicePort)
}

// registryHostFor 는 접근 도메인에서 레지스트리 호스트를 유도한다.
//
// 비어 있으면 빈 값을 돌려준다 — 그 경우 클라이언트는 GitLab API 응답의
// container_registry_url 을 그대로 쓴다.
func registryHostFor(accessDomain string) string {
	domain := strings.Trim(strings.TrimSpace(accessDomain), "/")
	if domain == "" {
		return ""
	}
	return "registry." + domain
}

// harborHostFor 는 접근 도메인에서 Harbor 호스트를 유도한다.
func harborHostFor(accessDomain string) string {
	domain := strings.Trim(strings.TrimSpace(accessDomain), "/")
	if domain == "" {
		return ""
	}
	return "harbor." + domain
}

func normalize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.NewReplacer(" ", "-", "_", "-").Replace(n)
}

// gatewayNameFor 는 접근 도메인에서 게이트웨이 이름을 만든다.
//
// 스택 모듈이 게이트웨이를 만들 때 쓰는 규약과 같아야 한다
// (accessDomain 에서 .internal 을 떼고 - 로 정규화한 뒤 "-gateway").
// 규약이 갈리면 앱 라우트가 존재하지 않는 게이트웨이를 가리키게 된다.
func gatewayNameFor(accessDomain string) string {
	domain := strings.TrimSpace(accessDomain)
	if domain == "" {
		return ""
	}
	label := strings.TrimSuffix(domain, ".internal")
	label = strings.NewReplacer(".", "-", "_", "-", " ", "-").Replace(strings.ToLower(label))
	label = strings.Trim(label, "-")
	if label == "" {
		return ""
	}
	return label + "-gateway"
}
