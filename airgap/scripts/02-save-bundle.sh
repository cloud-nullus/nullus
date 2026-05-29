#!/usr/bin/env bash
# =============================================================================
# 02-save-bundle.sh — Nullus Platform Air-Gap 이미지 번들 저장
# =============================================================================
# 목적  : 로컬 daemon에 있는 이미지들을 하나의 tar.gz 번들로 저장하고
#         SHA-256 체크섬과 MANIFEST.txt를 생성한다.
#         인터넷 접속 가능 환경에서 01-pull-images.sh 실행 후 사용한다.
#
# 사용법: bash 02-save-bundle.sh
#         DRY_RUN=1 bash 02-save-bundle.sh
#
# 환경변수:
#   BUNDLE_DIR   번들 저장 디렉토리 (기본: <ROOT>/bundle)
#   DRY_RUN=1    명령만 출력하고 실행하지 않음 (기본: 0)
#
# 종료 코드:
#   0  번들 생성 성공
#   1  이미지 누락 또는 저장 실패
#   127 docker not found
#
# 참고: docker save 는 sudo 없이 실행 가능해야 한다.
#       docker group 멤버십 또는 rootless docker 설정이 필요하다.
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGES_FILE="$ROOT_DIR/images/images.txt"
BUNDLE_DIR="${BUNDLE_DIR:-$ROOT_DIR/bundle}"
BUNDLE_TAR="$BUNDLE_DIR/images.tar"
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

[[ -f "$IMAGES_FILE" ]] || {
  log_err "images.txt not found: $IMAGES_FILE"
  exit 1
}

# ---------------------------------------------------------------------------
# SHA-256 유틸 추상화 (macOS shasum -a 256 / Linux sha256sum)
# ---------------------------------------------------------------------------
sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file"
  else
    shasum -a 256 "$file"
  fi
}

# ---------------------------------------------------------------------------
# 이미지 목록 파싱
# ---------------------------------------------------------------------------
images=()
while IFS= read -r line; do
  [[ -z "$line" || "$line" == \#* ]] && continue
  # @sha256:... digest 제거 — 01-pull 이 digest-pin 이미지를 digest-제거 태그형으로
  # 적재하므로, 저장/검증도 동일한 태그형 reference 를 사용한다.
  images+=("${line%@*}")
done < "$IMAGES_FILE"

if [[ ${#images[@]} -eq 0 ]]; then
  log_err "No images found in $IMAGES_FILE"
  exit 1
fi

log_info "Images to bundle: ${#images[@]}"

# ---------------------------------------------------------------------------
# bundle/ 디렉토리 생성
# ---------------------------------------------------------------------------
if [[ "$DRY_RUN" == "1" ]]; then
  log_warn "DRY_RUN enabled — commands will be printed only"
  printf '[DRY_RUN] mkdir -p %s\n' "$BUNDLE_DIR"
else
  mkdir -p "$BUNDLE_DIR"
  log_info "Bundle directory: $BUNDLE_DIR"
fi

# ---------------------------------------------------------------------------
# 이미지 로컬 존재 확인
# ---------------------------------------------------------------------------
missing=()
for img in "${images[@]}"; do
  if ! docker image inspect "$img" >/dev/null 2>&1; then
    log_warn "Image not found locally (run 01-pull-images.sh first): $img"
    missing+=("$img")
  fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
  log_err "${#missing[@]} image(s) missing locally. Aborting."
  exit 1
fi

# ---------------------------------------------------------------------------
# docker save
# ---------------------------------------------------------------------------
log_info "Saving images to tar..."
if [[ "$DRY_RUN" == "1" ]]; then
  printf '[DRY_RUN] docker save %s -o %s\n' "${images[*]}" "$BUNDLE_TAR"
else
  docker save "${images[@]}" -o "$BUNDLE_TAR"
  log_info "Saved: $BUNDLE_TAR ($(du -sh "$BUNDLE_TAR" | cut -f1))"
fi

# ---------------------------------------------------------------------------
# gzip 압축
# ---------------------------------------------------------------------------
log_info "Compressing with gzip..."
if [[ "$DRY_RUN" == "1" ]]; then
  printf '[DRY_RUN] gzip -f %s -> %s\n' "$BUNDLE_TAR" "$BUNDLE_GZ"
else
  gzip -f "$BUNDLE_TAR"
  log_info "Compressed: $BUNDLE_GZ ($(du -sh "$BUNDLE_GZ" | cut -f1))"
fi

# ---------------------------------------------------------------------------
# SHA-256 체크섬
# ---------------------------------------------------------------------------
log_info "Computing SHA-256..."
if [[ "$DRY_RUN" == "1" ]]; then
  printf '[DRY_RUN] sha256sum %s > %s\n' "$BUNDLE_GZ" "$BUNDLE_SHA"
else
  sha256_file "$BUNDLE_GZ" > "$BUNDLE_SHA"
  log_info "Checksum written: $BUNDLE_SHA"
  cat "$BUNDLE_SHA"
fi

# ---------------------------------------------------------------------------
# MANIFEST.txt — image@digest 형식
# ---------------------------------------------------------------------------
log_info "Writing MANIFEST.txt..."
if [[ "$DRY_RUN" == "1" ]]; then
  printf '[DRY_RUN] Write %s with image@digest lines\n' "$MANIFEST"
else
  {
    printf '# Nullus Platform Air-Gap Bundle Manifest\n'
    printf '# Generated: %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    printf '# Format: image@sha256:digest\n'
    printf '#\n'
    for img in "${images[@]}"; do
      digest="$(docker image inspect --format='{{index .RepoDigests 0}}' "$img" 2>/dev/null || true)"
      if [[ -z "$digest" ]]; then
        # digest가 없는 경우(로컬 빌드 이미지 등) 이미지명만 기록
        printf '%s\n' "$img"
      else
        printf '%s\n' "$digest"
      fi
    done
  } > "$MANIFEST"
  log_info "Manifest written: $MANIFEST"
fi

# ---------------------------------------------------------------------------
# 완료 요약
# ---------------------------------------------------------------------------
printf '\n'
log_info "Bundle creation complete."
if [[ "$DRY_RUN" != "1" ]]; then
  log_info "  Archive : $BUNDLE_GZ"
  log_info "  Checksum: $BUNDLE_SHA"
  log_info "  Manifest: $MANIFEST"
fi
