#!/usr/bin/env bash
# 목적: 로컬 Docker 레지스트리(registry:2) 컨테이너를 시작한다
# 용도: 에어갭 환경에서 kind 클러스터가 사용할 이미지 저장소 설정
# 사용법: ./10-setup-registry.sh
# 필수 환경변수:
#   REGISTRY_NAME  - 컨테이너 이름 (기본값: kind-registry)
#   REGISTRY_PORT  - 호스트 바인딩 포트 (기본값: 5001)
# 종료 코드:
#   0 - 성공 (신규 기동 또는 이미 실행 중)
#   1 - 실패

set -euo pipefail
IFS=$'\n\t'

# ── 경로 해석 ────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# ── 설정값 ───────────────────────────────────────────────────
REGISTRY_NAME="${REGISTRY_NAME:-kind-registry}"
REGISTRY_PORT="${REGISTRY_PORT:-5001}"
REGISTRY_IMAGE="registry:2"
KIND_NETWORK="kind"
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
  log_info "==> 로컬 레지스트리 설정 시작 (${REGISTRY_NAME}:${REGISTRY_PORT})"

  # 이미 실행 중인지 확인
  if docker inspect "${REGISTRY_NAME}" &>/dev/null; then
    local state
    state="$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null)"
    if [[ "${state}" == "true" ]]; then
      log_info "레지스트리 컨테이너 '${REGISTRY_NAME}' 이미 실행 중 — 건너뜀"
    else
      log_warn "레지스트리 컨테이너 '${REGISTRY_NAME}' 존재하나 중지 상태 — 재시작"
      run docker start "${REGISTRY_NAME}"
    fi
  else
    log_info "레지스트리 컨테이너 신규 기동: ${REGISTRY_IMAGE} → 127.0.0.1:${REGISTRY_PORT}:5000"
    run docker run \
      --detach \
      --restart=always \
      --name "${REGISTRY_NAME}" \
      --publish "127.0.0.1:${REGISTRY_PORT}:5000" \
      "${REGISTRY_IMAGE}"
    log_info "레지스트리 컨테이너 기동 완료"
  fi

  # kind 도커 네트워크 생성 (없을 경우)
  if ! docker network inspect "${KIND_NETWORK}" &>/dev/null; then
    log_info "도커 네트워크 '${KIND_NETWORK}' 생성"
    run docker network create "${KIND_NETWORK}"
  else
    log_info "도커 네트워크 '${KIND_NETWORK}' 이미 존재 — 건너뜀"
  fi

  log_info "==> 로컬 레지스트리 설정 완료: http://localhost:${REGISTRY_PORT}"
}

main "$@"
