package nullusclient

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// S-3 완료 기준: OIDC 로그인 플로우(Authorization Code + PKCE)와 만료/refresh
// 처리를 라이브러리로 제공. 명령 표면(브라우저 열기, 콜백 리슨)은 트랙 A(A-3).
// dev 모드(auth.mode=session)는 토큰 없이 통과해야 한다.

// newIdP 는 discovery + token endpoint 를 흉내내는 최소 IdP 다.
// tokenHandler 가 nil 이면 token endpoint 호출 자체가 실패로 간주된다.
func newIdP(t *testing.T, tokenHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/token",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if tokenHandler == nil {
			t.Error("token endpoint 가 호출되면 안 되는 시나리오다")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		tokenHandler(w, r)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func tokenJSON(access, refresh string, expiresIn int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{
			"access_token": access,
			"token_type":   "Bearer",
			"expires_in":   expiresIn,
		}
		if refresh != "" {
			body["refresh_token"] = refresh
		}
		_ = json.NewEncoder(w).Encode(body)
	}
}

func TestDiscoverOIDC_ReturnsEndpoints(t *testing.T) {
	srv := newIdP(t, nil)

	ep, err := DiscoverOIDC(context.Background(), nil, srv.URL)
	if err != nil {
		t.Fatalf("DiscoverOIDC: %v", err)
	}
	if ep.Authorization != srv.URL+"/auth" || ep.Token != srv.URL+"/token" {
		t.Errorf("ep = %+v", ep)
	}
}

func TestDiscoverOIDC_RejectsIssuerMismatch(t *testing.T) {
	// OIDC discovery 스펙: 응답의 issuer 는 요청한 issuer 와 일치해야 한다.
	// 프록시 오설정·피싱성 응답을 걸러 낸다.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 "https://evil.example.com",
			"authorization_endpoint": "https://evil.example.com/auth",
			"token_endpoint":         "https://evil.example.com/token",
		})
	}))
	defer srv.Close()

	if _, err := DiscoverOIDC(context.Background(), nil, srv.URL); err == nil {
		t.Fatal("issuer 불일치를 거부해야 한다")
	}
}

