package scaffold

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 소스 정적 분석(SAST) 단계 — 스택의 SonarQube 로 소스를 분석하고 Quality Gate 로
// 배포를 막는다. 이미지 스캔과 같은 원칙을 따른다: 주소는 스택이 정해 렌더 시점에 박고,
// 정책과 토큰은 플랫폼이 CI 변수로 싣는다.
const (
	sastStageID   = "sast"
	sastStageName = "SAST"

	// defaultSASTScannerImage 는 CI 잡이 쓸 sonar-scanner 이미지다. amd64 만 낸다 —
	// arm64 노드에서는 에뮬레이션이 있어야 돈다. 에어갭 설치는 내부 미러로 변수를 덮는다.
	defaultSASTScannerImage = "sonarsource/sonar-scanner-cli:12.2"
)

// sastPolicyVariableNames 는 분석 단계가 읽는, 플랫폼이 푸시하는 정책 변수다.
var sastPolicyVariableNames = []string{
	port.SASTOnGateFailureVariable,
	port.ScanOnUnreachableVariable,
}

// sastScriptLines 는 CI 3종이 공통으로 실행하는 분석 명령이다.
//
// sonar.qualitygate.wait 가 핵심이다 — 없으면 스캐너는 결과를 올리기만 하고 성공으로
// 끝나, Quality Gate 가 배포 판정에 끼지 못한다.
//
// 종료 코드로 둘을 가른다(sonar-scanner 실측): 3 은 Quality Gate 실패라 스택 정책
// (차단·경고)을 따르고, 그 밖의 실패는 분석 자체를 못 한 것(서버 장애·인증 실패)이라
// 이미지 스캔과 같은 장애 정책을 따른다. 경고 정책을 분석 실패에까지 적용하지 않는다 —
// 분석기가 죽은 동안 모든 배포가 초록불로 지나가는 것이 가장 나쁜 실패다.
func sastScriptLines(projectKey string) []string {
	defaults := domain.DefaultScanPolicy()
	return []string{
		fmt.Sprintf(`rc=0; sonar-scanner -Dsonar.projectKey=%s -Dsonar.qualitygate.wait=true -Dsonar.qualitygate.timeout=300 || rc=$?; `+
			`if [ "$rc" -eq 0 ]; then exit 0; fi; `+
			`if [ "$rc" -eq 3 ]; then `+
			`if [ "${%s:-%s}" = "%s" ]; then echo "Quality Gate 를 통과하지 못했습니다 — 경고 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "Quality Gate 를 통과하지 못했습니다 — 차단 정책입니다"; exit 1; fi; `+
			`if [ "${%s:-%s}" = "%s" ]; then echo "정적 분석을 수행하지 못했습니다 — 분석기 장애 허용 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "정적 분석을 수행하지 못했습니다 — 분석기 장애 시 차단 정책입니다"; exit 1`,
			shellQuoteSingle(projectKey),
			port.SASTOnGateFailureVariable, defaults.SASTGateActionOrDefault(), domain.SASTGateWarn,
			port.ScanOnUnreachableVariable, defaults.UnreachableActionOrDefault(), domain.UnreachableAllow),
	}
}

// shellQuoteSingle 는 셸 단어 하나로 감싼다. 앱 이름은 DNS 이름이라 따옴표가 없지만,
// 스크립트 조립이 입력을 믿지 않게 한다.
func shellQuoteSingle(s string) string {
	safe := true
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			safe = false
			break
		}
	}
	if safe && s != "" {
		return s
	}
	out := "'"
	for _, r := range s {
		if r == '\'' {
			out += `'\''`
			continue
		}
		out += string(r)
	}
	return out + "'"
}
