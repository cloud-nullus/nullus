package keycloak

import "testing"

// 이 스택의 SCM 은 Gitea, CI 는 Jenkins 인데 둘 다 SSO 대상 목록에 없었다.
// 그래서 gitea-jenkins-argocd 조합에서는 SSO 가 ArgoCD·Harbor 에만 걸렸다.

func TestToolSpecs_CoversGitea(t *testing.T) {
	spec, ok := newToolSpecs()["installing_gitea"]
	if !ok {
		t.Fatal("gitea 스펙이 없다 — 이 스택의 SCM 이 SSO 밖에 남는다")
	}
	if spec.Subdomain != "gitea" {
		t.Fatalf("subdomain=%q", spec.Subdomain)
	}
	// Gitea 의 콜백은 /user/oauth2/<소스이름>/callback 이다. 소스 이름은
	// 프로비저닝이 등록할 이름과 같아야 한다.
	if spec.CallbackPath != "/user/oauth2/keycloak/callback" {
		t.Fatalf("callback=%q", spec.CallbackPath)
	}
	// Harbor 와 같은 실패를 피한다 — 보내지 않는 도구에 PKCE 를 요구하면
	// 콜백이 "Missing parameter: code_challenge_method" 로 깨진다.
	if spec.PKCEMethod != "" {
		t.Fatalf("Gitea 에 PKCE 를 요구하면 안 된다: %q", spec.PKCEMethod)
	}
}

func TestToolSpecs_CoversJenkins(t *testing.T) {
	spec, ok := newToolSpecs()["installing_jenkins"]
	if !ok {
		t.Fatal("jenkins 스펙이 없다 — 이 스택의 CI 가 SSO 밖에 남는다")
	}
	if spec.Subdomain != "jenkins" {
		t.Fatalf("subdomain=%q", spec.Subdomain)
	}
	// oic-auth 플러그인의 콜백 경로다.
	if spec.CallbackPath != "/securityRealm/finishLogin" {
		t.Fatalf("callback=%q", spec.CallbackPath)
	}
	if spec.PKCEMethod != "" {
		t.Fatalf("Jenkins 에 PKCE 를 요구하면 안 된다: %q", spec.PKCEMethod)
	}
}

func TestToolSpecs_RedirectURIsUseAccessDomain(t *testing.T) {
	p := NewSSOProvisionerWithDomain(nil, "nullus.local")
	for step, want := range map[string]string{
		"installing_gitea":   "https://gitea.nullus.local/user/oauth2/keycloak/callback",
		"installing_jenkins": "https://jenkins.nullus.local/securityRealm/finishLogin",
	} {
		spec, ok := p.SpecFor(step)
		if !ok {
			t.Fatalf("%s 스펙이 없다", step)
		}
		got := buildRedirectURI(spec.Subdomain, "nullus.local", spec.CallbackPath)
		if got != want {
			t.Fatalf("%s: got %q, want %q", step, got, want)
		}
	}
}
