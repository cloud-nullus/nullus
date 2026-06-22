#!/usr/bin/env bash
# =============================================================================
# pre/generate-sbom.sh — 번들 구성요소 SBOM 생성 (syft, 온라인 빌드 단계)
# =============================================================================
# 용도: images/images.txt 의 컨테이너 이미지와 helm 차트의 SBOM 을 생성해
#       bundle/sbom/ 에 저장한다. package-bundle.sh 가 bundle/ 전체를 dist 에
#       포함하므로 SBOM 도 마스터 번들에 함께 반입된다.
#
# 사용법:
#   ./generate-sbom.sh
#   SBOM_FORMAT=cyclonedx-json ./generate-sbom.sh
#
# 동작:
#   - syft 미설치 시: 경고 후 건너뜀(exit 0) — 번들 빌드를 막지 않는다.
#   - 이미지별 SBOM → bundle/sbom/images/<sanitized>.<ext>
#   - helm 차트 SBOM → bundle/sbom/helm-charts.<ext>
#   - 인덱스 → bundle/sbom/INDEX.txt
#
# 요구: syft v1+ (https://github.com/anchore/syft), 온라인(이미지 접근 가능).
# 종료 코드: 0 — 성공 또는 정상 건너뜀
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

IMAGES_TXT="${IMAGES_TXT:-${ROOT_DIR}/images/images.txt}"
HELM_DIR="${HELM_DIR:-${ROOT_DIR}/helm}"
SBOM_DIR="${SBOM_DIR:-${ROOT_DIR}/bundle/sbom}"
SBOM_FORMAT="${SBOM_FORMAT:-spdx-json}"

case "$SBOM_FORMAT" in
  spdx-json)      EXT="spdx.json" ;;
  cyclonedx-json) EXT="cdx.json" ;;
  *)              EXT="json" ;;
esac

log()  { printf '[sbom] %s\n' "$*" >&2; }
warn() { printf '[sbom][WARN] %s\n' "$*" >&2; }

if ! command -v syft >/dev/null 2>&1; then
  warn "syft 미설치 — SBOM 생성을 건너뜁니다 (번들 빌드는 계속)."
  warn "설치: brew install syft  또는  https://github.com/anchore/syft#installation"
  exit 0
fi

if [[ ! -f "$IMAGES_TXT" ]]; then
  warn "이미지 목록 없음: ${IMAGES_TXT} — 건너뜀"
  exit 0
fi

mkdir -p "${SBOM_DIR}/images"
log "형식=${SBOM_FORMAT}  출력=${SBOM_DIR}  ($(syft version 2>/dev/null | head -1))"

count=0
failed=0
while IFS= read -r image; do
  image="${image%%#*}"                       # 줄 주석 제거
  image="$(printf '%s' "$image" | tr -d '[:space:]')"
  [[ -z "$image" ]] && continue
  safe="$(printf '%s' "$image" | sed 's#[/:@]#_#g')"
  if syft scan "$image" -o "${SBOM_FORMAT}=${SBOM_DIR}/images/${safe}.${EXT}" -q 2>/dev/null; then
    count=$((count + 1))
  else
    warn "SBOM 생성 실패(건너뜀): ${image}"
    failed=$((failed + 1))
  fi
done < "$IMAGES_TXT"
log "이미지 SBOM: ${count}건 생성, ${failed}건 실패"

if [[ -d "$HELM_DIR" ]]; then
  if syft scan "dir:${HELM_DIR}" -o "${SBOM_FORMAT}=${SBOM_DIR}/helm-charts.${EXT}" -q 2>/dev/null; then
    log "helm 차트 SBOM 생성 완료"
  else
    warn "helm 차트 SBOM 생성 실패"
  fi
fi

( cd "$SBOM_DIR" && find . -type f -name "*.${EXT}" | sort > INDEX.txt )
log "완료 — ${SBOM_DIR}/INDEX.txt 참조"
