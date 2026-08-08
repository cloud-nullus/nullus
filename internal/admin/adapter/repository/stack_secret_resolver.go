package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

// StackSecretResolver 는 스택 ID 로 해당 스택의 OpenBao Store 를 만든다.
//
// OpenBao 는 스택마다 배포되므로 주소가 전역 하나일 수 없다. 스택 레코드에서
// 네임스페이스와 클러스터를 찾고, 그 클러스터의 kubeconfig 로 Kubernetes Auth
// 기반 Store 를 생성한다.
type StackSecretResolver struct {
	pool               *pgxpool.Pool
	kubeconfigProvider KubeconfigDecryptor
}

// KubeconfigDecryptor 는 클러스터 ID 로 복호화된 kubeconfig 를 돌려준다.
type KubeconfigDecryptor interface {
	GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error)
}

func NewStackSecretResolver(pool *pgxpool.Pool, kubeconfigProvider KubeconfigDecryptor) *StackSecretResolver {
	return &StackSecretResolver{pool: pool, kubeconfigProvider: kubeconfigProvider}
}

func (r *StackSecretResolver) Resolve(ctx context.Context, provider, stackID string) (secrets.Store, error) {
	if r == nil || r.pool == nil || r.kubeconfigProvider == nil {
		return nil, secrets.ErrProviderNotConfigured
	}
	if !strings.EqualFold(strings.TrimSpace(provider), "openbao") {
		return nil, secrets.ErrProviderNotConfigured
	}

	// stacks.id 는 UUID 가 아니라 "stk_..." 형식의 VARCHAR 다.
	// ::uuid 로 캐스팅하면 모든 조회가 타입 오류로 실패한다.
	var namespace, clusterID string
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(namespace, ''), COALESCE(cluster_id::text, '')
		FROM stacks
		WHERE id = $1 AND deleted_at IS NULL`, stackID).Scan(&namespace, &clusterID)
	if err != nil {
		return nil, fmt.Errorf("스택 %s 조회 실패: %w", stackID, err)
	}
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(clusterID) == "" {
		return nil, secrets.ErrProviderNotConfigured
	}

	kubeconfig, err := r.kubeconfigProvider.GetKubeconfig(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("클러스터 %s kubeconfig 로드 실패: %w", clusterID, err)
	}
	if len(kubeconfig) == 0 {
		return nil, secrets.ErrProviderNotConfigured
	}

	return secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		Role:           secrets.ControllerRole,
		ServiceAccount: secrets.ControllerServiceAccount,
	})
}
