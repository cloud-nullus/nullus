// Package nullusclient 는 nullus CLI(트랙 A)와 MCP 서버(트랙 B)가 공유하는
// 유일한 기반이다 — Nullus API 클라이언트와 설정·토큰 해석.
//
// 두 트랙은 이 패키지에만 의존하고 서로에게 의존하지 않는다
// (CLI+MCP 구현 백로그 Phase 1). 여기에 명령 표면이나 tool 표면을 넣지 않는다.
package nullusclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// 환경변수 이름. env 는 설정 파일보다 우선한다 — CI 에서 파일 없이 실행하기
// 위한 경로다 (Automation 계약 §5).
const (
	EnvServer = "NULLUS_SERVER"
	EnvToken  = "NULLUS_TOKEN"
	// EnvConfigDir 은 ~/.nullus 를 통째로 옮긴다. 테스트 격리용이면서,
	// 멀티 서버 컨텍스트(v2)가 들어올 자리이기도 하다.
	EnvConfigDir = "NULLUS_CONFIG_DIR"
)

const (
	configFileName = "config"
	tokenFileName  = "token"
)

// Config 는 해석이 끝난 클라이언트 설정이다.
type Config struct {
	Server string // API 서버 base URL (예: https://nullus.example.com)
	Token  string
}

// Load 는 우선순위 명시 값(플래그) > NULLUS_* env > ~/.nullus/ 파일로 설정을
// 모은다. 값이 하나도 없어도 오류가 아니다 — 서버 필수 검증은 New 가 한다
// (dev 모드는 토큰 없이도 동작해야 하므로 토큰은 어디서도 강제하지 않는다).
func Load(explicit Config) (Config, error) {
	cfg := explicit

	if cfg.Server == "" {
		cfg.Server = os.Getenv(EnvServer)
	}
	if cfg.Token == "" {
		cfg.Token = os.Getenv(EnvToken)
	}

	if cfg.Server == "" {
		server, err := readConfigFileServer()
		if err != nil {
			return Config{}, err
		}
		cfg.Server = server
	}
	if cfg.Token == "" {
		token, err := readTokenFile()
		if err != nil {
			return Config{}, err
		}
		cfg.Token = token
	}
	return cfg, nil
}

// ReadToken 은 env → 토큰 파일 순으로 토큰을 찾는다. 파일이 없으면 빈 문자열을
// 반환한다 — 토큰 부재는 로그인 안내의 재료이지 오류가 아니다.
func ReadToken() (string, error) {
	if tok := os.Getenv(EnvToken); tok != "" {
		return tok, nil
	}
	return readTokenFile()
}

// SaveToken 은 토큰을 ~/.nullus/token 에 0600 으로 저장한다. 디렉토리가 없으면
// 0700 으로 만든다. 이미 있던 파일도 권한을 0600 으로 되돌린다.
func SaveToken(token string) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("설정 디렉토리 생성: %w", err)
	}
	path := filepath.Join(dir, tokenFileName)
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return fmt.Errorf("토큰 저장: %w", err)
	}
	// WriteFile 의 perm 은 파일이 이미 있으면 적용되지 않는다.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("토큰 파일 권한: %w", err)
	}
	return nil
}

func readTokenFile() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, tokenFileName)
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	// group/other 가 읽을 수 있는 토큰 파일은 쓰지 않는다. 자동으로 고쳐 주지도
	// 않는다 — 이미 노출된 뒤일 수 있으므로 사용자가 회전을 판단해야 한다.
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return "", fmt.Errorf("토큰 파일 %s 의 권한이 %04o 다 — 0600 이어야 한다. 노출됐을 수 있으니 토큰 회전을 검토하라", path, perm)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func readConfigFileServer() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(dir, configFileName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var f struct {
		Server string `yaml:"server"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return "", fmt.Errorf("설정 파일 파싱: %w", err)
	}
	return f.Server, nil
}

func configDir() (string, error) {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("홈 디렉토리 확인: %w", err)
	}
	return filepath.Join(home, ".nullus"), nil
}
