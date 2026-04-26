package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/pkg/crypto"
)

type PostgresKubeconfigProvider struct {
	pool          *pgxpool.Pool
	encryptionKey []byte
}

func NewPostgresKubeconfigProvider(pool *pgxpool.Pool, encryptionKey []byte) *PostgresKubeconfigProvider {
	return &PostgresKubeconfigProvider{pool: pool, encryptionKey: encryptionKey}
}

func (p *PostgresKubeconfigProvider) GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error) {
	const q = `SELECT kubeconfig FROM clusters WHERE id = $1`

	var encrypted []byte
	err := p.pool.QueryRow(ctx, q, clusterID).Scan(&encrypted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(encrypted) == 0 {
		return nil, nil
	}
	if len(p.encryptionKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes")
	}

	decrypted, err := crypto.Decrypt(p.encryptionKey, string(encrypted))
	if err != nil {
		return nil, fmt.Errorf("decrypt kubeconfig: %w", err)
	}
	return decrypted, nil
}
