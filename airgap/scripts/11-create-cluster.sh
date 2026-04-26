#!/usr/bin/env bash
# 목적: kind 클러스터(nullus-airgap)를 생성하고 로컬 레지스트리를 연결한다
# 용도: 에어갭 환경 kind 클러스터 초기화
# 사용법: ./11-create-cluster.sh
# 필수 환경변수:
#   CLUSTER_NAME   - 클러스터 이름 (기본값: nullus-airgap)
#   REGISTRY_NAME  - 레지스트리 컨테이너 이름 (기본값: kind-registry)
#   KIND_NETWORK   - 도커 네트워크 이름 (기본값: kind)
# 종료 코드:
#   0 - 성공 (신규 생성 또는 이미 존재)
#   1 - 실패

set -euo pipefail
IFS=$'\n\t'

# ── 경로 해석 ────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
AIRGAP_DIR="${ROOT_DIR}/airgap"

# ── 설정값 ───────────────────────────────────────────────────
CLUSTER_NAME="${CLUSTER_NAME:-nullus-airgap}"
REGISTRY_NAME="${REGISTRY_NAME:-kind-registry}"
KIND_NETWORK="${KIND_NETWORK:-kind}"
KIND_CONFIG="${AIRGAP_DIR}/kind/kind-airgap.yaml"
REGISTRY_MANIFEST="${AIRGAP_DIR}/kind/registry.yaml"
DRY_RUN="${DRY_RUN:-0}"

# ── 로그 헬퍼 ────────────────────────────────────────────────
_tty() { [[ -t 1 ]]; }
log_info() { _tty && printf '\033[0;32m[INFO]\033[0m %s\n' "$*" || printf '[INFO] %s\n' "$*"; }
log_warn() { _tty && printf '\033[0;33m[WARN]\033[0m %s\n' "$*" || printf '[WARN] %s\n' "$*"; }
log_err()  { _tty && printf '\033[0;31m[ERR ]\033[0m %s\n' "$*" >&2 || printf '[ERR ] %s\n' "$*" >&2; }

# ── DRY_RUN 래퍼 ─────────────────────────────────────────────
run() {
  if [[ "${DRY_RUN}" == "1" ]]; then
    log_info "[DRY_RUN] $*"
  else
    "$@"
  fi
}

# ── 메인 ─────────────────────────────────────────────────────
main() {
  log_info "==> kind 클러스터 생성 시작: ${CLUSTER_NAME}"

  # kind 설정 파일 존재 확인
  if [[ ! -f "${KIND_CONFIG}" ]]; then
    log_err "kind 설정 파일 없음: ${KIND_CONFIG}"
    exit 1
  fi

  # 클러스터 이미 존재하는지 확인 (멱등성)
  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    log_info "클러스터 '${CLUSTER_NAME}' 이미 존재 — 건너뜀"
  else
    log_info "kind 클러스터 생성 중 (설정: ${KIND_CONFIG})"
    run kind create cluster \
      --name "${CLUSTER_NAME}" \
      --config "${KIND_CONFIG}"
    log_info "클러스터 생성 완료"
  fi

  # 레지스트리 컨테이너를 kind 네트워크에 연결 (이미 연결된 경우 무시)
  if docker inspect "${REGISTRY_NAME}" &>/dev/null; then
    local connected
    connected="$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' "${REGISTRY_NAME}" 2>/dev/null)"
    if echo "${connected}" | grep -q "${KIND_NETWORK}"; then
      log_info "레지스트리 '${REGISTRY_NAME}' 이미 '${KIND_NETWORK}' 네트워크에 연결됨 — 건너뜀"
    else
      log_info "레지스트리 '${REGISTRY_NAME}'를 '${KIND_NETWORK}' 네트워크에 연결"
      run docker network connect "${KIND_NETWORK}" "${REGISTRY_NAME}"
    fi
  else
    log_warn "레지스트리 컨테이너 '${REGISTRY_NAME}' 없음 — 10-setup-registry.sh 를 먼저 실행하세요"
  fi

  # registry.yaml ConfigMap 적용
  if [[ -f "${REGISTRY_MANIFEST}" ]]; then
    log_info "레지스트리 탐색 ConfigMap 적용: ${REGISTRY_MANIFEST}"
    run kubectl apply -f "${REGISTRY_MANIFEST}" \
      --context "kind-${CLUSTER_NAME}"
  else
    log_warn "레지스트리 매니페스트 없음: ${REGISTRY_MANIFEST}"
  fi

  log_info "==> 클러스터 준비 완료: kubectl --context kind-${CLUSTER_NAME}"
}

main "$@"
