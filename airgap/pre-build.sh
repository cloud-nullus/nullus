#!/usr/bin/env bash
# =============================================================================
# pre-build.sh — Air-Gap 마스터 번들 원샷 빌더 (온라인 머신 전용)
# =============================================================================
# 용도: 인터넷이 가능한 머신에서 다음 산출물을 모두 생성·패키징한다.
#         1) 컨테이너 이미지 pull → bundle/images.tar.gz
#         2) helm 차트 dep update + package → helm/nullus-*.tgz
#         3) kind/kubectl/helm 바이너리 다운로드 → bin/<platform>/
#         4) 위 모두를 단일 마스터 tar.gz 로 묶음 → dist/nullus-airgap-bundle-*.tar.gz
#
# 사용법:
#   ./pre-build.sh
#   PLATFORMS="linux-amd64,linux-arm64" BUNDLE_VERSION=<yyyy-mm-dd> ./pre-build.sh
#   SKIP_IMAGES=1 ./pre-build.sh    # 이미지 pull/save 건너뜀 (이미 있을 때)
#   SKIP_BIN=1    ./pre-build.sh    # 바이너리 다운로드 건너뜀
#   SKIP_CHARTS=1 ./pre-build.sh    # helm 차트 번들 건너뜀
#   SKIP_SBOM=1   ./pre-build.sh    # SBOM 생성 건너뜀 (syft 미설치 시 자동 건너뜀)
#
# 환경 변수: pull-binaries.sh / package-bundle.sh / generate-sbom.sh 의 ENV 모두 통과
#
# 종료 코드:
#   0 — 성공
#   1 — 사전조건/의존성 오류
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR"
SCRIPTS="${ROOT_DIR}/scripts"

SKIP_IMAGES="${SKIP_IMAGES:-0}"
SKIP_BIN="${SKIP_BIN:-0}"
SKIP_CHARTS="${SKIP_CHARTS:-0}"
SKIP_SBOM="${SKIP_SBOM:-0}"

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_OK=$'\033[1;32m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_OK=""; CL_RST=""
fi
hdr() {
  local sep="===================================================="
  printf '%s\n%s==> %s%s\n%s\n' \
    "${CL_INFO}${sep}${CL_RST}" \
    "${CL_INFO}" "$*" "${CL_RST}" \
    "${CL_INFO}${sep}${CL_RST}" >&2
}

# 사전 도구 확인
for cmd in docker curl tar; do
  command -v "$cmd" >/dev/null || { echo "[ERR] $cmd 없음" >&2; exit 1; }
done
docker info >/dev/null 2>&1 || { echo "[ERR] docker daemon 미기동 — Docker Desktop / colima / systemctl 시작 필요" >&2; exit 1; }

START_TS=$(date +%s)

if [[ "$SKIP_IMAGES" != "1" ]]; then
  hdr "1/4 컨테이너 이미지 pull"
  bash "${SCRIPTS}/01-pull-images.sh"
  hdr "1/4 이미지 tar.gz 저장"
  bash "${SCRIPTS}/02-save-bundle.sh"
else
  hdr "1/4 이미지 단계 — SKIP_IMAGES=1 건너뜀"
fi

if [[ "$SKIP_CHARTS" != "1" ]]; then
  hdr "2/5 Helm 차트 번들 (Nullus)"
  bash "${SCRIPTS}/20-bundle-charts.sh"
else
  hdr "2/5 Helm 차트 단계 — SKIP_CHARTS=1 건너뜀"
fi

SKIP_CATALOG="${SKIP_CATALOG:-0}"
if [[ "$SKIP_CATALOG" != "1" ]]; then
  hdr "3/5 카탈로그 chart 다운로드 (Keycloak/Harbor/MinIO/ArgoCD/Prometheus)"
  bash "${SCRIPTS}/pre/pull-charts-catalog.sh"
  hdr "3/5 images.txt 재생성 (카탈로그 image 포함)"
  bash "${SCRIPTS}/00-generate-images.sh"
  hdr "3/5 추가 image pull (카탈로그)"
  bash "${SCRIPTS}/01-pull-images.sh"
  hdr "3/5 번들 tar.gz 갱신 (카탈로그 image 포함)"
  bash "${SCRIPTS}/02-save-bundle.sh"
else
  hdr "3/5 카탈로그 단계 — SKIP_CATALOG=1 건너뜀"
fi

if [[ "$SKIP_BIN" != "1" ]]; then
  hdr "4/5 CLI 바이너리 다운로드"
  bash "${SCRIPTS}/pre/pull-binaries.sh"
else
  hdr "4/5 바이너리 단계 — SKIP_BIN=1 건너뜀"
fi

if [[ "$SKIP_SBOM" != "1" ]]; then
  hdr "5/6 SBOM 생성 (syft) — bundle/sbom/"
  bash "${SCRIPTS}/pre/generate-sbom.sh"
else
  hdr "5/6 SBOM 단계 — SKIP_SBOM=1 건너뜀"
fi

hdr "6/6 마스터 번들 패키징"
bash "${SCRIPTS}/pre/package-bundle.sh"

DUR=$(( $(date +%s) - START_TS ))

hdr "완료 — 총 $((DUR / 60))분 $((DUR % 60))초"
printf '%s[ OK ]%s pre-build 완료\n' "$CL_OK" "$CL_RST" >&2
ls -lh "${ROOT_DIR}/dist/" 2>/dev/null | tail -n +2 >&2 || true
echo "" >&2
echo "오프라인 머신에서 사용:" >&2
echo "  scp ${ROOT_DIR}/dist/nullus-airgap-bundle-*.tar.gz user@offline-host:~/" >&2
echo "  ssh user@offline-host 'tar -xzf nullus-airgap-bundle-*.tar.gz && cd nullus-airgap-bundle-*/ && ./install.sh'" >&2
