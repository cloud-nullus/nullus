// nullus-bootstrap 은 무인 설치용 부트스트랩 자격을 발급하고 폐기한다.
//
// 에어갭 무인 설치는 Admin API 를 호출해야 하는데 그 시점에 로그인할 사람이
// 없다. 이 CLI 가 Keycloak service account 를 잠깐 만들어 토큰을 발급하고,
// 설치가 끝나면 클라이언트를 삭제한다.
//
//	nullus-bootstrap issue    # 클라이언트 보장 + 토큰 출력 (stdout)
//	nullus-bootstrap revoke   # 클라이언트 삭제
//
// 정책은 "폐기 + 멱등 재발급"이다. issue 는 여러 번 호출해도 안전하며
// 매번 secret 을 회전한다.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
)

const defaultBootstrapClientID = "nullus-bootstrap"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	kcURL := strings.TrimSpace(os.Getenv("KEYCLOAK_URL"))
	if kcURL == "" {
		fatal("KEYCLOAK_URL 이 필요합니다")
	}
	client := keycloak.NewKeycloakClient(
		kcURL,
		envOr("KEYCLOAK_REALM", "nullus"),
		envOr("KEYCLOAK_ADMIN_USER", "admin"),
		os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
	)
	clientID := envOr("BOOTSTRAP_CLIENT_ID", defaultBootstrapClientID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch os.Args[1] {
	case "issue":
		secret, err := randomSecret()
		if err != nil {
			fatal("secret 생성 실패: %v", err)
		}
		if _, err := client.EnsureBootstrapClient(ctx, keycloak.BootstrapClientSpec{
			ClientID: clientID,
			Secret:   secret,
			Roles:    []string{envOr("BOOTSTRAP_ROLE", "admin")},
		}); err != nil {
			fatal("부트스트랩 클라이언트 준비 실패: %v", err)
		}
		token, err := client.IssueBootstrapToken(ctx, clientID, secret)
		if err != nil {
			fatal("토큰 발급 실패: %v", err)
		}
		// 토큰만 stdout 으로 내보내 스크립트가 그대로 받을 수 있게 한다.
		fmt.Println(token)

	case "revoke":
		if err := client.RevokeBootstrapClient(ctx, clientID); err != nil {
			fatal("부트스트랩 클라이언트 폐기 실패: %v", err)
		}
		fmt.Fprintln(os.Stderr, "부트스트랩 자격을 폐기했습니다")

	default:
		usage()
		os.Exit(2)
	}
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: nullus-bootstrap <issue|revoke>")
	fmt.Fprintln(os.Stderr, "env: KEYCLOAK_URL, KEYCLOAK_REALM, KEYCLOAK_ADMIN_USER, KEYCLOAK_ADMIN_PASSWORD")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ERR ] "+format+"\n", args...)
	os.Exit(1)
}
