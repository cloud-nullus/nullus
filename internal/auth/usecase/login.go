// Package usecase 는 인증 컨텍스트의 애플리케이션 로직을 담는다.
package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/auth/domain"
	"github.com/cloud-nullus/draft/internal/auth/port"
)

// LoginOutput 은 로그인 결과다.
type LoginOutput struct {
	Token string
	User  domain.Credential
}

// Login 은 ID/PW 로 사용자를 인증하고 세션 토큰을 발급한다.
//
// OIDC 와 나란히 서는 두 번째 경로다. IdP 가 죽어도 들어갈 수단이 있어야 한다.
type Login struct {
	repo   port.CredentialRepository
	issuer port.SessionIssuer
}

func NewLogin(repo port.CredentialRepository, issuer port.SessionIssuer) *Login {
	return &Login{repo: repo, issuer: issuer}
}

func (uc *Login) Execute(ctx context.Context, email, password string) (LoginOutput, error) {
	cred, err := uc.repo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		// 저장소 장애를 자격 오류로 뭉개지 않는다 — 뭉개면 DB 가 죽었을 때
		// 전 사용자가 "비밀번호가 틀렸다" 는 답을 받고 원인을 못 찾는다.
		return LoginOutput{}, fmt.Errorf("자격 조회 실패: %w", err)
	}

	// 없는 이메일·비활성 계정·틀린 비밀번호를 구분해 알리지 않는다.
	// 구분하면 어떤 이메일이 가입돼 있는지 알아낼 수 있다.
	if cred == nil || !cred.CanLogInWithPassword() {
		return LoginOutput{}, domain.ErrInvalidCredentials
	}
	if err := domain.VerifyPassword(cred.PasswordHash, password); err != nil {
		return LoginOutput{}, err
	}

	token, err := uc.issuer.Issue(cred.Claims())
	if err != nil {
		return LoginOutput{}, fmt.Errorf("세션 토큰 발급 실패: %w", err)
	}
	return LoginOutput{Token: token, User: *cred}, nil
}
