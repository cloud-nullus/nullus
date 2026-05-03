#!/usr/bin/env bash
# =============================================================================
# 21-install-nullus.sh — 에어갭 kind 클러스터에 Nullus 플랫폼 설치
# =============================================================================
# 용도: 오프라인 환경의 kind 클러스터에 Helm 으로 Nullus 를 배포한다.
#       20-bundle-charts.sh 로 생성된 패키지와 values-airgap.yaml 을 사용한다.
#
# 사용법:
#   ./airgap/scripts/21-install-nullus.sh
#   RELEASE=nullus NAMESPACE=nullus EXTRA_ARGS="--set secrets.dbPassword=xxx" \
#     ./airgap/scripts/21-install-nullus.sh
#
# 환경 변수:
#   RELEASE    — Helm 릴리스명 (기본값: nullus)
#   NAMESPACE  — 대상 네임스페이스 (기본값: nullus)
#   EXTRA_ARGS — helm upgrade 에 추가할 인수 (예: --set secrets.dbPassword=xxx)
#   DRY_RUN    — 1 로 설정 시 실제 변경 없이 명령만 출력
#
# 종료 코드:
#   0 — 성공
#   1 — 의존성 오류 (helm/kubectl 미설치 등)
#   2 — 차트 파일 없음
#   3 — helm upgrade 실패
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"        # airgap/
REPO_ROOT="$(cd "${ROOT_DIR}/.." && pwd)"          # 리포지터리 루트

BUNDLE_HELM_DIR="${ROOT_DIR}/helm"
VALUES_FILE="${BUNDLE_HELM_DIR}/values-airgap.yaml"

RELEASE="${RELEASE:-nullus}"
NAMESPACE="${NAMESPACE:-nullus}"
EXTRA_ARGS="${EXTRA_ARGS:-}"
DRY_RUN="${DRY_RUN:-0}"

# -----------------------------------------------------------------------------
# 유틸리티
# -----------------------------------------------------------------------------
log_info()  { printf '\033[32m[INFO ]\033[0m %s\n' "$*" >&2; }
log_warn()  { printf '\033[33m[WARN ]\033[0m %s\n' "$*" >&2; }
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
  local missing=0
  for cmd in helm kubectl; do
    if ! command -v "${cmd}" &>/dev/null; then
      log_err "${cmd} 이 설치되어 있지 않습니다."
      (( missing++ )) || true
    fi
  done
  if [[ "${missing}" -gt 0 ]]; then
    exit 1
  fi
  log_info "helm: $(helm version --short), kubectl: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
}

# -----------------------------------------------------------------------------
# 차트 패키지 파일 검증
# -----------------------------------------------------------------------------
find_chart() {
  local chart
  chart=$(ls -t "${BUNDLE_HELM_DIR}"/nullus-*.tgz 2>/dev/null | head -1)
  if [[ -z "${chart}" ]]; then
    log_err "차트 파일을 찾을 수 없습니다: ${BUNDLE_HELM_DIR}/nullus-*.tgz"
    log_err "연결된 머신에서 20-bundle-charts.sh 를 실행한 뒤 파일을 전달하세요."
    exit 2
  fi

  # sha256 체크섬 검증 (파일이 있는 경우에만)
  local sum_file="${chart}.sha256"
  if [[ -f "${sum_file}" ]]; then
    log_info "sha256 체크섬 검증 중..."
    if shasum -a 256 --check "${sum_file}" &>/dev/null; then
      log_info "체크섬 검증 통과: $(basename "${chart}")"
    else
      log_warn "체크섬 불일치 — 파일이 손상되었을 수 있습니다. 계속 진행합니다."
    fi
  else
    log_warn "체크섬 파일 없음: ${sum_file} (검증 생략)"
  fi

  echo "${chart}"
}

# -----------------------------------------------------------------------------
# 네임스페이스 생성 (idempotent)
# -----------------------------------------------------------------------------
ensure_namespace() {
  log_info "네임스페이스 확인/생성: ${NAMESPACE}"
  if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
    log_info "  네임스페이스 이미 존재: ${NAMESPACE}"
  else
    run kubectl create namespace "${NAMESPACE}"
    log_info "  네임스페이스 생성 완료: ${NAMESPACE}"
  fi
}

# -----------------------------------------------------------------------------
# Helm 설치/업그레이드
# -----------------------------------------------------------------------------
install_chart() {
  local chart="$1"

  log_info "Helm 배포 시작..."
  log_info "  릴리스   : ${RELEASE}"
  log_info "  네임스페이스: ${NAMESPACE}"
  log_info "  차트     : $(basename "${chart}")"
  log_info "  Values   : ${VALUES_FILE}"

  if [[ ! -f "${VALUES_FILE}" ]]; then
    log_err "values 파일이 없습니다: ${VALUES_FILE}"
    exit 2
  fi

  # EXTRA_ARGS 는 단어 분리가 필요하므로 eval 대신 배열로 처리
  local extra_arr=()
  if [[ -n "${EXTRA_ARGS}" ]]; then
    # shellcheck disable=SC2086
    read -ra extra_arr <<< "${EXTRA_ARGS}"
  fi

  run helm upgrade --install "${RELEASE}" "${chart}" \
    --namespace "${NAMESPACE}" \
    --values "${VALUES_FILE}" \
    --wait \
    --timeout 10m \
    "${extra_arr[@]+"${extra_arr[@]}"}" \
    || { log_err "helm upgrade 실패"; exit 3; }

  log_info "Helm 배포 완료"
}

# -----------------------------------------------------------------------------
# 배포 상태 출력
# -----------------------------------------------------------------------------
show_status() {
  log_info "=== 배포 결과 ==="
  if [[ "${DRY_RUN}" != "1" ]]; then
    kubectl get pods -n "${NAMESPACE}"
  else
    log_info "[DRY] kubectl get pods -n ${NAMESPACE}"
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
  local chart
  chart=$(find_chart)
  ensure_namespace
  install_chart "${chart}"
  show_status

  log_info "=== Nullus 설치 완료 ==="
  log_info "  secrets.dbPassword / secrets.encryptionKey 가 기본값이라면"
  log_info "  반드시 --set 또는 SealedSecret 으로 교체하세요."
}

main "$@"
