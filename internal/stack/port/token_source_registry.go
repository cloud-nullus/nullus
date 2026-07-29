package port

import "context"

type TokenSourceInput struct {
	OrgID         string
	Module        string
	Provider      string
	Path          string
	TokenType     string
	Status        string
	SecretManager string
	TokenValue    string
	// ClusterID / Namespace 는 회전 후 반영(rolling restart) 대상을 찾는 데 쓴다.
	ClusterID string
	Namespace string
}

// TokenSourceRegistry tracks OpenBao token metadata for stack integrations.
type TokenSourceRegistry interface {
	Upsert(ctx context.Context, input TokenSourceInput) error
}
