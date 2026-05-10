package rotation

import "context"
import "strings"

type RouterReissuer struct {
	providers map[string]Reissuer
}

func NewRouterReissuer() *RouterReissuer {
	return &RouterReissuer{providers: map[string]Reissuer{}}
}

func (r *RouterReissuer) Register(provider string, reissuer Reissuer) {
	if r == nil || reissuer == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	if key == "" {
		return
	}
	r.providers[key] = reissuer
}

func (r *RouterReissuer) Reissue(ctx context.Context, input ReissueInput) (string, error) {
	if r == nil {
		return "", ErrReissueUnsupported
	}
	key := strings.ToLower(strings.TrimSpace(input.Provider))
	provider, ok := r.providers[key]
	if !ok {
		return "", ErrReissueUnsupported
	}
	return provider.Reissue(ctx, input)
}
