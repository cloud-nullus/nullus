package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type testUser struct {
	ID    string
	OrgID string
}

func TestRateLimiter_AuthenticatedUserLimit(t *testing.T) {
	e := echo.New()
	mw := newRateLimiter(rateLimitCategoryGeneral, RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 30,
		Login:           10,
		Deploy:          10,
	}, time.Now, 10*time.Second)

	h := mw(okHandler)
	for i := range 300 {
		rec := execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-1", OrgID: "o-1"}, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 at request %d, got %d", i+1, rec.Code)
		}
	}

	rec := execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-1", OrgID: "o-1"}, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 on 301st request, got %d", rec.Code)
	}
}

// development 모드는 인증 미들웨어를 아예 끄고 돌린다. 그래서 모든 요청이
// "미인증" 으로 분류되는데, 여기에 30/분 을 씌우면 폴링하는 화면 하나만 열어도
// 곧바로 429 가 난다. 로컬에서 인증이 없는 건 설계이지 익명 트래픽이 아니다.
func TestRateLimitConfigForMode_DevelopmentDoesNotThrottleTheLocalOperator(t *testing.T) {
	cfg := RateLimitConfigForMode("development", RateLimitConfig{})

	if cfg.Unauthenticated != cfg.Authenticated {
		t.Fatalf("development should treat every caller as the local operator: unauth=%d auth=%d",
			cfg.Unauthenticated, cfg.Authenticated)
	}
	if cfg.Unauthenticated < defaultAuthenticatedLimit {
		t.Fatalf("development limit must not be below the authenticated limit, got %d", cfg.Unauthenticated)
	}
}

func TestRateLimitConfigForMode_NonDevelopmentKeepsAnonymousLimitTight(t *testing.T) {
	for _, mode := range []string{"production", "staging", ""} {
		cfg := RateLimitConfigForMode(mode, RateLimitConfig{})

		if cfg.Unauthenticated != defaultUnauthenticatedLimit {
			t.Fatalf("mode %q: expected anonymous limit %d, got %d",
				mode, defaultUnauthenticatedLimit, cfg.Unauthenticated)
		}
		if cfg.Authenticated != defaultAuthenticatedLimit {
			t.Fatalf("mode %q: expected authenticated limit %d, got %d",
				mode, defaultAuthenticatedLimit, cfg.Authenticated)
		}
	}
}

// 전역 상한은 인증 미들웨어보다 먼저 도는 자리라 사용자를 알 수 없다. 그래서
// 사용자 정보가 우연히 있더라도 무시하고 항상 IP 로 센다 — 폭주 방어 전용이다.
func TestIPCeilingRateLimiter_AlwaysKeysByIPIgnoringUser(t *testing.T) {
	e := echo.New()
	mw := newRateLimiter(rateLimitCategoryIPCeiling, RateLimitConfig{
		Authenticated: 300, Unauthenticated: 30, IPCeiling: 5,
	}, time.Now, 10*time.Second)
	h := mw(okHandler)

	// 서로 다른 사용자라도 같은 IP 면 같은 버킷을 쓴다.
	for i := range 5 {
		rec := execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-" + strconv.Itoa(i)}, "203.0.113.9")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	rec := execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-other"}, "203.0.113.9")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once the IP ceiling is hit, got %d", rec.Code)
	}

	// 다른 IP 는 영향받지 않는다.
	rec = execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-0"}, "203.0.113.10")
	if rec.Code != http.StatusOK {
		t.Fatalf("a different IP must have its own bucket, got %d", rec.Code)
	}
}

