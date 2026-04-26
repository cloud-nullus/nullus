#!/usr/bin/env bash
# 목적: 에어갭 kind 클러스터 전체 부트스트랩 오케스트레이터
# 용도: 10(레지스트리) → 11(클러스터) → 12(이미지 푸시) → 20(차트 준비) → 21(설치) → 99(검증) 순서 실행
# 사용법: ./bootstrap.sh
# 필수 환경변수:
#   SKIP_LOAD    - "1" 이면 번들 로드 단계(03) 건너뜀 (기본값: 0)
#   SKIP_VERIFY  - "1" 이면 99-verify.sh 건너뜀 (기본값: 0)
#   CLUSTER_NAME - kind 클러스터 이름 (기본값: nullus-airgap)
# 종료 코드:
#   0 - 전체 성공
#   1 - 단계 실패

set -euo pipefail
IFS=$'\n\t'

# ── 경로 해석 ────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
AIRGAP_DIR="${ROOT_DIR}/airgap"

# ── 설정값 ───────────────────────────────────────────────────
SKIP_LOAD="${SKIP_LOAD:-0}"
SKIP_VERIFY="${SKIP_VERIFY:-0}"
CLUSTER_NAME="${CLUSTER_NAME:-nullus-airgap}"
export CLUSTER_NAME

# ── 로그 헬퍼 ────────────────────────────────────────────────
_tty() { [[ -t 1 ]]; }
log_info() { _tty && printf '\033[0;32m[INFO]\033[0m %s\n' "$*" || printf '[INFO] %s\n' "$*"; }
log_warn() { _tty && printf '\033[0;33m[WARN]\033[0m %s\n' "$*" || printf '[WARN] %s\n' "$*"; }
log_err()  { _tty && printf '\033[0;31m[ERR ]\033[0m %s\n' "$*" >&2 || printf '[ERR ] %s\n' "$*" >&2; }

# ── 단계 실행 헬퍼 ───────────────────────────────────────────
TOTAL_STEPS=6
CURRENT_STEP=0

step() {
  CURRENT_STEP=$((CURRENT_STEP + 1))
  local script="$1"
  local label="$2"
  log_info ""
  log_info "══════════════════════════════════════════════"
  log_info "  ==> 단계 ${CURRENT_STEP}/${TOTAL_STEPS}: ${label}"
  log_info "══════════════════════════════════════════════"

  if [[ ! -x "${script}" ]]; then
    log_warn "스크립트 없음 또는 실행 권한 없음: ${script} — 건너뜀"
    return 0
  fi

  "${script}"
}

# 다른 에이전트 소유 스크립트 — 존재할 경우에만 실행
optional_step() {
  CURRENT_STEP=$((CURRENT_STEP + 1))
  local script="$1"
  local label="$2"
  log_info ""
  log_info "══════════════════════════════════════════════"
  log_info "  ==> 단계 ${CURRENT_STEP}/${TOTAL_STEPS}: ${label} [선택]"
  log_info "══════════════════════════════════════════════"

  if [[ -x "${script}" ]]; then
    "${script}"
  else
    log_warn "스크립트 없음: ${script}"
    log_warn "  → Agent 3(차트 패키지)가 해당 스크립트를 제공합니다. 수동으로 실행하세요."
  fi
}

# ── 메인 ─────────────────────────────────────────────────────
main() {
  log_info "Nullus 에어갭 부트스트랩 시작"
  log_info "클러스터: ${CLUSTER_NAME}"
  log_info "SKIP_LOAD=${SKIP_LOAD}, SKIP_VERIFY=${SKIP_VERIFY}"

  # 단계 1/6: 로컬 레지스트리 기동 (Agent 2 소유)
  step "${AIRGAP_DIR}/scripts/10-setup-registry.sh" \
    "로컬 레지스트리 기동 (registry:2)"

  # 단계 2/6: kind 클러스터 생성 (Agent 2 소유)
  step "${AIRGAP_DIR}/scripts/11-create-cluster.sh" \
    "kind 클러스터 생성 (${CLUSTER_NAME})"

  # 단계 3/6: 번들 이미지 로드 (Agent 1 소유 — 03-load-images.sh)
  CURRENT_STEP=$((CURRENT_STEP + 1))
  log_info ""
  log_info "══════════════════════════════════════════════"
  log_info "  ==> 단계 ${CURRENT_STEP}/${TOTAL_STEPS}: 번들 이미지 docker load"
  log_info "══════════════════════════════════════════════"
  if [[ "${SKIP_LOAD}" == "1" ]]; then
    log_info "SKIP_LOAD=1 — 번들 로드 건너뜀 (이미 로드된 것으로 가정)"
  else
    local load_script="${AIRGAP_DIR}/scripts/03-load-bundle.sh"
    if [[ -x "${load_script}" ]]; then
      "${load_script}"
    else
      log_warn "로드 스크립트 없음: ${load_script}"
      log_warn "  → Agent 1(번들)이 해당 스크립트를 제공합니다."
      log_warn "  → SKIP_LOAD=1 로 이 단계를 건너뛸 수 있습니다."
    fi
  fi

  # 단계 4/6: 이미지 레지스트리 푸시 (Agent 2 소유)
  step "${AIRGAP_DIR}/scripts/12-push-to-registry.sh" \
    "이미지 로컬 레지스트리 푸시"

  # 단계 5/6: Helm 차트 준비 + 설치 (Agent 3 소유 — 20, 21)
  optional_step "${AIRGAP_DIR}/scripts/20-bundle-charts.sh" \
    "Helm 차트 번들 준비 (Agent 3)"

  optional_step "${AIRGAP_DIR}/scripts/21-install-nullus.sh" \
    "Helm 차트 설치 (Agent 3)"

  # 단계 6/6: 검증
  CURRENT_STEP=$((CURRENT_STEP + 1))
  log_info ""
  log_info "══════════════════════════════════════════════"
  log_info "  ==> 단계 ${CURRENT_STEP}/${TOTAL_STEPS}: 클러스터 검증"
  log_info "══════════════════════════════════════════════"
  if [[ "${SKIP_VERIFY}" == "1" ]]; then
    log_info "SKIP_VERIFY=1 — 검증 건너뜀"
  else
    local verify_script="${AIRGAP_DIR}/scripts/99-verify.sh"
    if [[ -x "${verify_script}" ]]; then
      "${verify_script}"
    else
      log_warn "검증 스크립트 없음: ${verify_script}"
    fi
  fi

  log_info ""
  log_info "══════════════════════════════════════════════"
  log_info "  부트스트랩 완료"
  log_info "  kubectl --context kind-${CLUSTER_NAME} get nodes"
  log_info "══════════════════════════════════════════════"
}

main "$@"
