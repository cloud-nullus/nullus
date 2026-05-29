#!/usr/bin/env bash
# 목적: airgap/images/images.txt 에 나열된 이미지를 로컬 레지스트리로 리태그 후 푸시
# 용도: 번들 이미지를 localhost:5001 레지스트리에 업로드
# 사용법: ./12-push-to-registry.sh
# 필수 환경변수:
#   REGISTRY_HOST  - 로컬 레지스트리 주소 (기본값: localhost:5001)
#   IMAGES_LIST    - 이미지 목록 파일 경로 (기본값: airgap/images/images.txt)
# 종료 코드:
#   0 - 전체 성공
#   1 - 하나 이상 실패

set -euo pipefail
IFS=$'\n\t'

# ── 경로 해석 ────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# ── 설정값 ───────────────────────────────────────────────────
REGISTRY_HOST="${REGISTRY_HOST:-localhost:5001}"
IMAGES_LIST="${IMAGES_LIST:-${ROOT_DIR}/airgap/images/images.txt}"
DRY_RUN="${DRY_RUN:-0}"

# ── 로그 헬퍼 ────────────────────────────────────────────────
_tty() { [[ -t 1 ]]; }
log_info() { _tty && printf '\033[0;32m[INFO]\033[0m %s\n' "$*" || printf '[INFO] %s\n' "$*"; }
log_warn() { _tty && printf '\033[0;33m[WARN]\033[0m %s\n' "$*" || printf '[WARN] %s\n' "$*"; }
log_err()  { _tty && printf '\033[0;31m[ERR ]\033[0m %s\n' "$*" >&2 || printf '[ERR ] %s\n' "$*" >&2; }

# ── DRY_RUN 래퍼 ─────────────────────────────────────────────
run() {
  if [[ "${DRY_RUN}" == "1" ]]; then
    log_info "[DRY_RUN] $*"
  else
    "$@"
  fi
}

# ── 레지스트리 경로 계산 ─────────────────────────────────────
# 입력: ghcr.io/cloud-nullus/nullus-api:0.1.0-alpha
# 출력: localhost:5001/cloud-nullus/nullus-api:0.1.0-alpha
#
# 입력: docker.io/library/nginx:1.25
# 출력: localhost:5001/library/nginx:1.25
#
# 입력: nginx:1.25  (레지스트리 없음)
# 출력: localhost:5001/library/nginx:1.25
compute_target() {
  local src="$1"
  local path

  # @sha256:... digest 제거 — 타깃 reference 에는 digest 를 포함할 수 없다
  # (docker tag <src> <host>/<path>:<tag>@sha256:... 는 invalid reference 로 실패)
  src="${src%@*}"

  # 알려진 레지스트리 프리픽스 제거
  if [[ "${src}" == ghcr.io/* ]]; then
    path="${src#ghcr.io/}"
  elif [[ "${src}" == docker.io/* ]]; then
    path="${src#docker.io/}"
  elif [[ "${src}" == registry-1.docker.io/* ]]; then
    path="${src#registry-1.docker.io/}"
  elif [[ "${src}" =~ ^[^/]+\.[^/]+/ ]]; then
    # 일반적인 <host>/<path> 패턴 — 첫 번째 슬래시 앞 부분(레지스트리 호스트) 제거
    path="${src#*/}"
  else
    # 레지스트리 없음 (예: nginx:1.25) — library/ 접두사 추가
    if [[ "${src}" != */* ]]; then
      path="library/${src}"
    else
      path="${src}"
    fi
  fi

  echo "${REGISTRY_HOST}/${path}"
}

# ── 로컬 소스 reference 해석 ─────────────────────────────────
# images.txt 항목(name:tag@sha256:digest 등)이 로컬 daemon 에 어떤 이름으로
# 적재돼 있는지는 pull 방식에 따라 다르다(태그형 vs digest형). 실제로 존재하는
# reference 를 찾아 docker tag 의 소스로 사용한다.
resolve_local() {
  local entry="$1"

  # 1) 항목 그대로
  if docker image inspect "${entry}" >/dev/null 2>&1; then echo "${entry}"; return 0; fi

  # 2) digest 형식: name:tag@sha256:... → name@sha256:...
  if [[ "${entry}" == *@sha256:* ]]; then
    local name_tag="${entry%@*}" dig="${entry##*@}"
    local name="${name_tag%:*}"
    if docker image inspect "${name}@${dig}" >/dev/null 2>&1; then echo "${name}@${dig}"; return 0; fi
  fi

  # 3) 태그 형식: digest 제거
  local no_digest="${entry%@*}"
  if docker image inspect "${no_digest}" >/dev/null 2>&1; then echo "${no_digest}"; return 0; fi

  # 마지막 폴백 — 원본 그대로(이후 docker tag 가 실패하면 건너뜀)
  echo "${entry}"
}

# ── 메인 ─────────────────────────────────────────────────────
main() {
  log_info "==> 이미지 로컬 레지스트리 푸시 시작 → ${REGISTRY_HOST}"

  if [[ ! -f "${IMAGES_LIST}" ]]; then
    log_err "이미지 목록 파일 없음: ${IMAGES_LIST}"
    exit 1
  fi

  local failed=0
  local total=0
  local succeeded=0

  while IFS= read -r image || [[ -n "${image}" ]]; do
    # 빈 줄 및 주석 건너뜀
    [[ -z "${image}" || "${image}" == \#* ]] && continue
    total=$((total + 1))

    local target source
    target="$(compute_target "${image}")"
    source="$(resolve_local "${image}")"

    log_info "[${total}] ${source} → ${target}"

    # 리태그
    if ! run docker tag "${source}" "${target}" 2>&1; then
      log_warn "  태그 실패: ${source} — 건너뜀"
      failed=$((failed + 1))
      continue
    fi

    # 푸시
    if ! run docker push "${target}" 2>&1; then
      log_warn "  푸시 실패: ${target}"
      failed=$((failed + 1))
      continue
    fi

    succeeded=$((succeeded + 1))
    log_info "  완료: ${target}"
  done < "${IMAGES_LIST}"

  log_info "==> 완료: 성공 ${succeeded}/${total}, 실패 ${failed}/${total}"

  if [[ ${failed} -gt 0 ]]; then
    log_err "일부 이미지 푸시 실패 (${failed}개) — 로그를 확인하세요"
    exit 1
  fi
}

main "$@"
