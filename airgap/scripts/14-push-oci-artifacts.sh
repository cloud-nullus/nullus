#!/usr/bin/env bash
# =============================================================================
# 14-push-oci-artifacts.sh — [오프라인] OCI 아티팩트를 로컬 레지스트리로 푸시
# =============================================================================
# 목적  : bundle/oci-artifacts.tar.gz 를 검증·해제하고, 안의 OCI layout 을
#         로컬 레지스트리로 올린다. 12-push-to-registry.sh 의 아티팩트 판이다.
#
#         이 단계가 빠지면 Trivy 서버가 취약점 DB 를 받지 못한다. 서버는 뜨지만
#         스캔이 전부 실패하고, 오류가 스캐너 장애처럼 보여 원인을 찾기 어렵다.
#
# 경로 계산은 12-push-to-registry.sh 와 같은 규칙을 쓴다:
#   ghcr.io/aquasecurity/trivy-db:2 → localhost:5001/aquasecurity/trivy-db:2
# 서버는 같은 레지스트리를 클러스터 안의 이름(kind-registry:5000)으로 부른다 —
# API 설치는 NULLUS_HELM_OCI_REGISTRY 에서, helm 직접 설치는 stack-values/trivy.yaml 에서.
# 경로가 갈라지면 서버가 없는 곳에서 DB 를 찾는다.
#
# 사용법: bash 14-push-oci-artifacts.sh
#         DRY_RUN=1 bash 14-push-oci-artifacts.sh
#
# 환경변수:
#   REGISTRY_HOST   로컬 레지스트리 (기본: localhost:5001)
#   BUNDLE_DIR      번들 디렉토리 (기본: <ROOT>/bundle)
#   ARTIFACTS_FILE  목록 파일 (기본: <ROOT>/images/oci-artifacts.txt)
#   PLAIN_HTTP=0    레지스트리가 HTTPS 면 0 (기본: 1 — kind 로컬은 HTTP)
#
# 종료 코드:
#   0    전부 성공
#   1    하나 이상 실패 / 번들 없음 / 체크섬 불일치
#   127  oras 없음
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

REGISTRY_HOST="${REGISTRY_HOST:-localhost:5001}"
BUNDLE_DIR="${BUNDLE_DIR:-$ROOT_DIR/bundle}"
ARTIFACTS_FILE="${ARTIFACTS_FILE:-$ROOT_DIR/images/oci-artifacts.txt}"
PLAIN_HTTP="${PLAIN_HTTP:-1}"
DRY_RUN="${DRY_RUN:-0}"

BUNDLE_GZ="$BUNDLE_DIR/oci-artifacts.tar.gz"
BUNDLE_SHA="$BUNDLE_GZ.sha256"
LAYOUT_DIR="$BUNDLE_DIR/oci-artifacts"

log_info() { printf '\033[0;32m[INFO]\033[0m  %s\n' "$*"; }
log_warn() { printf '\033[0;33m[WARN]\033[0m  %s\n' "$*"; }
log_err()  { printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2; }

verify_sha256() {
  local shafile="$1" dir
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$(dirname "$shafile")" && sha256sum --check "$(basename "$shafile")")
  else
    dir="$(dirname "$shafile")"
    (cd "$dir" && shasum -a 256 --check "$(basename "$shafile")")
  fi
}

command -v oras >/dev/null 2>&1 || {
  if [[ "$DRY_RUN" == "1" ]]; then
    log_warn "oras 없음 — DRY_RUN 이라 계속합니다 (실제 실행에는 필요)"
  else
    log_err "oras not found — OCI 아티팩트는 docker 로 올릴 수 없습니다."
    log_err "  설치: brew install oras"
    log_err "        또는 https://oras.land/docs/installation"
    exit 127
  fi
}

