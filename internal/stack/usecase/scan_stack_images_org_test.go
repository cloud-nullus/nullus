package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func orgStack(id, orgID string, withScanner bool) *domain.Stack {
	s := imageScanStack(id, withScanner)
	s.OrgID = orgID
	s.Name = "stack-" + id
	return s
}

// 보안 대시보드(#65)는 조직의 스택마다 설치 이미지 보고서를 한 번에 본다. 보고서는 단건 조회와 같은 조립을
// 쓴다 — 두 곳이 따로 만들면 같은 스택이 화면마다 다른 숫자로 보인다.
func TestScanStackImages_ReportsForOrg(t *testing.T) {
	results := newFakeImageScanResults()
	counts := shareddomain.SeverityCounts{Critical: 2}
	require.NoError(t, results.ReplaceForStack(context.Background(), "stk_a", []domain.StackImageScan{{
		Image: "redis:7", ImageDigest: "sha256:r", Status: domain.ImageScanStatusScanned, Counts: &counts, ScannedAt: time.Now(),
	}}))
	stacks := []*domain.Stack{
		orgStack("stk_a", "org-1", true),
		orgStack("stk_b", "org-1", false),
		orgStack("stk_c", "org-1", true),
		orgStack("stk_other", "org-2", true),
	}
	uc := newScanStackImagesUC(stacks, &fakeImageScanner{}, results, true)

	got, err := uc.ReportsForOrg(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, got, 3, "다른 조직의 스택은 섞이지 않는다")
	byID := map[string]domain.OrgStackImageScan{}
	for _, s := range got {
		byID[s.Stack.ID] = s
	}

	a := byID["stk_a"]
	assert.Equal(t, "stack-stk_a", a.Stack.Name)
	assert.Equal(t, "cl-1", a.Stack.ClusterID)
	assert.Equal(t, domain.ImageScanStateScanned, a.Report.Status)
	assert.Len(t, a.Report.Items, 1)
	assert.True(t, a.Report.Airgap)

	b := byID["stk_b"]
	assert.Equal(t, domain.ImageScanStateNotScanned, b.Report.Status)
	assert.Equal(t, domain.ImageScanReasonScannerNotInstalled, b.Report.Reason)

	c := byID["stk_c"]
	assert.Equal(t, domain.ImageScanStatePending, c.Report.Status, "스캐너는 있는데 결과가 아직 없다")
}

// 단건 스캔 조회가 다른 조직의 스택을 보이지 않도록 스택의 조직을 알려 준다(설계 §8 G4).
func TestScanStackImages_StackOrgID(t *testing.T) {
	uc := newScanStackImagesUC([]*domain.Stack{orgStack("stk_a", "org-1", true)}, &fakeImageScanner{}, newFakeImageScanResults(), false)

	org, err := uc.StackOrgID(context.Background(), " stk_a ")
	require.NoError(t, err)
	assert.Equal(t, "org-1", org)

	_, err = uc.StackOrgID(context.Background(), "missing")
	assert.Error(t, err)
}
