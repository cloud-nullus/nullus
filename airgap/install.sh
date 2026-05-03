#!/usr/bin/env bash
# =============================================================================
# install.sh — Air-Gap 마스터 번들 원샷 설치 (오프라인 머신 전용)
# =============================================================================
# 용도: nullus-airgap-bundle-*.tar.gz 한 개로 오프라인 환경에서 kind 클러스터
#       기동부터 Nullus 설치·검증까지 자동 수행한다.
#
# 사용법 (둘 다 지원):
#   1) 번들 디렉토리 안에서 (이미 압축 해제됨):
#        ./install.sh
#
#   2) 압축 해제 + 설치 자동:
#        ./install.sh /path/to/nullus-airgap-bundle-*.tar.gz
#
# 환경 변수:
#   CLUSTER_NAME    kind 클러스터 이름 (기본: nullus-airgap)
#   SKIP_VERIFY     1 = 마지막 verify 건너뜀
#   PLATFORM_OVR    수동 platform 지정 (기본: 자동탐지)
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_OK=$'\033[1;32m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_WARN=""; CL_ERR=""; CL_OK=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }
hdr() {
  local sep="--------------------------------------------"
  printf '%s\n[%s] %s\n%s\n' \
    "${CL_INFO}${sep}${CL_RST}" \
    "${CL_INFO}$1${CL_RST}" "$2" \
    "${CL_INFO}${sep}${CL_RST}" >&2
}

# -----------------------------------------------------------------------------
# 인자 처리: 번들 tar.gz 경로가 주어지면 압축 해제 후 그 안의 install.sh 호출
# -----------------------------------------------------------------------------
if [[ $# -gt 0 && -f "$1" && "$1" =~ \.tar\.gz$ ]]; then
  BUNDLE_TAR="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
  log_info "번들 압축 해제: $BUNDLE_TAR"
  WORKDIR="$(pwd)"
  tar -xzf "$BUNDLE_TAR" -C "$WORKDIR"
  EXTRACTED_DIR="$(find "$WORKDIR" -maxdepth 1 -type d -name 'nullus-airgap-bundle-*' | head -1)"
  if [[ -z "$EXTRACTED_DIR" ]]; then
    log_err "압축 해제 디렉토리를 찾을 수 없음"
    exit 1
  fi
  log_info "이동 후 재실행: $EXTRACTED_DIR"
  exec "${EXTRACTED_DIR}/install.sh"
fi

# -----------------------------------------------------------------------------
# 번들 루트 결정
#
# 두 가지 레이아웃 모두 지원:
#   1) 패키징된 번들  : <bundle>/install.sh  +  <bundle>/airgap/{scripts,bin,...}
#   2) 원본 repo 트리 : <repo>/airgap/install.sh  +  <repo>/airgap/{scripts,bin,...}
# -----------------------------------------------------------------------------
INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -d "${INSTALL_DIR}/airgap" ]]; then
  AIRGAP_DIR="${INSTALL_DIR}/airgap"        # 패키징된 번들 레이아웃
else
  AIRGAP_DIR="${INSTALL_DIR}"               # 원본 트리 (airgap/ 안에서 직접 실행)
fi
SCRIPTS="${AIRGAP_DIR}/scripts"
BIN_DIR="${AIRGAP_DIR}/bin"
ROOT_DIR="$AIRGAP_DIR"  # 호환용 (이후 코드 일부가 ROOT_DIR 참조)

CLUSTER_NAME="${CLUSTER_NAME:-nullus-airgap}"
SKIP_VERIFY="${SKIP_VERIFY:-0}"

# -----------------------------------------------------------------------------
# Platform 탐지 + PATH 세팅
# -----------------------------------------------------------------------------
detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) log_err "지원하지 않는 arch: $arch"; exit 1 ;;
  esac
  echo "${os}-${arch}"
}

PLATFORM="${PLATFORM_OVR:-$(detect_platform)}"
log_info "감지된 platform: $PLATFORM"

if [[ ! -d "${BIN_DIR}/${PLATFORM}" ]]; then
  log_err "이 번들에는 $PLATFORM 바이너리가 없습니다."
  log_err "사용 가능: $(ls "$BIN_DIR" 2>/dev/null | tr '\n' ' ')"
  exit 1
fi

