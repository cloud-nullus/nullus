package nullusclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// S-1 완료 기준: 인증 헤더 자동 첨부, 에러 매핑(automation 계약 정합), 재시도 없음.

func TestClient_Do_AttachesBearerTokenAndDecodesJSON(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"stk_1","name":"team-alpha"}`))
	}))
	defer srv.Close()

	c, err := New(Config{Server: srv.URL, Token: "tok-123"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.Do(context.Background(), http.MethodGet, "/api/v1/stacks/stk_1", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want Bearer tok-123", gotAuth)
	}
	if out.ID != "stk_1" || out.Name != "team-alpha" {
		t.Errorf("decoded = %+v", out)
	}
}

func TestClient_Do_NoTokenSendsNoAuthHeader(t *testing.T) {
	// dev 모드(auth.mode=session)는 토큰 없이 동작해야 한다 — 컨셉 문서 §5.
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := New(Config{Server: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Do(context.Background(), http.MethodGet, "/api/v1/stacks", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty", gotAuth)
	}
}

func TestClient_Do_SendsJSONBody(t *testing.T) {
	var gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"stk_9"}`))
	}))
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	in := map[string]string{"name": "team-a"}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.Do(context.Background(), http.MethodPost, "/api/v1/stacks", in, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if gotBody != `{"name":"team-a"}` {
		t.Errorf("body = %q", gotBody)
	}
	if out.ID != "stk_9" {
		t.Errorf("out = %+v", out)
	}
}

func TestClient_Do_MapsStatusToErrorKind(t *testing.T) {
	// Automation 계약 §1: 400/409→Usage(2), 401/403→Auth(3), 404→NotFound(4), 5xx→Server(5).
	// 응답 형태는 실서버(internal/shared/middleware/error_handler.go)의
	// {"error": {code, http_status, message, ..., trace_id}} 중첩 envelope 그대로다.
	cases := []struct {
		status int
		kind   Kind
	}{
		{http.StatusBadRequest, KindUsage},
		{http.StatusConflict, KindUsage},
		{http.StatusUnprocessableEntity, KindUsage},
		{http.StatusUnauthorized, KindAuth},
		{http.StatusForbidden, KindAuth},
		{http.StatusNotFound, KindNotFound},
		{http.StatusInternalServerError, KindServer},
		{http.StatusBadGateway, KindServer},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"error":{"code":"SOME_CODE","http_status":` +
				fmt.Sprint(tc.status) + `,"message":"boom","retryable":false,"trace_id":"req-42"}}`))
		}))
		c, _ := New(Config{Server: srv.URL})
		err := c.Do(context.Background(), http.MethodGet, "/api/v1/x", nil, nil)
		srv.Close()

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("status %d: err = %v, want *APIError", tc.status, err)
		}
		if apiErr.Kind != tc.kind {
			t.Errorf("status %d: Kind = %v, want %v", tc.status, apiErr.Kind, tc.kind)
		}
		if apiErr.StatusCode != tc.status {
			t.Errorf("status %d: StatusCode = %d", tc.status, apiErr.StatusCode)
		}
		if apiErr.Message != "boom" {
			t.Errorf("status %d: Message = %q, want envelope 의 message", tc.status, apiErr.Message)
		}
		if apiErr.Code != "SOME_CODE" {
			t.Errorf("status %d: Code = %q, want envelope 의 code", tc.status, apiErr.Code)
		}
		if apiErr.TraceID != "req-42" {
			t.Errorf("status %d: TraceID = %q, want envelope 의 trace_id", tc.status, apiErr.TraceID)
		}
	}
}

func TestClient_Do_ErrorCodeDistinguishesCompatWarn(t *testing.T) {
	// A-5 의 --ack-warnings UX 는 같은 400 이라도 DEPLOY_COMPAT_WARN_UNACK 를
	// 구분해야 한다 — Kind 만으로는 부족하고 Code 가 살아 있어야 한다.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"DEPLOY_COMPAT_WARN_UNACK","http_status":400,"message":"acknowledge warnings to proceed","trace_id":"req-7"}}`))
	}))
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	err := c.Do(context.Background(), http.MethodPost, "/api/v1/stacks/s/deploy", nil, nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Code != "DEPLOY_COMPAT_WARN_UNACK" {
		t.Errorf("Code = %q", apiErr.Code)
	}
	if apiErr.Kind != KindUsage {
		t.Errorf("Kind = %v, want KindUsage", apiErr.Kind)
	}
}

func TestClient_Do_FallsBackToTopLevelMessage(t *testing.T) {
	// 미들웨어를 거치지 않는 응답(구버전·프록시 등)은 top-level message 만 온다.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"plain not found"}`))
	}))
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	err := c.Do(context.Background(), http.MethodGet, "/api/v1/x", nil, nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Message != "plain not found" {
		t.Errorf("Message = %q", apiErr.Message)
	}
	if apiErr.Code != "" {
		t.Errorf("Code = %q, want empty", apiErr.Code)
	}
}

func TestClient_Do_ExitCodesMatchAutomationContract(t *testing.T) {
	cases := []struct {
		kind Kind
		code int
	}{
		{KindUsage, 2},
		{KindAuth, 3},
		{KindNotFound, 4},
		{KindServer, 5},
	}
	for _, tc := range cases {
		if got := tc.kind.ExitCode(); got != tc.code {
			t.Errorf("%v.ExitCode() = %d, want %d", tc.kind, got, tc.code)
		}
	}
}

func TestClient_Do_DoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	_ = c.Do(context.Background(), http.MethodGet, "/api/v1/x", nil, nil)
	if n := calls.Load(); n != 1 {
		t.Errorf("호출 %d회 — 재시도 없음 원칙 위반", n)
	}
}

func TestClient_Do_ConnectionFailureIsServerKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // 즉시 닫아 연결 실패 유도

	c, _ := New(Config{Server: srv.URL})
	err := c.Do(context.Background(), http.MethodGet, "/api/v1/x", nil, nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Kind != KindServer {
		t.Errorf("Kind = %v, want KindServer (연결 실패→5)", apiErr.Kind)
	}
	if apiErr.Unwrap() == nil {
		t.Error("원인 에러가 Unwrap 으로 노출되어야 한다")
	}
}

func TestNew_RequiresServer(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("New: 서버 미설정을 거부해야 한다")
	}
}
