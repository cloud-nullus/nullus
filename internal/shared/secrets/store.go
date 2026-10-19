package secrets

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
)

var ErrProviderNotConfigured = errors.New("secret provider not configured")

type Store interface {
	PutToken(ctx context.Context, path, value string) error
	GetToken(ctx context.Context, path string) (string, error)
}

type HealthChecker interface {
	Check(ctx context.Context) error
}

// Resolver 는 스택 단위로 Store 를 만들어 준다.
//
// OpenBao 는 스택마다 배포되므로 "provider 하나 = 인스턴스 하나" 가정이
// 성립하지 않는다. Router 는 전역 등록(로컬 개발)과 스택별 지연 생성(운영)을
// 함께 지원한다.
type Resolver interface {
	Resolve(ctx context.Context, provider, stackID string) (Store, error)
}

type Router struct {
	mu        sync.RWMutex
	providers map[string]Store
	// stackScoped 는 (provider, stackID) 로 만든 Store 캐시다.
	stackScoped map[string]Store
	resolver    Resolver
}

func NewRouter() *Router {
	return &Router{
		providers:   map[string]Store{},
		stackScoped: map[string]Store{},
	}
}

// WithResolver 는 스택별 Store 생성기를 등록한다.
func (r *Router) WithResolver(resolver Resolver) *Router {
	if r == nil {
		return r
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolver = resolver
	return r
}

func stackScopeKey(provider, stackID string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "::" + strings.TrimSpace(stackID)
}

// ForStack 은 특정 스택의 Store 를 돌려준다.
// resolver 가 없거나 stackID 가 비면 전역 등록분으로 되돌아간다.
func (r *Router) ForStack(ctx context.Context, provider, stackID string) (Store, error) {
	if r == nil {
		return nil, ErrProviderNotConfigured
	}
	if strings.TrimSpace(stackID) == "" {
		return r.resolve(provider)
	}

	key := stackScopeKey(provider, stackID)
	r.mu.RLock()
	cached, ok := r.stackScoped[key]
	resolver := r.resolver
	r.mu.RUnlock()
	if ok {
		return cached, nil
	}
	if resolver == nil {
		return r.resolve(provider)
	}

	store, err := resolver.Resolve(ctx, provider, stackID)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, ErrProviderNotConfigured
	}

	r.mu.Lock()
	r.stackScoped[key] = store
	r.mu.Unlock()
	return store, nil
}

// InvalidateStack 은 스택 삭제/재설치 시 캐시된 Store 를 버린다.
func (r *Router) InvalidateStack(stackID string) {
	if r == nil || strings.TrimSpace(stackID) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	suffix := "::" + strings.TrimSpace(stackID)
	for key := range r.stackScoped {
		if strings.HasSuffix(key, suffix) {
			delete(r.stackScoped, key)
		}
	}
}

// PutTokenForStack / GetTokenForStack / CheckForStack 은 스택 범위 접근이다.
func (r *Router) PutTokenForStack(ctx context.Context, provider, stackID, path, value string) error {
	store, err := r.ForStack(ctx, provider, stackID)
	if err != nil {
		return err
	}
	return store.PutToken(ctx, path, value)
}

func (r *Router) GetTokenForStack(ctx context.Context, provider, stackID, path string) (string, error) {
	store, err := r.ForStack(ctx, provider, stackID)
	if err != nil {
		return "", err
	}
	return store.GetToken(ctx, path)
}

// KVDeleter 는 KV 트리를 지우는 확장 창구다. 파이프라인을 지울 때 그 파이프라인 몫의
// 자격증명을 회수하는 데 쓴다 — Store 를 넓히지 않는 이유는 KVBrowser 와 같다.
type KVDeleter interface {
	ListKeys(ctx context.Context, path string) ([]string, error)
	DeleteSecret(ctx context.Context, path string) error
}

