package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func vulnFixture() []shareddomain.ImageVulnerability {
	return []shareddomain.ImageVulnerability{
		{ID: "CVE-2", PkgName: "libc", Severity: "high", Class: shareddomain.VulnerabilityClassOS, Target: "debian 12"},
		{ID: "CVE-1", PkgName: "openssl", Severity: "critical", Class: shareddomain.VulnerabilityClassOS, Target: "debian 12", FixedVersion: "3.0.2"},
		{ID: "GHSA-3", PkgName: "x/net", Severity: "medium", Class: shareddomain.VulnerabilityClassLibrary, Target: "app"},
	}
}

func TestScanStackImages_Vulnerabilities_PagesRecordedList(t *testing.T) {
	results := newFakeImageScanResults()
	require.NoError(t, results.ReplaceForStack(context.Background(), "stk_1", []domain.StackImageScan{{
		ImageDigest: "sha256:p", Status: domain.ImageScanStatusScanned,
		VulnerabilitiesRecorded: true, Vulnerabilities: vulnFixture(),
	}}))
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, &fakeImageScanner{}, results, false)

	page, err := uc.Vulnerabilities(context.Background(), "stk_1", "sha256:p",
		shareddomain.VulnerabilityFilter{Class: shareddomain.VulnerabilityClassOS})
	require.NoError(t, err)

	assert.Equal(t, shareddomain.VulnerabilityListAvailable, page.Status)
	assert.Equal(t, 2, page.Total)
	assert.Equal(t, "CVE-1", page.Items[0].ID, "심각한 것부터")
	assert.Len(t, page.Targets, 2, "대상 요약은 필터와 무관하다")
}

// 목록 기능 전에 스캔한 이미지는 목록이 없다. 0건으로 보이면 안 된다.
func TestScanStackImages_Vulnerabilities_NotRecorded(t *testing.T) {
	results := newFakeImageScanResults()
	require.NoError(t, results.ReplaceForStack(context.Background(), "stk_1", []domain.StackImageScan{{
		ImageDigest: "sha256:old", Status: domain.ImageScanStatusScanned,
	}}))
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, &fakeImageScanner{}, results, false)

	page, err := uc.Vulnerabilities(context.Background(), "stk_1", "sha256:old", shareddomain.VulnerabilityFilter{})
	require.NoError(t, err)
	assert.Equal(t, shareddomain.VulnerabilityListUnavailable, page.Status)
	assert.Equal(t, shareddomain.VulnerabilityReasonNotRecorded, page.Reason)
}

func TestScanStackImages_Vulnerabilities_UnknownImage(t *testing.T) {
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, &fakeImageScanner{}, newFakeImageScanResults(), false)

	_, err := uc.Vulnerabilities(context.Background(), "stk_1", "sha256:nope", shareddomain.VulnerabilityFilter{})
	assert.ErrorIs(t, err, ErrStackImageScanNotFound)
}

// 스캔하지 않는 스택은 목록도 없다. 이유를 그대로 돌려준다.
func TestScanStackImages_Vulnerabilities_NotScannedStack(t *testing.T) {
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, &fakeImageScanner{}, newFakeImageScanResults(), true)

	page, err := uc.Vulnerabilities(context.Background(), "stk_1", "sha256:p", shareddomain.VulnerabilityFilter{})
	require.NoError(t, err)
	assert.Equal(t, shareddomain.VulnerabilityListUnavailable, page.Status)
	assert.Equal(t, string(domain.ImageScanReasonAirgap), page.Reason)
}
