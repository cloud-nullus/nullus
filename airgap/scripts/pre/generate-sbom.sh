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
#   SBOM_REQUIRED=1 ./generate-sbom.sh        # 릴리스 경로 — 빠지면 실패
#
# 동작:
#   - 이미지별 SBOM → bundle/sbom/images/<sanitized>.<ext>
#   - helm 차트 SBOM → bundle/sbom/helm-charts.<ext>
#   - 인덱스 → bundle/sbom/INDEX.txt
#
# SBOM_REQUIRED (nullus-plan#77):
#   - 0 (기본, 로컬 개발): syft 가 없으면 경고 후 건너뛴다(exit 0). 이미지 하나의 실패도 경고다.
#   - 1 (릴리스 경로 — CI 는 기본 1): syft 부재 · 이미지 목록 부재 · 이미지/차트 SBOM 실패 하나라도
#     exit 1. 번들이 초록불인 채로 SBOM 만 빠지는 일이 실제로 있었다 — 아무도 모르는 채로 나갔다.
#     GitHub Actions 는 CI=true 를 주므로 CI 에서는 따로 켜지 않아도 엄격하다.
#
# 형식: 기본 spdx-json(SPDX 2.3) — 릴리스 이미지의 SBOM(cd.yml)과 같은 형식이다(릴리즈 정책 §7.4).
#
# 요구: syft v1+ (https://github.com/anchore/syft), 온라인(이미지 접근 가능).
# 종료 코드: 0 — 성공 또는 정상 건너뜀(SBOM_REQUIRED=0) / 1 — 엄격 모드에서 빠진 것이 있음
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

IMAGES_TXT="${IMAGES_TXT:-${ROOT_DIR}/images/images.txt}"
HELM_DIR="${HELM_DIR:-${ROOT_DIR}/helm}"
SBOM_DIR="${SBOM_DIR:-${ROOT_DIR}/bundle/sbom}"
SBOM_FORMAT="${SBOM_FORMAT:-spdx-json}"
# CI(GitHub Actions 등)에서는 기본 엄격 — 릴리스 경로에서 조용히 빠지지 않는다.
SBOM_REQUIRED="${SBOM_REQUIRED:-${CI:+1}}"
SBOM_REQUIRED="${SBOM_REQUIRED:-0}"

case "$SBOM_FORMAT" in
  spdx-json)      EXT="spdx.json" ;;
  cyclonedx-json) EXT="cdx.json" ;;
  *)              EXT="json" ;;
esac

log()  { printf '[sbom] %s\n' "$*" >&2; }
warn() { printf '[sbom][WARN] %s\n' "$*" >&2; }
err()  { printf '[sbom][ERR] %s\n' "$*" >&2; }

# skip_or_fail 은 "SBOM 이 빠지는 상황" 의 단일 출구다 — 엄격 모드면 실패, 아니면 건너뜀.
skip_or_fail() {
  if [[ "$SBOM_REQUIRED" == "1" ]]; then
    err "$* — SBOM_REQUIRED=1 이라 중단합니다 (릴리스 경로에서는 SBOM 이 빠지면 안 된다)"
    exit 1
  fi
  warn "$* — SBOM 생성을 건너뜁니다 (번들 빌드는 계속). 릴리스 경로라면 SBOM_REQUIRED=1 로 돌리세요."
  exit 0
}

if ! command -v syft >/dev/null 2>&1; then
  skip_or_fail "syft 미설치 (설치: brew install syft 또는 https://github.com/anchore/syft#installation)"
fi

if [[ ! -f "$IMAGES_TXT" ]]; then
  skip_or_fail "이미지 목록 없음: ${IMAGES_TXT}"
fi

mkdir -p "${SBOM_DIR}/images"
log "형식=${SBOM_FORMAT}  출력=${SBOM_DIR}  엄격=${SBOM_REQUIRED}  ($(syft version 2>/dev/null | head -1))"

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
    warn "SBOM 생성 실패: ${image}"
    failed=$((failed + 1))
  fi
done < "$IMAGES_TXT"
log "이미지 SBOM: ${count}건 생성, ${failed}건 실패"

helm_failed=0
if [[ -d "$HELM_DIR" ]]; then
  if syft scan "dir:${HELM_DIR}" -o "${SBOM_FORMAT}=${SBOM_DIR}/helm-charts.${EXT}" -q 2>/dev/null; then
    log "helm 차트 SBOM 생성 완료"
  else
    warn "helm 차트 SBOM 생성 실패"
    helm_failed=1
  fi
fi

( cd "$SBOM_DIR" && find . -type f -name "*.${EXT}" | sort > INDEX.txt )
log "완료 — ${SBOM_DIR}/INDEX.txt 참조"

if [[ "$SBOM_REQUIRED" == "1" ]] && (( failed > 0 || helm_failed > 0 )); then
  err "엄격 모드: 이미지 ${failed}건 · 차트 ${helm_failed}건의 SBOM 이 빠졌습니다 — 번들을 내보내지 마세요"
  exit 1
fi
