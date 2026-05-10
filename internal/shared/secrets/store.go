package secrets

import (
	"context"
	"errors"
	"strings"
)

var ErrProviderNotConfigured = errors.New("secret provider not configured")

type Store interface {
	PutToken(ctx context.Context, path, value string) error
	GetToken(ctx context.Context, path string) (string, error)
}

type HealthChecker interface {
	Check(ctx context.Context) error
}

type Router struct {
	providers map[string]Store
}

func NewRouter() *Router {
	return &Router{providers: map[string]Store{}}
}

func (r *Router) Register(provider string, store Store) {
	if r == nil || store == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	if key == "" {
		return
	}
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

func (r *Router) Has(provider string) bool {
	if r == nil {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	_, ok := r.providers[key]
	return ok
}

func (r *Router) resolve(provider string) (Store, error) {
	if r == nil {
		return nil, ErrProviderNotConfigured
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	if key == "" {
		return nil, ErrProviderNotConfigured
	}
	store, ok := r.providers[key]
	if !ok {
		return nil, ErrProviderNotConfigured
	}
	return store, nil
}
