package domain

import (
	"sort"
	"strings"
	"time"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 설치 이미지 스캔 — 스택이 설치한 OSS 가 실제로 돌리는 이미지의 취약점 보고다.
//
// 보고용이다. 설치를 막지 않는다 — 업스트림 이미지의 CVE 는 사용자가 고칠 수 없는
// 경우가 많아, 막으면 스택을 세울 방법이 사라진다. 대신 무엇이 얼마나 있는지를
// 숨기지 않고, 스캔하지 못했으면 못 했다고 남긴다.

// ImageScanStatus 는 이미지 하나의 스캔 결과 상태다.
type ImageScanStatus string

const (
	ImageScanStatusScanned ImageScanStatus = "scanned"
	// ImageScanStatusFailed 는 그 이미지를 스캔하지 못했다는 뜻이다. 건수를 모른다.
	ImageScanStatusFailed ImageScanStatus = "failed"
)

// ImageScanState 는 스택 단위의 스캔 상태다.
type ImageScanState string

const (
	ImageScanStateScanned ImageScanState = "scanned"
	// ImageScanStatePending 은 스캐너가 있지만 아직 결과가 없다는 뜻이다.
	ImageScanStatePending ImageScanState = "pending"
	// ImageScanStateNotScanned 는 이 스택의 설치 이미지를 스캔하지 않는다는 뜻이다.
	ImageScanStateNotScanned ImageScanState = "not_scanned"
)

// ImageScanSkipReason 은 설치 이미지를 스캔하지 않는 이유다.
type ImageScanSkipReason string

const (
	ImageScanReasonScannerNotInstalled ImageScanSkipReason = "scanner_not_installed"
	// ImageScanReasonAirgap 은 에어갭 설치라 스캔하지 않던 때의 이유다. 이제 만들지 않지만, 화면이 옛
	// 응답을 읽을 수 있게 남긴다.
	ImageScanReasonAirgap ImageScanSkipReason = "airgap"
)

// StackImageScan 은 스택이 돌리는 이미지 하나의 스캔 결과다.
type StackImageScan struct {
	StackID string
	// Image 는 파드 스펙에 적힌 표기(태그)다. 사람이 읽는 이름이다.
	Image string
	// ImageDigest 에 결과를 붙인다 — 태그는 움직인다.
	ImageDigest string
	Release     string
	Workloads   []string
	// Namespace 는 이미지를 돌리는 파드의 네임스페이스다. Shared 는 그것이 스택 네임스페이스 밖의
	// 클러스터 공용 구성요소(cert-manager 등)라는 뜻이다.
	Namespace string
	Shared    bool
	// Exposed 는 이 이미지를 쓰는 파드가 게이트웨이(HTTPRoute)로 클러스터 밖에 열려 있는지다.
	// ExposedHosts 는 그 HTTPRoute 의 호스트 이름이다(호스트를 적지 않은 경로면 비어 있다).
	// 같은 CVE 라도 밖에서 닿는 서비스의 것을 먼저 고친다(설치 OSS CVE 대응 정책).
	Exposed      bool
	ExposedHosts []string

	Status ImageScanStatus
	Error  string

	// Counts 는 모든 취약점, FixableCounts 는 수정본이 있는 것만 센 값이다.
	// 스캔하지 못한 이미지는 nil 이다 — 0 으로 채우면 "취약점 0건" 으로 읽힌다.
	Counts        *shareddomain.SeverityCounts
	FixableCounts *shareddomain.SeverityCounts

	ScannerVersion string
	DBUpdatedAt    *time.Time
	ScannedAt      time.Time

	// Vulnerabilities 는 취약점 목록이다. VulnerabilitiesRecorded 가 거짓이면 목록을
	// 모른다(스캔 실패, 목록 기능 전 스캔) — 빈 목록과 다르다.
	Vulnerabilities         []shareddomain.ImageVulnerability
	VulnerabilitiesRecorded bool
}

// StackImageVulnerabilities 는 설치 이미지 하나의 저장된 취약점 목록이다.
type StackImageVulnerabilities struct {
	// Found 는 그 이미지의 스캔 결과가 있는지다.
	Found bool
	// Recorded 는 목록을 저장했는지다. 거짓이면 Items 가 비어도 0건이 아니다.
	Recorded bool
	Items    []shareddomain.ImageVulnerability
}

// StackImageScanReport 는 스택 하나의 설치 이미지 스캔 보고서다.
type StackImageScanReport struct {
	StackID string
	Status  ImageScanState
	Reason  ImageScanSkipReason
	// Airgap 은 에어갭 설치의 결과라는 뜻이다. 취약점 DB 가 번들을 반입한 날짜 기준이다.
	Airgap        bool
	LastScannedAt *time.Time
	// Summary 는 스캔에 성공한 이미지의 건수 합이다.
	Summary *shareddomain.SeverityCounts
	Items   []StackImageScan
}

// HasImageScanner 는 스택 안에 이미지 스캐너를 세우는 선택인지 본다.
//
// 이름을 보지 않는다 — 스캐너 슬롯의 선택지는 현재 Trivy 하나뿐이고, 고른 사실
// 자체가 곧 설치 여부다. 외부(external) 선택은 스택 안에 서지 않는다.
func HasImageScanner(sel ToolSelection) bool {
	return sel.Enabled && !strings.EqualFold(strings.TrimSpace(sel.Version), "external")
}

// ImageScanSkipReasonFor 는 스택의 설치 이미지를 스캔하지 않는 이유다.
// 빈 값이면 스캔할 수 있다.
//
// 에어갭 설치도 스캔한다 — 이미지는 내부 레지스트리(번들이 올린 경로)에서 받고, 취약점 DB 는 번들이
// 반입한 것을 쓴다. DB 가 반입한 날짜에 멈춰 있으므로 보고서가 그것을 밝힌다(StackImageScanReport.Airgap).
func ImageScanSkipReasonFor(cfg StackConfig) ImageScanSkipReason {
	if !HasImageScanner(cfg.Security.ImageScanner) {
		return ImageScanReasonScannerNotInstalled
	}
	return ""
}

// BuildImageScanReport 는 저장된 결과로 보고서를 만든다.
//
// 스캔하지 않는 스택은 예전 결과를 싣지 않는다 — 에어갭으로 옮긴 스택의 오래된
// 결과가 현재 상태처럼 읽히면 안 된다.
func BuildImageScanReport(stackID string, reason ImageScanSkipReason, scans []StackImageScan) StackImageScanReport {
	r := StackImageScanReport{StackID: stackID, Reason: reason}
	if reason != "" {
		r.Status = ImageScanStateNotScanned
		return r
	}
	if len(scans) == 0 {
		r.Status = ImageScanStatePending
		return r
	}

	items := append([]StackImageScan(nil), scans...)
	sort.SliceStable(items, func(i, j int) bool { return moreSevere(items[i], items[j]) })

	var summary shareddomain.SeverityCounts
	var last time.Time
	for _, s := range items {
		if s.ScannedAt.After(last) {
			last = s.ScannedAt
		}
		if s.Status == ImageScanStatusScanned && s.Counts != nil {
			summary = summary.Plus(*s.Counts)
		}
	}

	r.Status = ImageScanStateScanned
	r.Summary = &summary
	r.LastScannedAt = &last
	r.Items = items
	return r
}

// moreSevere 는 심각한 이미지를 앞에 둔다. 건수를 모르는 이미지는 뒤로 보낸다.
func moreSevere(a, b StackImageScan) bool {
	if (a.Counts == nil) != (b.Counts == nil) {
		return a.Counts != nil
	}
	if a.Counts != nil {
		ka := []int{a.Counts.Critical, a.Counts.High, a.Counts.Medium, a.Counts.Low}
		kb := []int{b.Counts.Critical, b.Counts.High, b.Counts.Medium, b.Counts.Low}
		for i := range ka {
			if ka[i] != kb[i] {
				return ka[i] > kb[i]
			}
		}
	}
	return a.Image < b.Image
}
