package token

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-at-least-32-bytes-long!!"

func TestIssueAndVerify_RoundTripsClaims(t *testing.T) {
	issuer := NewLocalIssuer(testSecret, time.Hour)

	raw, err := issuer.Issue(Claims{
		UserID: "u-1", Email: "a@nullus.dev", Name: "A", Role: "admin", OrgID: "org-1",
	})
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	got, err := issuer.Verify(raw)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if got.UserID != "u-1" || got.Email != "a@nullus.dev" || got.Role != "admin" || got.OrgID != "org-1" {
		t.Fatalf("클레임이 왕복하지 않았다: %+v", got)
	}
}

// DualAuth 가 토큰을 보고 어느 검증기로 보낼지 정한다. issuer 가 우리 것이 아니면
// OIDC 로 넘겨야 하므로, 발급한 토큰에는 우리 issuer 가 찍혀야 한다.
func TestIssue_StampsLocalIssuer(t *testing.T) {
	raw, err := NewLocalIssuer(testSecret, time.Hour).Issue(Claims{UserID: "u-1"})
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	if iss := IssuerOf(raw); iss != LocalIssuer {
		t.Fatalf("expected issuer %q, got %q", LocalIssuer, iss)
	}
}

// IssuerOf 는 서명 검증 전에 읽는다 — 어디로 보낼지 정하는 용도이므로 신뢰하면
// 안 된다. Keycloak 토큰은 우리 issuer 가 아니어야 한다.
func TestIssuerOf_ForeignTokenIsNotLocal(t *testing.T) {
	if iss := IssuerOf("not.a.jwt"); iss == LocalIssuer {
		t.Fatal("깨진 토큰이 로컬 발급으로 판정됐다")
	}
}

func TestVerify_RejectsWrongSecret(t *testing.T) {
	raw, _ := NewLocalIssuer(testSecret, time.Hour).Issue(Claims{UserID: "u-1"})
	if _, err := NewLocalIssuer("another-secret-at-least-32-bytes!!", time.Hour).Verify(raw); err == nil {
		t.Fatal("다른 키로 서명 검증이 통과했다")
	}
}

func TestVerify_RejectsExpiredToken(t *testing.T) {
	raw, _ := NewLocalIssuer(testSecret, -time.Minute).Issue(Claims{UserID: "u-1"})
	if _, err := NewLocalIssuer(testSecret, time.Hour).Verify(raw); err == nil {
		t.Fatal("만료된 토큰이 통과했다")
	}
}

func TestVerify_RejectsTamperedToken(t *testing.T) {
	raw, _ := NewLocalIssuer(testSecret, time.Hour).Issue(Claims{UserID: "u-1", Role: "developer"})
	parts := strings.Split(raw, ".")
	// 페이로드를 건드리면 서명이 맞지 않는다.
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	if _, err := NewLocalIssuer(testSecret, time.Hour).Verify(tampered); err == nil {
		t.Fatal("변조된 토큰이 통과했다")
	}
}

// 비밀키가 비면 누구나 같은 토큰을 만들 수 있다. 발급 자체를 막는다.
func TestIssue_RefusesEmptySecret(t *testing.T) {
	if _, err := NewLocalIssuer("  ", time.Hour).Issue(Claims{UserID: "u-1"}); err == nil {
		t.Fatal("빈 비밀키로 토큰이 발급됐다")
	}
}

// alg=none 은 서명 없이 통과시키려는 고전적 우회다.
func TestVerify_RejectsNoneAlgorithm(t *testing.T) {
	// {"alg":"none","typ":"JWT"} . {"sub":"u-1","iss":"nullus-local"} . (빈 서명)
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ1LTEiLCJpc3MiOiJudWxsdXMtbG9jYWwifQ."
	if _, err := NewLocalIssuer(testSecret, time.Hour).Verify(none); err == nil {
		t.Fatal("alg=none 토큰이 통과했다")
	}
}
