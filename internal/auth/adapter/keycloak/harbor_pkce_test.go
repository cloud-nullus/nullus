package keycloak

import "testing"

// Harbor 는 인가 요청에 PKCE 파라미터를 보내지 않는다. 그런데 클라이언트를
// pkce.code.challenge.method=S256 으로 등록해 두면 Keycloak 이 그것을 요구하고,
// 콜백이 이렇게 깨진다:
//
//	OIDC callback returned error: invalid_request -
//	Missing parameter: code_challenge_method
//
// Harbor 는 client secret 을 갖는 confidential client 다. PKCE 는 원래 공개
// 클라이언트를 위한 방어책이라, 여기서 빼도 보안 등급이 내려가지 않는다.
// (ArgoCD 도 같은 이유로 처음부터 PKCE 를 쓰지 않는다.)
func TestToolSpecs_HarborDoesNotRequirePKCE(t *testing.T) {
	spec, ok := newToolSpecs()["installing_harbor"]
	if !ok {
		t.Fatal("harbor 스펙이 없다")
	}
	if spec.PKCEMethod != "" {
		t.Fatalf("Harbor 는 PKCE 를 보내지 않는다. PKCEMethod=%q 면 로그인이 깨진다", spec.PKCEMethod)
	}
}

// PKCE 를 쓰는 도구는 그대로 유지돼야 한다 — 일괄로 꺼 버리면 안 된다.
func TestToolSpecs_PKCEKeptForToolsThatSendIt(t *testing.T) {
	specs := newToolSpecs()
	for _, step := range []string{"installing_grafana", "installing_gitlab"} {
		if specs[step].PKCEMethod != "S256" {
			t.Fatalf("%s 는 PKCE 를 보낸다 — 유지해야 한다", step)
		}
	}
}

// 이미 PKCE 로 등록된 클라이언트를 되돌릴 수 있어야 한다.
//
// 빈 맵만 보내면 Keycloak 구현에 따라 기존 속성이 남을 수 있다. 빈 값으로
// 명시해 확실히 지운다 — 남으면 스펙을 고쳐도 로그인이 계속 깨진다.
func TestPKCEAttributes_ClearsExistingMethodWhenUnset(t *testing.T) {
	attrs := pkceAttributes("")
	value, present := attrs["pkce.code.challenge.method"]
	if !present {
		t.Fatal("속성을 아예 빼면 기존 등록이 그대로 남을 수 있다")
	}
	if value != "" {
		t.Fatalf("빈 값으로 지워야 한다, got %v", value)
	}
}

func TestPKCEAttributes_SetsMethodWhenGiven(t *testing.T) {
	if got := pkceAttributes("S256")["pkce.code.challenge.method"]; got != "S256" {
		t.Fatalf("expected S256, got %v", got)
	}
}
