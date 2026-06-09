#!/usr/bin/env bash
# 00-generate-images.sh
# Purpose : Regenerate airgap/images/images.txt from `helm template` output of the
#           Nullus chart + fixed infra images (kind node, local registry).
# Usage   :
#   ./00-generate-images.sh                 # write airgap/images/images.txt
#   MODE=check ./00-generate-images.sh      # diff-only, exit 1 on drift (CI)
#   DRY_RUN=1  ./00-generate-images.sh      # print result, do not touch FS
# Requires: helm >=3.14, awk, grep, sort, diff, mktemp. Internet (for `helm dep update`).
# Exits   : 0 ok / 1 drift (MODE=check) or render error / 2 bad MODE / 127 missing tool
#
# Notes   :
#   - This script is for the ONLINE machine. It refreshes chart dependencies so
#     helm template can render every sub-chart image. On an offline machine run
#     `MODE=check` only after shipping a populated charts/ cache.
#   - Infra images (kindest/node, registry:2) are not part of the helm render;
#     they are appended explicitly. Edit INFRA_IMAGES to change versions.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$ROOT_DIR/.." && pwd)"
CHART_DIR="$REPO_ROOT/deploy/helm/nullus"
VALUES_AIRGAP="$ROOT_DIR/helm/values-airgap.yaml"
OUT_FILE="$ROOT_DIR/images/images.txt"

INFRA_IMAGES=(
  "kindest/node:v1.30.0"
  "registry:2"
  # OpenBao: installed via inline manifest by stack orchestrator (not a helm chart)
  "openbao/openbao:latest"
)

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_WARN=""; CL_ERR=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*"; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }

MODE="${MODE:-write}"
DRY_RUN="${DRY_RUN:-0}"

command -v helm >/dev/null || { log_err "helm not found in PATH"; exit 127; }
[[ -d "$CHART_DIR" ]] || { log_err "chart dir not found: $CHART_DIR"; exit 1; }

run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    printf 'DRY_RUN: %s\n' "$*" >&2
  else
    "$@"
  fi
}

if [[ "$MODE" == "write" ]]; then
  log_info "helm dep update: $CHART_DIR"
  run helm dep update "$CHART_DIR" >/dev/null 2>&1 || {
    log_warn "helm dep update failed (offline?). Continuing with existing charts/ cache."
  }
fi

HELM_ARGS=(template nullus "$CHART_DIR")
if [[ -f "$VALUES_AIRGAP" ]]; then
  HELM_ARGS+=(-f "$VALUES_AIRGAP")
fi

log_info "helm ${HELM_ARGS[*]}"
if [[ "$DRY_RUN" == "1" ]]; then
  RENDERED=""
else
  RENDERED="$(helm "${HELM_ARGS[@]}" 2>/dev/null || true)"
  if [[ -z "$RENDERED" ]]; then
    log_err "helm template produced empty output. Did you run 'helm dep update'?"
    exit 1
  fi
fi

