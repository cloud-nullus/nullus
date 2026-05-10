#!/usr/bin/env bash
# =============================================================================
# pre/pull-binaries.sh — 오프라인 설치에 필요한 CLI 바이너리 다운로드
# =============================================================================
# 용도: kind, kubectl, helm 을 PLATFORMS 별로 받아 airgap/bin/<platform>/ 에
#       저장. 마스터 번들에 포함되어 오프라인 머신에서 PATH 로 사용된다.
#
# 사용법:
#   ./pull-binaries.sh
#   PLATFORMS="linux-amd64,linux-arm64,darwin-arm64" ./pull-binaries.sh
#   KIND_VERSION=v0.31.0 KUBECTL_VERSION=v1.30.0 HELM_VERSION=v3.16.0 \
#     ./pull-binaries.sh
#
# 환경 변수:
#   PLATFORMS         쉼표 구분 (기본: linux-amd64,linux-arm64)
#   KIND_VERSION      kind 버전 (기본: v0.31.0)
#   KUBECTL_VERSION   kubectl 버전 (기본: v1.30.0 — 클러스터 k8s 버전과 일치)
#   HELM_VERSION      helm 버전 (기본: v3.16.0)
#   DRY_RUN           1 = 명령 출력만
#
# 출력:
#   airgap/bin/<platform>/{kind,kubectl,helm}
#   airgap/bin/<platform>/SHA256SUMS
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"

PLATFORMS="${PLATFORMS:-linux-amd64,linux-arm64}"
KIND_VERSION="${KIND_VERSION:-v0.31.0}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.30.0}"
HELM_VERSION="${HELM_VERSION:-v3.16.0}"
DRY_RUN="${DRY_RUN:-0}"

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_OK=$'\033[1;32m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_WARN=""; CL_ERR=""; CL_OK=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }

command -v curl >/dev/null || { log_err "curl not found"; exit 127; }
command -v tar  >/dev/null || { log_err "tar not found";  exit 127; }
if command -v shasum >/dev/null 2>&1; then
  SHA_CMD=(shasum -a 256)
elif command -v sha256sum >/dev/null 2>&1; then
  SHA_CMD=(sha256sum)
else
  log_err "shasum / sha256sum not found"; exit 127
fi

run_curl() {
  local url="$1" out="$2"
  if [[ "$DRY_RUN" == "1" ]]; then
    printf 'DRY_RUN: curl -fsSL %s -o %s\n' "$url" "$out" >&2
    return 0
  fi
  curl -fsSL "$url" -o "$out"
}

download_kind() {
  local platform="$1" outdir="$2"
  local url="https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-${platform}"
  local out="${outdir}/kind"
  if [[ -x "$out" ]]; then
    log_info "kind 이미 존재: $out — 건너뜀"
    return 0
  fi
  log_info "kind ${KIND_VERSION} 다운로드 (${platform})"
  run_curl "$url" "$out"
  chmod +x "$out"
}

download_kubectl() {
  local platform="$1" outdir="$2"
  # kubectl URL 형식: https://dl.k8s.io/release/<ver>/bin/<os>/<arch>/kubectl
  local os="${platform%-*}" arch="${platform##*-}"
  local url="https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${os}/${arch}/kubectl"
  local out="${outdir}/kubectl"
  if [[ -x "$out" ]]; then
    log_info "kubectl 이미 존재: $out — 건너뜀"
    return 0
  fi
  log_info "kubectl ${KUBECTL_VERSION} 다운로드 (${platform})"
  run_curl "$url" "$out"
  chmod +x "$out"
}

download_helm() {
  local platform="$1" outdir="$2"
  # helm URL: https://get.helm.sh/helm-<ver>-<os>-<arch>.tar.gz
  local url="https://get.helm.sh/helm-${HELM_VERSION}-${platform}.tar.gz"
  local out="${outdir}/helm"
  if [[ -x "$out" ]]; then
    log_info "helm 이미 존재: $out — 건너뜀"
    return 0
  fi
  log_info "helm ${HELM_VERSION} 다운로드 (${platform})"
  local tmp
  tmp="$(mktemp -d)"
  run_curl "$url" "${tmp}/helm.tar.gz"
  if [[ "$DRY_RUN" != "1" ]]; then
    tar -xzf "${tmp}/helm.tar.gz" -C "$tmp"
    mv "${tmp}/${platform}/helm" "$out"
    chmod +x "$out"
  fi
  rm -rf "$tmp"
}

write_sha256sums() {
  local dir="$1"
  if [[ "$DRY_RUN" == "1" ]]; then
    printf 'DRY_RUN: sha256 sums for %s\n' "$dir" >&2
    return
  fi
  ( cd "$dir" && "${SHA_CMD[@]}" kind kubectl helm > SHA256SUMS )
}

log_info "=== 바이너리 다운로드 시작 ==="
log_info "Platforms : $PLATFORMS"
log_info "kind      : $KIND_VERSION"
log_info "kubectl   : $KUBECTL_VERSION"
log_info "helm      : $HELM_VERSION"
log_info "Output    : $BIN_DIR"

mkdir -p "$BIN_DIR"
IFS=',' read -ra PLATFORM_LIST <<< "$PLATFORMS"

for platform in "${PLATFORM_LIST[@]}"; do
  case "$platform" in
    linux-amd64|linux-arm64|darwin-amd64|darwin-arm64) ;;
    *) log_err "지원하지 않는 platform: $platform"; exit 2 ;;
  esac
  outdir="${BIN_DIR}/${platform}"
  mkdir -p "$outdir"
  log_info "── ${platform} ──────────────────────────────"
  download_kind    "$platform" "$outdir"
  download_kubectl "$platform" "$outdir"
  download_helm    "$platform" "$outdir"
  write_sha256sums "$outdir"
  log_ok "${platform} 완료"
done

log_ok "=== 모든 바이너리 다운로드 완료 ==="
log_info "총 출력 디렉토리: $BIN_DIR"
