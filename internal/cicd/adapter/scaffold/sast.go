package scaffold

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 소스 정적 분석(SAST) 단계 — 스택의 SonarQube 로 소스를 분석하고 Quality Gate 로
// 배포를 막는다. 이미지 스캔과 같은 원칙을 따른다: 주소는 스택이 정해 렌더 시점에 박고,
// 정책과 토큰은 플랫폼이 CI 변수로 싣는다.
const (
	sastStageID   = "sast"
	sastStageName = "SAST"

	// defaultSASTScannerImage 는 CI 잡이 쓸 sonar-scanner 이미지다. amd64 만 낸다 —
	// arm64 노드에서는 에뮬레이션이 있어야 돈다. 에어갭 설치는 이름을 바꾸지 않고
	// 노드의 레지스트리 미러가 번들에서 내준다(RuntimeImages 가 번들에 싣는다).
	defaultSASTScannerImage = shareddomain.SonarScannerImage
)

// sastPolicyVariableNames 는 분석 단계가 읽는, 플랫폼이 푸시하는 정책 변수다.
var sastPolicyVariableNames = []string{
	port.SASTOnGateFailureVariable,
	port.ScanOnUnreachableVariable,
}

// sastScriptLines 는 CI 3종이 공통으로 실행하는 분석 명령이다. 분석 → 리포트 → 판정 순이다.
//
// sonar.qualitygate.wait 가 핵심이다 — 없으면 스캐너는 결과를 올리기만 하고 성공으로
// 끝나, Quality Gate 가 배포 판정에 끼지 못한다.
//
// 종료 코드로 둘을 가른다(sonar-scanner 실측): 3 은 Quality Gate 실패라 스택 정책
// (차단·경고)을 따르고, 그 밖의 실패는 분석 자체를 못 한 것(서버 장애·인증 실패)이라
// 이미지 스캔과 같은 장애 정책을 따른다. 경고 정책을 분석 실패에까지 적용하지 않는다 —
// 분석기가 죽은 동안 모든 배포가 초록불로 지나가는 것이 가장 나쁜 실패다.
//
// 명령에 역슬래시를 쓰지 않는다. Jenkins 의 sh ”'…”' 는 Groovy 문자열이라 역슬래시를
// 이스케이프로 읽는다.
func sastScriptLines(projectKey string) []string {
	defaults := domain.DefaultScanPolicy()
	return []string{
		fmt.Sprintf(`rc=0; sonar-scanner -Dsonar.projectKey=%s -Dsonar.qualitygate.wait=true -Dsonar.qualitygate.timeout=300 || rc=$?`,
			shellQuoteSingle(projectKey)),
		sastReportLine(projectKey),
		fmt.Sprintf(`if [ "$rc" -eq 0 ]; then exit 0; fi; `+
			`if [ "$rc" -eq 3 ]; then `+
			`if [ "${%s:-%s}" = "%s" ]; then echo "Quality Gate 를 통과하지 못했습니다 — 경고 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "Quality Gate 를 통과하지 못했습니다 — 차단 정책입니다"; exit 1; fi; `+
			`if [ "${%s:-%s}" = "%s" ]; then echo "정적 분석을 수행하지 못했습니다 — 분석기 장애 허용 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "정적 분석을 수행하지 못했습니다 — 분석기 장애 시 차단 정책입니다"; exit 1`,
			port.SASTOnGateFailureVariable, defaults.SASTGateActionOrDefault(), domain.SASTGateWarn,
			port.ScanOnUnreachableVariable, defaults.UnreachableActionOrDefault(), domain.UnreachableAllow),
	}
}

// sastReportMetrics 는 리포트에 싣는 프로젝트 지표다(domain.SASTMetrics 와 같다).
const sastReportMetrics = "bugs,vulnerabilities,code_smells,security_hotspots,coverage,duplicated_lines_density,ncloc"

// sastReportLine 은 이번 분석의 판정·조건과 프로젝트 지표를 SonarQube 에서 읽어 리포트로 남긴다.
//
// 실행 기록 동기화가 이 파일을 읽어 결과를 남긴다 — 플랫폼 API 는 클러스터 밖에서 돌 수 있어
// SonarQube 에 닿는다는 보장이 없고, 분석 토큰은 잡 안에만 있다. 분석 토큰으로 읽을 수 있는 것만
// 쓴다(qualitygates/project_status·measures/component·ce/task 는 되고 project_analyses 는 403, 실측).
//
//   - 최신 판정이 아니라 이번 분석의 것을 읽는다. 스캐너가 남긴 작업(ceTaskId)에서 analysisId 를
//     찾아 그 분석의 판정을 묻는다 — 그 사이 다른 분석이 끝나도 섞이지 않는다.
//   - 지표(measures/component)는 분석을 고를 수 없어 프로젝트의 최신 값이다. 같은 프로젝트의 분석이
//     겹치지 않으면(GitLab 은 기본 브랜치만 분석한다) 이번 분석의 것과 같다.
//   - 분석하지 못했으면(작업 없음) 지표를 읽지 않는다. 지난 분석의 지표가 이번 것으로 보인다.
//   - JSON 이 아닌 응답(프록시·로그인 화면이 200 으로 준 HTML)은 null 로 둔다. curl -f 는 4xx·5xx 만
//     거르고, 그대로 실으면 리포트가 깨져 스캐너 종료 코드까지 잃는다.
//   - 덤이다. 조회가 실패해도 null 로 남기고, 이 줄이 단계의 판정을 바꾸지 않는다.
//   - 토큰은 curl 의 표준입력 설정(-K -)으로 넘기고 트레이스를 끈다. Jenkins 는 sh -xe 로 돌고
//     토큰이 마스킹되지 않아, 인자에 실으면 빌드 로그와 파드 프로세스 목록에 남는다.
func sastReportLine(projectKey string) string {
	return fmt.Sprintf(`{ set +x; } 2>/dev/null; ( k=%s; `+
		`sq() { r=$(printf 'user = "%%s:"' "${%s:-}" | curl -sf --max-time 30 -K - "${%s:-}/$1") || r=""; case "$r" in "{"*) printf '%%s' "$r" ;; *) printf null ;; esac; }; `+
		`t=.scannerwork/report-task.txt; ce=""; url=""; aid=""; `+
		`if [ -f "$t" ]; then ce=$(sed -n 's/^ceTaskId=//p' "$t"); url=$(sed -n 's/^dashboardUrl=//p' "$t"); fi; `+
		`if [ -n "$ce" ]; then aid=$(sq "api/ce/task?id=$ce" | sed -n 's/.*"analysisId":"//p' | cut -d '"' -f 1); fi; `+
		`qg=null; ms=null; `+
		`if [ -n "$aid" ]; then qg=$(sq "api/qualitygates/project_status?analysisId=$aid"); ms=$(sq "api/measures/component?component=$k&metricKeys=%s"); fi; `+
		`printf '{"scanner_exit_code":%%s,"project_key":"%%s","ce_task_id":"%%s","analysis_id":"%%s","dashboard_url":"%%s","quality_gate":%%s,"measures":%%s}' `+
		`"$rc" "$k" "$ce" "$aid" "$url" "$qg" "$ms" > %s ) || true`,
		shellQuoteSingle(projectKey),
		port.SASTTokenVariable, port.SASTServerVariable,
		sastReportMetrics,
		port.SASTReportFile)
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
