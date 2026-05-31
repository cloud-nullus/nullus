#!/usr/bin/env bash
# =============================================================================
# 24-register-hosts.sh — 외부 접근 도메인을 /etc/hosts 에 등록/해제
# =============================================================================
# 용도: 23-setup-gateway.sh 가 생성한 HTTPRoute 의 hostname 들을 읽어
#       /etc/hosts 의 관리 블록(마커 구간)에 멱등적으로 기록한다.
#       kind 환경엔 실 LoadBalancer/DNS 가 없어 도메인을 127.0.0.1 로 물려
#       `kubectl port-forward` 한 envoy 데이터플레인으로 접근하기 위함이다.
#
#   - 호스트네임 출처: 클러스터의 모든 HTTPRoute (.spec.hostnames) — 실제 라우팅 진실
#   - /etc/hosts 수정엔 sudo 필요
#   - 마커 블록으로 관리하므로 재실행/해제 안전 (멱등)
#
# 사용법:
#   ./24-register-hosts.sh            # 등록 (127.0.0.1 매핑)
#   HOSTS_IP=192.168.0.10 ./24-register-hosts.sh   # 다른 IP 로 매핑 (실 LB/노드 IP)
#   DRY_RUN=1 ./24-register-hosts.sh  # 미리보기 (파일 수정 안 함)
#   REMOVE=1  ./24-register-hosts.sh  # 관리 블록 제거 (해제)
#
# 환경 변수:
#   HOSTS_IP    매핑할 IP (기본: 127.0.0.1)
#   HOSTS_FILE  대상 파일 (기본: /etc/hosts)
#   ACCESS_DOMAIN  클러스터 미접근 시 fallback 도메인 (기본: nullus.internal)
#   DRY_RUN     1 = 미리보기
#   REMOVE      1 = 블록 제거
#
# 종료 코드:
#   0 — 성공
#   1 — 사전조건 실패 / 등록할 호스트 없음
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_OK=$'\033[1;32m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_WARN=""; CL_ERR=""; CL_OK=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }

HOSTS_IP="${HOSTS_IP:-127.0.0.1}"
HOSTS_FILE="${HOSTS_FILE:-/etc/hosts}"
ACCESS_DOMAIN="${ACCESS_DOMAIN:-nullus.internal}"
DRY_RUN="${DRY_RUN:-0}"
REMOVE="${REMOVE:-0}"

BEGIN_MARK="# >>> nullus-airgap (managed by 24-register-hosts.sh) >>>"
END_MARK="# <<< nullus-airgap <<<"

[[ -f "$HOSTS_FILE" ]] || { log_err "$HOSTS_FILE 없음"; exit 1; }

# sudo 래퍼: 대상 파일에 쓰기권한이 있으면 sudo 불필요(루트/소유자/테스트용 임시파일),
# 없으면 sudo 사용. DRY_RUN 은 어차피 쓰지 않으므로 빈 값.
need_sudo() {
  if [[ "$DRY_RUN" == "1" || -w "$HOSTS_FILE" ]]; then SUDO=""; else SUDO="sudo"; fi
}
need_sudo

# 기존 관리 블록 제거한 본문을 stdout 으로
strip_block() {
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
    $0==b {inblock=1; next}
    $0==e {inblock=0; next}
    !inblock {print}
  ' "$HOSTS_FILE"
}

# -----------------------------------------------------------------------------
# 해제 모드
# -----------------------------------------------------------------------------
if [[ "$REMOVE" == "1" ]]; then
  if ! grep -qF "$BEGIN_MARK" "$HOSTS_FILE"; then
    log_info "관리 블록 없음 — 변경 사항 없음"
    exit 0
  fi
  NEW_CONTENT="$(strip_block)"
  if [[ "$DRY_RUN" == "1" ]]; then
    log_info "[DRY_RUN] 아래 내용으로 $HOSTS_FILE 교체 예정 (블록 제거):"
    printf '%s\n' "$NEW_CONTENT" | tail -5 >&2
    exit 0
  fi
  printf '%s\n' "$NEW_CONTENT" | $SUDO tee "$HOSTS_FILE" >/dev/null
  log_ok "관리 블록 제거 완료"
  exit 0
fi

