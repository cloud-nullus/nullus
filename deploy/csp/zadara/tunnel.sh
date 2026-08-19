#!/usr/bin/env bash
# =============================================================================
# tunnel.sh — Zadara PoC 웹 UI 를 로컬 브라우저로 여는 SSH 터널
# =============================================================================
# Zadara 보안 그룹은 외부 → node-10 에 22/tcp 만 열어 둔다. ingress-nginx 는
# NodePort 30080 이라 그대로는 브라우저로 못 붙는다. 콘솔에서 포트를 여는 대신
# 이미 열린 22/tcp 위로 터널을 뚫는다.
#
# 두 가지 모드가 있다.
#
#   direct  (기본)  브라우저 → localhost → ssh → bastion → kubectl port-forward
#                   → svc/nullus-web
#                   ingress 를 거치지 않으므로 Host 헤더가 필요 없다. hosts 파일도
#                   sudo 도 필요 없어 바로 열린다. web 컨테이너의 nginx 가 /api/·/ws/
#                   를 프록시하므로 애플리케이션 기능은 그대로 동작한다.
#
#   ingress         브라우저 → localhost → ssh → node-10:30080 → ingress-nginx
#                   → svc/nullus-web
#                   실제 외부 접근 경로와 같다. ingress 가 Host 헤더로 라우팅하므로
#                   hosts 파일에 nullus.zadara.poc 매핑이 필요하고, 그 등록에 sudo 가
#                   필요하다. IP 로 직접 붙으면 404 가 난다.
#
# 사용법:
#   ./tunnel.sh                  direct 모드로 열고 브라우저 실행
#   MODE=ingress ./tunnel.sh     ingress 경유 (sudo 로 hosts 등록)
#   ./tunnel.sh status           현재 상태
#   ./tunnel.sh stop             터널 종료
#   ./tunnel.sh clean            터널 종료 + hosts 항목 제거
#
# 환경 변수:
#   MODE         direct | ingress          (기본: direct)
#   SSH_KEY      SSH 키 경로               (기본: nullus-key.pem 자동 탐색)
#   BASTION      bastion 접속 대상         (필수, .env 또는 환경변수)
#   LOCAL_PORT   로컬 포트                 (기본: 30080)
#   NAMESPACE    Nullus 네임스페이스       (기본: nullus)
#   KUBE_CONTEXT bastion 의 kube 컨텍스트  (기본: platform)
#   HOST_NAME    ingress host              (기본: nullus.zadara.poc)
#   NODE_IP      ingress 모드 목적지       (기본: 172.31.0.10)
#   NODE_PORT    ingress NodePort          (기본: 30080)
#   NO_OPEN=1    브라우저를 열지 않는다
#
# 종료 코드: 0 성공 / 1 사전조건·검증 실패
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

# 환경별 값은 저장소가 아니라 .env 에 둔다 (gitignore). 없으면 환경변수로 받는다.
# env.example 을 복사해 채운다: cp env.example .env
# .env 의 값은 "${VAR:-...}" 형태라 명령줄 환경변수가 우선한다.
# shellcheck source=/dev/null
[[ -f "$SCRIPT_DIR/.env" ]] && source "$SCRIPT_DIR/.env"

MODE="${MODE:-direct}"
BASTION="${BASTION:-}"
# 로컬 포트는 NodePort 와 같은 30080 을 기본으로 쓴다. 8080 은 Docker 등이
# 이미 잡고 있는 경우가 흔하다.
LOCAL_PORT="${LOCAL_PORT:-30080}"
NAMESPACE="${NAMESPACE:-nullus}"
KUBE_CONTEXT="${KUBE_CONTEXT:-platform}"
WEB_SVC="${WEB_SVC:-nullus-web}"
REMOTE_PORT="${REMOTE_PORT:-18080}"   # direct 모드에서 bastion 이 여는 임시 포트
HOST_NAME="${HOST_NAME:-nullus.zadara.poc}"
NODE_IP="${NODE_IP:-172.31.0.10}"
NODE_PORT="${NODE_PORT:-30080}"

CTRL_SOCK="${TMPDIR:-/tmp}/nullus-zadara-tunnel.sock"
HOSTS_FILE="${HOSTS_FILE:-/etc/hosts}"
HOSTS_MARK="# nullus-zadara-poc (deploy/csp/zadara/tunnel.sh)"

if [[ -t 1 ]]; then
  C_OK=$'\033[1;32m'; C_ERR=$'\033[1;31m'; C_WARN=$'\033[1;33m'; C_DIM=$'\033[2m'; C_RST=$'\033[0m'
else
  C_OK=""; C_ERR=""; C_WARN=""; C_DIM=""; C_RST=""
fi
ok()   { printf '%s✔%s %s\n' "$C_OK"   "$C_RST" "$*"; }
warn() { printf '%s!%s %s\n' "$C_WARN" "$C_RST" "$*"; }
info() { printf '%s·%s %s\n' "$C_DIM"  "$C_RST" "$*"; }
die()  { printf '%s✘%s %s\n' "$C_ERR"  "$C_RST" "$*" >&2; exit 1; }

