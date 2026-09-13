package port

import (
	"strconv"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 이미지 스캔 단계가 읽는 변수 이름이다.
//
// 스캐폴딩은 이 이름으로 읽고, 정책 푸시는 같은 이름으로 싣는다 — 한쪽만
// 바꾸면 정책을 바꿔도 파이프라인은 기본값으로 판정한다.
const (
	// ScanServerVariable 은 스택 Trivy 서버 주소다. 정책이 아니라 스택이 정한 값이다.
	ScanServerVariable = "NULLUS_TRIVY_SERVER"
	// ScanImageVariable 은 CI 잡이 쓸 Trivy CLI 이미지다(에어갭 미러로 덮는다).
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
	}
}
