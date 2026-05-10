package rotation

import (
	"context"
	"fmt"
	"strings"
)

var ErrReissueUnsupported = fmt.Errorf("reissue unsupported")

type Reissuer interface {
	Reissue(ctx context.Context, input ReissueInput) (string, error)
}

type ReissueInput struct {
	Provider     string
	CurrentToken string
	Metadata     map[string]any
}

type NoopReissuer struct{}

func NewNoopReissuer() *NoopReissuer {
	return &NoopReissuer{}
}

func (r *NoopReissuer) Reissue(_ context.Context, input ReissueInput) (string, error) {
	_ = strings.TrimSpace(input.Provider)
	return "", ErrReissueUnsupported
}
