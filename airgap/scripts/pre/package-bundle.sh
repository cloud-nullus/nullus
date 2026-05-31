#!/usr/bin/env bash
# =============================================================================
# pre/package-bundle.sh — 마스터 air-gap 번들 tar.gz 생성
# =============================================================================
# 용도: 이미지/차트/바이너리/스크립트/config 를 단일 tar.gz 하나로 묶어
#       오프라인 머신에 한 번에 전달할 수 있게 한다.
#
# 사전조건 (이 순서로 실행되어 있어야 함):
#   1) scripts/01-pull-images.sh   — 이미지 daemon 적재
#   2) scripts/02-save-bundle.sh   — bundle/images.tar.gz 생성
#   3) scripts/20-bundle-charts.sh — helm/nullus-*.tgz 생성
#   4) scripts/pre/pull-binaries.sh — bin/<platform>/* 생성
#
# 사용법:
#   ./package-bundle.sh
#   BUNDLE_VERSION=<yyyy-mm-dd> ./package-bundle.sh
#
# 환경 변수:
#   BUNDLE_VERSION   번들 태그 (기본: 오늘 날짜 YYYY-MM-DD)
#   DIST_DIR         출력 디렉토리 (기본: airgap/dist)
#   DRY_RUN          1 = 명령만 출력
#
# 출력:
#   airgap/dist/nullus-airgap-bundle-<version>.tar.gz
#   airgap/dist/nullus-airgap-bundle-<version>.tar.gz.sha256
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

BUNDLE_VERSION="${BUNDLE_VERSION:-$(date +%Y-%m-%d)}"
DIST_DIR="${DIST_DIR:-${ROOT_DIR}/dist}"
DRY_RUN="${DRY_RUN:-0}"

BUNDLE_NAME="nullus-airgap-bundle-${BUNDLE_VERSION}"
BUNDLE_TAR="${DIST_DIR}/${BUNDLE_NAME}.tar.gz"

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_ERR=$'\033[1;31m'; CL_OK=$'\033[1;32m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_ERR=""; CL_OK=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }

if command -v shasum >/dev/null 2>&1; then SHA_CMD=(shasum -a 256)
else SHA_CMD=(sha256sum); fi

# 사전조건 체크
require() {
  local path="$1" hint="$2"
  if [[ ! -e "$path" ]]; then
    log_err "필수 산출물 없음: $path"
    log_err "  → $hint"
    exit 1
  fi
}