export PATH="${BIN_DIR}/${PLATFORM}:${PATH}"
log_info "PATH prepend: ${BIN_DIR}/${PLATFORM}"
log_info "  kind    : $(kind version 2>&1 | head -1)"
log_info "  kubectl : $(kubectl version --client 2>&1 | head -1)"
log_info "  helm    : $(helm version --short 2>&1 | head -1)"

# -----------------------------------------------------------------------------
# 사전 점검
# -----------------------------------------------------------------------------
command -v docker >/dev/null || { log_err "docker 명령 없음"; exit 1; }
docker info >/dev/null 2>&1 || { log_err "docker daemon 미기동"; exit 1; }

# 산출물 경로 — 번들은 원본 airgap/ 레이아웃과 1:1 매칭되도록 패키징됨
[[ -f "${AIRGAP_DIR}/bundle/images.tar.gz"      ]] || { log_err "bundle/images.tar.gz 없음";       exit 1; }
[[ -f "${AIRGAP_DIR}/images/images.txt"          ]] || { log_err "images/images.txt 없음";          exit 1; }
[[ -f "${AIRGAP_DIR}/helm/values-airgap.yaml"   ]] || { log_err "helm/values-airgap.yaml 없음";    exit 1; }
ls "${AIRGAP_DIR}/helm"/nullus-*.tgz >/dev/null 2>&1 || { log_err "helm/nullus-*.tgz 없음";           exit 1; }
[[ -f "${AIRGAP_DIR}/kind/kind-airgap.yaml"     ]] || { log_err "kind/kind-airgap.yaml 없음";       exit 1; }

# bootstrap.sh 가 참조하는 path 들 모두 ROOT_DIR 기준이라 그대로 사용 가능
START_TS=$(date +%s)

# -----------------------------------------------------------------------------
# 단계 실행
# -----------------------------------------------------------------------------
hdr "STEP 1/6" "이미지 docker load (03-load-bundle.sh)"
bash "${SCRIPTS}/03-load-bundle.sh"

hdr "STEP 2/6" "로컬 레지스트리 기동 (10-setup-registry.sh)"
bash "${SCRIPTS}/10-setup-registry.sh"

hdr "STEP 3/6" "kind 클러스터 생성 (11-create-cluster.sh)"
bash "${SCRIPTS}/11-create-cluster.sh"

hdr "STEP 4/6" "이미지 → 로컬 레지스트리 push (12-push-to-registry.sh)"
bash "${SCRIPTS}/12-push-to-registry.sh"

hdr "STEP 5/7" "Helm 설치 (21-install-nullus.sh)"
bash "${SCRIPTS}/21-install-nullus.sh"

# Platform stack: Keycloak (필수) + 옵션으로 kube-prometheus-stack
SKIP_PLATFORM="${SKIP_PLATFORM:-0}"
if [[ "$SKIP_PLATFORM" != "1" && -d "${AIRGAP_DIR}/helm/charts-catalog" ]]; then
  hdr "STEP 6/7" "Platform stack 설치 (Keycloak${INSTALL_FULL:+ + kube-prometheus-stack})"
  bash "${SCRIPTS}/22-install-platform-stack.sh" || log_warn "platform stack 설치 일부 실패 — 수동 확인 권장"
else
  log_info "STEP 6/7 건너뜀 (SKIP_PLATFORM=1 또는 카탈로그 없음)"
fi

if [[ "$SKIP_VERIFY" != "1" ]]; then
  hdr "STEP 7/7" "검증 (99-verify.sh + kubectl get pods)"
  bash "${SCRIPTS}/99-verify.sh" || log_warn "verify 실패 — pods 상태로 직접 확인 권장"
  bash "${SCRIPTS}/13-set-config.sh" cert || true
  echo "--- nullus 네임스페이스 ---" >&2
  kubectl get pods -n nullus 2>&1 || true
  echo "--- nullus-auth 네임스페이스 ---" >&2
  kubectl get pods -n nullus-auth 2>&1 || true
  if [[ "${INSTALL_FULL:-0}" == "1" ]]; then
    echo "--- nullus-monitoring 네임스페이스 ---" >&2
    kubectl get pods -n nullus-monitoring 2>&1 || true
  fi
fi

DUR=$(( $(date +%s) - START_TS ))
log_ok "=== 설치 완료 — 총 $((DUR / 60))분 $((DUR % 60))초 ==="
log_info "이후 사용 명령:"
log_info "  kubectl get pods -n nullus"
log_info "  helm list -n nullus"
log_info "정리:  kind delete cluster --name $CLUSTER_NAME && docker rm -f kind-registry"
