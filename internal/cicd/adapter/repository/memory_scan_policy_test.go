package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestMemoryScanPolicyRepository_ImplementsPort(t *testing.T) {
	var _ port.ScanPolicyRepository = (*MemoryScanPolicyRepository)(nil)
}

// 저장한 적 없는 스택은 "없음" 이다. 기본값을 지어 돌려주면 호출부가 기본값인지
// 운영자가 고른 값인지 구분할 수 없다.
func TestMemoryScanPolicyRepository_MissingIsNotFound(t *testing.T) {
	repo := NewMemoryScanPolicyRepository()
	_, found, err := repo.Get(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestMemoryScanPolicyRepository_UpsertThenGet(t *testing.T) {
	repo := NewMemoryScanPolicyRepository()
	policy := domain.ScanPolicy{
		BlockSeverity: domain.SeverityHigh, IgnoreUnfixed: false, OnScannerUnreachable: domain.UnreachableAllow,
	}
	require.NoError(t, repo.Upsert(context.Background(), "stk_1", policy, "alice"))
	require.NoError(t, repo.Upsert(context.Background(), "stk_2", domain.DefaultScanPolicy(), "bob"))

	got, found, err := repo.Get(context.Background(), "stk_1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, policy, got)

	policy.BlockSeverity = domain.SeverityCritical
	require.NoError(t, repo.Upsert(context.Background(), "stk_1", policy, "alice"))
	got, _, _ = repo.Get(context.Background(), "stk_1")
	assert.Equal(t, domain.SeverityCritical, got.BlockSeverity, "같은 스택은 덮어쓴다")
}