func TestAuthCodeFlow_AuthURLCarriesPKCEAndState(t *testing.T) {
	srv := newIdP(t, nil)
	ep, _ := DiscoverOIDC(context.Background(), nil, srv.URL)

	f, err := NewAuthCodeFlow(OIDCConfig{Issuer: srv.URL, ClientID: "nullus-cli"}, ep, "http://127.0.0.1:8765/callback")
	if err != nil {
		t.Fatalf("NewAuthCodeFlow: %v", err)
	}
	u, err := url.Parse(f.AuthURL())
	if err != nil {
		t.Fatalf("AuthURL 파싱: %v", err)
	}
	q := u.Query()
	if q.Get("client_id") != "nullus-cli" || q.Get("response_type") != "code" {
		t.Errorf("query = %v", q)
	}
	if q.Get("redirect_uri") != "http://127.0.0.1:8765/callback" {
		t.Errorf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if !strings.Contains(q.Get("scope"), "openid") {
		t.Errorf("scope = %q, want openid 포함", q.Get("scope"))
	}
	if q.Get("state") == "" {
		t.Error("state 가 비었다 — CSRF 방어 불가")
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Errorf("PKCE 파라미터 누락: challenge=%q method=%q",
			q.Get("code_challenge"), q.Get("code_challenge_method"))
	}
}

func TestAuthCodeFlow_Exchange_VerifierMatchesChallenge(t *testing.T) {
	var gotGrant, gotCode, gotVerifier string
	srv := newIdP(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotGrant = r.PostForm.Get("grant_type")
		gotCode = r.PostForm.Get("code")
		gotVerifier = r.PostForm.Get("code_verifier")
		tokenJSON("at-new", "rt-new", 300)(w, r)
	})
	ep, _ := DiscoverOIDC(context.Background(), nil, srv.URL)

	f, _ := NewAuthCodeFlow(OIDCConfig{Issuer: srv.URL, ClientID: "nullus-cli"}, ep, "http://127.0.0.1:8765/callback")
	challenge := url.QueryEscape("")
	if u, err := url.Parse(f.AuthURL()); err == nil {
		challenge = u.Query().Get("code_challenge")
	}

	s, err := f.Exchange(context.Background(), nil, "code-1", f.State())
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if gotGrant != "authorization_code" || gotCode != "code-1" {
		t.Errorf("grant=%q code=%q", gotGrant, gotCode)
	}
	// 보낸 verifier 가 AuthURL 의 challenge 와 실제로 짝이어야 한다 (S256).
	sum := sha256.Sum256([]byte(gotVerifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
		t.Error("code_verifier 가 code_challenge 와 짝이 아니다")
	}
	if s.AccessToken != "at-new" || s.RefreshToken != "rt-new" {
		t.Errorf("session = %+v", s)
	}
	if !s.Expiry.After(time.Now()) {
		t.Errorf("Expiry = %v, want 미래", s.Expiry)
	}
	if s.Issuer != srv.URL || s.ClientID != "nullus-cli" || s.TokenEndpoint != srv.URL+"/token" {
		t.Errorf("refresh 재료 누락: %+v", s)
	}
}

func TestAuthCodeFlow_Exchange_RejectsStateMismatch(t *testing.T) {
	srv := newIdP(t, nil) // state 가 틀리면 token endpoint 까지 가면 안 된다
	ep, _ := DiscoverOIDC(context.Background(), nil, srv.URL)

	f, _ := NewAuthCodeFlow(OIDCConfig{Issuer: srv.URL, ClientID: "nullus-cli"}, ep, "http://127.0.0.1:8765/callback")
	if _, err := f.Exchange(context.Background(), nil, "code-1", "wrong-state"); err == nil {
		t.Fatal("state 불일치를 거부해야 한다")
	}
}

func TestRefreshSession_RotatesRefreshToken(t *testing.T) {
	var gotGrant, gotRefresh string
	srv := newIdP(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotGrant = r.PostForm.Get("grant_type")
		gotRefresh = r.PostForm.Get("refresh_token")
		tokenJSON("at-2", "rt-2", 300)(w, r)
	})

	s := testSession()
	s.TokenEndpoint = srv.URL + "/token"
	next, err := RefreshSession(context.Background(), nil, s)
	if err != nil {
		t.Fatalf("RefreshSession: %v", err)
	}
	if gotGrant != "refresh_token" || gotRefresh != "rt-1" {
		t.Errorf("grant=%q refresh=%q", gotGrant, gotRefresh)
	}
	if next.AccessToken != "at-2" || next.RefreshToken != "rt-2" {
		t.Errorf("next = %+v", next)
	}
	if next.Issuer != s.Issuer || next.ClientID != s.ClientID || next.TokenEndpoint != s.TokenEndpoint {
		t.Errorf("refresh 재료가 유실됐다: %+v", next)
	}
}

func TestRefreshSession_KeepsRefreshTokenWhenNotRotated(t *testing.T) {
	// Keycloak 은 기본으로 refresh token 을 회전하지만, 스펙상 응답에서 빠질 수도
	// 있다 — 그때 기존 것을 버리면 다음 갱신이 불가능해진다.
	srv := newIdP(t, tokenJSON("at-2", "", 300))

	s := testSession()
	s.TokenEndpoint = srv.URL + "/token"
	next, err := RefreshSession(context.Background(), nil, s)
	if err != nil {
		t.Fatalf("RefreshSession: %v", err)
	}
	if next.RefreshToken != "rt-1" {
		t.Errorf("RefreshToken = %q, want 기존 rt-1 유지", next.RefreshToken)
	}
}

func TestRefreshSession_InvalidGrantMeansLoginRequired(t *testing.T) {
	// refresh token 만료·폐기(SSO Session Max 초과 등) — 재로그인만이 답이다.
	srv := newIdP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Token is not active"}`))
	})

	s := testSession()
	s.TokenEndpoint = srv.URL + "/token"
	_, err := RefreshSession(context.Background(), nil, s)
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("err = %v, want ErrLoginRequired", err)
	}
}

func TestRefreshSession_WithoutRefreshTokenIsLoginRequired(t *testing.T) {
	s := testSession()
	s.RefreshToken = ""
	if _, err := RefreshSession(context.Background(), nil, s); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("err = %v, want ErrLoginRequired", err)
	}
}

