package secrets

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// 정적 토큰 전략은 로컬 개발 전용이며 항상 같은 값을 돌려준다.
func TestStaticTokenProvider_ReturnsConfiguredToken(t *testing.T) {
	p := NewStaticTokenProvider("root")
	got, err := p.Token(context.Background())
	if err != nil {
		t.Fatalf("Token 실패: %v", err)
	}
	if got != "root" {
		t.Fatalf("expected root, got %q", got)
	}
}

// Kubernetes Auth 전략은 발급받은 client_token 을 캐시하고,
// TTL 이 남아 있으면 재로그인하지 않는다.
func TestKubernetesTokenProvider_CachesUntilExpiry(t *testing.T) {
	var logins int32
	p := &KubernetesTokenProvider{
		login: func(_ context.Context) (string, time.Duration, error) {
			atomic.AddInt32(&logins, 1)
			return "s.client-token", time.Hour, nil
		},
	}

	for i := 0; i < 3; i++ {
		got, err := p.Token(context.Background())
		if err != nil {
			t.Fatalf("Token 실패: %v", err)
		}
		if got != "s.client-token" {
			t.Fatalf("expected client token, got %q", got)
		}
	}

	if n := atomic.LoadInt32(&logins); n != 1 {
		t.Fatalf("로그인이 캐시되지 않음: %d회 호출", n)
	}
}

// 만료가 임박하면 다시 로그인해야 한다.
// 갱신 여유(renewBefore)보다 짧은 TTL 은 즉시 만료로 취급한다.
func TestKubernetesTokenProvider_RefreshesBeforeExpiry(t *testing.T) {
	var logins int32
	p := &KubernetesTokenProvider{
		login: func(_ context.Context) (string, time.Duration, error) {
			n := atomic.AddInt32(&logins, 1)
			if n == 1 {
				// 갱신 여유(10분)보다 짧으므로 캐시되지 않아야 한다.
				return "s.short", 1 * time.Minute, nil
			}
			return "s.fresh", time.Hour, nil
		},
	}

	if _, err := p.Token(context.Background()); err != nil {
		t.Fatalf("첫 Token 실패: %v", err)
	}
	got, err := p.Token(context.Background())
	if err != nil {
		t.Fatalf("두번째 Token 실패: %v", err)
	}
	if got != "s.fresh" {
		t.Fatalf("만료 임박 토큰이 갱신되지 않음: %q", got)
	}
	if n := atomic.LoadInt32(&logins); n != 2 {
		t.Fatalf("예상 로그인 2회, 실제 %d회", n)
	}
}

// 로그인 실패는 그대로 전파되어야 한다 — 조용히 빈 토큰을 쓰면
// 이후 요청이 403 으로 실패하고 원인 추적이 어려워진다.
func TestKubernetesTokenProvider_PropagatesLoginError(t *testing.T) {
	sentinel := errors.New("login failed")
	p := &KubernetesTokenProvider{
		login: func(_ context.Context) (string, time.Duration, error) {
			return "", 0, sentinel
		},
	}

	if _, err := p.Token(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("로그인 오류가 전파되지 않음: %v", err)
	}
}
