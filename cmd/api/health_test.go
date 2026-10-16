package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func TestHealthHandler_ReturnsCurrentVersion(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := healthHandler(&mockPinger{err: nil})
	if err := handler(c); err != nil {
		t.Fatalf("healthHandler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("status = %q, want healthy", resp["status"])
	}
	if resp["db"] != "connected" {
		t.Errorf("db = %q, want connected", resp["db"])
	}
	if resp["version"] != version {
		t.Errorf("version = %q, want %q", resp["version"], version)
	}
}

func TestHealthHandler_ReflectsInjectedVersion(t *testing.T) {
	origVersion := version
	defer func() { version = origVersion }()

	version = "v0.5.0"

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := healthHandler(&mockPinger{err: nil})
	if err := handler(c); err != nil {
		t.Fatalf("healthHandler returned error: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if resp["version"] != "v0.5.0" {
		t.Errorf("version = %q, want v0.5.0", resp["version"])
	}
}

func TestHealthHandler_DBUnavailableStillReportsVersion(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := healthHandler(&mockPinger{err: errors.New("connection refused")})
	if err := handler(c); err != nil {
		t.Fatalf("healthHandler returned error: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if resp["db"] != "unavailable" {
		t.Errorf("db = %q, want unavailable", resp["db"])
	}
	if resp["version"] != version {
		t.Errorf("version = %q, want %q", resp["version"], version)
	}
}

func TestDefaultVersionIsDev(t *testing.T) {
	// ldflags 주입이 없는 일반 빌드에서는 "dev" 여야 한다.
	// 하드코딩된 "0.1.0-alpha" 가 남아 있으면 안 된다.
	if version == "0.1.0-alpha" {
		t.Errorf("version is still hardcoded to 0.1.0-alpha; want default dev")
	}
}
