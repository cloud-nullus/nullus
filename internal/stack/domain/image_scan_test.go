package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

func trivyStackConfig() StackConfig {
	return StackConfig{Security: SecurityConfig{
		ImageScanner: ToolSelection{Name: "trivy", Version: "0.74.0", Enabled: true},
	}}
}

// 설치 이미지 스캔은 보고용이다. 할 수 없는 스택은 "0건" 이 아니라 스캔하지
// 않았다는 사실과 이유를 남겨야 한다.
func TestImageScanSkipReasonFor(t *testing.T) {
	external := StackConfig{Security: SecurityConfig{
		ImageScanner: ToolSelection{Name: "trivy", Version: "external", Enabled: true},
	}}

	assert.Equal(t, ImageScanSkipReason(""), ImageScanSkipReasonFor(trivyStackConfig(), false))
	assert.Equal(t, ImageScanReasonScannerNotInstalled, ImageScanSkipReasonFor(StackConfig{}, false))
	// 외부 스캐너는 스택 안에 서지 않는다 — 스캔 Job 이 붙을 서버가 없다.
	assert.Equal(t, ImageScanReasonScannerNotInstalled, ImageScanSkipReasonFor(external, false))
	// 에어갭 설치는 스캐너가 있어도 스캔하지 않는다. 이미지를 받을 외부 레지스트리에
	// 닿지 못하고, DB 도 사람이 넣은 만큼만 새것이다.
	assert.Equal(t, ImageScanReasonAirgap, ImageScanSkipReasonFor(trivyStackConfig(), true))
	assert.Equal(t, ImageScanReasonAirgap, ImageScanSkipReasonFor(StackConfig{}, true))
}

func TestHasImageScanner(t *testing.T) {
	assert.True(t, HasImageScanner(ToolSelection{Name: "trivy", Enabled: true}))
	assert.False(t, HasImageScanner(ToolSelection{Name: "trivy"}))
	assert.False(t, HasImageScanner(ToolSelection{Name: "trivy", Version: "External", Enabled: true}))
}

// 스캔하지 않는 스택은 예전 결과를 보여 주지 않는다. 에어갭으로 옮긴 스택의 오래된
// 결과가 현재 상태처럼 읽히면 안 된다.
func TestBuildImageScanReport_NotScanned(t *testing.T) {
	old := []StackImageScan{{StackID: "stk_1", ImageDigest: "sha256:a", Status: ImageScanStatusScanned,
		Counts: &shareddomain.SeverityCounts{Critical: 1}, ScannedAt: time.Now()}}

	r := BuildImageScanReport("stk_1", ImageScanReasonAirgap, old)

	assert.Equal(t, ImageScanStateNotScanned, r.Status)
	assert.Equal(t, ImageScanReasonAirgap, r.Reason)
	assert.Empty(t, r.Items)
	assert.Nil(t, r.Summary)
	assert.Nil(t, r.LastScannedAt)
}

// 스캐너는 있는데 결과가 없으면 대기다. 설치 직후 스캔이 끝나기 전의 상태다.
func TestBuildImageScanReport_Pending(t *testing.T) {
	r := BuildImageScanReport("stk_1", "", nil)

	assert.Equal(t, ImageScanStatePending, r.Status)
	assert.Empty(t, r.Reason)
	assert.Nil(t, r.Summary)
}

// 요약은 스캔에 성공한 이미지만 더한다. 실패한 이미지는 건수를 모르므로 0 으로
// 섞지 않는다. 목록은 심각한 것부터 보인다.
func TestBuildImageScanReport_SummarizesAndSorts(t *testing.T) {
	early := time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC)
	late := early.Add(5 * time.Minute)
	scans := []StackImageScan{
		{Image: "docker.io/library/redis:7", ImageDigest: "sha256:r", Status: ImageScanStatusScanned,
			Counts: &shareddomain.SeverityCounts{High: 3, Low: 1}, ScannedAt: early},
		{Image: "quay.io/argoproj/argocd:v2.13.3", ImageDigest: "sha256:a", Status: ImageScanStatusFailed,
			Error: "manifest unknown", ScannedAt: late},
		{Image: "docker.io/bitnami/postgresql:16", ImageDigest: "sha256:p", Status: ImageScanStatusScanned,
			Counts: &shareddomain.SeverityCounts{Critical: 1, High: 1}, ScannedAt: late},
	}

	r := BuildImageScanReport("stk_1", "", scans)

	assert.Equal(t, ImageScanStateScanned, r.Status)
	assert.Equal(t, &shareddomain.SeverityCounts{Critical: 1, High: 4, Low: 1}, r.Summary)
	assert.Equal(t, late, *r.LastScannedAt)
	assert.Equal(t, []string{"sha256:p", "sha256:r", "sha256:a"},
		[]string{r.Items[0].ImageDigest, r.Items[1].ImageDigest, r.Items[2].ImageDigest})
}
