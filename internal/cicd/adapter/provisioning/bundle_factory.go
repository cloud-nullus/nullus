// Package provisioning 은 스택 정보를 보고 프로비저닝 도구 묶음을 조립한다.
//
// GitLab 주소·토큰·레지스트리 종류가 모두 스택마다 다르므로 기동 시점에 하나로
// 만들 수 없다. 요청 시점에 스택을 읽어 조립한다.
package provisioning

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/gitlab"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/registry"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// gitLabWebservicePort 는 GitLab 웹서비스의 클러스터 내부 포트다.
const gitLabWebservicePort = 8181

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
}

// BundleFactory 는 port.SCMBundleFactory 구현체다.
type BundleFactory struct {
	stacks port.StackReader
	tokens port.SCMTokenIssuer
	opts   Options
}

// NewBundleFactory 는 팩토리를 만든다.
func NewBundleFactory(stacks port.StackReader, tokens port.SCMTokenIssuer, opts Options) *BundleFactory {
	if strings.TrimSpace(opts.GroupPath) == "" {
		opts.GroupPath = "nullus"
	}
	return &BundleFactory{stacks: stacks, tokens: tokens, opts: opts}
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
		// 설치 중인 스택은 GitLab 이 아직 응답하지 않을 수 있다.
		return nil, fmt.Errorf("stack %s 가 아직 준비되지 않았습니다 (state=%q)", stackID, summary.State)
	}

	// 저장소를 만들려면 우리가 관리하는 GitLab 이어야 한다.
	// GitHub 조합은 외부 SaaS 라 이 경로로 프로비저닝할 수 없다.
	if !isSelfHostedGitLab(summary.SourceRepository) {
		return nil, fmt.Errorf(
			"소스 저장소가 %q 라 Nullus 가 프로젝트를 만들 수 없습니다 (GitLab 스택에서만 지원)",
			summary.SourceRepository)
	}

	namespace := strings.TrimSpace(summary.Namespace)
	if namespace == "" {
		return nil, fmt.Errorf("stack %s 의 네임스페이스를 알 수 없습니다", stackID)
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
		return nil, fmt.Errorf("resolve image registry for stack %s: %w", stackID, err)
	}

	return &port.SCMBundle{
		Provisioner:   client,
		Pipeline:      client,
		Registry:      resolver,
		GroupPath:     f.opts.GroupPath,
		ArgoNamespace: namespace,
		ClusterID:     summary.ClusterID,
		AccessDomain:  summary.AccessDomain,
		GatewayName:   gatewayNameFor(summary.AccessDomain),
	}, nil
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

// isSelfHostedGitLab 은 우리가 관리하는 GitLab 인지 본다.
func isSelfHostedGitLab(name string) bool {
	switch normalize(name) {
	case "gitlab", "gitlab-ce", "gitlab-ee":
		return true
	}
	return false
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
