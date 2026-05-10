#!/usr/bin/env bash
# =============================================================================
# 22-install-platform-stack.sh — Nullus 플랫폼 동작에 필요한 부수 컴포넌트 설치
# =============================================================================
# 용도: airgap/helm/charts-catalog/ 에 번들된 chart 로 다음을 설치한다.
#
#   기본 (필수): Keycloak — OIDC 인증 (auth.mode=oidc 일 때 필요)
#   옵션 (INSTALL_FULL=1):
#     - kube-prometheus-stack — 모니터링 (Prometheus + Grafana)
#
# Harbor / MinIO / ArgoCD 는 카탈로그에 포함되어 있으나 본 스크립트는 설치하지
# 않는다. Nullus UI 의 "Stack 설치" 기능에서 사용자가 선택 시 설치된다.
#
# 사용법:
#   ./22-install-platform-stack.sh
#   INSTALL_FULL=1 ./22-install-platform-stack.sh    # + kube-prometheus-stack
#   SKIP_KEYCLOAK=1 ./22-install-platform-stack.sh   # Keycloak 건너뜀
#
# 환경 변수:
#   NAMESPACE_AUTH      Keycloak 네임스페이스 (기본: nullus-auth)
#   NAMESPACE_OBSERV    Prometheus stack 네임스페이스 (기본: nullus-monitoring)
#   KEYCLOAK_ADMIN      관리자 계정 (기본: admin)
#   KEYCLOAK_PASSWORD   관리자 비밀번호 (기본: admin)
#   INSTALL_FULL        1 이면 kube-prometheus-stack 도 설치
#   SKIP_KEYCLOAK       1 이면 Keycloak 건너뜀
#
# 종료 코드:
#   0 — 성공
#   1 — chart 없음 / helm 실패
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CATALOG_DIR="${ROOT_DIR}/helm/charts-catalog"

NAMESPACE_AUTH="${NAMESPACE_AUTH:-nullus-auth}"
NAMESPACE_OBSERV="${NAMESPACE_OBSERV:-nullus-monitoring}"
KEYCLOAK_ADMIN="${KEYCLOAK_ADMIN:-admin}"
KEYCLOAK_PASSWORD="${KEYCLOAK_PASSWORD:-admin}"
INSTALL_FULL="${INSTALL_FULL:-0}"
SKIP_KEYCLOAK="${SKIP_KEYCLOAK:-0}"
REGISTRY_HOST="${REGISTRY_HOST:-localhost:5001}"

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_OK=$'\033[1;32m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_OK=""; CL_WARN=""; CL_ERR=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }

command -v helm    >/dev/null || { log_err "helm not found";    exit 1; }
command -v kubectl >/dev/null || { log_err "kubectl not found"; exit 1; }

[[ -d "$CATALOG_DIR" ]] || { log_err "카탈로그 디렉토리 없음: $CATALOG_DIR"; exit 1; }

find_chart() {
  local prefix="$1"
  local found
  found="$(ls "$CATALOG_DIR"/${prefix}-*.tgz 2>/dev/null | head -1 || true)"
  [[ -z "$found" ]] && return 1
  echo "$found"
}

ensure_ns() {
  local ns="$1"
  kubectl get ns "$ns" >/dev/null 2>&1 || kubectl create namespace "$ns" >/dev/null
}

