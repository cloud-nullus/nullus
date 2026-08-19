// Package token 은 Nullus 가 직접 발급하는 세션 토큰을 다룬다.
//
// IdP(OIDC)와 별개의 경로다. IdP 가 죽어도 들어갈 수단이 있어야 하므로 ID/PW 로
// 인증한 사용자에게 이 토큰을 준다. 검증 주체가 우리이므로 issuer 를 우리 이름으로
// 찍어 두고, 미들웨어가 그 값을 보고 어느 검증기로 보낼지 정한다.
package token

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LocalIssuer 는 우리가 발급한 토큰에 찍는 issuer 다. Keycloak 이 발급한 토큰과
// 구분하는 유일한 표식이므로 IdP 의 issuer 와 겹치지 않아야 한다.
const LocalIssuer = "nullus-local"

// Claims 는 토큰이 실어 나르는 사용자 정보다. 미들웨어가 이 값으로
// admindomain.User 를 만들어 컨텍스트에 넣는다.
type Claims struct {
	UserID string
	Email  string
	Name   string
	Role   string
	OrgID  string
}

type localClaims struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role,omitempty"`
	OrgID string `json:"org_id,omitempty"`
	jwt.RegisteredClaims
}

type LocalIssuerService struct {
	secret []byte
	ttl    time.Duration
}

func NewLocalIssuer(secret string, ttl time.Duration) *LocalIssuerService {
	return &LocalIssuerService{secret: []byte(strings.TrimSpace(secret)), ttl: ttl}
}

// Enabled 는 서명키가 있어 발급·검증이 가능한지 알려준다.
func (s *LocalIssuerService) Enabled() bool {
	return s != nil && len(s.secret) > 0
}

func (s *LocalIssuerService) Issue(c Claims) (string, error) {
	// 키가 비면 누구나 같은 토큰을 만들 수 있다. 발급 자체를 막는다.
	if !s.Enabled() {
		return "", errors.New("세션 서명키가 설정되지 않았습니다 (auth.session.secret)")
	}
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, localClaims{
		Email: c.Email,
		Name:  c.Name,
		Role:  c.Role,
		OrgID: c.OrgID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.UserID,
			Issuer:    LocalIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	})
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("세션 토큰 서명 실패: %w", err)
	}
	return signed, nil
}

func (s *LocalIssuerService) Verify(raw string) (Claims, error) {
	if !s.Enabled() {
		return Claims{}, errors.New("세션 서명키가 설정되지 않았습니다")
	}

	var claims localClaims
	// 서명 방식을 HS256 으로 못박는다. 지정하지 않으면 alg=none 이나 알고리즘
	// 혼동(RS256 공개키를 HMAC 키로 쓰는) 공격이 통한다.
	_, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(LocalIssuer))
	if err != nil {
		return Claims{}, fmt.Errorf("세션 토큰 검증 실패: %w", err)
	}

	return Claims{
		UserID: claims.Subject,
		Email:  claims.Email,
		Name:   claims.Name,
		Role:   claims.Role,
		OrgID:  claims.OrgID,
	}, nil
}

// IssuerOf 는 서명을 확인하지 않고 issuer 만 읽는다.
//
// 어느 검증기로 보낼지 고르는 용도일 뿐이므로 이 값을 신뢰하면 안 된다 —
// 실제 판단은 뒤따르는 Verify 가 한다.
func IssuerOf(raw string) string {
	var claims jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(raw, &claims); err != nil {
		return ""
	}
	return claims.Issuer
}
