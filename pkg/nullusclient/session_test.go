package nullusclient

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// S-3 저장 계층: 로그인 세션(refresh token 포함)도 토큰 파일과 같은 0600 규율을
// 따른다 — ADR-0001 "토큰 캐시 보안 명시" 항목.

func testSession() Session {
	return Session{
		AccessToken:   "at-1",
		RefreshToken:  "rt-1",
		Expiry:        time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		Issuer:        "https://idp.example.com/realms/nullus",
		ClientID:      "nullus-cli",
		TokenEndpoint: "https://idp.example.com/realms/nullus/protocol/openid-connect/token",
	}
}

func TestSaveSession_WritesSessionAndTokenWith0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	if err := SaveSession(testSession()); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	for _, name := range []string{sessionFileName, tokenFileName} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s 권한 = %04o, want 0600", name, perm)
		}
	}
	// S-2 경로 동기화: 로그인 직후에도 ReadToken(토큰 파일)으로 access token 이
	// 잡혀야 한다 — MCP 설계 §5 의 "~/.nullus/ 토큰 캐시 (S-2·S-3)" 단일 경로.
	tok, err := ReadToken()
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if tok != "at-1" {
		t.Errorf("ReadToken = %q, want 로그인 access token", tok)
	}
}

func TestReadSession_RoundTrip(t *testing.T) {
	t.Setenv(EnvConfigDir, t.TempDir())

	want := testSession()
	if err := SaveSession(want); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	got, found, err := ReadSession()
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if !found {
		t.Fatal("found = false")
	}
	if got != want {
		t.Errorf("got = %+v\nwant = %+v", got, want)
	}
}

func TestReadSession_MissingIsNotError(t *testing.T) {
	// 세션 부재는 로그인 안내의 재료이지 오류가 아니다 — S-2 토큰 파일과 동일.
	t.Setenv(EnvConfigDir, t.TempDir())

	_, found, err := ReadSession()
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
}

func TestReadSession_RejectsLoosePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	if err := SaveSession(testSession()); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if err := os.Chmod(filepath.Join(dir, sessionFileName), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadSession(); err == nil {
		t.Fatal("group/other 가 읽을 수 있는 세션 파일을 거부해야 한다")
	}
}

func TestDeleteSession_RemovesLocalCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	if err := SaveSession(testSession()); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if err := DeleteSession(); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, found, _ := ReadSession(); found {
		t.Error("세션이 남아 있다")
	}
	if tok, _ := ReadToken(); tok != "" {
		t.Errorf("토큰 파일이 남아 있다: %q", tok)
	}
	// 지울 것이 없어도 오류가 아니다 — logout 멱등성.
	if err := DeleteSession(); err != nil {
		t.Errorf("빈 상태 DeleteSession: %v", err)
	}
}
