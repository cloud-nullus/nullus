package secrets

import (
	"context"
	"sync"
	"time"
)

// renewBefore 는 client_token 만료 전에 미리 재로그인하는 여유 시간이다.
// TTL 이 1시간이므로 10분 여유면 갱신 실패 시에도 재시도할 시간이 남는다.
const renewBefore = 10 * time.Minute

// TokenProvider 는 OpenBao 요청에 사용할 토큰을 공급한다.
//
// 구현이 두 가지인 이유는 실행 위치가 다르기 때문이다.
//   - StaticTokenProvider: 로컬 개발 전용. .env 의 고정 토큰을 그대로 쓴다
//   - KubernetesTokenProvider: 운영. Kubernetes Auth 로 단기 자격을 발급받는다
//
// PRD 5.2 는 "정적 토큰 하드코딩 금지, Kubernetes auth 기반 short-lived token
// 사용을 기본으로 한다" 고 규정한다. 정적 전략은 로컬 예외로만 허용된다.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// StaticTokenProvider 는 고정 토큰을 돌려준다. 로컬 개발 전용이다.
type StaticTokenProvider struct {
	token string
}

func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{token: token}
}

func (p *StaticTokenProvider) Token(context.Context) (string, error) {
	return p.token, nil
}

// KubernetesTokenProvider 는 Kubernetes Auth 로 로그인해 얻은 client_token 을
// 캐시하고, 만료가 임박하면 다시 로그인한다.
//
// login 은 (토큰, TTL, 오류) 를 돌려준다. 테스트에서 교체할 수 있도록
// 필드로 주입한다.
type KubernetesTokenProvider struct {
	login func(ctx context.Context) (string, time.Duration, error)

	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

// NewKubernetesTokenProvider 는 주어진 로그인 함수로 provider 를 만든다.
func NewKubernetesTokenProvider(login func(ctx context.Context) (string, time.Duration, error)) *KubernetesTokenProvider {
	return &KubernetesTokenProvider{login: login}
}

func (p *KubernetesTokenProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cached != "" && time.Now().Before(p.expiresAt) {
		return p.cached, nil
	}

	token, ttl, err := p.login(ctx)
	if err != nil {
		// 조용히 빈 토큰을 쓰면 이후 요청이 403 으로 실패하고
		// 원인 추적이 어려워지므로 그대로 전파한다.
		return "", err
	}

	p.cached = token
	// 갱신 여유보다 TTL 이 짧으면 캐시하지 않는다 (다음 호출에서 재로그인).
	if ttl > renewBefore {
		p.expiresAt = time.Now().Add(ttl - renewBefore)
	} else {
		p.expiresAt = time.Time{}
	}
	return token, nil
}
