package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/auth/domain"
)

// CredentialRepository 는 ID/PW 로그인에 쓸 사용자 자격을 읽는다.
type CredentialRepository interface {
	// FindByEmail 은 이메일로 자격을 찾는다. 없으면 (nil, nil) 이다 —
	// "없음" 은 오류가 아니라 로그인 실패의 한 경우이고, 저장소 장애와 구분해야
	// 한다(장애를 자격 오류로 뭉개면 진짜 원인을 못 찾는다).
	FindByEmail(ctx context.Context, email string) (*domain.Credential, error)
}

// SessionIssuer 는 로그인 성공 시 세션 토큰을 발급한다.
type SessionIssuer interface {
	Issue(claims domain.SessionClaims) (string, error)
}
