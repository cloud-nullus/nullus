#!/usr/bin/env bash
# =============================================================================
# 28-push-charts-oci.sh — DevSecOps 카탈로그 Helm 차트 → 로컬 OCI 레지스트리 push
# =============================================================================
# 용도: airgap/helm/charts-catalog/*.tgz 를 로컬 OCI 레지스트리에 push 하여
#       인-클러스터 Nullus 오케스트레이터(UI "Stack 설치")가 오프라인으로 차트를
#       pull 할 수 있게 한다.
#
# 사용법:
#   ./28-push-charts-oci.sh
#
# 환경 변수:
#   REGISTRY_HOST   OCI 레지스트리 호스트:포트 (기본: localhost:5001)
#                   인-클러스터에서는 kind-registry:5000 으로 사용
#
# 레지스트리 경로 규칙:
#   oci://<REGISTRY_HOST>/charts/<chartName>:<chartVersion>
#   예) oci://localhost:5001/charts/cert-manager:v1.16.3
#       oci://kind-registry:5000/charts/cert-manager:v1.16.3  (클러스터 내부)
#
# nullus-api 배포 시 필요한 환경 변수:
#   NULLUS_HELM_OCI_REGISTRY=kind-registry:5000/charts
#
# 종료 코드:
#   0 — 전체 성공
#   1 — 1개 이상 push 실패
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

# ---------------------------------------------------------------------------
# 경로 해석: 두 가지 레이아웃 지원
#   1) 원본 repo 트리  : <repo>/airgap/scripts/28-push-charts-oci.sh
#      → AIRGAP_DIR = <repo>/airgap/
#   2) 패키징된 번들   : <bundle>/scripts/28-push-charts-oci.sh
#      → AIRGAP_DIR = <bundle>/
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -d "${PARENT_DIR}/helm/charts-catalog" ]]; then
  AIRGAP_DIR="${PARENT_DIR}"
elif [[ -d "${PARENT_DIR}/airgap/helm/charts-catalog" ]]; then
  AIRGAP_DIR="${PARENT_DIR}/airgap"
else
  AIRGAP_DIR="${SCRIPT_DIR%/scripts}"
fi

CATALOG_DIR="${AIRGAP_DIR}/helm/charts-catalog"

# ---------------------------------------------------------------------------
# 색상 로그 헬퍼
# ---------------------------------------------------------------------------
if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_OK=$'\033[1;32m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_OK=""; CL_WARN=""; CL_ERR=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }
hdr() {
  local sep="------------------------------------------------------------"
  printf '%s\n[%s] %s\n%s\n' \
    "${CL_INFO}${sep}${CL_RST}" \
    "${CL_INFO}$1${CL_RST}" "$2" \
    "${CL_INFO}${sep}${CL_RST}" >&2
}

# ---------------------------------------------------------------------------
# 환경 변수
# ---------------------------------------------------------------------------
REGISTRY_HOST="${REGISTRY_HOST:-localhost:5001}"
OCI_PREFIX="oci://${REGISTRY_HOST}/charts"

# ---------------------------------------------------------------------------
# 사전 점검
# ---------------------------------------------------------------------------
command -v helm >/dev/null || { log_err "helm not found"; exit 1; }

if [[ ! -d "$CATALOG_DIR" ]]; then
  log_err "카탈로그 디렉토리 없음: $CATALOG_DIR"
  exit 1
fi

CHARTS=()
for f in "${CATALOG_DIR}"/*.tgz; do
  [[ -f "$f" ]] && CHARTS+=("$f")
done
if [[ ${#CHARTS[@]} -eq 0 ]]; then
  log_err "*.tgz 파일 없음: $CATALOG_DIR"
  exit 1
fi

hdr "OCI Push" "카탈로그 차트 → ${OCI_PREFIX}  (차트 수: ${#CHARTS[@]})"
log_info "카탈로그 경로: $CATALOG_DIR"

# ---------------------------------------------------------------------------
# push 루프
# ---------------------------------------------------------------------------
RESULTS=()
FAILED=0

for tgz in "${CHARTS[@]}"; do
  name="$(basename "$tgz")"
  log_info "── $name ──────────────────────────────────────"

  if helm push "$tgz" "$OCI_PREFIX" --plain-http 2>&1 | sed 's/^/    /' >&2; then
    log_ok "  $name => OK"
    RESULTS+=("${name}|OK")
  else
    log_warn "  $name => FAIL (계속 진행)"
    RESULTS+=("${name}|FAIL")
    FAILED=1
  fi
done

# ---------------------------------------------------------------------------
# 결과 요약 테이블
# ---------------------------------------------------------------------------
log_info ""
log_info "═══════════════════════════════════════"
log_info " Push 결과 요약"
log_info "═══════════════════════════════════════"
for row in "${RESULTS[@]}"; do
  IFS='|' read -r chart status <<<"$row"
  if [[ "$status" == "OK" ]]; then
    log_ok "  $(printf '%-50s' "$chart") $status"
  else
    log_err "  $(printf '%-50s' "$chart") $status"
  fi
done
log_info "═══════════════════════════════════════"

if [[ "$FAILED" == "0" ]]; then
  log_ok "전체 ${#CHARTS[@]}개 차트 push 완료"
else
  log_warn "일부 차트 push 실패 — 위 요약 확인"
fi

# ---------------------------------------------------------------------------
# 오케스트레이터 안내
# ---------------------------------------------------------------------------
log_info ""
log_info "OCI 레지스트리 경로 프리픽스:"
log_info "  호스트(pull 테스트):   oci://localhost:5001/charts"
log_info "  인-클러스터(api 사용): oci://kind-registry:5000/charts"
log_info ""
log_info "nullus-api 배포 시 아래 환경 변수를 설정하세요:"
log_info "  NULLUS_HELM_OCI_REGISTRY=kind-registry:5000/charts"

exit "$FAILED"