// minDeleteTreeDepth 는 트리 삭제가 받는 접두사의 최소 깊이다. "kv/" · "kv/nullus" 처럼
// 넓은 접두사로 금고를 통째로 비우는 실수를 막는다.
const minDeleteTreeDepth = 4

// DeleteTreeForStack 은 접두사 아래의 값을 하위 디렉터리까지 모두 지운다. 없으면 성공이다.
//
// 하나를 못 지워도 나머지는 지운다 — 중간에 멈추면 무엇이 남았는지 알 수 없다.
func (r *Router) DeleteTreeForStack(ctx context.Context, provider, stackID, prefix string) error {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	// "." · ".." · 빈 세그먼트는 거부한다. proxy 요청은 경로를 정리(path.Join)해서 보내므로
	// pipelines/../pipelines/other 가 다른 파이프라인을 지운다. 깊이도 정리한 뒤의 것을 센다.
	if path.Clean(prefix) != prefix || !allSegments(prefix) {
		return fmt.Errorf("금고 트리 삭제 접두사가 올바르지 않습니다: %q", prefix)
	}
	if len(strings.Split(prefix, "/")) < minDeleteTreeDepth {
		return fmt.Errorf("금고 트리 삭제 접두사가 너무 넓습니다: %q", prefix)
	}
	store, err := r.ForStack(ctx, provider, stackID)
	if err != nil {
		return err
	}
	deleter, ok := store.(KVDeleter)
	if !ok {
		return fmt.Errorf("이 금고(%s)는 값 삭제를 지원하지 않습니다", provider)
	}
	return deleteTree(ctx, deleter, prefix)
}

func allSegments(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if !IsPathSegment(seg) {
			return false
		}
	}
	return true
}

func deleteTree(ctx context.Context, d KVDeleter, prefix string) error {
	keys, err := d.ListKeys(ctx, prefix)
	if err != nil {
		return err
	}
	var errs []error
	for _, key := range keys {
		name := strings.TrimSuffix(key, "/")
		if !IsPathSegment(name) {
			errs = append(errs, fmt.Errorf("금고가 돌려준 키 이름 %q 을 건너뜁니다 (%s)", key, prefix))
			continue
		}
		child := prefix + "/" + name
		if strings.HasSuffix(key, "/") {
			if err := deleteTree(ctx, d, child); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := d.DeleteSecret(ctx, child); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *Router) CheckForStack(ctx context.Context, provider, stackID string) error {
	store, err := r.ForStack(ctx, provider, stackID)
	if err != nil {
		return err
	}
	hc, ok := store.(HealthChecker)
	if !ok {
		return ErrProviderNotConfigured
	}
	return hc.Check(ctx)
}

func (r *Router) Register(provider string, store Store) {
	if r == nil || store == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[key] = store
}

func (r *Router) PutToken(ctx context.Context, provider, path, value string) error {
	store, err := r.resolve(provider)
	if err != nil {
		return err
	}
	return store.PutToken(ctx, path, value)
}

func (r *Router) GetToken(ctx context.Context, provider, path string) (string, error) {
	store, err := r.resolve(provider)
	if err != nil {
		return "", err
	}
	return store.GetToken(ctx, path)
}

func (r *Router) Check(ctx context.Context, provider string) error {
	store, err := r.resolve(provider)
	if err != nil {
		return err
	}
	hc, ok := store.(HealthChecker)
	if !ok {
		return ErrProviderNotConfigured
	}
	return hc.Check(ctx)
}

// Has 는 전역 등록 또는 스택별 resolver 중 하나라도 있으면 true 다.
func (r *Router) Has(provider string) bool {
	if r == nil {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.providers[key]; ok {
		return true
	}
	return r.resolver != nil
}

func (r *Router) resolve(provider string) (Store, error) {
	if r == nil {
		return nil, ErrProviderNotConfigured
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	if key == "" {
		return nil, ErrProviderNotConfigured
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	store, ok := r.providers[key]
	if !ok {
		return nil, ErrProviderNotConfigured
	}
	return store, nil
}
