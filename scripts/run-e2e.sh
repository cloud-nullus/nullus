#!/usr/bin/env bash
set -euo pipefail

#
# Nullus E2E / UAT 테스트 실행 스크립트
#
# 사용법:
#   ./scripts/run-e2e.sh              # 전체 E2E + UAT 실행
#   ./scripts/run-e2e.sh uat          # UAT만 실행 (역할별 시나리오)
#   ./scripts/run-e2e.sh uat admin    # Admin UAT만
#   ./scripts/run-e2e.sh uat devops   # DevOps UAT만
#   ./scripts/run-e2e.sh uat developer # Developer UAT만
#   ./scripts/run-e2e.sh uat rbac     # RBAC 메뉴 가시성만
#   ./scripts/run-e2e.sh e2e          # 기존 E2E만 (navigation, sidebar 등)
#   ./scripts/run-e2e.sh --headed     # 브라우저 표시 모드
#   ./scripts/run-e2e.sh --report     # 마지막 리포트 열기
#

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WEB_DIR="$PROJECT_ROOT/web"
ENV_FILE="$WEB_DIR/.env"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }

# ── Mock Auth 보장 ──────────────────────────────────────────────────
ensure_mock_auth() {
  if [ ! -f "$ENV_FILE" ]; then
    info ".env 파일 생성 (VITE_AUTH_MODE=mock)"
    cat > "$ENV_FILE" <<'EOF'
VITE_AUTH_MODE=mock
VITE_OIDC_PROVIDER=keycloak
VITE_OIDC_AUTHORITY=http://localhost:8180/realms/nullus
VITE_OIDC_CLIENT_ID=nullus-web
EOF
  fi

  if ! grep -q "VITE_AUTH_MODE=mock" "$ENV_FILE" 2>/dev/null; then
    warn ".env에서 VITE_AUTH_MODE=mock이 아닙니다. 테스트 실행 중 mock으로 전환합니다."
    ORIGINAL_AUTH_MODE=$(grep "VITE_AUTH_MODE" "$ENV_FILE" 2>/dev/null || echo "")
    sed -i.bak 's/VITE_AUTH_MODE=.*/VITE_AUTH_MODE=mock/' "$ENV_FILE"
    RESTORE_ENV=true
  fi
}

restore_env() {
  if [ "${RESTORE_ENV:-false}" = true ] && [ -f "$ENV_FILE.bak" ]; then
    mv "$ENV_FILE.bak" "$ENV_FILE"
    info ".env 복원 완료"
  fi
}

# ── Dev 서버 확인 ───────────────────────────────────────────────────
check_dev_server() {
  if curl -s -o /dev/null -w "%{http_code}" http://localhost:5173 2>/dev/null | grep -q "200"; then
    ok "Dev 서버 구동 중 (localhost:5173)"
    return 0
  else
    info "Dev 서버가 꺼져있습니다. Playwright가 자동으로 시작합니다."
    return 0
  fi
}

# ── Playwright 설치 확인 ────────────────────────────────────────────
check_playwright() {
  if ! npx playwright --version &>/dev/null; then
    warn "Playwright 미설치. 설치 중..."
    npx playwright install chromium
    return
  fi

  if ! npx playwright install --check chromium &>/dev/null 2>&1; then
    warn "Chromium 브라우저 바이너리 미설치. 설치 중..."
    npx playwright install chromium
  fi
}

# ── 테스트 실행 ─────────────────────────────────────────────────────
run_tests() {
  local target="$1"
  local extra_args="${2:-}"
  local test_path=""
  local label=""

  case "$target" in
    all)
      test_path="e2e/"
      label="전체 E2E + UAT"
      ;;
    uat)
      test_path="e2e/uat/"
      label="UAT 전체 (역할별 시나리오)"
      ;;
    uat-admin|admin)
      test_path="e2e/uat/admin-scenarios.spec.ts"
      label="Admin UAT (A1-A7)"
      ;;
    uat-devops|devops)
      test_path="e2e/uat/devops-scenarios.spec.ts"
      label="DevOps UAT (D1-D13)"
      ;;
    uat-developer|developer)
      test_path="e2e/uat/developer-scenarios.spec.ts"
      label="Developer UAT (V1-V5)"
      ;;
    uat-rbac|rbac)
      test_path="e2e/uat/rbac-menu-visibility.spec.ts"
      label="RBAC 메뉴 가시성"
      ;;
    e2e)
      test_path="e2e/*.spec.ts"
      label="기존 E2E (navigation, sidebar, theme)"
      ;;
    *)
      fail "알 수 없는 타겟: $target"
      usage
      exit 1
      ;;
  esac

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  info "실행: $label"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""

  cd "$WEB_DIR"
  npx playwright test "$test_path" --reporter=list $extra_args
  local exit_code=$?

  echo ""
  if [ $exit_code -eq 0 ]; then
    ok "$label — 전체 PASS"
  else
    fail "$label — 일부 실패 (exit code: $exit_code)"
  fi

  return $exit_code
}

# ── 도움말 ──────────────────────────────────────────────────────────
usage() {
  cat <<'HELP'

Nullus E2E / UAT 테스트 실행 스크립트

사용법:
  ./scripts/run-e2e.sh [타겟] [옵션]

타겟:
  (없음), all     전체 E2E + UAT 실행
  uat             UAT만 실행 (역할별 시나리오 55개)
  uat admin       Admin UAT (A1-A7, 9개)
  uat devops      DevOps UAT (D1-D13, 18개)
  uat developer   Developer UAT (V1-V5, 18개)
  uat rbac        RBAC 메뉴 가시성 (10개)
  e2e             기존 E2E (navigation, sidebar, theme)

옵션:
  --headed        브라우저 표시 모드 (디버깅용)
  --report        마지막 테스트 리포트 열기

예시:
  ./scripts/run-e2e.sh                    # 전체 실행
  ./scripts/run-e2e.sh uat                # UAT만
  ./scripts/run-e2e.sh uat admin          # Admin만
  ./scripts/run-e2e.sh uat devops --headed # DevOps를 브라우저로
  ./scripts/run-e2e.sh --report           # 리포트 보기

HELP
}

# ── 메인 ────────────────────────────────────────────────────────────
main() {
  if [ "${1:-}" = "--report" ]; then
    cd "$WEB_DIR"
    npx playwright show-report
    exit 0
  fi

  if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    usage
    exit 0
  fi

  local target="all"
  local extra_args=""

  for arg in "$@"; do
    case "$arg" in
      --headed) extra_args="$extra_args --headed" ;;
      uat|e2e|all) target="$arg" ;;
      admin|devops|developer|rbac)
        target="uat-$arg"
        ;;
      *) target="$arg" ;;
    esac
  done

  ensure_mock_auth
  check_dev_server
  check_playwright

  trap restore_env EXIT

  run_tests "$target" "$extra_args"
}

main "$@"
