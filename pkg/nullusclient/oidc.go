package nullusclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// S-3: OIDC 토큰 획득(Authorization Code + PKCE)과 만료/refresh 처리.
// 명령 표면(브라우저 열기, 콜백 리슨, 안내 문구)은 트랙 A(A-3)가 얹는다.
// Keycloak·Authentik 모두 discovery 와 PKCE 를 지원한다 — OIDC Provider 가이드.

// ErrLoginRequired 는 재로그인 없이는 진행할 수 없는 상태다 — refresh token
// 만료·폐기, 또는 만료된 세션에 refresh token 이 없는 경우. 호출측은
// errors.Is 로 구분해 `nullus login` 안내를 낸다 (exit code 3, 계약 §1).
var ErrLoginRequired = errors.New("로그인이 필요하다 — nullus login 을 실행하라")

// tokenExpirySkew 만큼 만료가 남았어도 갱신한다 — 전송 중 만료로 401 을
// 받는 경계를 피한다.
const tokenExpirySkew = 30 * time.Second

// OIDCConfig 는 로그인에 필요한 IdP 좌표다. 서버는 이를 노출하는 API 가
// 없으므로(auth 모듈은 POST /auth/login 뿐) 트랙 A 가 플래그/env/설정
// 파일에서 모아 전달한다.
type OIDCConfig struct {
	Issuer   string // 예: https://keycloak.example.com/realms/nullus
	ClientID string // public client — PKCE 전제, client secret 없음
	Scopes   []string // 비면 ["openid"]
}

// OIDCEndpoints 는 discovery 결과 중 이 라이브러리가 쓰는 것만 담는다.
type OIDCEndpoints struct {
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
}

// DiscoverOIDC 는 issuer 의 /.well-known/openid-configuration 을 읽는다.
// hc 가 nil 이면 http.DefaultClient 를 쓴다.
func DiscoverOIDC(ctx context.Context, hc *http.Client, issuer string) (OIDCEndpoints, error) {
	wellKnown := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return OIDCEndpoints{}, fmt.Errorf("discovery 요청 생성: %w", err)
	}
	resp, err := httpClientOr(hc).Do(req)
	if err != nil {
		return OIDCEndpoints{}, fmt.Errorf("OIDC discovery (%s): %w", wellKnown, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return OIDCEndpoints{}, fmt.Errorf("OIDC discovery (%s): HTTP %d", wellKnown, resp.StatusCode)
	}
	var doc struct {
		Issuer string `json:"issuer"`
		OIDCEndpoints
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return OIDCEndpoints{}, fmt.Errorf("discovery 응답 파싱: %w", err)
	}
	// OIDC 스펙: 응답 issuer 는 요청 issuer 와 일치해야 한다 — 프록시 오설정이나
	// 바꿔치기된 응답을 여기서 거른다.
	if strings.TrimRight(doc.Issuer, "/") != strings.TrimRight(issuer, "/") {
		return OIDCEndpoints{}, fmt.Errorf("issuer 불일치: 요청 %q, 응답 %q — IdP 주소·프록시 설정을 확인하라", issuer, doc.Issuer)
	}
	if doc.Authorization == "" || doc.Token == "" {
		return OIDCEndpoints{}, fmt.Errorf("discovery 응답에 authorization/token endpoint 가 없다 (%s)", wellKnown)
	}
	return doc.OIDCEndpoints, nil
}

// AuthCodeFlow 는 Authorization Code + PKCE 로그인 시도 하나다. verifier 와
// state 는 생성 시 고정된다 — AuthURL 로 사용자를 보내고, 콜백으로 받은
// code·state 를 Exchange 에 넘긴다.
type AuthCodeFlow struct {
	oidc     OIDCConfig
	conf     oauth2.Config
	verifier string
	state    string
}

// NewAuthCodeFlow 는 로그인 시도를 만든다. redirectURI 는 콜백 수신 주소
// (예: http://127.0.0.1:<port>/callback) — 리슨은 트랙 A 몫이다.
func NewAuthCodeFlow(cfg OIDCConfig, ep OIDCEndpoints, redirectURI string) (*AuthCodeFlow, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" {
		return nil, fmt.Errorf("OIDC issuer 와 client ID 는 필수다")
	}
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid"}
	}
	return &AuthCodeFlow{
		oidc: cfg,
		conf: oauth2.Config{
			ClientID:    cfg.ClientID,
			Endpoint:    oauth2.Endpoint{AuthURL: ep.Authorization, TokenURL: ep.Token},
			RedirectURL: redirectURI,
			Scopes:      scopes,
		},
		verifier: oauth2.GenerateVerifier(),
		// state 도 같은 crypto/rand 소스면 충분하다 — 의미는 CSRF 방어뿐.
		state: oauth2.GenerateVerifier(),
	}, nil
}

