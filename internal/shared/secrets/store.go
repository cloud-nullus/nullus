package secrets

import (
	"context"
	"errors"
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
