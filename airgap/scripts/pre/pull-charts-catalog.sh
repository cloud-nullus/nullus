#!/usr/bin/env bash
# =============================================================================
# pre/pull-charts-catalog.sh — Stack 카탈로그 helm chart 다운로드
# =============================================================================
# 용도: Nullus stack orchestrator (internal/stack/adapter/helm/orchestrator.go)
#       가 사용자 클러스터에 설치하는 모든 helm chart 를 에어갭 번들에 포함.
#       대상: cert-manager, metrics-server, openbao(=manifest 직접 설치, chart 없음),
#             minio, gitlab, gitlab-runner, argo-cd, kube-prometheus-stack,
#             grafana, loki, opensearch, opentelemetry-collector,
#             envoy gateway-helm(OCI), keycloak(OIDC), harbor(선택).
#
# 사용법:
#   ./pull-charts-catalog.sh
#   CATALOG_FILTER="cert-manager,gitlab" ./pull-charts-catalog.sh
#
# 환경 변수:
#   CATALOG_FILTER  쉼표 구분 (기본: 전체)
#   DRY_RUN         1 = 명령 출력만
#
# 버전 정합성: orchestrator.go 의 ChartSpec 과 일치 유지. drift 시 helm template
# 결과가 달라져 images.txt 가 잘못 생성될 수 있음.
#
# 출력: airgap/helm/charts-catalog/<chart>-<version>.tgz
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CATALOG_DIR="${ROOT_DIR}/helm/charts-catalog"

CATALOG_FILTER="${CATALOG_FILTER:-}"
DRY_RUN="${DRY_RUN:-0}"

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_OK=$'\033[1;32m'; CL_ERR=$'\033[1;31m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_OK=""; CL_ERR=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }

command -v helm >/dev/null || { log_err "helm not found"; exit 127; }

# 카탈로그 정의: name|repo-name|repo-url|chart-ref|version
# repo-name='oci' 인 경우 OCI 처리(별도 repo add 불필요, helm pull oci://... 사용).
# 버전은 orchestrator.go (internal/stack/adapter/helm/orchestrator.go) ChartSpec 과 일치 유지.
CATALOG=(
  # --- stack orchestrator 등록 chart ---
  "cert-manager|jetstack|https://charts.jetstack.io|jetstack/cert-manager|v1.16.3"
  "metrics-server|metrics-server|https://kubernetes-sigs.github.io/metrics-server/|metrics-server/metrics-server|3.12.2"
  "minio|minio-official|https://charts.min.io/|minio-official/minio|5.4.0"
  "gitlab|gitlab|https://charts.gitlab.io/|gitlab/gitlab|8.7.2"
  "gitlab-runner|gitlab|https://charts.gitlab.io/|gitlab/gitlab-runner|0.72.0"
  "argo-cd|argo|https://argoproj.github.io/argo-helm|argo/argo-cd|7.7.16"
  "kube-prometheus-stack|prometheus-community|https://prometheus-community.github.io/helm-charts|prometheus-community/kube-prometheus-stack|69.3.0"
  "grafana|grafana|https://grafana.github.io/helm-charts|grafana/grafana|8.9.0"
  "loki|grafana|https://grafana.github.io/helm-charts|grafana/loki|2.10.3"
  "opensearch|opensearch|https://opensearch-project.github.io/helm-charts|opensearch/opensearch|2.22.0"
  "opentelemetry-collector|open-telemetry|https://open-telemetry.github.io/opentelemetry-helm-charts|open-telemetry/opentelemetry-collector|0.75.0"
  "gateway-helm|oci|-|oci://registry-1.docker.io/envoyproxy/gateway-helm|v1.4.3"
  # --- platform/optional chart ---
  "keycloak|bitnami|https://charts.bitnami.com/bitnami|bitnami/keycloak|24.4.5"
  "harbor|harbor|https://helm.goharbor.io|harbor/harbor|1.15.0"
)

mkdir -p "$CATALOG_DIR"

want() {
  local name="$1"
  [[ -z "$CATALOG_FILTER" ]] && return 0
  IFS=',' read -ra arr <<< "$CATALOG_FILTER"
  for f in "${arr[@]}"; do [[ "$f" == "$name" ]] && return 0; done
  return 1
}

run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    printf 'DRY_RUN: %s\n' "$*" >&2
  else
    "$@"
  fi
}

# 레지스트리 1회 등록 (bash 3.2 호환 — assoc array 미사용)
REPO_ADDED=" "
add_repo_once() {
  local name="$1" url="$2"
  case "$REPO_ADDED" in *" $name "*) return 0 ;; esac
  if helm repo list 2>/dev/null | awk 'NR>1 {print $1}' | grep -qx "$name"; then
    log_info "helm repo '$name' 이미 등록됨"
  else
    log_info "helm repo add $name $url"
    run helm repo add "$name" "$url" >/dev/null
  fi
  REPO_ADDED+="$name "
}

log_info "=== 카탈로그 chart 다운로드 ==="
log_info "출력: $CATALOG_DIR"

for entry in "${CATALOG[@]}"; do
  IFS='|' read -r name repo url chart_ref ver <<< "$entry"
  if ! want "$name"; then
    log_info "건너뜀: $name (필터)"
    continue
  fi
  # OCI chart 는 repo add 불필요 (helm pull oci://... 직접 사용)
  [[ "$repo" == "oci" ]] && continue
  add_repo_once "$repo" "$url"
done

log_info "helm repo update"
run helm repo update >/dev/null

for entry in "${CATALOG[@]}"; do
  IFS='|' read -r name repo url chart_ref ver <<< "$entry"
  want "$name" || continue

  out="${CATALOG_DIR}/${name}-${ver}.tgz"
  if [[ -f "$out" ]]; then
    log_info "이미 존재: $(basename "$out") — 건너뜀"
    continue
  fi
  log_info "$chart_ref --version $ver"
  run helm pull "$chart_ref" --version "$ver" --destination "$CATALOG_DIR" >/dev/null
done

if [[ "$DRY_RUN" != "1" ]]; then
  log_ok "=== 카탈로그 다운로드 완료 ==="
  ls -lh "$CATALOG_DIR" | awk 'NR>1 {printf "  %s  %s\n", $5, $NF}' >&2
fi
