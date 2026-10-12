package scaffold

import (
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 시크릿 탐지(Secret Detection) 단계 — 사용자 저장소의 코드·이력에 섞여 들어간 자격증명·
// 토큰·키를 gitleaks 로 찾아 배포를 막는다(nullus-plan#80, 설계 docs/11_기능설계/
// Nullus_시크릿_탐지_설계.md).
//
// 이미지 스캔·SAST 와 두 가지가 다르다.
//
//   - 스택에 설치하는 것이 없다. 스캐너는 CI 잡 컨테이너 이미지 하나라 **모든 파이프라인에
//     단계가 있다** — 끄는 것은 정책(NULLUS_SECRET_DETECTION=off)이지 렌더링이 아니다.
//   - 빌드 **앞**에 선다. 시크릿이 든 소스로 이미지를 만들어 레지스트리에 올리는 것부터
//     막아야 한다. SAST 는 빌드와 나란히 돌지만, 이 단계는 빌드가 기다린다.
//
// 그리고 한 가지가 같다 — 시크릿은 **푸시되는 순간 유출**이다. CI 단계는 사후 통보라 탐지 시
// 메시지에 "폐기·회전" 안내가 반드시 붙는다. 유출 전에 잡는 길은 저장소에 함께 넣는
// pre-commit 훅(renderPreCommitConfig)이다 — 둘을 병행한다.
const (
	secretScanStageID   = "secret-scan"
	secretScanStageName = "SecretScan"

	// defaultSecretScannerImage 는 CI 잡이 쓸 gitleaks 이미지다. 에어갭 설치는 이름을 바꾸지
	// 않고 노드의 레지스트리 미러가 번들에서 내준다(RuntimeImages 가 번들에 싣는다).
	defaultSecretScannerImage = shareddomain.GitleaksImage

	// PreCommitConfigPath / GitleaksConfigPath 는 앱 저장소에 함께 넣는 개발자 로컬 훅 설정과
	// gitleaks 설정(오탐 allowlist)이다. 처음 만들 때만 넣고 **정의 갱신에서는 덮어쓰지 않는다** —
	// allowlist 는 사용자가 다듬는 파일이다.
	PreCommitConfigPath = ".pre-commit-config.yaml"
	GitleaksConfigPath  = ".gitleaks.toml"
)

// secretScanPolicyVariableNames 는 이 단계가 읽는, 플랫폼이 푸시하는 정책 변수다.
var secretScanPolicyVariableNames = []string{
	port.SecretDetectionVariable,
	port.ScanOnUnreachableVariable,
}

// zeroSHA 는 첫 푸시·브랜치 생성에서 CI 가 "이전 커밋" 으로 주는 값이다. 범위 끝으로 쓸 수 없다.
const zeroSHA = "0000000000000000000000000000000000000000"

// secretScanScriptLines 는 CI 3종이 공통으로 실행하는 탐지 명령이다.
//
// 범위는 **신규 커밋만**이다 — 이번 푸시가 더한 커밋(NULLUS_SCAN_BASE..HEAD)만 보고, 기준 커밋이
// 없거나(첫 푸시·수동 실행·얕은 클론) 클론에 없으면 작업 트리(현재 파일)를 본다. 전체 이력을
// 매 실행마다 훑으면 저장소가 커질수록 느려지고, 이미 지난 이력의 유출은 이 게이트가 막을 수 있는
// 것이 아니다 — 그것은 온보딩 때 한 번 하는 일이다(설계 §4).
//
// 종료 코드로 셋을 가른다(gitleaks --exit-code 2): 0 은 깨끗함, 2 는 탐지라 정책(차단·경고)을
// 따르고, 그 밖(1)은 스캔 자체를 못 한 것(이미지·git 장애)이라 이미지 스캔과 같은 장애 정책을 따른다.
// 경고 정책을 스캔 실패에까지 적용하지 않는다 — 스캐너가 죽은 동안 모든 배포가 초록불로 지나가는
// 것이 가장 나쁜 실패다.
//
// 명령에 역슬래시를 쓰지 않는다. Jenkins 의 sh ”'…”' 는 Groovy 문자열이라 역슬래시를 이스케이프로
// 읽는다. 리포트의 비밀값은 --redact 로 가린다 — 리포트가 또 다른 유출 경로가 되면 안 된다.
func secretScanScriptLines() []string {
	defaults := domain.DefaultScanPolicy()
	common := fmt.Sprintf(`--redact --no-banner --exit-code 2 --report-format json --report-path %s`, port.SecretScanReportFile)
	return []string{
		// 컨테이너 잡은 작업 디렉터리 소유자가 다른 uid 다. git 이 "dubious ownership" 으로 멈추면
		// gitleaks git 이 이력을 읽지 못한다.
		`git config --global --add safe.directory '*' >/dev/null 2>&1 || true`,
		fmt.Sprintf(`if [ "${%s:-%s}" = "%s" ]; then echo "시크릿 탐지가 정책으로 꺼져 있습니다 — 건너뜁니다"; printf '[]' > %s; exit 0; fi`,
			port.SecretDetectionVariable, defaults.SecretDetectionOrDefault(), domain.SecretDetectionOff, port.SecretScanReportFile),
		fmt.Sprintf(`rc=0; base="${%s:-}"; case "$base" in %s) base="" ;; esac`, port.SecretScanBaseVariable, zeroSHA),
		fmt.Sprintf(`if [ -n "$base" ] && git cat-file -e "$base^{commit}" 2>/dev/null; then `+
			`echo "신규 커밋 $base..HEAD 를 검사합니다"; gitleaks git %s --log-opts "$base..HEAD" . || rc=$?; `+
			`else echo "기준 커밋이 없어 작업 트리를 검사합니다"; gitleaks dir %s . || rc=$?; fi`, common, common),
		`if [ "$rc" -eq 0 ]; then echo "시크릿 탐지: 없음"; exit 0; fi`,
		fmt.Sprintf(`if [ "$rc" -eq 2 ]; then `+
			`echo "시크릿이 탐지됐습니다 — 커밋을 지워도 이미 유출입니다. 탐지된 자격증명은 즉시 폐기·회전하세요(%s 참조). 오탐이면 %s 의 allowlist 나 .gitleaksignore 의 Fingerprint 로 다룹니다"; `+
			`if [ "${%s:-%s}" = "%s" ]; then echo "경고 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "차단 정책입니다"; exit 1; fi`,
			port.SecretScanReportFile, GitleaksConfigPath,
			port.SecretDetectionVariable, defaults.SecretDetectionOrDefault(), domain.SecretDetectionWarn),
		fmt.Sprintf(`if [ "${%s:-%s}" = "%s" ]; then echo "시크릿 탐지를 수행하지 못했습니다(종료 코드 $rc) — 스캐너 장애 허용 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "시크릿 탐지를 수행하지 못했습니다(종료 코드 $rc) — 스캐너 장애 시 차단 정책입니다"; exit 1`,
			port.ScanOnUnreachableVariable, defaults.UnreachableActionOrDefault(), domain.UnreachableAllow),
	}
}

// gitleaksVersionTag 는 스캐너 이미지 태그에서 pre-commit 훅의 rev 를 뽑는다 — 로컬 훅과 CI 가
// 같은 버전으로 같은 규칙을 봐야 "로컬에서는 통과했는데 CI 에서 막힌다" 가 생기지 않는다.
func gitleaksVersionTag() string {
	img := defaultSecretScannerImage
	if i := strings.LastIndex(img, ":"); i >= 0 {
		return img[i+1:]
	}
	return "latest"
}

// renderPreCommitConfig 는 개발자 로컬에서 커밋 전에 같은 gitleaks 를 돌리는 pre-commit 설정이다.
//
// 강제할 수 없다 — 훅은 개발자가 켜야 돈다. 그래서 CI 단계와 병행한다: 훅은 유출 전에 잡고,
// CI 는 훅을 켜지 않은 사람의 커밋을 잡는다.
func renderPreCommitConfig() string {
	var b strings.Builder
	b.WriteString("# Nullus 가 생성했습니다 — 커밋 전에 시크릿을 잡는 pre-commit 훅입니다.\n")
	b.WriteString("# CI 의 secret-scan 단계와 같은 gitleaks, 같은 버전, 같은 규칙(.gitleaks.toml)을 씁니다.\n")
	b.WriteString("#\n")
	b.WriteString("# 켜기(한 번): pipx install pre-commit  (또는 brew install pre-commit)\n")
	b.WriteString("#            pre-commit install\n")
	b.WriteString("# 그 뒤 git commit 마다 스테이지된 변경에서 시크릿을 찾고, 있으면 커밋을 막습니다.\n")
	b.WriteString("repos:\n")
	b.WriteString("  - repo: https://github.com/gitleaks/gitleaks\n")
	fmt.Fprintf(&b, "    rev: %s\n", gitleaksVersionTag())
	b.WriteString("    hooks:\n")
	b.WriteString("      - id: gitleaks\n")
	return b.String()
}

// renderGitleaksConfig 는 gitleaks 설정이다. 기본 규칙을 그대로 쓰고 오탐만 allowlist 에 적는다.
//
// 비어 있는 채로 넣는 이유: 파일이 있어야 사용자가 "어디에 적는지" 를 찾지 않는다. 기본값은
// 아무것도 허용하지 않는다 — 배포 매니페스트 경로를 미리 풀어 두면 그 안의 진짜 Secret 이 가려진다.
//
// allowlist 표는 **주석으로** 둔다. gitleaks v8.30 은 검사 항목(paths·regexes 등)이 없는 allowlist 를
// 설정 오류로 거부한다 — 2026-10-12 실측에서 비어 있는 [allowlist] 때문에 모든 앱의 첫 파이프라인이
// secret-scan 에서 막혔다.
func renderGitleaksConfig() string {
	var b strings.Builder
	b.WriteString("# Nullus 가 생성했습니다 — gitleaks 설정. CI 의 secret-scan 단계와 pre-commit 훅이 함께 읽습니다.\n")
	b.WriteString("#\n")
	b.WriteString("# 기본 규칙(useDefault)을 그대로 쓰고, **오탐만** 아래 allowlist 에 적습니다. 진짜 시크릿은\n")
	b.WriteString("# allowlist 가 아니라 폐기·회전이 답입니다 — 커밋을 지워도 이미 유출입니다.\n")
	b.WriteString("# 특정 한 건만 넘기려면 .gitleaksignore 에 리포트의 Fingerprint 한 줄을 적는 쪽이 좁습니다.\n")
	b.WriteString("[extend]\n")
	b.WriteString("useDefault = true\n\n")
	b.WriteString("# 오탐을 적을 때 아래 블록의 주석을 풀고 paths 나 regexes 를 하나 이상 채웁니다.\n")
	b.WriteString("# 항목이 하나도 없는 [allowlist] 는 gitleaks 가 설정 오류로 거부합니다.\n")
	b.WriteString("# [allowlist]\n")
	b.WriteString("# description = \"이 앱의 오탐 — 경로·정규식. 비밀값이 아닌 것만 적으세요\"\n")
	b.WriteString("# paths = [\n")
	b.WriteString("#   '''src/fixtures/sample-keys\\.json$''',\n")
	b.WriteString("# ]\n")
	b.WriteString("# regexes = [\n")
	b.WriteString("#   '''EXAMPLE_KEY_[A-Z0-9]+''',\n")
	b.WriteString("# ]\n")
	return b.String()
}
