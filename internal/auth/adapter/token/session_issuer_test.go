package token

import (
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/auth/domain"
)

// 매핑이 빠지면 로그인은 되는데 권한이 없거나 조직이 비어 보인다 — 인증 문제로는
// 보이지 않아 원인을 찾기 어렵다.
func TestSessionIssuerAdapter_CarriesEveryClaim(t *testing.T) {
	svc := NewLocalIssuer("adapter-test-secret-32-bytes-ok!!", time.Hour)
	raw, err := NewSessionIssuer(svc).Issue(domain.SessionClaims{
		UserID: "u-1", Email: "a@nullus.dev", Name: "A", Role: "devops", OrgID: "org-7",
	})
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	got, err := svc.Verify(raw)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if got.UserID != "u-1" || got.Email != "a@nullus.dev" || got.Name != "A" ||
		got.Role != "devops" || got.OrgID != "org-7" {
		t.Fatalf("클레임이 유실됐다: %+v", got)
	}
}