require "${ROOT_DIR}/bundle/images.tar.gz"          "scripts/02-save-bundle.sh 실행"
require "${ROOT_DIR}/bundle/images.tar.gz.sha256"   "scripts/02-save-bundle.sh 실행"
require "${ROOT_DIR}/bin"                            "scripts/pre/pull-binaries.sh 실행"
require "${ROOT_DIR}/images/images.txt"              "이미지 목록 누락 — 00-generate-images.sh 또는 수동"
# helm 차트 패키지 — 가장 최신 nullus-*.tgz 하나
shopt -s nullglob
chart_files=("${ROOT_DIR}"/helm/nullus-*.tgz)
shopt -u nullglob
if [[ ${#chart_files[@]} -eq 0 ]]; then
  log_err "helm 차트 패키지 없음: airgap/helm/nullus-*.tgz"
  log_err "  → scripts/20-bundle-charts.sh 실행"
  exit 1
fi

mkdir -p "$DIST_DIR"

# 임시 staging 디렉토리에 정리된 트리 구성 후 tar
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
DEST="${STAGE}/${BUNDLE_NAME}"
mkdir -p "$DEST"

log_info "=== 번들 staging 구성 (원본 airgap/ 레이아웃 1:1 미러) ==="
log_info "Stage: $DEST"

# 번들 트리 구조:
#   nullus-airgap-bundle-X/
#   ├── install.sh                ← 진입점
#   ├── INSTALL.md, README.md, VERSION
#   └── airgap/                   ← 원본 airgap/ 와 동일한 구조
#       ├── Makefile
#       ├── scripts/
#       ├── bundle/               ← 02-save-bundle 출력 경로 (images.tar.gz)
#       ├── images/               ← images.txt
#       ├── helm/                 ← values-airgap.yaml + nullus-*.tgz
#       ├── kind/
#       └── bin/                  ← <platform>/{kind,kubectl,helm}

AIRGAP_DST="${DEST}/airgap"
mkdir -p "$AIRGAP_DST"

stage_copy() {
  local src="$1" dst="$2"
  if [[ "$DRY_RUN" == "1" ]]; then
    printf 'DRY_RUN: cp -R %s %s\n' "$src" "$dst" >&2
  else
    cp -R "$src" "$dst"
  fi
}

# airgap/ 하위 구조 채우기 (스크립트들의 ROOT_DIR=$repo, $repo/airgap/... 해석과 호환)
stage_copy "${ROOT_DIR}/bundle"                  "${AIRGAP_DST}/bundle"
stage_copy "${ROOT_DIR}/bin"                     "${AIRGAP_DST}/bin"
stage_copy "${ROOT_DIR}/kind"                    "${AIRGAP_DST}/kind"
mkdir -p "${AIRGAP_DST}/images"
stage_copy "${ROOT_DIR}/images/images.txt"       "${AIRGAP_DST}/images/images.txt"
mkdir -p "${AIRGAP_DST}/helm"
stage_copy "${ROOT_DIR}/helm/values-airgap.yaml" "${AIRGAP_DST}/helm/values-airgap.yaml"
for f in "${chart_files[@]}"; do
  stage_copy "$f" "${AIRGAP_DST}/helm/$(basename "$f")"
  if [[ -f "${f}.sha256" ]]; then
    stage_copy "${f}.sha256" "${AIRGAP_DST}/helm/$(basename "$f").sha256"
  fi
done

# 카탈로그 chart (Keycloak/Harbor/MinIO/ArgoCD/Prometheus 등)
if [[ -d "${ROOT_DIR}/helm/charts-catalog" ]]; then
  stage_copy "${ROOT_DIR}/helm/charts-catalog" "${AIRGAP_DST}/helm/charts-catalog"
fi
# 카탈로그 values override (gitlab/otel) + 스택 설치 values (27-install-stacks.sh)
if [[ -d "${ROOT_DIR}/helm/charts-catalog-values" ]]; then
  stage_copy "${ROOT_DIR}/helm/charts-catalog-values" "${AIRGAP_DST}/helm/charts-catalog-values"
fi
if [[ -d "${ROOT_DIR}/helm/stack-values" ]]; then
  stage_copy "${ROOT_DIR}/helm/stack-values" "${AIRGAP_DST}/helm/stack-values"
fi

# 오프라인 설치에 필요한 스크립트만 선별 복사
mkdir -p "${AIRGAP_DST}/scripts"
for s in 03-load-bundle.sh 10-setup-registry.sh 11-create-cluster.sh \
         12-push-to-registry.sh 13-set-config.sh 21-install-nullus.sh \
         22-install-platform-stack.sh 23-setup-gateway.sh 24-register-hosts.sh \
         25-port-forward.sh 26-migrate-db.sh 27-install-stacks.sh \
         99-verify.sh bootstrap.sh; do
  if [[ -f "${ROOT_DIR}/scripts/${s}" ]]; then
    stage_copy "${ROOT_DIR}/scripts/${s}" "${AIRGAP_DST}/scripts/${s}"
  fi
done

# Makefile 은 airgap/ 하위, install.sh 는 번들 최상위 (사용자 진입점)
[[ -f "${ROOT_DIR}/Makefile"   ]] && stage_copy "${ROOT_DIR}/Makefile"   "${AIRGAP_DST}/Makefile"
[[ -f "${ROOT_DIR}/install.sh" ]] && stage_copy "${ROOT_DIR}/install.sh" "${DEST}/install.sh"

# VERSION + INSTALL.md 작성/복사
if [[ "$DRY_RUN" != "1" ]]; then
  cat > "${DEST}/VERSION" <<EOF
bundle_version: ${BUNDLE_VERSION}
created_at:     $(date -u +%Y-%m-%dT%H:%M:%SZ)
created_by:     $(id -un)@$(hostname -s)
host_platform:  $(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)
EOF

  # INSTALL.md 는 airgap/INSTALL.md 를 그대로 번들에 포함 (단일 진실 공급원)
  if [[ -f "${ROOT_DIR}/INSTALL.md" ]]; then
    cp "${ROOT_DIR}/INSTALL.md" "${DEST}/INSTALL.md"
  else
    log_err "airgap/INSTALL.md 없음 — 번들 가이드 누락"
    exit 1
  fi
  # README 는 INSTALL.md 로 단일화 — 호환성을 위해 README.md → INSTALL.md 심볼릭 링크 대신 짧은 redirect 생성
  cat > "${DEST}/README.md" <<'EOF'
# Nullus Air-Gap Bundle

전체 설치 가이드는 [INSTALL.md](./INSTALL.md) 를 참고하세요.

## 한 줄 요약

```bash
./install.sh
```
EOF
fi

log_info "=== 마스터 tar.gz 생성 ==="
log_info "출력: $BUNDLE_TAR"

if [[ "$DRY_RUN" == "1" ]]; then
  printf 'DRY_RUN: tar -czf %s -C %s %s\n' "$BUNDLE_TAR" "$STAGE" "$BUNDLE_NAME" >&2
else
  tar -czf "$BUNDLE_TAR" -C "$STAGE" "$BUNDLE_NAME"
  ( cd "$DIST_DIR" && "${SHA_CMD[@]}" "$(basename "$BUNDLE_TAR")" > "${BUNDLE_TAR}.sha256" )
fi

if [[ "$DRY_RUN" != "1" ]]; then
  size=$(du -h "$BUNDLE_TAR" | awk '{print $1}')
  log_ok "=== 번들 생성 완료 ==="
  log_info "  파일 : $BUNDLE_TAR"
  log_info "  크기 : $size"
  log_info "  SHA256: $(cat "${BUNDLE_TAR}.sha256")"
fi