# -----------------------------------------------------------------------------
# 등록 모드: 호스트네임 수집
# -----------------------------------------------------------------------------
HOSTS_LIST=""
if command -v kubectl >/dev/null && kubectl cluster-info >/dev/null 2>&1; then
  HOSTS_LIST="$(kubectl get httproute -A \
    -o jsonpath='{range .items[*]}{range .spec.hostnames[*]}{@}{"\n"}{end}{end}' 2>/dev/null \
    | grep -vE '^\s*$' | sort -u || true)"
fi

# fallback: 클러스터 미접근 시 표준 서브도메인으로 생성
if [[ -z "$HOSTS_LIST" ]]; then
  log_warn "클러스터에서 HTTPRoute 호스트네임을 못 읽음 — 표준 서브도메인으로 fallback (도메인: $ACCESS_DOMAIN)"
  # apex = Nullus 포털 (nullus-web)
  HOSTS_LIST+="${ACCESS_DOMAIN}"$'\n'
  for sub in api argocd harbor minio opensearch gitlab grafana prometheus keycloak openbao; do
    HOSTS_LIST+="${sub}.${ACCESS_DOMAIN}"$'\n'
  done
  HOSTS_LIST="$(printf '%s' "$HOSTS_LIST" | grep -vE '^\s*$' | sort -u)"
fi

if [[ -z "$HOSTS_LIST" ]]; then
  log_err "등록할 호스트네임이 없음"
  exit 1
fi

COUNT="$(printf '%s\n' "$HOSTS_LIST" | grep -c .)"
log_info "대상 후보 호스트 ${COUNT}개 (IP: $HOSTS_IP)"

# 관리 블록을 제외한 기존 본문 — 여기에 이미 있는 호스트는 중복 추가하지 않는다.
REST="$(strip_block)"

# 기존 본문(주석 제외)에 해당 호스트네임이 토큰 단위로 존재하는가?
host_in_rest() {
  local h="$1" he
  he="$(printf '%s' "$h" | sed 's/[.[\*^$()+?{|]/\\&/g')"  # 정규식 메타 이스케이프
  printf '%s\n' "$REST" | grep -vE '^[[:space:]]*#' \
    | grep -qE "(^|[[:space:]])${he}([[:space:]]|\$)"
}

# 새 블록 구성 — 기존에 없는 호스트만 (IP 당 한 줄)
BLOCK="$BEGIN_MARK"$'\n'
added=0; skipped=0
while IFS= read -r h; do
  [[ -z "$h" ]] && continue
  if host_in_rest "$h"; then
    log_warn "스킵(이미 존재): $h"
    skipped=$((skipped+1))
    continue
  fi
  BLOCK+="${HOSTS_IP} ${h}"$'\n'
  added=$((added+1))
done <<<"$HOSTS_LIST"
BLOCK+="$END_MARK"

log_info "추가 ${added}개 / 스킵 ${skipped}개 (기존 존재)"
if [[ "$added" -eq 0 ]]; then
  log_info "추가할 신규 호스트 없음 — 관리 블록만 정리"
fi

NEW_CONTENT="$(strip_block)"$'\n'"$BLOCK"

if [[ "$DRY_RUN" == "1" ]]; then
  log_info "[DRY_RUN] $HOSTS_FILE 에 추가될 블록:"
  printf '%s\n' "$BLOCK" >&2
  exit 0
fi

# 백업 후 기록
BAK="${HOSTS_FILE}.nullus.bak.$(date +%Y%m%d-%H%M%S)"
$SUDO cp "$HOSTS_FILE" "$BAK" && log_info "백업: $BAK"
printf '%s\n' "$NEW_CONTENT" | $SUDO tee "$HOSTS_FILE" >/dev/null
log_ok "/etc/hosts 등록 완료 (신규 ${added}개 추가 / ${skipped}개 기존 스킵 → $HOSTS_IP)"

echo ""
echo "다음 단계 (별도 터미널에서 envoy 데이터플레인 포워딩):"
ENVOY_SVC="$(kubectl get svc -n nullus -l gateway.envoyproxy.io/owning-gateway-name=nullus-gateway -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
if [[ -n "$ENVOY_SVC" && "$HOSTS_IP" == "127.0.0.1" ]]; then
  echo "  sudo kubectl port-forward -n nullus svc/${ENVOY_SVC} 80:80"
fi
echo "  → 브라우저: http://$(printf '%s' "$HOSTS_LIST" | head -1)/"