// 이 테스트가 원래 버그를 고정한다: 전역 리미터가 인증보다 먼저 돌면 사용자를
// 못 봐서 인증 한도가 영영 안 걸렸다. 2단 구성에서는 인증 뒤에 붙은 리미터가
// 사용자 단위로 정확히 세야 한다.
func TestRateLimiterTiers_UserLimitCountsPerUserAfterAuth(t *testing.T) {
	e := echo.New()
	cfg := RateLimitConfig{Authenticated: 3, Unauthenticated: 30, IPCeiling: 1000}

	ceiling := newRateLimiter(rateLimitCategoryIPCeiling, cfg, time.Now, 10*time.Second)
	perUser := newRateLimiter(rateLimitCategoryGeneral, cfg, time.Now, 10*time.Second)

	// 실제 배선을 흉내낸다: 전역 상한 → (인증이 user 를 심음) → 사용자 리미터 → 핸들러.
	authStub := func(userID string) echo.MiddlewareFunc {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user", &testUser{ID: userID, OrgID: "o-1"})
				return next(c)
			}
		}
	}

	chainFor := func(userID string) echo.HandlerFunc {
		return ceiling(authStub(userID)(perUser(okHandler)))
	}

	// 같은 IP 를 공유해도 사용자별로 따로 센다.
	for i := range 3 {
		if rec := execRequest(t, e, chainFor("alice"), "/api/v1/stacks", "", nil, "198.51.100.7"); rec.Code != http.StatusOK {
			t.Fatalf("alice request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	if rec := execRequest(t, e, chainFor("alice"), "/api/v1/stacks", "", nil, "198.51.100.7"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("alice should be limited on her 4th request, got %d", rec.Code)
	}

	if rec := execRequest(t, e, chainFor("bob"), "/api/v1/stacks", "", nil, "198.51.100.7"); rec.Code != http.StatusOK {
		t.Fatalf("bob shares the IP but must have his own budget, got %d", rec.Code)
	}
}

func TestRateLimitConfigForMode_SetsIPCeilingAboveTheUserLimit(t *testing.T) {
	for _, mode := range []string{"development", "production"} {
		cfg := RateLimitConfigForMode(mode, RateLimitConfig{})
		if cfg.IPCeiling < cfg.Authenticated {
			t.Fatalf("mode %q: an IP ceiling below the per-user limit would throttle a single legitimate user: ceiling=%d user=%d",
				mode, cfg.IPCeiling, cfg.Authenticated)
		}
	}
}

func TestRateLimiter_UnauthenticatedLimit(t *testing.T) {
	e := echo.New()
	mw := newRateLimiter(rateLimitCategoryGeneral, RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 30,
		Login:           10,
		Deploy:          10,
	}, time.Now, 10*time.Second)

	h := mw(okHandler)
	for i := range 30 {
		rec := execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.1")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 at request %d, got %d", i+1, rec.Code)
		}
	}

	rec := execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 on 31st request, got %d", rec.Code)
	}
}

