package keycloak

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const defaultAccessDomain = "nullus.local"

// ToolSSOSpec defines SSO client parameters for an OSS tool.
type ToolSSOSpec struct {
	ClientID     string
	DisplayName  string
	Subdomain    string
	CallbackPath string
	// PKCEMethod 는 도구가 PKCE 를 요구할 때만 채운다.
	// MinIO/ArgoCD 는 PKCE 를 쓰지 않아 비워 둔다.
	PKCEMethod string
}

// buildRedirectURI constructs the OIDC redirect URI for a tool.
func buildRedirectURI(subdomain, accessDomain, callbackPath string) string {
	return fmt.Sprintf("https://%s.%s%s", subdomain, accessDomain, callbackPath)
}

type SSOProvisioner struct {
	kc           *KeycloakClient
	accessDomain string
	// stackSlug 는 client ID 네임스페이싱에 쓰인다.
	stackSlug string
	toolSpecs map[string]ToolSSOSpec
}

func newToolSpecs() map[string]ToolSSOSpec {
	return map[string]ToolSSOSpec{
		"installing_gitlab": {
			ClientID:     "gitlab",
			DisplayName:  "GitLab CE",
			Subdomain:    "gitlab",
			CallbackPath: "/users/auth/openid_connect/callback",
			PKCEMethod:   "S256",
		},
		"installing_grafana": {
			ClientID:     "grafana",
			DisplayName:  "Grafana",
			Subdomain:    "grafana",
			CallbackPath: "/login/generic_oauth",
			PKCEMethod:   "S256",
		},
		"installing_argocd": {
			ClientID:     "argocd",
			DisplayName:  "Argo CD",
			Subdomain:    "argocd",
			CallbackPath: "/auth/callback",
			// ArgoCD 는 PKCE 를 사용하지 않는다.
		},
		"installing_harbor": {
			ClientID:     "harbor",
			DisplayName:  "Harbor",
			Subdomain:    "harbor",
			CallbackPath: "/c/oidc/callback",
			// Harbor 는 인가 요청에 PKCE 파라미터를 보내지 않는다. 요구하도록
			// 등록하면 콜백이 "Missing parameter: code_challenge_method" 로 깨진다.
			// client secret 을 갖는 confidential client 라 PKCE 없이도 등급이
			// 내려가지 않는다(ArgoCD 도 같은 이유로 쓰지 않는다).
		},
		"installing_minio": {
			ClientID:     "minio",
			DisplayName:  "MinIO",
			Subdomain:    "minio",
			CallbackPath: "/oauth_callback",
			// MinIO 는 PKCE 를 사용하지 않는다.
		},
	}
}

// NewSSOProvisioner creates a provisioner using the default access domain ("nullus.local").
// Preserves backward-compatible signature.
func NewSSOProvisioner(kc *KeycloakClient) *SSOProvisioner {
	return NewSSOProvisionerWithDomain(kc, defaultAccessDomain)
}

// NewSSOProvisionerWithDomain creates a provisioner with a custom access domain.
// redirect URIs are built as https://{subdomain}.{accessDomain}{callbackPath}.
func NewSSOProvisionerWithDomain(kc *KeycloakClient, accessDomain string) *SSOProvisioner {
	if accessDomain == "" {
		accessDomain = defaultAccessDomain
	}
	return &SSOProvisioner{
		kc:           kc,
		accessDomain: accessDomain,
		toolSpecs:    newToolSpecs(),
	}
}

// WithStackSlug 는 client ID 네임스페이싱에 쓸 스택 식별자를 설정한다.
//
// 공용 realm 에 여러 스택이 클라이언트를 등록하면 ID 가 충돌한다.
// 두 스택의 Grafana 가 같은 clientId 를 두고 redirect URI 를 서로 덮어쓰기
// 때문에, 스택 단위로 네임스페이스를 나눈다.
func (p *SSOProvisioner) WithStackSlug(slug string) *SSOProvisioner {
	if p != nil {
		p.stackSlug = normalizeSlug(slug)
	}
	return p
}

var slugInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)

func normalizeSlug(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = slugInvalidChars.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ClientIDFor 는 스택 네임스페이스가 적용된 client ID 를 돌려준다.
func (p *SSOProvisioner) ClientIDFor(stepName string) (string, bool) {
	spec, ok := p.toolSpecs[stepName]
	if !ok {
		return "", false
	}
	if p.stackSlug == "" {
		return spec.ClientID, true
	}
	return p.stackSlug + "-" + spec.ClientID, true
}

// SpecFor 는 도구 스펙을 돌려준다.
func (p *SSOProvisioner) SpecFor(stepName string) (ToolSSOSpec, bool) {
	spec, ok := p.toolSpecs[stepName]
	return spec, ok
}

// ToolSteps 는 SSO 대상 스텝 이름 목록이다.
func (p *SSOProvisioner) ToolSteps() []string {
	steps := make([]string, 0, len(p.toolSpecs))
	for step := range p.toolSpecs {
		steps = append(steps, step)
	}
	return steps
}

// ProvisionSSO 는 도구의 OIDC 클라이언트를 등록/갱신한다.
//
// secret 은 호출자가 넘긴다. Nullus 가 생성해 OpenBao 에 기록한 값을 그대로
// Keycloak 에 push 하므로, Keycloak 이 유실돼도 OpenBao 에서 복원할 수 있다.
func (p *SSOProvisioner) ProvisionSSO(ctx context.Context, stepName, clientSecret string) error {
	spec, ok := p.toolSpecs[stepName]
	if !ok {
		return fmt.Errorf("unknown SSO tool: %s", stepName)
	}
	clientID, _ := p.ClientIDFor(stepName)

	return p.kc.UpsertOIDCClient(ctx, OIDCClientSpec{
		ClientID:     clientID,
		Name:         spec.DisplayName,
		Secret:       clientSecret,
		RedirectURIs: []string{buildRedirectURI(spec.Subdomain, p.accessDomain, spec.CallbackPath)},
		PKCEMethod:   spec.PKCEMethod,
	})
}

func (p *SSOProvisioner) DeprovisionSSO(ctx context.Context, stepName string) error {
	clientID, ok := p.ClientIDFor(stepName)
	if !ok {
		return nil
	}
	return p.kc.DeleteOIDCClient(ctx, clientID)
}