extract_images() {
  awk '
    /^[[:space:]]*image:[[:space:]]*/ {
      sub(/^[[:space:]]*image:[[:space:]]*/, "");
      gsub(/["'\'']/, "");
      sub(/[[:space:]]*#.*/, "");
      sub(/[[:space:]]+$/, "");
      if (length($0) > 0 && $0 !~ /^\{\{/ && $0 !~ /^\$/) print
    }
  ' | sort -u
}

# Airgap values override registries to localhost:5001. For the bundle manifest
# we want upstream references so pull/push scripts can retag deterministically.
rewrite_upstream() {
  sed \
    -e 's#^localhost:5001/cloud-nullus/#ghcr.io/cloud-nullus/#' \
    -e 's#^localhost:5001/dasomel/#ghcr.io/dasomel/#' \
    -e 's#^localhost:5001/bitnamilegacy/#docker.io/bitnamilegacy/#' \
    -e 's#^localhost:5001/bitnami/#docker.io/bitnami/#'
}

if [[ "$DRY_RUN" == "1" ]]; then
  CHART_IMAGES="ghcr.io/cloud-nullus/nullus/nullus-api:main
ghcr.io/cloud-nullus/nullus/nullus-web:main
docker.io/bitnamilegacy/postgresql:17.5.0-debian-12-r20"
else
  CHART_IMAGES="$(printf '%s\n' "$RENDERED" | extract_images | rewrite_upstream | sort -u)"
fi

# 카탈로그 chart 가 helm/charts-catalog/ 에 있으면 각각 helm template 으로 image 추출
# chart-specific values 가 helm/charts-catalog-values/<base>.yaml 에 있으면 자동 적용
# (일부 chart 는 필수 values 없이는 렌더 실패: gitlab, opentelemetry-collector 등)
CATALOG_DIR="${ROOT_DIR}/helm/charts-catalog"
CATALOG_VALUES_DIR="${ROOT_DIR}/helm/charts-catalog-values"
CATALOG_IMAGES=""
if [[ -d "$CATALOG_DIR" && "$DRY_RUN" != "1" ]]; then
  log_info "카탈로그 chart 스캔: $CATALOG_DIR"
  shopt -s nullglob
  for tgz in "$CATALOG_DIR"/*.tgz; do
    name="$(basename "$tgz" .tgz)"
    # base chart name = strip trailing version suffix (e.g., gitlab-8.7.2 → gitlab, cert-manager-v1.16.3 → cert-manager)
    base="$(printf '%s' "$name" | sed -E 's/-v?[0-9].*$//')"
    extra_args=()
    if [[ -f "$CATALOG_VALUES_DIR/$base.yaml" ]]; then
      extra_args+=(-f "$CATALOG_VALUES_DIR/$base.yaml")
      log_info "  helm template $name (with values: $base.yaml)"
    else
      log_info "  helm template $name"
    fi
    rendered="$(helm template "$name" "$tgz" ${extra_args[@]+"${extra_args[@]}"} 2>/dev/null || true)"
    [[ -z "$rendered" ]] && { log_warn "    렌더 실패 — 건너뜀 (values override 필요?)"; continue; }
    imgs="$(printf '%s\n' "$rendered" | extract_images | rewrite_upstream | sort -u)"
    CATALOG_IMAGES+="$imgs"$'\n'
  done
  shopt -u nullglob
  # 2025-08 Bitnami 정책: 버전 태그는 docker.io/bitnamilegacy/* 로 이관됨
  CATALOG_IMAGES="$(printf '%s' "$CATALOG_IMAGES" \
    | sed -e 's#^docker.io/bitnami/#docker.io/bitnamilegacy/#' \
    | grep -v '^$' | sort -u || true)"
  # Nullus chart 와 중복되는 이미지 제거 (CHART_IMAGES 에 이미 있는 것)
  if [[ -n "$CATALOG_IMAGES" ]]; then
    CATALOG_IMAGES="$(comm -23 <(printf '%s\n' "$CATALOG_IMAGES") <(printf '%s\n' "$CHART_IMAGES" | sort -u))"
  fi
fi

tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT

{
  printf '# AUTO-GENERATED by airgap/scripts/00-generate-images.sh\n'
  printf '# Source : helm template %s' "$CHART_DIR"
  [[ -f "$VALUES_AIRGAP" ]] && printf ' -f %s' "$VALUES_AIRGAP"
  printf '\n'
  printf '# Regenerate whenever chart deps or app versions change.\n'
  printf '# Format : <registry>/<repo>:<tag>, one per line. Blank / "#" lines ignored.\n'
  printf '# CI     : MODE=check ./00-generate-images.sh blocks drift.\n'
  printf '\n'
  printf '# --- Nullus app + chart dependency images (rendered from chart) ---\n'
  printf '%s\n' "$CHART_IMAGES"
  if [[ -n "${CATALOG_IMAGES:-}" ]]; then
    printf '\n'
    printf '# --- Catalog images (Stack: Keycloak/Harbor/MinIO/ArgoCD/Prometheus) ---\n'
    printf '%s\n' "$CATALOG_IMAGES"
  fi
  printf '\n'
  printf '# --- Infra images (not part of helm render) ---\n'
  for img in "${INFRA_IMAGES[@]}"; do printf '%s\n' "$img"; done
} > "$tmp_file"

case "$MODE" in
  write)
    if [[ "$DRY_RUN" == "1" ]]; then
      log_info "DRY_RUN: would write $OUT_FILE:"
      cat "$tmp_file"
    else
      mkdir -p "$(dirname "$OUT_FILE")"
      mv "$tmp_file" "$OUT_FILE"
      trap - EXIT
      log_info "wrote $OUT_FILE ($(grep -cvE '^\s*(#|$)' "$OUT_FILE") images)"
    fi
    ;;
  check)
    if [[ ! -f "$OUT_FILE" ]]; then
      log_err "$OUT_FILE does not exist; run in MODE=write first"
      exit 1
    fi
    if diff -u "$OUT_FILE" "$tmp_file" >/dev/null; then
      log_info "$OUT_FILE is up to date"
    else
      log_warn "$OUT_FILE differs from rendered chart — diff follows:"
      diff -u "$OUT_FILE" "$tmp_file" || true
      exit 1
    fi
    ;;
  *)
    log_err "unknown MODE=$MODE (expected: write|check)"
    exit 2
    ;;
esac