# BASTION 은 기본값을 두지 않는다. 특정 환경의 주소를 코드에 박으면 다른 환경에서
# 조용히 엉뚱한 곳에 붙는다 — 틀린 기본값보다 멈추는 편이 낫다.
[[ -n "${BASTION:-}" ]] || die "BASTION 이 필요합니다.
    cp $SCRIPT_DIR/env.example $SCRIPT_DIR/.env 로 복사해 채우거나
    BASTION=ubuntu@<공인IP> $0 ... 로 넘기십시오."


# -----------------------------------------------------------------------------
# SSH 키 탐색 — 저장소 밖에 두는 경우가 많아 몇 군데를 훑는다.
# -----------------------------------------------------------------------------
find_key() {
  if [[ -n "${SSH_KEY:-}" ]]; then
    [[ -f "$SSH_KEY" ]] || die "SSH_KEY 가 없습니다: $SSH_KEY"
    printf '%s\n' "$SSH_KEY"; return
  fi
  local c
  for c in \
    "$SCRIPT_DIR/../../../../nullus-key.pem" \
    "$SCRIPT_DIR/../../../nullus-key.pem" \
    "$HOME/.ssh/nullus-key.pem" \
    "$HOME/nullus-key.pem"
  do
    [[ -f "$c" ]] && { printf '%s\n' "$(cd -- "$(dirname -- "$c")" && pwd)/$(basename -- "$c")"; return; }
  done
  die "nullus-key.pem 을 찾지 못했습니다. SSH_KEY=<경로> 로 지정하세요."
}

