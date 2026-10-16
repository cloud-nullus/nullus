package port

import (
	"strconv"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 이미지 스캔 단계가 읽는 변수 이름이다.
//
// 스캐폴딩은 이 이름으로 읽고, 정책 푸시는 같은 이름으로 싣는다 — 한쪽만
// 바꾸면 정책을 바꿔도 파이프라인은 기본값으로 판정한다.
const (
	// ScanServerVariable 은 스택 Trivy 서버 주소다. 정책이 아니라 스택이 정한 값이다.
	ScanServerVariable = "NULLUS_TRIVY_SERVER"
	// ScanImageVariable 은 CI 잡이 쓸 Trivy CLI 이미지다. 에어갭도 이름을 바꾸지 않는다 —
	// 노드의 레지스트리 미러가 번들에서 내준다. 플랫폼은 이 변수를 푸시하지 않는다.
	ScanImageVariable = "NULLUS_TRIVY_IMAGE"

	// ScanSeverityVariable 은 게이트가 막을 심각도 목록이다(Trivy --severity 형식).
	ScanSeverityVariable = "NULLUS_SCAN_SEVERITY"
	// ScanIgnoreUnfixedVariable 이 "true" 면 수정본 없는 취약점을 판정에서 뺀다.
	ScanIgnoreUnfixedVariable = "NULLUS_SCAN_IGNORE_UNFIXED"
	// ScanOnUnreachableVariable 이 "allow" 면 스캔을 못 돌려도 단계를 통과시킨다.
	ScanOnUnreachableVariable = "NULLUS_SCAN_ON_UNREACHABLE"

	// ScanPolicyConfigMapName 은 CI 변수 저장소가 없는 CI(Jenkins)가 정책을 읽는
	// 스택 네임스페이스의 ConfigMap 이다.
	ScanPolicyConfigMapName = "nullus-scan-policy"

	// SASTServerVariable 은 스택 SonarQube 의 클러스터 내 주소다. sonar-scanner 가
	// 이 이름의 환경변수를 그대로 읽는다.
	SASTServerVariable = "SONAR_HOST_URL"
	// SASTTokenVariable 은 분석 토큰이다. 스택 설치가 발급해 OpenBao 에 두고, 플랫폼이
	// 시크릿 CI 변수로 등록한다. sonar-scanner 가 이 이름을 그대로 읽는다.
	SASTTokenVariable = "SONAR_TOKEN"
	// SASTImageVariable 은 CI 잡이 쓸 sonar-scanner 이미지다. ScanImageVariable 과 같다.
	SASTImageVariable = "NULLUS_SONAR_SCANNER_IMAGE"
	// SASTOnGateFailureVariable 이 "warn" 이면 Quality Gate 가 실패해도 단계를 통과시킨다.
	SASTOnGateFailureVariable = "NULLUS_SAST_ON_GATE_FAILURE"
	// SecretDetectionVariable 은 시크릿 탐지 단계의 동작이다 — block(기본)·warn·off (nullus-plan#80).
	SecretDetectionVariable = "NULLUS_SECRET_DETECTION"
	// SecretScanImageVariable 은 CI 잡이 쓸 gitleaks 이미지다. ScanImageVariable 과 같다.
	SecretScanImageVariable = "NULLUS_GITLEAKS_IMAGE"
	// SecretScanBaseVariable 은 이번 푸시 직전 커밋이다 — 탐지 범위(신규 커밋만)의 시작점. CI 마다
	// 이름이 달라(CI_COMMIT_BEFORE_SHA · github.event.before · GIT_PREVIOUS_SUCCESSFUL_COMMIT) 렌더러가
	// 이 이름으로 옮겨 적는다. 플랫폼이 푸시하는 변수가 아니다.
	SecretScanBaseVariable = "NULLUS_SCAN_BASE"
)

// ScanPolicyVariables 는 정책을 CI 가 읽는 변수로 옮긴다.
//
// 가리지 않는다. 정책은 비밀이 아니고, GitLab 은 8자 미만 값(true·allow)의
// 마스킹 등록을 통째로 거부해 변수가 아예 생기지 않는다.
func ScanPolicyVariables(p domain.ScanPolicy) []ProjectVariable {
	return []ProjectVariable{
		{Key: ScanSeverityVariable, Value: p.TrivySeverities()},
		{Key: ScanIgnoreUnfixedVariable, Value: strconv.FormatBool(p.IgnoreUnfixed)},
		{Key: ScanOnUnreachableVariable, Value: string(p.UnreachableActionOrDefault())},
		{Key: SASTOnGateFailureVariable, Value: string(p.SASTGateActionOrDefault())},
		// 스캐너 이미지도 정책과 함께 민다. 플랫폼이 이미지를 올리면(예: 멀티아치
		// sonar-scanner) 파이프라인 정의를 다시 쓰지 않아도 다음 실행부터 따라온다 —
		// GitLab 은 프로젝트 변수가 .gitlab-ci.yml 전역 variables 보다 앞서고, GitHub
		// 워크플로는 vars 를 먼저 본다. Jenkins 파드 템플릿은 정적이라 정의 갱신이 답이다.
		{Key: ScanImageVariable, Value: shareddomain.TrivyCLIImage},
		{Key: SASTImageVariable, Value: shareddomain.SonarScannerImage},
		{Key: SecretDetectionVariable, Value: string(p.SecretDetectionOrDefault())},
		{Key: SecretScanImageVariable, Value: shareddomain.GitleaksImage},
	}
}
