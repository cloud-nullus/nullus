package scaffold

import (
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 의존성 취약점 스캔(SCA) 단계 — 소스의 락파일로 서드파티 의존성의 취약점을 찾아 빌드 전에 막는다
// (nullus-plan#79, 설계 docs/11_기능설계/Nullus_의존성_취약점_스캔_설계.md).
//
// 이미지 스캔과 같은 Trivy 를 쓴다 — `trivy image` 대신 `trivy fs` 로 저장소를 보고, 스택의 Trivy
// 서버에 client 로 붙어 취약점 DB 는 서버에서 쓴다. 정책(차단 심각도·수정판 없는 것 제외·스캐너
// 장애 시 동작)도 이미지 스캔과 **같은 변수**를 읽는다 — 한 스택에 두 기준이 있으면 같은 취약점이
// 한쪽에서는 막히고 다른 쪽에서는 지나간다.
//
// 이미지 스캔과 다른 점은 시점이다. 이미지는 빌드 뒤에 레지스트리에서 다시 받아 보지만, 이 단계는
// 소스만 보므로 빌드 **앞**에 서서 빨리 실패한다.
const (
	dependencyScanStageID   = "dependency-scan"
	dependencyScanStageName = "DependencyScan"

	dependencyScanReportFile = port.DependencyScanReportFile
)

// dependencyScanPolicyVariableNames 는 이 단계가 읽는, 플랫폼이 푸시하는 정책 변수다 — 이미지 스캔과 같다.
var dependencyScanPolicyVariableNames = scanPolicyVariableNames

// dependencyScanScriptLines 는 CI 3종이 공통으로 실행하는 의존성 스캔 명령이다.
//
// 이미지 스캔처럼 두 번 돈다 — 한 번은 사람이 볼 리포트(심각도를 거르지 않는다), 한 번은 게이트.
// 한 번에 하면 차단된 실행에서 리포트가 비어 무엇에 걸렸는지 알 수 없다. 리포트 단계가 실패하면
// 스캔 자체를 못 한 것(서버 도달 불가 등)이라 장애 정책을 따른다.
//
// 락파일이 없는 저장소는 찾을 것이 없어 통과한다(Trivy 는 package.json 만으로는 의존성을 풀지
// 않는다) — 그래서 탐지 대상이 몇 개였는지를 로그에 남긴다. 0 이면 락파일을 커밋하라는 뜻이다.
//
// 예외는 저장소의 .trivyignore 다(Trivy 가 작업 디렉터리에서 스스로 읽는다). 명령에 역슬래시를
// 쓰지 않는다 — Jenkins 의 sh ”'…”' 는 Groovy 문자열이다.
func dependencyScanScriptLines() []string {
	// --offline-scan: 원격 저장소를 묻지 않는다. Trivy 는 pom.xml 의 전이 의존성을 풀려고 pom 을 하나씩 Maven Central 에서
	// 받는데, 전자정부 샘플(Boot · Web) 둘 다 `429 Too Many Requests`(Retry-After 1800) 로 스캔 자체가 죽어 빌드까지 못 갔다
	// (10-26). 락파일 · pom 에 적힌 것만 보고, Maven 의 전이 의존성은 이미지 스캔이 빌드된 jar 로 본다. 에어갭은 원래 이렇게 돈다.
	server := fmt.Sprintf(`--server "$%s" --scanners vuln --offline-scan`, port.ScanServerVariable)
	unfixed := fmt.Sprintf(`$([ "${%s:-true}" = "true" ] && echo --ignore-unfixed)`,
		port.ScanIgnoreUnfixedVariable)
	defaults := domain.DefaultScanPolicy()
	// 저장소 밖의 산출물·의존성 캐시는 보지 않는다 — 이번 커밋의 락파일만 판정 대상이다.
	skip := `--skip-dirs node_modules --skip-dirs .git`

	return []string{
		fmt.Sprintf(`if ! trivy fs %s %s %s --format json --output %s .; then `+
			`if [ "${%s:-%s}" = "%s" ]; then echo "의존성 스캔을 수행하지 못했습니다 — 스캐너 장애 허용 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "의존성 스캔을 수행하지 못했습니다 — 스캐너 장애 시 차단 정책입니다"; exit 1; fi`,
			server, unfixed, skip, dependencyScanReportFile,
			port.ScanOnUnreachableVariable, defaults.UnreachableActionOrDefault(), domain.UnreachableAllow),
		// 몇 개의 락파일을 봤는지 남긴다. 0 이면 이 단계는 아무것도 검사하지 않았다.
		// grep 은 하나도 못 찾으면 1 로 끝난다 — GitLab Runner 의 pipefail 아래서 잡이 죽지 않게 받아 둔다.
		fmt.Sprintf(`n=$(grep -o '"Target":' %s | wc -l | tr -d ' ' || true); echo "의존성 스캔 대상(락파일): $n 개"; `+
			`if [ "$n" = "0" ]; then echo "락파일(package-lock.json · go.sum · requirements.txt 등)이 없어 검사할 의존성이 없습니다 — 락파일을 커밋하세요"; fi`,
			dependencyScanReportFile),
		fmt.Sprintf(`trivy fs %s --severity "${%s:-%s}" %s %s --exit-code 1 --quiet .`,
			server, port.ScanSeverityVariable, defaults.TrivySeverities(), unfixed, skip),
	}
}

// writeGitLabDependencyScanJob 은 빌드 앞 stage 의 의존성 스캔 잡이다. 모든 브랜치에서 돈다.
func writeGitLabDependencyScanJob(b *strings.Builder, in Input) {
	fmt.Fprintf(b, "%s:\n", dependencyScanStageID)
	fmt.Fprintf(b, "  stage: %s\n", dependencyScanStageID)
	// trivy 이미지의 entrypoint(trivy)가 잡 스크립트를 가로채지 않게 비운다.
	b.WriteString("  image:\n")
	fmt.Fprintf(b, "    name: $%s\n", scanImageVar)
	b.WriteString("    entrypoint: [\"\"]\n")
	b.WriteString("  needs: []\n")
	if repo := strings.TrimSpace(in.ImageScannerJavaDBRepository); repo != "" {
		// 에어갭 Java DB 미러(JAR·pom 을 볼 때 client 가 받는다). 내부 레지스트리는 plain HTTP 다.
		b.WriteString("  variables:\n")
		fmt.Fprintf(b, "    TRIVY_JAVA_DB_REPOSITORY: %q\n", repo)
		b.WriteString("    TRIVY_INSECURE: \"true\"\n")
	}
	b.WriteString("  script:\n")
	writeScriptLines(b, dependencyScanScriptLines())
	b.WriteString("  artifacts:\n")
	b.WriteString("    when: always\n")
	fmt.Fprintf(b, "    paths:\n      - %s\n", dependencyScanReportFile)
	b.WriteString("\n")
}

// writeGitHubDependencyScanJob 은 GitHub Actions 판이다. build 가 needs 로 기다린다.
func writeGitHubDependencyScanJob(b *strings.Builder, in Input) {
	fmt.Fprintf(b, "  %s:\n", dependencyScanStageID)
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    env:\n")
	fmt.Fprintf(b, "      %s: %q\n", scanServerVar, in.DependencyScanEndpoint)
	for _, v := range dependencyScanPolicyVariableNames {
		fmt.Fprintf(b, "      %s: ${{ vars.%s }}\n", v, v)
	}
	b.WriteString("    container:\n")
	fmt.Fprintf(b, "      image: %s\n", githubImageExpression(scanImageVar, defaultScannerImage))
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n")
	b.WriteString("      - name: Scan dependencies\n")
	b.WriteString("        run: |\n")
	for _, line := range dependencyScanScriptLines() {
		fmt.Fprintf(b, "          %s\n", line)
	}
	b.WriteString("      - name: Upload report\n")
	b.WriteString("        if: always()\n")
	b.WriteString("        uses: actions/upload-artifact@v4\n")
	b.WriteString("        with:\n")
	fmt.Fprintf(b, "          name: %s\n", port.DependencyScanReportArtifact)
	fmt.Fprintf(b, "          path: %s\n", dependencyScanReportFile)
	b.WriteString("\n")
}

// writeJenkinsDependencyScanStage 는 Jenkinsfile 의 stage 다. 스캐너 컨테이너(trivy)는 이미지 스캔과
// 같은 것을 쓴다 — 정책은 그 컨테이너가 envFrom 으로 읽는 ConfigMap 에서 온다.
func writeJenkinsDependencyScanStage(b *strings.Builder, in Input) {
	fmt.Fprintf(b, "    stage('%s') {\n", dependencyScanStageName)
	b.WriteString("      when {\n")
	b.WriteString("        beforeAgent true\n")
	b.WriteString("        not { changeset pattern: 'deploy/**' , comparator: 'ANT' }\n")
	b.WriteString("      }\n")
	b.WriteString("      environment {\n")
	fmt.Fprintf(b, "        %s = %q\n", scanServerVar, in.DependencyScanEndpoint)
	b.WriteString("      }\n")
	b.WriteString("      steps {\n")
	b.WriteString("        container('scanner') {\n")
	b.WriteString("          sh '''\n")
	b.WriteString("            set -eu\n")
	if repo := strings.TrimSpace(in.ImageScannerJavaDBRepository); repo != "" {
		fmt.Fprintf(b, "            export TRIVY_JAVA_DB_REPOSITORY=%q TRIVY_INSECURE=true\n", repo)
	}
	for _, line := range dependencyScanScriptLines() {
		fmt.Fprintf(b, "            %s\n", line)
	}
	b.WriteString("          '''\n")
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("      post {\n")
	b.WriteString("        always {\n")
	fmt.Fprintf(b, "          archiveArtifacts artifacts: %q, allowEmptyArchive: true\n", dependencyScanReportFile)
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("    }\n\n")
}
