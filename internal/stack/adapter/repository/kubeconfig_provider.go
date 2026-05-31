package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/client-go/tools/clientcmd"

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
	const q = `SELECT name, endpoint, kubeconfig FROM clusters WHERE id = $1`

	var clusterName string
	var storedEndpoint string
	var encrypted []byte
	err := p.pool.QueryRow(ctx, q, clusterID).Scan(&clusterName, &storedEndpoint, &encrypted)
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

	kubeconfig := []byte(decrypted)
	if synced, updated, newEndpoint, syncErr := p.syncKindEndpointFromLocal(clusterName, storedEndpoint, kubeconfig); syncErr != nil {
		slog.Warn("kind endpoint auto-sync skipped", "cluster_id", clusterID, "cluster_name", clusterName, "error", syncErr)
	} else if updated {
		kubeconfig = synced
		if enc, encErr := crypto.Encrypt(p.encryptionKey, synced); encErr == nil {
			const uq = `UPDATE clusters SET endpoint = $2, kubeconfig = $3, updated_at = NOW() WHERE id = $1`
			if _, dbErr := p.pool.Exec(ctx, uq, clusterID, newEndpoint, enc); dbErr != nil {
				slog.Warn("failed to persist kind endpoint auto-sync", "cluster_id", clusterID, "cluster_name", clusterName, "error", dbErr)
			}
		} else {
			slog.Warn("failed to encrypt auto-synced kubeconfig", "cluster_id", clusterID, "cluster_name", clusterName, "error", encErr)
		}
	}

	return kubeconfig, nil
}

func (p *PostgresKubeconfigProvider) syncKindEndpointFromLocal(clusterName, storedEndpoint string, kubeconfig []byte) ([]byte, bool, string, error) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(clusterName)), "kind-") {
		return kubeconfig, false, "", nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	localCfg, err := rules.Load()
	if err != nil {
		return kubeconfig, false, "", fmt.Errorf("load local kubeconfig: %w", err)
	}
	ctx := localCfg.Contexts[clusterName]
	if ctx == nil {
		return kubeconfig, false, "", nil
	}
	cluster := localCfg.Clusters[ctx.Cluster]
	if cluster == nil || strings.TrimSpace(cluster.Server) == "" {
		return kubeconfig, false, "", nil
	}
	runtimeEndpoint := strings.TrimSpace(cluster.Server)
	if strings.TrimSpace(storedEndpoint) == runtimeEndpoint {
		return kubeconfig, false, runtimeEndpoint, nil
	}

	stackCfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return kubeconfig, false, "", fmt.Errorf("load stored kubeconfig: %w", err)
	}
	applyServer := func(clusterRef string) {
		if c := stackCfg.Clusters[clusterRef]; c != nil {
			c.Server = runtimeEndpoint
		}
	}
	if stackCtx := stackCfg.Contexts[clusterName]; stackCtx != nil {
		applyServer(stackCtx.Cluster)
	}
	for _, c := range stackCfg.Contexts {
		if c != nil {
			applyServer(c.Cluster)
		}
	}
	written, err := clientcmd.Write(*stackCfg)
	if err != nil {
		return kubeconfig, false, "", fmt.Errorf("write synced kubeconfig: %w", err)
	}
	return written, true, runtimeEndpoint, nil
}
