package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/pkg/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresClusterTargetProvider struct {
	pool          *pgxpool.Pool
	encryptionKey []byte
}

func NewPostgresClusterTargetProvider(pool *pgxpool.Pool, encryptionKey []byte) *PostgresClusterTargetProvider {
	return &PostgresClusterTargetProvider{pool: pool, encryptionKey: encryptionKey}
}

func (p *PostgresClusterTargetProvider) GetTarget(ctx context.Context, clusterID string) (*port.ClusterTarget, error) {
	var name string
	var encryptedKubeconfig []byte

	const q = `SELECT name, kubeconfig FROM clusters WHERE id = $1`
	if err := p.pool.QueryRow(ctx, q, clusterID).Scan(&name, &encryptedKubeconfig); err != nil {
		return nil, fmt.Errorf("get cluster target %s: %w", clusterID, err)
	}

	var kubeconfig []byte
	if len(encryptedKubeconfig) > 0 {
		decrypted, err := crypto.Decrypt(p.encryptionKey, string(encryptedKubeconfig))
		if err != nil {
			return nil, fmt.Errorf("decrypt kubeconfig: %w", err)
		}
		kubeconfig = decrypted
	}

	clusterName := strings.TrimPrefix(name, "kind-")

	return &port.ClusterTarget{
		Kubeconfig:  kubeconfig,
		ClusterName: clusterName,
	}, nil
}