tunnel_up()   { ssh -S "$CTRL_SOCK" -O check "$BASTION" >/dev/null 2>&1; }
port_in_use() { lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1; }
http_code()   { curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$@" || true; }

# -----------------------------------------------------------------------------
# hosts 매핑 — ingress 모드에서만 필요하다. 같은 줄이 있으면 건드리지 않는다.
# -----------------------------------------------------------------------------
hosts_present() { grep -qE "^127\.0\.0\.1[[:space:]]+${HOST_NAME}([[:space:]]|$)" "$HOSTS_FILE" 2>/dev/null; }

hosts_add() {
  hosts_present && { info "hosts 항목 이미 존재: $HOST_NAME"; return; }
  warn "hosts 등록에 sudo 가 필요합니다 ($HOSTS_FILE) — 암호를 입력하세요"
  printf '127.0.0.1\t%s\t%s\n' "$HOST_NAME" "$HOSTS_MARK" | sudo tee -a "$HOSTS_FILE" >/dev/null \
    || die "hosts 등록 실패. MODE=direct 로 실행하면 hosts 없이 접속할 수 있습니다."
  ok "hosts 등록: 127.0.0.1 → $HOST_NAME"
}

hosts_remove() {
  hosts_present || { info "hosts 항목 없음 — 건너뜁니다"; return; }
  warn "hosts 항목 제거에 sudo 가 필요합니다"
  sudo sed -i.bak "/^127\.0\.0\.1[[:space:]]\{1,\}${HOST_NAME}\([[:space:]]\|$\)/d" "$HOSTS_FILE" \
    || die "hosts 제거 실패"
  ok "hosts 항목 제거 (백업: ${HOSTS_FILE}.bak)"
}

# -----------------------------------------------------------------------------
# direct — bastion 에서 kubectl port-forward 를 띄우고 그 포트를 로컬로 당긴다.
# 원격 명령을 SSH 마스터 세션에 붙여 두어, 터널을 닫으면 port-forward 도 함께
# 죽는다 (별도 pidfile 관리가 필요 없다).
# -----------------------------------------------------------------------------
start_direct() {
  local key="$1"
  ssh -i "$key" -o StrictHostKeyChecking=accept-new -o ExitOnForwardFailure=yes \
      -M -S "$CTRL_SOCK" -f \
      -L "${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}" "$BASTION" \
      "kubectl --context=${KUBE_CONTEXT} -n ${NAMESPACE} port-forward --address 127.0.0.1 svc/${WEB_SVC} ${REMOTE_PORT}:80" \
    || die "SSH 터널 생성 실패"
}

# -----------------------------------------------------------------------------
# ingress — node-10 의 NodePort 로 바로 붙는다. 실제 외부 접근 경로와 동일하다.
# -----------------------------------------------------------------------------
start_ingress() {
  local key="$1"
  ssh -i "$key" -o StrictHostKeyChecking=accept-new -o ExitOnForwardFailure=yes \
      -M -S "$CTRL_SOCK" -fN \
      -L "${LOCAL_PORT}:${NODE_IP}:${NODE_PORT}" "$BASTION" \
    || die "SSH 터널 생성 실패"
}

do_start() {
  command -v ssh  >/dev/null || die "ssh 가 필요합니다"
  command -v curl >/dev/null || die "curl 이 필요합니다"
  [[ "$MODE" == "direct" || "$MODE" == "ingress" ]] || die "MODE 는 direct 또는 ingress 여야 합니다: $MODE"

  local key; key="$(find_key)"

  if tunnel_up; then
    info "터널이 이미 열려 있습니다 (localhost:${LOCAL_PORT})"
  else
    port_in_use "$LOCAL_PORT" && \
      die "localhost:${LOCAL_PORT} 가 이미 사용 중입니다. LOCAL_PORT=<다른포트> 로 실행하세요."
    if [[ "$MODE" == "direct" ]]; then
      info "터널 생성 [direct] localhost:${LOCAL_PORT} → ${BASTION} → svc/${WEB_SVC}:80"
      start_direct "$key"
    else
      info "터널 생성 [ingress] localhost:${LOCAL_PORT} → ${BASTION} → ${NODE_IP}:${NODE_PORT}"
      start_ingress "$key"
    fi
    # port-forward 가 원격에서 리스닝을 시작할 때까지 잠깐 기다린다.
    local i
    for i in 1 2 3 4 5 6 7 8 9 10; do
      [[ "$(http_code "http://127.0.0.1:${LOCAL_PORT}/")" != "000" ]] && break
      sleep 1
    done
    tunnel_up || die "터널이 올라오지 않았습니다"
    ok "터널 열림 (제어 소켓: $CTRL_SOCK)"
  fi

  local url code api_code
  if [[ "$MODE" == "direct" ]]; then
    url="http://127.0.0.1:${LOCAL_PORT}"
    code="$(http_code "$url/")"
    api_code="$(http_code "http://127.0.0.1:${LOCAL_PORT}/api/v1/stacks")"
  else
    hosts_add
    url="http://${HOST_NAME}:${LOCAL_PORT}"
    code="$(http_code -H "Host: ${HOST_NAME}" "http://127.0.0.1:${LOCAL_PORT}/")"
    api_code="$(http_code -H "Host: ${HOST_NAME}" "http://127.0.0.1:${LOCAL_PORT}/api/v1/stacks")"
  fi
  [[ "$code" == "200" ]] || die "웹 응답이 200 이 아닙니다 (HTTP ${code:-none}). 'kubectl -n ${NAMESPACE} get pods' 로 배포 상태를 확인하세요."
  ok "웹 응답 확인 HTTP 200"

  # web 의 nginx 가 /api/ 를 nullus-api 로 넘긴다. 인증이 걸린 엔드포인트라
  # 401 이 정상이고, 404 면 프록시가 끊긴 것이다.
  if [[ "$api_code" == "401" ]]; then
    ok "API 프록시 확인 HTTP 401 (인증 필요 — 정상)"
  else
    warn "API 프록시 응답이 예상과 다릅니다 (HTTP ${api_code:-none}) — 401 이어야 합니다"
  fi

  echo
  ok "접속 URL: ${url}"
  echo
  warn "로그인은 동작하지 않습니다 — Keycloak OIDC(PKCE)가 secure context(HTTPS)를 요구합니다."
  info "종료: $0 stop        상태: $0 status"
  echo

  if [[ "${NO_OPEN:-0}" != "1" ]]; then
    if   command -v open     >/dev/null 2>&1; then open "$url"
    elif command -v xdg-open >/dev/null 2>&1; then xdg-open "$url" >/dev/null 2>&1 &
    fi
  fi
}

do_stop() {
  if tunnel_up; then
    ssh -S "$CTRL_SOCK" -O exit "$BASTION" >/dev/null 2>&1 || true
    ok "터널 종료 (원격 port-forward 도 함께 종료)"
  else
    info "열린 터널이 없습니다"
  fi
  rm -f "$CTRL_SOCK"
}

do_status() {
  if tunnel_up; then
    ok "터널 열림 — localhost:${LOCAL_PORT} [mode=${MODE}]"
    local code
    if [[ "$MODE" == "direct" ]]; then code="$(http_code "http://127.0.0.1:${LOCAL_PORT}/")"
    else code="$(http_code -H "Host: ${HOST_NAME}" "http://127.0.0.1:${LOCAL_PORT}/")"; fi
    if [[ "$code" == "200" ]]; then ok "웹 HTTP 200"; else warn "웹 HTTP ${code:-none}"; fi
  else
    info "터널 닫힘"
  fi
  if hosts_present; then ok "hosts 등록됨 — 127.0.0.1 → ${HOST_NAME}"
  else info "hosts 미등록 (direct 모드에서는 불필요)"; fi
}

case "${1:-start}" in
  start)  do_start ;;
  stop)   do_stop ;;
  clean)  do_stop; hosts_remove ;;
  status) do_status ;;
  *)      die "사용법: $0 [start|stop|clean|status]   (MODE=direct|ingress)" ;;
esac
