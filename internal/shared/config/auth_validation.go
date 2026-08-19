package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	// AuthModeOIDC 는 IdP 가 서명한 JWT 를 실제로 검증한다.
	AuthModeOIDC = "oidc"
	// AuthModeSession 은 알파 시절의 단순화된 방식으로, 클라이언트가 보낸
	// X-User-* 헤더를 그대로 신뢰한다 — 검증이 아니다.
	AuthModeSession = "session"

	serverModeDevelopment = "development"
)

func (c Config) normalizedAuthMode() string {
	return strings.ToLower(strings.TrimSpace(c.Auth.Mode))
}

func (c Config) isDevelopment() bool {
	return strings.ToLower(strings.TrimSpace(c.Server.Mode)) == serverModeDevelopment
}

// ValidateAuth rejects auth settings that cannot work at runtime.
//
// development 모드는 인증 미들웨어를 붙이지 않으므로 검사에서 제외한다.
// 그 밖의 모드에서 oidc 를 골랐는데 issuer 가 비어 있으면 JWKS 를 못 받아
// **모든 요청이 401** 이 된다. 그 상태로 기동시키면 원인 찾기 어려운 장애가 되므로
// 기동 시점에 끊는다.
func (c Config) ValidateAuth() error {
	if c.isDevelopment() {
		return nil
	}

	if c.normalizedAuthMode() == AuthModeOIDC && strings.TrimSpace(c.Auth.OIDC.IssuerURL) == "" {
		return fmt.Errorf(
			"auth.mode=%s 인데 auth.oidc.issuer_url 이 비어 있습니다. "+
				"IdP 의 공개 issuer 주소를 설정하세요 (helm: config.auth.oidcIssuerUrl)",
			AuthModeOIDC)
	}

	return nil
}

// TrustsClientSuppliedIdentity reports whether the running config lets callers
// assert their own identity.
//
// session 모드는 X-User-ID / X-User-Role 헤더를 그대로 믿는다. 즉 누구나
// 관리자를 자칭할 수 있다. development 는 인증을 끄는 것이 의도된 동작이라 제외한다 —
// 여기서 참을 돌려주는 경우는 "운영처럼 돌리는데 사실 무인증" 인 상황뿐이다.
func (c Config) TrustsClientSuppliedIdentity() bool {
	if c.isDevelopment() {
		return false
	}
	return c.normalizedAuthMode() != AuthModeOIDC
}

// defaultSessionTTL 은 auth.session.max_age 가 없을 때 쓰는 세션 수명이다.
const defaultSessionTTL = 24 * time.Hour

// SessionTTL 은 ID/PW 로그인이 발급하는 세션 토큰의 수명이다.
//
// max_age 가 0 이면 발급 즉시 만료된 토큰이 나온다 — 로그인은 성공하는데 다음
// 요청이 401 이라 원인을 찾기 어렵다. 그래서 0 이하는 기본값으로 물러난다.
func (c Config) SessionTTL() time.Duration {
	if c.Auth.Session.MaxAge > 0 {
		return time.Duration(c.Auth.Session.MaxAge) * time.Second
	}
	return defaultSessionTTL
}
