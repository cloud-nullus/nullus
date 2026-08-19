package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/cloud-nullus/draft/internal/auth/domain"
	"github.com/cloud-nullus/draft/internal/auth/port"
)

type fakeCredentialRepo struct {
	cred *domain.Credential
	err  error
}

func (f fakeCredentialRepo) FindByEmail(context.Context, string) (*domain.Credential, error) {
	return f.cred, f.err
}

type fakeIssuer struct {
	issued domain.SessionClaims
	token  string
	err    error
}

func (f *fakeIssuer) Issue(c domain.SessionClaims) (string, error) {
	f.issued = c
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

func activeCredential(t *testing.T) *domain.Credential {
	t.Helper()
	hash, err := domain.HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	return &domain.Credential{
		UserID: "u-1", Email: "admin@nullus.dev", Name: "Admin",
		Role: "admin", OrgID: "org-1", PasswordHash: hash, IsActive: true,
	}
}

func TestLogin_IssuesTokenForValidCredentials(t *testing.T) {
	issuer := &fakeIssuer{token: "signed-token"}
	uc := NewLogin(fakeCredentialRepo{cred: activeCredential(t)}, issuer)

	out, err := uc.Execute(context.Background(), "admin@nullus.dev", "correct-horse-battery")
	if err != nil {
		t.Fatalf("로그인이 실패했다: %v", err)
	}
	if out.Token != "signed-token" {
		t.Fatalf("토큰이 반환되지 않았다: %+v", out)
	}
	if issuer.issued.UserID != "u-1" || issuer.issued.Role != "admin" || issuer.issued.OrgID != "org-1" {
		t.Fatalf("토큰 클레임이 사용자와 다르다: %+v", issuer.issued)
	}
}

func TestLogin_RejectsWrongPassword(t *testing.T) {
	uc := NewLogin(fakeCredentialRepo{cred: activeCredential(t)}, &fakeIssuer{token: "t"})
	if _, err := uc.Execute(context.Background(), "admin@nullus.dev", "wrong"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

// 없는 이메일과 틀린 비밀번호를 구분해 알리면 어떤 이메일이 가입돼 있는지
// 알아낼 수 있다. 같은 오류로 답한다.
func TestLogin_UnknownEmailIsIndistinguishable(t *testing.T) {
	uc := NewLogin(fakeCredentialRepo{cred: nil}, &fakeIssuer{token: "t"})
	if _, err := uc.Execute(context.Background(), "nobody@nullus.dev", "whatever"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

// 비밀번호를 설정하지 않은 계정(OIDC 전용)은 ID/PW 로 들어올 수 없다.
func TestLogin_AccountWithoutPasswordCannotLogIn(t *testing.T) {
	cred := activeCredential(t)
	cred.PasswordHash = ""
	uc := NewLogin(fakeCredentialRepo{cred: cred}, &fakeIssuer{token: "t"})

	if _, err := uc.Execute(context.Background(), cred.Email, ""); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

// 비활성 계정은 비밀번호가 맞아도 들어올 수 없다.
func TestLogin_InactiveAccountIsRejected(t *testing.T) {
	cred := activeCredential(t)
	cred.IsActive = false
	uc := NewLogin(fakeCredentialRepo{cred: cred}, &fakeIssuer{token: "t"})

	if _, err := uc.Execute(context.Background(), cred.Email, "correct-horse-battery"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

// 저장소 장애를 자격 오류로 뭉개면 진짜 원인을 못 찾는다.
func TestLogin_RepositoryErrorIsNotMaskedAsInvalidCredentials(t *testing.T) {
	repoErr := errors.New("db down")
	uc := NewLogin(fakeCredentialRepo{err: repoErr}, &fakeIssuer{token: "t"})

	_, err := uc.Execute(context.Background(), "a@b.c", "x")
	if errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatal("저장소 오류가 자격 오류로 뭉개졌다")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("원인이 보존되지 않았다: %v", err)
	}
}

var _ port.CredentialRepository = fakeCredentialRepo{}
