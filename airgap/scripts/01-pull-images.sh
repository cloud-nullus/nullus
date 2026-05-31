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
# 타깃 플랫폼 — 멀티아치 인덱스를 단일 플랫폼으로 평탄화한다.
#   docker(containerd 스토어)의 pull→save→load→push 라운드트립은 멀티아치
#   인덱스의 플랫폼별 blob 을 온전히 보존하지 못해, 설치 시 push 가
#   "does not provide any platform" 으로 실패한다. crane 으로 단일 플랫폼만
#   받아 docker 로 load 하면 단일 매니페스트 이미지가 되어 정상 동작한다.
# ---------------------------------------------------------------------------
_host_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *)             uname -m ;;
  esac
}
TARGET_PLATFORM="${TARGET_PLATFORM:-linux/$(_host_arch)}"

# USE_CRANE: auto|1|0 — auto 면 docker 런타임 + crane 존재 시 활성화
USE_CRANE="${USE_CRANE:-auto}"
if [[ "$USE_CRANE" == "auto" ]]; then
  if [[ "$RUNTIME" == "docker" ]] && command -v crane >/dev/null 2>&1; then
    USE_CRANE=1
  else
    USE_CRANE=0
  fi
fi

if [[ "$USE_CRANE" != "1" && "$RUNTIME" == "docker" ]]; then
  log_warn "crane 미사용 — 멀티아치 이미지가 docker save/load 라운드트립에서 손상돼"
  log_warn "  설치 시 push 가 'does not provide any platform' 으로 실패할 수 있습니다."
  log_warn "  권장: crane 설치 후 재실행 (go install github.com/google/go-containerregistry/cmd/crane@latest"
  log_warn "        또는 brew install crane). 강제 활성화: USE_CRANE=1"
fi

# 단일 이미지 pull — crane 평탄화 우선, 실패 시 런타임 --platform 폴백
pull_one() {
  local image="$1"
  if [[ "$USE_CRANE" == "1" ]]; then
    local tmp loaded desired
    tmp="$(mktemp -t airgap-img.XXXXXX)"
    if crane pull --platform "$TARGET_PLATFORM" "$image" "$tmp" 2>/dev/null; then
      # docker load 가 실제 적재한 reference 파싱
      loaded="$(docker load -i "$tmp" 2>/dev/null | sed -n 's/^Loaded image: //p' | head -1)"
      rm -f "$tmp"
      if [[ -n "$loaded" ]]; then
        # digest-pin 항목(name:tag@sha256:...)은 crane 이 ':i-was-a-digest' 로 적재하므로
        # 이후 save/push 가 기대하는 digest-제거 태그 형식으로 재태그한다.
        desired="${image%@*}"
        if [[ "$loaded" != "$desired" ]]; then
          docker tag "$loaded" "$desired" >/dev/null 2>&1 || true
        fi
        return 0
      fi
    else
      rm -f "$tmp"
    fi
    log_warn "crane pull 실패 → ${RUNTIME} pull --platform 폴백: $image"
  fi
  "$RUNTIME" pull --platform "$TARGET_PLATFORM" "$image"
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
log_info "Target platform: $TARGET_PLATFORM (USE_CRANE=$USE_CRANE)"
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

  log_info "Pulling ($TARGET_PLATFORM): $image"
  if pull_one "$image"; then
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
