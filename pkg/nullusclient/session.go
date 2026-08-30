package nullusclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const sessionFileName = "session"

// Session 은 nullus login(OIDC)이 남기는 로그인 상태다. access token 외에
// refresh 에 필요한 재료(refresh token, token endpoint, client ID)까지 담아
// 다음 프로세스가 discovery 없이 갱신을 이어받는다.
//
// bootstrap 토큰(무인 경로)은 세션을 만들지 않는다 — 그쪽은 S-2 의 토큰
// 파일만 쓰고 만료 관리는 외부(발급 스크립트) 책임이다.
type Session struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	Expiry        time.Time `json:"expiry"` // access token 만료 시각
	Issuer        string    `json:"issuer"`
	ClientID      string    `json:"client_id"`
	TokenEndpoint string    `json:"token_endpoint"`
}

// SaveSession 은 세션을 ~/.nullus/session 에 0600 으로 저장하고, access token
// 을 S-2 토큰 파일에도 함께 기록한다 — ReadToken/Load 만 아는 소비자(MCP 설계
// §5 의 단일 토큰 캐시 경로)가 로그인 결과를 그대로 보게 하기 위해서다.
func SaveSession(s Session) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("설정 디렉토리 생성: %w", err)
	}
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("세션 직렬화: %w", err)
	}
	path := filepath.Join(dir, sessionFileName)
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("세션 저장: %w", err)
	}
	// WriteFile 의 perm 은 파일이 이미 있으면 적용되지 않는다.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("세션 파일 권한: %w", err)
	}
	return SaveToken(s.AccessToken)
}

// ReadSession 은 저장된 세션을 읽는다. 부재는 (zero, false, nil) — 로그인
// 안내의 재료이지 오류가 아니다.
func ReadSession() (Session, bool, error) {
	dir, err := configDir()
	if err != nil {
		return Session{}, false, err
	}
	path := filepath.Join(dir, sessionFileName)
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	// refresh token 은 access token 보다 수명이 길어 노출 시 피해가 크다 —
	// 토큰 파일과 같은 규율로 거부한다 (ADR-0001 토큰 캐시 보안).
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return Session{}, false, fmt.Errorf("세션 파일 %s 의 권한이 %04o 다 — 0600 이어야 한다. 노출됐을 수 있으니 재로그인으로 세션을 회전하라", path, perm)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Session{}, false, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return Session{}, false, fmt.Errorf("세션 파일 파싱: %w", err)
	}
	return s, true, nil
}

// DeleteSession 은 로컬 자격(세션 + 토큰 파일)을 지운다. logout(A-3)용 —
// 지울 것이 없어도 오류가 아니다(멱등).
func DeleteSession() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	for _, name := range []string{sessionFileName, tokenFileName} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s 삭제: %w", name, err)
		}
	}
	return nil
}
