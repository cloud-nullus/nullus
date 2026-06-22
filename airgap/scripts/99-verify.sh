#!/usr/bin/env bash
# 목적: 에어갭 kind 클러스터 상태를 검증하고 pass/fail 요약 출력
# 용도: 부트스트랩 완료 후 환경 정상 여부 최종 확인
# 사용법: ./99-verify.sh
# 필수 환경변수:
#   CLUSTER_NAME  - kind 클러스터 이름 (기본값: nullus-airgap)
#   EXPECTED_ARCH - 기대 노드 아키텍처 (기본값: amd64. arm64 번들이면 arm64)
# 종료 코드:
#   0 - 전체 검증 통과
#   1 - 하나 이상 검증 실패

set -euo pipefail
IFS=$'\n\t'

# ── 경로 해석 ────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# ── 설정값 ───────────────────────────────────────────────────
CLUSTER_NAME="${CLUSTER_NAME:-nullus-airgap}"
KUBE_CONTEXT="kind-${CLUSTER_NAME}"
EXPECTED_ARCH="${EXPECTED_ARCH:-amd64}"

# ── 로그 헬퍼 ────────────────────────────────────────────────
_tty() { [[ -t 1 ]]; }
log_info() { _tty && printf '\033[0;32m[INFO]\033[0m %s\n' "$*" || printf '[INFO] %s\n' "$*"; }
log_warn() { _tty && printf '\033[0;33m[WARN]\033[0m %s\n' "$*" || printf '[WARN] %s\n' "$*"; }
log_err()  { _tty && printf '\033[0;31m[ERR ]\033[0m %s\n' "$*" >&2 || printf '[ERR ] %s\n' "$*" >&2; }

# ── 검증 결과 추적 ────────────────────────────────────────────
declare -a CHECK_NAMES=()
declare -a CHECK_RESULTS=()

record() {
  local name="$1"
  local result="$2"  # PASS | FAIL | WARN
  CHECK_NAMES+=("${name}")
  CHECK_RESULTS+=("${result}")
}

run_check() {
  local name="$1"
  shift
  log_info "검사 중: ${name}"
  if "$@" 2>&1; then
    record "${name}" "PASS"
  else
    record "${name}" "FAIL"
  fi
}

# ── 개별 검증 함수 ────────────────────────────────────────────

check_cluster_info() {
  kubectl cluster-info --context "${KUBE_CONTEXT}"
}

check_nodes() {
  log_info "── 노드 상태 ──────────────────────────────────"
  kubectl get nodes -o wide --context "${KUBE_CONTEXT}"
  # 모든 노드가 Ready 인지 확인
  local not_ready
  not_ready="$(kubectl get nodes --context "${KUBE_CONTEXT}" \
    --no-headers | grep -v " Ready" | wc -l | tr -d ' ')"
  if [[ "${not_ready}" -gt 0 ]]; then
    log_warn "Ready 상태가 아닌 노드: ${not_ready}개"
    return 1
  fi
}

check_architecture() {
  log_info "── 노드 아키텍처 검증 (기대값: ${EXPECTED_ARCH}) ──"
  local archs node_name node_arch mismatched=0
  archs="$(kubectl get nodes --context "${KUBE_CONTEXT}" \
    -o jsonpath='{range .items[*]}{.metadata.name}={.status.nodeInfo.architecture}{"\n"}{end}')"
  if [[ -z "${archs}" ]]; then
    log_warn "노드 아키텍처 정보를 가져올 수 없음"
    return 1
  fi
  while IFS='=' read -r node_name node_arch; do
    [[ -z "${node_name}" ]] && continue
    if [[ "${node_arch}" == "${EXPECTED_ARCH}" ]]; then
      log_info "  ${node_name}: ${node_arch} (일치)"
    else
      log_err "  ${node_name}: ${node_arch} — 기대값 ${EXPECTED_ARCH} 와 불일치"
      mismatched=$((mismatched + 1))
    fi
  done <<< "${archs}"
  if [[ "${mismatched}" -gt 0 ]]; then
    log_err "아키텍처 불일치 노드 ${mismatched}개 — 번들 플랫폼과 타겟 호스트 아키텍처를 확인하세요 (arm64 번들을 x86 에 반입했을 가능성)."
    return 1
  fi
}

check_pods() {
  log_info "── 전체 파드 상태 ─────────────────────────────"
  kubectl get pods -A --context "${KUBE_CONTEXT}"
}

check_registry_reachable() {
  log_info "── 로컬 레지스트리 응답 확인 ──────────────────"
  if curl -sf http://localhost:5001/v2/ >/dev/null; then
    log_info "레지스트리 응답 정상: http://localhost:5001/v2/"
  else
    log_warn "레지스트리 응답 없음 (10-setup-registry.sh 실행 필요)"
    return 1
  fi
}

check_node_images() {
  log_info "── 클러스터 노드 이미지 목록 (상위 20개) ──────"
  local control_plane_node
  control_plane_node="$(kubectl get nodes --context "${KUBE_CONTEXT}" \
    -l node-role.kubernetes.io/control-plane \
    --no-headers -o custom-columns=':metadata.name' | head -1)"

  if [[ -z "${control_plane_node}" ]]; then
    log_warn "컨트롤 플레인 노드를 찾을 수 없음"
    return 1
  fi

  log_info "노드: ${control_plane_node}"
  docker exec "${control_plane_node}" crictl images 2>/dev/null | head -20 || {
    log_warn "crictl images 실행 실패 (노드가 kind 컨테이너가 아닐 수 있음)"
    return 1
  }
}

# ── 요약 출력 ─────────────────────────────────────────────────
print_summary() {
  log_info ""
  log_info "══════════════════════════════════════════════"
  log_info "  검증 결과 요약"
  log_info "══════════════════════════════════════════════"

  local failed=0
  local i
  for i in "${!CHECK_NAMES[@]}"; do
    local name="${CHECK_NAMES[$i]}"
    local result="${CHECK_RESULTS[$i]}"
    if [[ "${result}" == "PASS" ]]; then
      _tty && printf '  \033[0;32m%-8s\033[0m %s\n' "[PASS]" "${name}" \
             || printf '  %-8s %s\n' "[PASS]" "${name}"
    elif [[ "${result}" == "WARN" ]]; then
      _tty && printf '  \033[0;33m%-8s\033[0m %s\n' "[WARN]" "${name}" \
             || printf '  %-8s %s\n' "[WARN]" "${name}"
    else
      _tty && printf '  \033[0;31m%-8s\033[0m %s\n' "[FAIL]" "${name}" \
             || printf '  %-8s %s\n' "[FAIL]" "${name}"
      failed=$((failed + 1))
    fi
  done

  log_info "══════════════════════════════════════════════"

  if [[ ${failed} -gt 0 ]]; then
    log_err "최종 결과: FAIL (${failed}개 실패)"
    return 1
  else
    log_info "최종 결과: PASS — 클러스터 준비 완료"
  fi
}

# ── 메인 ─────────────────────────────────────────────────────
main() {
  log_info "==> 에어갭 클러스터 검증 시작: ${CLUSTER_NAME}"
  log_info ""

  run_check "클러스터 API 서버 응답" check_cluster_info
  run_check "노드 Ready 상태" check_nodes
  run_check "노드 아키텍처 (${EXPECTED_ARCH})" check_architecture
  run_check "전체 파드 조회" check_pods
  run_check "로컬 레지스트리 응답" check_registry_reachable
  run_check "노드 이미지 목록 (crictl)" check_node_images

  print_summary
}

main "$@"
