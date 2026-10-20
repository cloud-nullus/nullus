package domain

import "time"

// PipelineSecuritySnapshot 은 파이프라인 하나의 단계별 최신 보안 결과다(Security Dashboard 설계 §5).
//
// 결과 테이블은 cicd 가 소유한다. 대시보드(관측 모듈)는 테이블을 직접 읽지 않고 이 모양으로 받는다.
// 셀은 단계마다 그 단계의 최신 기록이다 — 앞 단계가 막으면 뒤 단계는 돌지 않으므로 최신 실행 하나의
// 결과만 보면 지금 돌고 있는 이미지의 CRITICAL 이 사라진다(설계 §2.1). 없는 결과는 nil 이다.
type PipelineSecuritySnapshot struct {
	PipelineID string
	Name       string
	StackID    string
	ClusterID  string
	// Stages 는 파이프라인이 실제로 가진 단계다. 비어 있으면 모른다(단계 기록 전의 파이프라인).
	Stages []string
	// HasSecretScan · HasDependencyScan · HasSAST · HasImageScan 은 Stages 에 그 보안 단계가 있는지다.
	// CI 마다 다른 표기(ImageScan · image-scan)를 단계 키로 맞춰 본 것이다. Stages 가 비면 모두 거짓이다.
	HasSecretScan     bool
	HasDependencyScan bool
	HasSAST           bool
	HasImageScan      bool

	// LatestRun 은 최근 실행이다. 실행이 없으면 nil 이다.
	LatestRun *RunSummary

	SecretScan     *SecretScanResult
	DependencyScan *DependencyScanResult
	SAST           *SASTResult
	ImageScan      *ImageScanResult

	// RunVersions 는 결과 행이 가리키는 실행의 표시 번호(#42)다. 키는 deployment_id 다.
	RunVersions map[string]string

	// SecretDetection 은 스택 정책의 시크릿 탐지 동작이다(정책 행이 없으면 기본값 block). 스택에 묶이지
	// 않은 파이프라인은 빈 값이다 — 정책이 없어 단계 유무로만 본다.
	SecretDetection SecretDetectionAction
}

// PipelineSecurityList 는 조직 파이프라인의 단계별 최신 보안 결과다.
type PipelineSecurityList struct {
	Pipelines []*PipelineSecuritySnapshot
	// Truncated 는 상한에 닿아 최근 만든 것부터 잘랐다는 뜻이다.
	Truncated bool
}

// RunSummary 는 실행 하나의 요약이다.
type RunSummary struct {
	DeploymentID string
	Version      string
	Status       DeploymentStatus
	StartedAt    time.Time
}
