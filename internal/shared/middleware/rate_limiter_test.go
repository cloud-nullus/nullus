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
