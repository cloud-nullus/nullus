#!/usr/bin/env bash
# =============================================================================
# 03-load-bundle.sh — Nullus Platform Air-Gap 이미지 번들 적재
# =============================================================================
# 목적  : air-gap 환경에서 bundle/images.tar.gz 의 SHA-256 을 검증하고
#         docker load 로 이미지를 복원한다.
#
# 사용법: bash 03-load-bundle.sh
#         BUNDLE_DIR=/mnt/usb/bundle bash 03-load-bundle.sh
#         DRY_RUN=1 bash 03-load-bundle.sh
#
# 환경변수:
#   BUNDLE_DIR   번들 디렉토리 경로 (기본: <ROOT>/bundle)
#   DRY_RUN=1    명령만 출력하고 실행하지 않음 (기본: 0)
#
# 종료 코드:
#   0   이미지 적재 성공
#   1   SHA-256 검증 실패 또는 적재 실패
#   127 docker not found
#
# 참고: docker load 는 sudo 없이 실행 가능해야 한다.
#       docker group 멤버십 또는 rootless docker 설정이 필요하다.
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BUNDLE_DIR="${BUNDLE_DIR:-$ROOT_DIR/bundle}"
BUNDLE_GZ="$BUNDLE_DIR/images.tar.gz"
BUNDLE_SHA="$BUNDLE_DIR/images.tar.gz.sha256"
MANIFEST="$BUNDLE_DIR/MANIFEST.txt"

# ---------------------------------------------------------------------------
# 색상 로그 헬퍼
# ---------------------------------------------------------------------------
log_info() { printf '\033[0;32m[INFO]\033[0m  %s\n' "$*"; }
log_warn() { printf '\033[0;33m[WARN]\033[0m  %s\n' "$*"; }
log_err()  { printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2; }

# ---------------------------------------------------------------------------
# 환경 변수 기본값
# ---------------------------------------------------------------------------
DRY_RUN="${DRY_RUN:-0}"

# ---------------------------------------------------------------------------
# 사전 조건 확인
# ---------------------------------------------------------------------------
command -v docker >/dev/null 2>&1 || {
  log_err "docker not found. PATH=$PATH"
  exit 127
}

command -v gzip >/dev/null 2>&1 || {
  log_err "gzip not found."
  exit 127
}

command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 || {
  log_err "sha256sum (or shasum) not found."
  exit 127
}

[[ -f "$BUNDLE_GZ" ]] || {
  log_err "Bundle not found: $BUNDLE_GZ"
  log_err "BUNDLE_DIR=$BUNDLE_DIR"
  exit 1
}

[[ -f "$BUNDLE_SHA" ]] || {
  log_err "Checksum file not found: $BUNDLE_SHA"
  exit 1
}

# ---------------------------------------------------------------------------
# SHA-256 유틸 추상화 (macOS shasum -a 256 / Linux sha256sum)
# ---------------------------------------------------------------------------
verify_sha256() {
  local file="$1"
  local shafile="$2"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum --check "$shafile"
  else
    # macOS: shasum -c 는 같은 디렉토리에서 동작해야 함
    local dir
    dir="$(dirname "$shafile")"
    (cd "$dir" && shasum -a 256 --check "$(basename "$shafile")")
  fi
}

# ---------------------------------------------------------------------------
# DRY_RUN 모드
# ---------------------------------------------------------------------------
if [[ "$DRY_RUN" == "1" ]]; then
  log_warn "DRY_RUN enabled — commands will be printed only"
  printf '[DRY_RUN] sha256 verify: %s against %s\n' "$BUNDLE_GZ" "$BUNDLE_SHA"
  printf '[DRY_RUN] gzip -dc %s | docker load\n' "$BUNDLE_GZ"
  exit 0
fi

# ---------------------------------------------------------------------------
# SHA-256 검증
# ---------------------------------------------------------------------------
log_info "Verifying SHA-256 checksum..."
log_info "  Archive : $BUNDLE_GZ"
log_info "  Expected: $(cat "$BUNDLE_SHA")"

# sha 파일에 기록된 경로를 번들 디렉토리 기준으로 재작성해 검증
# (번들이 다른 경로로 옮겨진 경우에도 동작하도록 임시 파일 사용)
TMP_SHA="$(mktemp)"
trap 'rm -f "$TMP_SHA"' EXIT

# 체크섬 파일의 파일명 부분만 가져와 현재 번들 파일명으로 교체
printf '%s  %s\n' "$(awk '{print $1}' "$BUNDLE_SHA")" "$BUNDLE_GZ" > "$TMP_SHA"

if command -v sha256sum >/dev/null 2>&1; then
  sha256sum --check "$TMP_SHA" || {
    log_err "SHA-256 verification FAILED. Bundle may be corrupted or tampered."
    exit 1
  }
else
  actual="$(shasum -a 256 "$BUNDLE_GZ" | awk '{print $1}')"
  expected="$(awk '{print $1}' "$BUNDLE_SHA")"
  if [[ "$actual" != "$expected" ]]; then
    log_err "SHA-256 verification FAILED."
    log_err "  Expected: $expected"
    log_err "  Actual  : $actual"
    exit 1
  fi
fi

log_info "SHA-256 verification passed."

# ---------------------------------------------------------------------------
# docker load
# ---------------------------------------------------------------------------
log_info "Loading images from bundle..."
load_output="$(gzip -dc "$BUNDLE_GZ" | docker load 2>&1)"
exit_code=$?

if [[ $exit_code -ne 0 ]]; then
  log_err "docker load failed (exit $exit_code):"
  printf '%s\n' "$load_output" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# 완료 요약
# ---------------------------------------------------------------------------
printf '\n'
log_info "Images loaded successfully."
printf '\n'
log_info "Loaded images:"
printf '%s\n' "$load_output" | grep -E '^Loaded image' | while IFS= read -r line; do
  log_info "  $line"
done

if [[ -f "$MANIFEST" ]]; then
  printf '\n'
  log_info "Manifest reference: $MANIFEST"
fi

log_info "Done. Run 'docker images' to verify."
