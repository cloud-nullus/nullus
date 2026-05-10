package port

import "context"

type TokenSourceInput struct {
	OrgID     string
	Module    string
	Provider  string
	Path      string
	TokenType string
	Status    string
	SecretManager string
	TokenValue    string
}

// TokenSourceRegistry tracks OpenBao token metadata for stack integrations.
type TokenSourceRegistry interface {
	Upsert(ctx context.Context, input TokenSourceInput) error
}
