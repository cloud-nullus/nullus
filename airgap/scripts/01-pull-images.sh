#!/usr/bin/env bash
# =============================================================================
# 01-pull-images.sh — Nullus Platform Air-Gap 이미지 Pull
# =============================================================================
# 목적  : images/images.txt 에 나열된 모든 이미지를 로컬 daemon으로 pull한다.
#         인터넷 접속이 가능한 환경에서 실행해야 한다.
#
# 사용법: bash 01-pull-images.sh
#         PODMAN=1 bash 01-pull-images.sh   # podman 사용
#         DRY_RUN=1 bash 01-pull-images.sh  # 명령 출력만 (실행 안 함)
#
# 환경변수:
#   PODMAN=1     docker 대신 podman 사용 (기본: 0)
#   DRY_RUN=1    명령만 출력하고 실행하지 않음 (기본: 0)
#
# 종료 코드:
#   0  모든 이미지 pull 성공
#   1  하나 이상의 이미지 pull 실패
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGES_FILE="$ROOT_DIR/images/images.txt"

# ---------------------------------------------------------------------------
# 색상 로그 헬퍼
# ---------------------------------------------------------------------------
log_info() { printf '\033[0;32m[INFO]\033[0m  %s\n' "$*"; }
log_warn() { printf '\033[0;33m[WARN]\033[0m  %s\n' "$*"; }
log_err()  { printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2; }

# ---------------------------------------------------------------------------
# 환경 변수 기본값
# ---------------------------------------------------------------------------
PODMAN="${PODMAN:-0}"
DRY_RUN="${DRY_RUN:-0}"

# ---------------------------------------------------------------------------
# 컨테이너 런타임 결정
# ---------------------------------------------------------------------------
if [[ "$PODMAN" == "1" ]]; then
  RUNTIME="podman"
else
  RUNTIME="docker"
fi

command -v "$RUNTIME" >/dev/null 2>&1 || {
  log_err "$RUNTIME not found. PATH=$PATH"
  exit 127
}

# ---------------------------------------------------------------------------
# images.txt 존재 확인
# ---------------------------------------------------------------------------
[[ -f "$IMAGES_FILE" ]] || {
  log_err "images.txt not found: $IMAGES_FILE"
  exit 1
}

# ---------------------------------------------------------------------------
# Pull 루프
# ---------------------------------------------------------------------------
failed_images=()
total=0
success=0

log_info "Using runtime: $RUNTIME"
log_info "Image list: $IMAGES_FILE"
[[ "$DRY_RUN" == "1" ]] && log_warn "DRY_RUN enabled — commands will be printed only"

while IFS= read -r line; do
  # 빈 줄 및 주석 스킵
  [[ -z "$line" || "$line" == \#* ]] && continue

  image="$line"
  total=$((total + 1))

  if [[ "$DRY_RUN" == "1" ]]; then
    printf '[DRY_RUN] %s pull %s\n' "$RUNTIME" "$image"
    success=$((success + 1))
    continue
  fi

  log_info "Pulling: $image"
  if "$RUNTIME" pull "$image"; then
    success=$((success + 1))
  else
    log_err "Failed to pull: $image"
    failed_images+=("$image")
  fi
done < "$IMAGES_FILE"

# ---------------------------------------------------------------------------
# 결과 요약
# ---------------------------------------------------------------------------
printf '\n'
log_info "$(printf 'Pull complete: %d/%d succeeded' "$success" "$total")"

if [[ ${#failed_images[@]} -gt 0 ]]; then
  log_err "The following images failed to pull:"
  for img in "${failed_images[@]}"; do
    log_err "  - $img"
  done
  exit 1
fi

log_info "All images pulled successfully."
