package port

import "context"

// SecretReader reads a token value from a secret backend (e.g. OpenBao).
// The CI/CD module depends on this port — it never imports shared/secrets directly.
type SecretReader interface {
	GetToken(ctx context.Context, path string) (string, error)
}
