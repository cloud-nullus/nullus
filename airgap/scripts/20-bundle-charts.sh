#!/usr/bin/env bash
# =============================================================================
# 20-bundle-charts.sh — Helm 차트 번들 생성 (연결된 머신에서 실행)
# =============================================================================
# 용도: helm dep update 실행 후 차트 아티팩트를 airgap/helm/ 에 복사한다.
#       에어갭 환경 전달 전 반드시 이 스크립트를 실행해야 한다.
#
# 사용법:
#   ./airgap/scripts/20-bundle-charts.sh
#   DRY_RUN=1 ./airgap/scripts/20-bundle-charts.sh  # 실제 변경 없이 확인
#
# 필수 환경:
#   - helm 3.x 설치 및 PATH 등록
#   - 인터넷 연결 (bitnami repo 접근 가능)
#
# 종료 코드:
#   0 — 성공
#   1 — 의존성 오류 (helm 미설치 등)
#   2 — helm dep update 실패
#   3 — helm package 실패
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"        # airgap/
REPO_ROOT="$(cd "${ROOT_DIR}/.." && pwd)"          # 리포지터리 루트

CHART_SRC="${REPO_ROOT}/deploy/helm/nullus"
BUNDLE_CHARTS_DIR="${ROOT_DIR}/helm/charts"
BUNDLE_HELM_DIR="${ROOT_DIR}/helm"

DRY_RUN="${DRY_RUN:-0}"

# -----------------------------------------------------------------------------
# 유틸리티
# -----------------------------------------------------------------------------
log_info()  { printf '\033[32m[INFO ]\033[0m %s\n' "$*"; }
log_warn()  { printf '\033[33m[WARN ]\033[0m %s\n' "$*"; }
log_err()   { printf '\033[31m[ERROR]\033[0m %s\n' "$*" >&2; }

run() {
  if [[ "${DRY_RUN}" == "1" ]]; then
    printf '\033[36m[DRY  ]\033[0m %s\n' "$*"
  else
    "$@"
  fi
}

# -----------------------------------------------------------------------------
# 의존성 확인
# -----------------------------------------------------------------------------
check_deps() {
  log_info "의존성 확인 중..."
  if ! command -v helm &>/dev/null; then
    log_err "helm 이 설치되어 있지 않습니다. 설치 후 재실행하세요."
    exit 1
  fi
  log_info "helm 버전: $(helm version --short)"
}

# -----------------------------------------------------------------------------
# helm dep update
# -----------------------------------------------------------------------------
update_deps() {
  log_info "helm dep update 실행: ${CHART_SRC}"
  run helm dep update "${CHART_SRC}"
  log_info "의존성 업데이트 완료"
}

# -----------------------------------------------------------------------------
# 의존성 차트(.tgz) 및 Chart.lock 복사
# -----------------------------------------------------------------------------
copy_charts() {
  log_info "차트 번들 디렉터리 준비: ${BUNDLE_CHARTS_DIR}"
  run mkdir -p "${BUNDLE_CHARTS_DIR}"

  local charts_src="${CHART_SRC}/charts"
  if [[ ! -d "${charts_src}" ]]; then
    log_err "charts/ 디렉터리가 없습니다. helm dep update 를 먼저 실행하세요: ${charts_src}"
    exit 2
  fi

  # 의존성 .tgz 파일 복사
  local count=0
  for tgz in "${charts_src}"/*.tgz; do
    [[ -f "${tgz}" ]] || continue
    log_info "  복사: $(basename "${tgz}")"
    run cp -f "${tgz}" "${BUNDLE_CHARTS_DIR}/"
    (( count++ )) || true
  done

  if [[ "${count}" -eq 0 ]]; then
    log_warn "복사할 .tgz 파일이 없습니다. helm dep update 결과를 확인하세요."
  fi

  # Chart.lock 복사
  local lock_src="${CHART_SRC}/Chart.lock"
  if [[ -f "${lock_src}" ]]; then
    log_info "  복사: Chart.lock"
    run cp -f "${lock_src}" "${BUNDLE_HELM_DIR}/Chart.lock"
  else
    log_warn "Chart.lock 파일이 없습니다: ${lock_src}"
  fi

  log_info "${count}개 차트 복사 완료"
}

# -----------------------------------------------------------------------------
# helm package 실행 및 sha256 체크섬 생성
# -----------------------------------------------------------------------------
package_chart() {
  log_info "helm package 실행: ${CHART_SRC} → ${BUNDLE_HELM_DIR}"
  run helm package "${CHART_SRC}" -d "${BUNDLE_HELM_DIR}"

  # sha256 체크섬 생성
  local pkg
  if [[ "${DRY_RUN}" != "1" ]]; then
    pkg=$(ls -t "${BUNDLE_HELM_DIR}"/nullus-*.tgz 2>/dev/null | head -1)
    if [[ -z "${pkg}" ]]; then
      log_err "패키지된 차트 파일을 찾을 수 없습니다."
      exit 3
    fi
    local sum_file="${pkg}.sha256"
    shasum -a 256 "${pkg}" > "${sum_file}"
    log_info "체크섬 저장: ${sum_file}"
    log_info "  $(cat "${sum_file}")"
  else
    log_info "[DRY] sha256 체크섬 파일 생성 (nullus-*.tgz.sha256)"
  fi
}

# -----------------------------------------------------------------------------
# 메인
# -----------------------------------------------------------------------------
main() {
  if [[ "${DRY_RUN}" == "1" ]]; then
    log_warn "DRY_RUN 모드 활성화 — 실제 변경 없이 명령만 출력합니다."
  fi

  check_deps
  update_deps
  copy_charts
  package_chart

  log_info "=== 번들 생성 완료 ==="
  log_info "  차트 아티팩트 : ${BUNDLE_CHARTS_DIR}/"
  log_info "  패키지         : ${BUNDLE_HELM_DIR}/nullus-*.tgz"
  log_info "  에어갭 환경에 전달 후 21-install-nullus.sh 를 실행하세요."
}

main "$@"