func TestEnsureFreshToken_PrefersEnvToken(t *testing.T) {
	// 무인 경로(NULLUS_TOKEN, bootstrap)가 최우선 — env 우선순위는 S-2 와 동일.
	t.Setenv(EnvConfigDir, t.TempDir())
	t.Setenv(EnvToken, "env-tok")

	expired := testSession()
	expired.Expiry = time.Now().Add(-time.Minute)
	if err := SaveSession(expired); err != nil {
		t.Fatal(err)
	}
	tok, err := EnsureFreshToken(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureFreshToken: %v", err)
	}
	if tok != "env-tok" {
		t.Errorf("tok = %q, want env-tok (만료 세션보다 env 가 먼저)", tok)
	}
}

func TestEnsureFreshToken_ValidSessionSkipsRefresh(t *testing.T) {
	t.Setenv(EnvConfigDir, t.TempDir())
	t.Setenv(EnvToken, "")
	srv := newIdP(t, nil) // 유효한 세션이면 token endpoint 를 건드리지 않는다

	s := testSession()
	s.TokenEndpoint = srv.URL + "/token"
	if err := SaveSession(s); err != nil {
		t.Fatal(err)
	}
	tok, err := EnsureFreshToken(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureFreshToken: %v", err)
	}
	if tok != "at-1" {
		t.Errorf("tok = %q", tok)
	}
}

func TestEnsureFreshToken_RefreshesExpiredSessionAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	t.Setenv(EnvToken, "")
	var calls atomic.Int32
	srv := newIdP(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		tokenJSON("at-2", "rt-2", 300)(w, r)
	})

	s := testSession()
	s.Expiry = time.Now().Add(-time.Minute)
	s.TokenEndpoint = srv.URL + "/token"
	if err := SaveSession(s); err != nil {
		t.Fatal(err)
	}

	tok, err := EnsureFreshToken(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureFreshToken: %v", err)
	}
	if tok != "at-2" {
		t.Errorf("tok = %q, want 갱신된 at-2", tok)
	}
	if calls.Load() != 1 {
		t.Errorf("token endpoint 호출 %d회", calls.Load())
	}
	// 갱신 결과는 세션·토큰 파일 양쪽에 남아야 다음 프로세스가 이어받는다.
	got, found, err := ReadSession()
	if err != nil || !found {
		t.Fatalf("ReadSession: found=%v err=%v", found, err)
	}
	if got.AccessToken != "at-2" || got.RefreshToken != "rt-2" {
		t.Errorf("영속된 세션 = %+v", got)
	}
	if fileTok, _ := ReadToken(); fileTok != "at-2" {
		t.Errorf("토큰 파일 = %q, want at-2", fileTok)
	}
}

func TestEnsureFreshToken_ExpiredWithoutRefreshIsLoginRequired(t *testing.T) {
	t.Setenv(EnvConfigDir, t.TempDir())
	t.Setenv(EnvToken, "")

	s := testSession()
	s.Expiry = time.Now().Add(-time.Minute)
	s.RefreshToken = ""
	if err := SaveSession(s); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureFreshToken(context.Background(), nil); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("err = %v, want ErrLoginRequired", err)
	}
}

func TestEnsureFreshToken_FallsBackToStaticTokenFile(t *testing.T) {
	// bootstrap issue 가 저장한 정적 토큰(세션 없음) — 만료 관리는 외부 책임.
	t.Setenv(EnvConfigDir, t.TempDir())
	t.Setenv(EnvToken, "")

	if err := SaveToken("boot-tok"); err != nil {
		t.Fatal(err)
	}
	tok, err := EnsureFreshToken(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureFreshToken: %v", err)
	}
	if tok != "boot-tok" {
		t.Errorf("tok = %q", tok)
	}
}

func TestEnsureFreshToken_NothingIsEmptyNotError(t *testing.T) {
	// dev 모드(auth.mode=session): 자격 부재는 오류가 아니다 — 토큰 없이 통과.
	t.Setenv(EnvConfigDir, t.TempDir())
	t.Setenv(EnvToken, "")

	tok, err := EnsureFreshToken(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureFreshToken: %v", err)
	}
	if tok != "" {
		t.Errorf("tok = %q, want empty", tok)
	}
}