# -----------------------------------------------------------------------------
# 1) Keycloak 설치
# -----------------------------------------------------------------------------
install_keycloak() {
  if [[ "$SKIP_KEYCLOAK" == "1" ]]; then
    log_warn "Keycloak 설치 건너뜀 (SKIP_KEYCLOAK=1)"
    return 0
  fi

  local chart
  chart="$(find_chart keycloak)" || { log_err "keycloak chart 없음 in $CATALOG_DIR"; return 1; }

  log_info "── Keycloak 설치 ──"
  log_info "  chart      : $(basename "$chart")"
  log_info "  namespace  : $NAMESPACE_AUTH"
  log_info "  registry   : $REGISTRY_HOST"

  ensure_ns "$NAMESPACE_AUTH"

  # Bitnami keycloak chart 는 docker.io/bitnami/* 를 기본 사용 →
  # 2025-08 정책에 따라 bitnamilegacy/* 로 override + Bitnami security 가드 우회.
  helm upgrade --install keycloak "$chart" \
    --namespace "$NAMESPACE_AUTH" \
    --wait --timeout 10m \
    --set "global.security.allowInsecureImages=true" \
    --set "image.registry=${REGISTRY_HOST}" \
    --set "image.repository=bitnamilegacy/keycloak" \
    --set "postgresql.image.registry=${REGISTRY_HOST}" \
    --set "postgresql.image.repository=bitnamilegacy/postgresql" \
    --set "postgresql.volumePermissions.image.registry=${REGISTRY_HOST}" \
    --set "postgresql.volumePermissions.image.repository=bitnamilegacy/os-shell" \
    --set "auth.adminUser=${KEYCLOAK_ADMIN}" \
    --set "auth.adminPassword=${KEYCLOAK_PASSWORD}" \
    --set "service.type=ClusterIP" \
    --set "production=false" \
    --set "proxy=edge" \
    --set "resources.requests.cpu=200m" \
    --set "resources.requests.memory=512Mi" \
    --set "resources.limits.cpu=1000m" \
    --set "resources.limits.memory=1Gi"

  log_ok "Keycloak 설치 완료"
  log_info "  접근: kubectl port-forward -n $NAMESPACE_AUTH svc/keycloak 8180:80"
  log_info "  로그인: ${KEYCLOAK_ADMIN} / ${KEYCLOAK_PASSWORD}"
  log_info "  Realm 설정 (수동): scripts/setup-keycloak.sh 또는 admin UI 에서 'nullus' realm 생성"
}

# -----------------------------------------------------------------------------
# 2) kube-prometheus-stack 설치 (옵션)
# -----------------------------------------------------------------------------
install_kps() {
  if [[ "$INSTALL_FULL" != "1" ]]; then
    log_info "kube-prometheus-stack 건너뜀 (INSTALL_FULL=1 로 활성화 가능)"
    return 0
  fi

  local chart
  chart="$(find_chart kube-prometheus-stack)" || { log_err "kube-prometheus-stack chart 없음"; return 1; }

  log_info "── kube-prometheus-stack 설치 ──"
  log_info "  chart      : $(basename "$chart")"
  log_info "  namespace  : $NAMESPACE_OBSERV"

  ensure_ns "$NAMESPACE_OBSERV"

  # 모든 sub-component image 는 quay.io / registry.k8s.io / docker.io 에 있음.
  # kind 노드 containerd mirror 가 이를 모두 localhost:5001 로 redirection 하므로
  # values 로 추가 override 없이도 동작한다.
  helm upgrade --install kps "$chart" \
    --namespace "$NAMESPACE_OBSERV" \
    --wait --timeout 15m \
    --set "grafana.adminPassword=admin" \
    --set "prometheus.prometheusSpec.resources.requests.cpu=100m" \
    --set "prometheus.prometheusSpec.resources.requests.memory=400Mi" \
    --set "prometheus.prometheusSpec.resources.limits.cpu=500m" \
    --set "prometheus.prometheusSpec.resources.limits.memory=1Gi"

  log_ok "kube-prometheus-stack 설치 완료"
  log_info "  Grafana : kubectl port-forward -n $NAMESPACE_OBSERV svc/kps-grafana 3000:80"
  log_info "  로그인  : admin / admin"
}

# -----------------------------------------------------------------------------
# 메인
# -----------------------------------------------------------------------------
log_info "=== Platform Stack 설치 ==="
install_keycloak
install_kps
log_ok "=== 완료 ==="

if [[ "$INSTALL_FULL" != "1" && "$SKIP_KEYCLOAK" != "1" ]]; then
  log_info ""
  log_info "kube-prometheus-stack 도 설치하려면: INSTALL_FULL=1 $0"
fi