// AuthURL 은 사용자를 보낼 인가 URL 이다 (PKCE S256 challenge + state 포함).
func (f *AuthCodeFlow) AuthURL() string {
	return f.conf.AuthCodeURL(f.state, oauth2.S256ChallengeOption(f.verifier))
}

// State 는 콜백 검증용 state 값이다.
func (f *AuthCodeFlow) State() string { return f.state }

// Exchange 는 콜백으로 받은 code 를 토큰으로 바꾼다. state 가 이 시도의 것과
// 다르면 token endpoint 에 가지 않고 거부한다.
func (f *AuthCodeFlow) Exchange(ctx context.Context, hc *http.Client, code, state string) (Session, error) {
	if state != f.state {
		return Session{}, fmt.Errorf("state 불일치 — 이 로그인 시도의 콜백이 아니다 (CSRF 가능성)")
	}
	tok, err := f.conf.Exchange(oauth2Context(ctx, hc), code, oauth2.VerifierOption(f.verifier))
	if err != nil {
		return Session{}, fmt.Errorf("토큰 교환: %w", err)
	}
	return Session{
		AccessToken:   tok.AccessToken,
		RefreshToken:  tok.RefreshToken,
		Expiry:        tok.Expiry,
		Issuer:        f.oidc.Issuer,
		ClientID:      f.oidc.ClientID,
		TokenEndpoint: f.conf.Endpoint.TokenURL,
	}, nil
}

// RefreshSession 은 refresh token 으로 access token 을 갱신한 세션을 돌려준다.
// refresh token 이 없거나 IdP 가 invalid_grant 로 거부하면 ErrLoginRequired.
func RefreshSession(ctx context.Context, hc *http.Client, s Session) (Session, error) {
	if s.RefreshToken == "" {
		return Session{}, fmt.Errorf("세션에 refresh token 이 없다: %w", ErrLoginRequired)
	}
	conf := oauth2.Config{
		ClientID: s.ClientID,
		Endpoint: oauth2.Endpoint{TokenURL: s.TokenEndpoint},
	}
	tok, err := conf.TokenSource(oauth2Context(ctx, hc), &oauth2.Token{RefreshToken: s.RefreshToken}).Token()
	if err != nil {
		var rErr *oauth2.RetrieveError
		if errors.As(err, &rErr) && rErr.ErrorCode == "invalid_grant" {
			return Session{}, fmt.Errorf("세션이 만료·폐기됐다 (%s): %w", rErr.ErrorDescription, ErrLoginRequired)
		}
		return Session{}, fmt.Errorf("토큰 갱신: %w", err)
	}
	next := s
	next.AccessToken = tok.AccessToken
	next.Expiry = tok.Expiry
	// IdP 가 refresh token 을 회전하면 새것을, 응답에서 빠지면 기존 것을 유지한다.
	if tok.RefreshToken != "" {
		next.RefreshToken = tok.RefreshToken
	}
	return next, nil
}

// EnsureFreshToken 은 지금 쓸 수 있는 access token 을 돌려준다. 우선순위는
// S-2 와 같다: NULLUS_TOKEN env → 로그인 세션(만료 임박 시 refresh 후 영속)
// → 정적 토큰 파일(bootstrap) → 빈 값. 자격 부재는 오류가 아니다 —
// dev 모드(auth.mode=session)는 토큰 없이 동작한다.
func EnsureFreshToken(ctx context.Context, hc *http.Client) (string, error) {
	if tok := os.Getenv(EnvToken); tok != "" {
		return tok, nil
	}
	s, found, err := ReadSession()
	if err != nil {
		return "", err
	}
	if found {
		if s.Expiry.IsZero() || time.Now().Add(tokenExpirySkew).Before(s.Expiry) {
			return s.AccessToken, nil
		}
		next, err := RefreshSession(ctx, hc, s)
		if err != nil {
			return "", err
		}
		if err := SaveSession(next); err != nil {
			return "", fmt.Errorf("갱신된 세션 저장: %w", err)
		}
		return next.AccessToken, nil
	}
	return readTokenFile()
}

func httpClientOr(hc *http.Client) *http.Client {
	if hc == nil {
		return http.DefaultClient
	}
	return hc
}

// oauth2Context 는 x/oauth2 가 쓸 http.Client 를 ctx 에 싣는다.
func oauth2Context(ctx context.Context, hc *http.Client) context.Context {
	if hc == nil {
		return ctx
	}
	return context.WithValue(ctx, oauth2.HTTPClient, hc)
}