func TestRateLimiter_ExceededIncludesRetryAfterHeaderAndBody(t *testing.T) {
	e := echo.New()
	now := time.Unix(1_700_000_000, 0)
	nowFn := func() time.Time { return now }
	mw := newRateLimiter(rateLimitCategoryGeneral, RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 1,
		Login:           10,
		Deploy:          10,
	}, nowFn, 10*time.Second)

	h := mw(okHandler)
	rec := execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.9")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for first request, got %d", rec.Code)
	}

	rec = execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.9")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 for second request, got %d", rec.Code)
	}

	if rec.Header().Get(echo.HeaderRetryAfter) != "60" {
		t.Fatalf("expected Retry-After=60, got %q", rec.Header().Get(echo.HeaderRetryAfter))
	}

	var body struct {
		Error struct {
			Code       string `json:"code"`
			Message    string `json:"message"`
			RetryAfter int64  `json:"retry_after"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Error.Code != "RATE_LIMITED" {
		t.Fatalf("expected error code RATE_LIMITED, got %q", body.Error.Code)
	}
	if body.Error.RetryAfter != 60 {
		t.Fatalf("expected retry_after=60, got %d", body.Error.RetryAfter)
	}
}

func TestRateLimiter_HeadersPresent(t *testing.T) {
	e := echo.New()
	mw := newRateLimiter(rateLimitCategoryGeneral, RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 3,
		Login:           10,
		Deploy:          10,
	}, time.Now, 10*time.Second)

	h := mw(okHandler)
	rec := execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.2")

	if rec.Header().Get("X-RateLimit-Limit") != "3" {
		t.Fatalf("expected X-RateLimit-Limit=3, got %q", rec.Header().Get("X-RateLimit-Limit"))
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "2" {
		t.Fatalf("expected X-RateLimit-Remaining=2, got %q", rec.Header().Get("X-RateLimit-Remaining"))
	}

	reset := rec.Header().Get("X-RateLimit-Reset")
	if reset == "" {
		t.Fatal("expected X-RateLimit-Reset header")
	}
	if _, err := strconv.ParseInt(reset, 10, 64); err != nil {
		t.Fatalf("expected valid unix timestamp in X-RateLimit-Reset, got %q", reset)
	}
}

func TestRateLimiter_DifferentUsersIndependentCounters(t *testing.T) {
	e := echo.New()
	mw := newRateLimiter(rateLimitCategoryGeneral, RateLimitConfig{
		Authenticated:   2,
		Unauthenticated: 30,
		Login:           10,
		Deploy:          10,
	}, time.Now, 10*time.Second)

	h := mw(okHandler)
	rec := execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-1", OrgID: "o-1"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for user u-1 first request, got %d", rec.Code)
	}
	rec = execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-1", OrgID: "o-1"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for user u-1 second request, got %d", rec.Code)
	}
	rec = execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-1", OrgID: "o-1"}, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 for user u-1 third request, got %d", rec.Code)
	}

	rec = execRequest(t, e, h, "/api/v1/stacks", "", &testUser{ID: "u-2", OrgID: "o-1"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for user u-2 first request, got %d", rec.Code)
	}
}

func TestRateLimiter_WindowResetsAfterExpiry(t *testing.T) {
	e := echo.New()
	now := time.Unix(1_700_000_000, 0)
	var mu sync.Mutex
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	mw := newRateLimiter(rateLimitCategoryGeneral, RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 1,
		Login:           10,
		Deploy:          10,
	}, nowFn, 10*time.Second)

	h := mw(okHandler)
	rec := execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.3")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for first request, got %d", rec.Code)
	}

	rec = execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.3")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 before reset, got %d", rec.Code)
	}

	mu.Lock()
	now = now.Add(time.Minute + time.Second)
	mu.Unlock()

	rec = execRequest(t, e, h, "/api/v1/stacks", "", nil, "203.0.113.3")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 after reset, got %d", rec.Code)
	}
}

func TestDeployRateLimiter_EnforcesHourlyLimit(t *testing.T) {
	e := echo.New()
	mw := newRateLimiter(rateLimitCategoryDeploy, RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 30,
		Login:           10,
		Deploy:          10,
	}, time.Now, 10*time.Second)

	h := mw(okHandler)
	for i := range 10 {
		rec := execRequest(t, e, h, "/api/v1/stacks/s-1/deploy", "", &testUser{ID: "u-1", OrgID: "org-a"}, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 at deploy request %d, got %d", i+1, rec.Code)
		}
	}

	rec := execRequest(t, e, h, "/api/v1/stacks/s-1/deploy", "", &testUser{ID: "u-1", OrgID: "org-a"}, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 on 11th deploy request, got %d", rec.Code)
	}
}

func TestLoginRateLimiter_UsesLoginLimit(t *testing.T) {
	e := echo.New()
	mw := newRateLimiter(rateLimitCategoryLogin, RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 30,
		Login:           2,
		Deploy:          10,
	}, time.Now, 10*time.Second)

	h := mw(okHandler)
	rec := execRequest(t, e, h, "/api/v1/auth/login", "", nil, "203.0.113.4")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on login request 1, got %d", rec.Code)
	}
	rec = execRequest(t, e, h, "/api/v1/auth/login", "", nil, "203.0.113.4")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on login request 2, got %d", rec.Code)
	}
	rec = execRequest(t, e, h, "/api/v1/auth/login", "", nil, "203.0.113.4")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 on login request 3, got %d", rec.Code)
	}
}

func okHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func execRequest(t *testing.T, e *echo.Echo, h echo.HandlerFunc, path, method string, user any, realIP string) *httptest.ResponseRecorder {
	t.Helper()

	if method == "" {
		method = http.MethodGet
	}

	req := httptest.NewRequest(method, path, nil)
	if realIP != "" {
		req.Header.Set(echo.HeaderXForwardedFor, realIP)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		c.Set("user", user)
	}

	if err := h(c); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return rec
}

// 한도는 설정(server.rate_limit.*)으로 덮을 수 있다. e2e 처럼 한 IP 에서 여러 화면을 동시에 여는
// 곳은 개발 모드의 분당 300 을 넘는다(#360 이 화면마다 어시스턴트 상태 조회를 더하자 넘었다).
func TestRateLimitConfigForMode_OverridesReplaceDefaults(t *testing.T) {
	cfg := RateLimitConfigForMode("production", RateLimitConfig{Authenticated: 1000, IPCeiling: 3000})

	if cfg.Authenticated != 1000 || cfg.IPCeiling != 3000 {
		t.Fatalf("overrides should replace the defaults: auth=%d ceiling=%d", cfg.Authenticated, cfg.IPCeiling)
	}
	if cfg.Unauthenticated != defaultUnauthenticatedLimit {
		t.Fatalf("an unset override must keep the default anonymous limit, got %d", cfg.Unauthenticated)
	}
}

// development 는 모든 호출을 로컬 운영자로 본다 — 사용자 한도를 올렸으면 미인증 한도도 따라간다.
func TestRateLimitConfigForMode_DevelopmentFollowsOverriddenAuthenticatedLimit(t *testing.T) {
	cfg := RateLimitConfigForMode("development", RateLimitConfig{Authenticated: 5000})

	if cfg.Unauthenticated != 5000 {
		t.Fatalf("development anonymous limit should follow the overridden user limit, got %d", cfg.Unauthenticated)
	}
	if cfg.IPCeiling < cfg.Authenticated {
		t.Fatalf("raising the user limit must not leave the IP ceiling below it: ceiling=%d user=%d", cfg.IPCeiling, cfg.Authenticated)
	}
}

func TestRateLimitConfigForMode_ExplicitUnauthenticatedWinsInDevelopment(t *testing.T) {
	cfg := RateLimitConfigForMode("development", RateLimitConfig{Authenticated: 5000, Unauthenticated: 50})

	if cfg.Unauthenticated != 50 {
		t.Fatalf("an explicit anonymous limit should win over the development rule, got %d", cfg.Unauthenticated)
	}
}

// 명시한 IP 상한은 그대로 둔다 — 낮게 잡은 것도 운영자의 선택이다.
func TestRateLimitConfigForMode_ExplicitIPCeilingIsKept(t *testing.T) {
	cfg := RateLimitConfigForMode("production", RateLimitConfig{Authenticated: 5000, IPCeiling: 400})

	if cfg.IPCeiling != 400 {
		t.Fatalf("an explicit IP ceiling should be kept, got %d", cfg.IPCeiling)
	}
}

func TestRateLimitConfigForMode_IgnoresNonPositiveOverrides(t *testing.T) {
	cfg := RateLimitConfigForMode("production", RateLimitConfig{Authenticated: -1, Unauthenticated: 0, IPCeiling: -5})

	if cfg.Authenticated != defaultAuthenticatedLimit || cfg.Unauthenticated != defaultUnauthenticatedLimit || cfg.IPCeiling != defaultIPCeilingLimit {
		t.Fatalf("non-positive overrides should keep the defaults, got %+v", cfg)
	}
}

// 덮은 값이 없으면 예전과 같다 — 운영 기본값이 이 변경으로 바뀌지 않았다.
func TestRateLimitConfigForMode_NoOverridesKeepsPreviousDefaults(t *testing.T) {
	want := map[string]RateLimitConfig{
		"production":  {Authenticated: 300, Unauthenticated: 30, Login: 10, Deploy: 10, IPCeiling: 600},
		"development": {Authenticated: 300, Unauthenticated: 300, Login: 10, Deploy: 10, IPCeiling: 600},
	}
	for mode, w := range want {
		if got := RateLimitConfigForMode(mode, RateLimitConfig{}); got != w {
			t.Fatalf("mode %q: got %+v, want %+v", mode, got, w)
		}
	}
}

func TestRateLimitConfigForMode_LiftedIPCeilingIsTwiceTheUserLimit(t *testing.T) {
	if got := RateLimitConfigForMode("production", RateLimitConfig{Authenticated: 5000}).IPCeiling; got != 10000 {
		t.Fatalf("expected the IP ceiling lifted to 2x the user limit, got %d", got)
	}
	// 사용자 한도를 낮추면 상한은 기본값에 남는다 — 낮추는 쪽으로는 따라가지 않는다.
	if got := RateLimitConfigForMode("production", RateLimitConfig{Authenticated: 100}).IPCeiling; got != defaultIPCeilingLimit {
		t.Fatalf("lowering the user limit should keep the default IP ceiling, got %d", got)
	}
}

// 미인증 한도만 올려도 IP 상한이 그 아래에 남으면 올린 값이 먹지 않는다.
func TestRateLimitConfigForMode_LiftedIPCeilingCoversTheAnonymousLimit(t *testing.T) {
	cfg := RateLimitConfigForMode("production", RateLimitConfig{Unauthenticated: 1000})

	if cfg.IPCeiling < cfg.Unauthenticated {
		t.Fatalf("the IP ceiling must not stay below a raised anonymous limit: ceiling=%d anon=%d", cfg.IPCeiling, cfg.Unauthenticated)
	}
}

// 로그인 · 배포 한도는 보안 한도라 설정으로 덮지 않는다.
func TestRateLimitConfigForMode_LoginAndDeployAreNotOverridable(t *testing.T) {
	cfg := RateLimitConfigForMode("production", RateLimitConfig{Login: 1000, Deploy: 1000})

	if cfg.Login != defaultLoginLimit || cfg.Deploy != defaultDeployLimit {
		t.Fatalf("login/deploy limits must stay at their defaults, got login=%d deploy=%d", cfg.Login, cfg.Deploy)
	}
}

func TestRateLimitConfig_CeilingBelowUserLimit(t *testing.T) {
	if !(RateLimitConfig{Authenticated: 5000, IPCeiling: 400}).CeilingBelowUserLimit() {
		t.Fatal("a ceiling below the user limit should be reported")
	}
	if (RateLimitConfig{Authenticated: 300, Unauthenticated: 30, IPCeiling: 600}).CeilingBelowUserLimit() {
		t.Fatal("the defaults should not be reported")
	}
}
