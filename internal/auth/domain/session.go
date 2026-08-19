package domain

// Credential 은 ID/PW 로그인에 필요한 사용자 정보다.
//
// PasswordHash 가 비면 비밀번호를 설정하지 않은 계정(OIDC 전용)이고, ID/PW 로는
// 들어올 수 없다.
type Credential struct {
	UserID       string
	Email        string
	Name         string
	Role         string
	OrgID        string
	PasswordHash string
	IsActive     bool
}

// SessionClaims 는 로그인 성공 시 세션 토큰에 실을 정보다.
type SessionClaims struct {
	UserID string
	Email  string
	Name   string
	Role   string
	OrgID  string
}

// Claims 는 이 자격에 대응하는 세션 클레임을 만든다.
func (c Credential) Claims() SessionClaims {
	return SessionClaims{
		UserID: c.UserID,
		Email:  c.Email,
		Name:   c.Name,
		Role:   c.Role,
		OrgID:  c.OrgID,
	}
}

// CanLogInWithPassword 는 ID/PW 경로로 들어올 수 있는 계정인지 알려준다.
func (c Credential) CanLogInWithPassword() bool {
	return c.IsActive && c.PasswordHash != ""
}
