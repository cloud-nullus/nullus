#!/usr/bin/env bash
# =============================================================================
# pull-oci-artifacts.sh — [온라인] OCI 아티팩트 반입
# =============================================================================
# 목적  : images/oci-artifacts.txt 의 OCI 아티팩트를 로컬 OCI layout 으로 받아
#         bundle/oci-artifacts.tar.gz 로 묶는다.
#
#         이 산출물이 없으면 에어갭에서 Trivy 서버가 취약점 DB 를 받지 못해
#         스캔이 **전부 실패**한다. 스캐너를 고른 스택은 배포가 멈춘다.
#
# 왜 별도 경로인가:
#   trivy-db 는 컨테이너 이미지가 아니라 OCI 아티팩트다. 이미지 미디어 타입이
#   아니므로 docker pull / docker save 라운드트립을 태울 수 없다. oras 가
#   필요하고, 폴백이 없다 — 그래서 도구 부재를 경고가 아니라 실패로 다룬다.
#
# 사용법: bash pre/pull-oci-artifacts.sh
#         DRY_RUN=1 bash pre/pull-oci-artifacts.sh
#
# 환경변수:
#   BUNDLE_DIR      번들 디렉토리 (기본: <ROOT>/bundle)
#   ARTIFACTS_FILE  목록 파일 (기본: <ROOT>/images/oci-artifacts.txt)
#   DRY_RUN=1       명령만 출력
#
# 종료 코드:
#   0    전부 성공
#   1    하나 이상 실패
#   127  oras 없음
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

BUNDLE_DIR="${BUNDLE_DIR:-$ROOT_DIR/bundle}"
ARTIFACTS_FILE="${ARTIFACTS_FILE:-$ROOT_DIR/images/oci-artifacts.txt}"
DRY_RUN="${DRY_RUN:-0}"

LAYOUT_DIR="$BUNDLE_DIR/oci-artifacts"
BUNDLE_GZ="$BUNDLE_DIR/oci-artifacts.tar.gz"
BUNDLE_SHA="$BUNDLE_GZ.sha256"
MANIFEST="$BUNDLE_DIR/OCI-ARTIFACTS.txt"

log_info() { printf '\033[0;32m[INFO]\033[0m  %s\n' "$*"; }
log_warn() { printf '\033[0;33m[WARN]\033[0m  %s\n' "$*"; }
log_err()  { printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2; }

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file"
  else
    shasum -a 256 "$file"
  fi
}

# oras 는 선택이 아니다. docker 로 대체할 수 없으므로 조용히 건너뛰면
# 번들이 초록불인 채로 DB 만 빠진다 — generate-sbom.sh 가 syft 부재를
# exit 0 으로 넘겨 SBOM 이 통째로 빠졌던 것과 같은 실패다.
command -v oras >/dev/null 2>&1 || {
  if [[ "$DRY_RUN" == "1" ]]; then
    # DRY_RUN 은 무엇을 할지 보여주는 것이 목적이다. 도구가 없다고 막으면
    # 스크립트를 읽어볼 수도 없다.
    log_warn "oras 없음 — DRY_RUN 이라 계속합니다 (실제 실행에는 필요)"
  else
    log_err "oras not found — OCI 아티팩트는 docker 로 받을 수 없습니다."
    log_err "  설치: brew install oras"
    log_err "        또는 https://oras.land/docs/installation"
    exit 127
  fi
}

[[ -f "$ARTIFACTS_FILE" ]] || {
  log_err "목록 파일이 없습니다: $ARTIFACTS_FILE"
  exit 1
}

# layout 디렉토리 이름 — 레지스트리 경로의 슬래시·콜론을 파일명에 쓸 수 없다.
layout_name() {
  printf '%s' "$1" | sed -e 's#[/:]#_#g'
}

artifacts=()
while IFS= read -r line; do
  line="${line%%$'\r'}"
  [[ -z "$line" || "$line" == \#* ]] && continue
  artifacts+=("$line")
done < "$ARTIFACTS_FILE"

[[ ${#artifacts[@]} -gt 0 ]] || {
  log_err "목록이 비어 있습니다: $ARTIFACTS_FILE"
  exit 1
}

log_info "아티팩트 ${#artifacts[@]}건: $ARTIFACTS_FILE"
[[ "$DRY_RUN" == "1" ]] && log_warn "DRY_RUN — 명령만 출력합니다"

failed=()
for ref in "${artifacts[@]}"; do
  tag="${ref##*:}"
  dest="$LAYOUT_DIR/$(layout_name "$ref")"
  if [[ "$DRY_RUN" == "1" ]]; then
    printf '[DRY_RUN] mkdir -p %s\n' "$dest"
    printf '[DRY_RUN] oras cp --to-oci-layout %s %s:%s\n' "$ref" "$dest" "$tag"
    continue
  fi
  log_info "pull: $ref"
  mkdir -p "$dest"
  if ! oras cp --to-oci-layout "$ref" "${dest}:${tag}"; then
    log_err "  실패: $ref"
    failed+=("$ref")
  fi
done

if [[ "$DRY_RUN" == "1" ]]; then
  printf '[DRY_RUN] tar -czf %s -C %s .\n' "$BUNDLE_GZ" "$LAYOUT_DIR"
  printf '[DRY_RUN] sha256 %s > %s\n' "$BUNDLE_GZ" "$BUNDLE_SHA"
  exit 0
fi

if [[ ${#failed[@]} -gt 0 ]]; then
  log_err "실패 ${#failed[@]}건 — 번들을 만들지 않습니다:"
  for f in "${failed[@]}"; do log_err "  - $f"; done
  exit 1
fi

log_info "압축: $BUNDLE_GZ"
tar -czf "$BUNDLE_GZ" -C "$LAYOUT_DIR" .
sha256_file "$BUNDLE_GZ" > "$BUNDLE_SHA"

# 무엇이 언제 들어갔는지 남긴다. 취약점 DB 는 나이가 곧 신뢰도라
# 반입 시각이 없으면 결과를 얼마나 믿을지 판단할 수 없다.
{
  printf '# OCI 아티팩트 번들\n'
  printf '# 생성: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '#\n'
  printf '# 취약점 DB 는 이 시각 기준이다. 에어갭에서는 다시 반입할 때까지\n'
  printf '# 더 새로워지지 않는다.\n'
  for ref in "${artifacts[@]}"; do printf '%s\n' "$ref"; done
} > "$MANIFEST"

log_info "완료: $(du -sh "$BUNDLE_GZ" | cut -f1) — $BUNDLE_GZ"
log_info "매니페스트: $MANIFEST"
