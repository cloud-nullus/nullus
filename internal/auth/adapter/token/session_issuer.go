package token

import "github.com/cloud-nullus/draft/internal/auth/domain"

// SessionIssuerAdapter 는 LocalIssuerService 를 auth 유스케이스의 포트에 맞춘다.
//
// 매핑이 하나라도 빠지면 로그인은 되는데 권한이 없거나 조직이 비어 보이는,
// 원인을 찾기 어려운 상태가 된다.
type SessionIssuerAdapter struct {
	svc *LocalIssuerService
}

func NewSessionIssuer(svc *LocalIssuerService) SessionIssuerAdapter {
	return SessionIssuerAdapter{svc: svc}
}

func (a SessionIssuerAdapter) Issue(c domain.SessionClaims) (string, error) {
	return a.svc.Issue(Claims{
		UserID: c.UserID,
		Email:  c.Email,
		Name:   c.Name,
		Role:   c.Role,
		OrgID:  c.OrgID,
	})
}
