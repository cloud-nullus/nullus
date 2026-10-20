package domain

import "time"

// DependencyScanResult 는 실행 하나의 의존성 취약점 스캔(SCA, trivy fs) 결과 요약이다
// (nullus-plan#79, Security Dashboard 설계 §8 G1).
//
// 이미지 스캔과 같은 틀이다 — 요약 + 판정 + 원본 위치만 둔다. 원본 리포트는 CI 산출물로만 남는다.
type DependencyScanResult struct {
	ID           string `json:"id"`
	PipelineID   string `json:"pipeline_id"`
	DeploymentID string `json:"deployment_id"`

	Scanner        string     `json:"scanner"`
	ScannerVersion string     `json:"scanner_version,omitempty"`
	DBUpdatedAt    *time.Time `json:"db_updated_at,omitempty"`

	// Counts 는 리포트를 읽었을 때만 있다. nil 은 "모름" 이다 — 0 으로 채우면 "취약점 0건" 으로 읽힌다.
	// 스택 정책의 ignore_unfixed 를 따른다(판정이 본 숫자와 같아야 한다).
	Counts *SeverityCounts `json:"counts,omitempty"`
	// Targets 는 검사한 락파일 수(리포트의 Results 수)다. nil 은 모름, 0 은 검사할 것이 없었다는 뜻이다 —
	// 스캐폴딩은 락파일을 만들지 않아 "0 개 검사 · 통과" 가 흔하다(#79 §1). 통과와 가른다.
	Targets *int `json:"targets,omitempty"`

	GateResult GateResult `json:"gate_result"`
	ReportURI  string     `json:"report_uri,omitempty"`
	ScannedAt  time.Time  `json:"scanned_at"`

	// ReportRef 는 CI 리포트를 다시 읽을 위치다. 화면에는 내보내지 않는다.
	ReportRef *ScanReportRef `json:"-"`
}

// DependencyScanResultID 는 실행 하나에 대응하는 의존성 스캔 결과 ID 다. 같은 실행을 다시 동기화해도
// 기록이 늘지 않는다.
func DependencyScanResultID(deploymentID string) string {
	return "depscan_" + deploymentID
}
