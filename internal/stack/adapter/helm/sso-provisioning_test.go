package helm

import (
	"context"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type recordingProvisioner struct{ accessDomain, slug string }

func (r recordingProvisioner) ClientIDFor(string) (string, bool)                   { return r.slug, true }
func (r recordingProvisioner) ToolSteps() []string                                 { return nil }
func (r recordingProvisioner) Provision(context.Context, port.SSOClientSpec) error { return nil }
func (r recordingProvisioner) Deprovision(context.Context, string) error           { return nil }

func slugFor(t *testing.T, namespace, accessDomain, orgID string) string {
	t.Helper()
	o := NewOrchestrator(nil, nil, namespace)
	o.secretOrgID = orgID
	o.stackConfig = &domain.StackConfig{AccessDomain: accessDomain}
	o.ssoFactory = func(ad, slug string) port.SSOProvisioner {
		return recordingProvisioner{accessDomain: ad, slug: slug}
	}
	got, _ := o.ssoProvisioner().ClientIDFor("installing_argocd")
	return got
}

// client ID 는 공용 realm 안에서 스택을 구분하는 이름이다. 접속 도메인에서 슬러그를
// 뽑으면 도메인을 공유하는 스택끼리 같은 client ID 를 써서 서로의 등록을 덮어쓴다.
// 네임스페이스는 스택마다 다르므로 이쪽이 안전하다.
func TestSSOProvisioner_SlugIsPerStackEvenWhenDomainIsShared(t *testing.T) {
	a := slugFor(t, "team-a", "nullus.local", "org-1")
	b := slugFor(t, "team-b", "nullus.local", "org-1")
	if a == b {
		t.Fatalf("expected different stacks to get different slugs, both got %q", a)
	}
}

func TestSSOProvisioner_SlugUsesNamespace(t *testing.T) {
	if got := slugFor(t, "ssowire", "nullus.local", "org-1"); got != "ssowire" {
		t.Fatalf("expected the namespace to be the slug, got %q", got)
	}
}

// 접속 도메인은 여전히 provisioner 로 넘어간다 — redirect URI 를 만드는 데 쓴다.
func TestSSOProvisioner_AccessDomainStillReachesProvisioner(t *testing.T) {
	o := NewOrchestrator(nil, nil, "ssowire")
	o.stackConfig = &domain.StackConfig{AccessDomain: "nullus.local"}
	var seen string
	o.ssoFactory = func(ad, slug string) port.SSOProvisioner {
		seen = ad
		return recordingProvisioner{accessDomain: ad, slug: slug}
	}
	_ = o.ssoProvisioner()
	if seen != "nullus.local" {
		t.Fatalf("expected the access domain to reach the provisioner, got %q", seen)
	}
}