# 12-push-to-registry.sh 의 compute_target 과 같은 규칙.
compute_target() {
  local src="${1%@*}" path
  if [[ "$src" == ghcr.io/* ]]; then
    path="${src#ghcr.io/}"
  elif [[ "$src" == docker.io/* ]]; then
    path="${src#docker.io/}"
  elif [[ "$src" =~ ^[^/]+\.[^/]+/ ]]; then
    path="${src#*/}"
  else
    path="$src"
  fi
  printf '%s/%s' "$REGISTRY_HOST" "$path"
}

layout_name() {
  printf '%s' "$1" | sed -e 's#[/:]#_#g'
}

[[ -f "$ARTIFACTS_FILE" ]] || { log_err "목록 파일이 없습니다: $ARTIFACTS_FILE"; exit 1; }

artifacts=()
while IFS= read -r line; do
  line="${line%%$'\r'}"
  [[ -z "$line" || "$line" == \#* ]] && continue
  artifacts+=("$line")
done < "$ARTIFACTS_FILE"

[[ ${#artifacts[@]} -gt 0 ]] || { log_err "목록이 비어 있습니다: $ARTIFACTS_FILE"; exit 1; }

oras_flags=()
[[ "$PLAIN_HTTP" == "1" ]] && oras_flags+=(--to-plain-http)

if [[ "$DRY_RUN" == "1" ]]; then
  log_warn "DRY_RUN — 명령만 출력합니다"
  printf '[DRY_RUN] sha256 verify: %s\n' "$BUNDLE_SHA"
  printf '[DRY_RUN] tar -xzf %s -C %s\n' "$BUNDLE_GZ" "$LAYOUT_DIR"
  for ref in "${artifacts[@]}"; do
    printf '[DRY_RUN] oras cp --from-oci-layout %s %s:%s %s\n' \
      "${oras_flags[*]}" "$LAYOUT_DIR/$(layout_name "$ref")" "${ref##*:}" "$(compute_target "$ref")"
  done
  exit 0
fi

# 번들이 없으면 조용히 넘기지 않는다. 스캐너를 고른 스택은 DB 없이 서면
# 스캔이 전부 실패하므로, 여기서 멈추는 편이 낫다.
[[ -f "$BUNDLE_GZ" ]] || {
  log_err "번들이 없습니다: $BUNDLE_GZ"
  log_err "  온라인 환경에서 먼저: bash scripts/pre/pull-oci-artifacts.sh"
  exit 1
}

if [[ -f "$BUNDLE_SHA" ]]; then
  log_info "체크섬 검증: $BUNDLE_SHA"
  verify_sha256 "$BUNDLE_SHA" || { log_err "체크섬 불일치 — 전송 중 손상됐습니다"; exit 1; }
else
  log_warn "체크섬 파일이 없습니다: $BUNDLE_SHA (검증 생략)"
fi

log_info "해제: $BUNDLE_GZ → $LAYOUT_DIR"
mkdir -p "$LAYOUT_DIR"
tar -xzf "$BUNDLE_GZ" -C "$LAYOUT_DIR"

failed=()
for ref in "${artifacts[@]}"; do
  tag="${ref##*:}"
  src="$LAYOUT_DIR/$(layout_name "$ref")"
  dst="$(compute_target "$ref")"
  if [[ ! -d "$src" ]]; then
    log_err "layout 이 번들에 없습니다: $ref ($src)"
    failed+=("$ref")
    continue
  fi
  log_info "push: $ref → $dst"
  if ! oras cp --from-oci-layout ${oras_flags[@]+"${oras_flags[@]}"} "${src}:${tag}" "$dst"; then
    log_err "  실패: $ref"
    failed+=("$ref")
  fi
done

if [[ ${#failed[@]} -gt 0 ]]; then
  log_err "실패 ${#failed[@]}건:"
  for f in "${failed[@]}"; do log_err "  - $f"; done
  exit 1
fi

log_info "완료: ${#artifacts[@]}건을 $REGISTRY_HOST 로 올렸습니다"
log_info "  Trivy 서버는 클러스터 안에서 같은 경로를 kind-registry:5000 으로 읽습니다 (NULLUS_HELM_OCI_REGISTRY / stack-values/trivy.yaml)"
