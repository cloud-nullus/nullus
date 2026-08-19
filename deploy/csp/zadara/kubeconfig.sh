#!/usr/bin/env bash
# =============================================================================
# kubeconfig.sh — 로컬에서 Zadara 클러스터에 kubectl 을 붙인다
# =============================================================================
# API 서버(172.31.0.10:6443)는 사설망에 있다. 붙는 길이 두 가지다.
#
#   tunnel (기본)  kubectl → 127.0.0.1:16443 → (ssh) → 172.31.0.10:6443
#                  이미 열린 22/tcp 만 쓰므로 보안 그룹을 건드리지 않는다.
#                  zadara_cloud_poc.md §1.2·§1.3 이 규정한 접근 경로 그대로다 —
#                  외부에서 열리는 것은 node-10 의 22/tcp 하나뿐이다.
#                  터널이 살아 있는 동안에만 동작한다.
#
#   public         kubectl → <공인IP>:36443 → (노드 REDIRECT) → 6443
#                  **문서 설계를 벗어난다.** 없던 외부 노출면을 만드는 선택이므로
#                  터널로 감당이 안 되는 이유가 있을 때만 쓴다. 6443 을 직접 열지
#                  않고 비표준 포트만 열며, 노드 규칙은 expose-apiserver.sh 가
#                  넣는다. 인증서 SAN 에 퍼블릭 IP 가 없어 tls-server-name 으로
#                  검증 이름을 고정한다.
#
# 어느 쪽이든 insecure-skip-tls-verify 는 쓰지 않는다. 인증서 SAN 이
#   DNS:kubernetes, DNS:localhost, IP:172.31.0.10, IP:127.0.0.1
# 이라, tunnel 은 127.0.0.1 로 그대로 검증되고 public 은 SNI 를 "kubernetes" 로
# 고정해 검증한다(노드는 자기 퍼블릭 IP 를 모르는 NAT 구성이라 SAN 에 없다).
#
# 별도 파일(~/.kube/nullus-zadara.conf)에 쓰고 ~/.kube/config 에 병합하지 않는다.
# Kubespray 가 만든 두 클러스터의 admin.conf 가 **모두 cluster.local 이라는 같은
# 식별자**를 써서, 그대로 병합하면 cluster 항목이 하나로 합쳐지고 엉뚱한 클러스터로
# 붙는다 (README §2.1 에서 실제로 겪은 문제다).
#
# 사용법:
#   ./kubeconfig.sh                     SSH 터널 경유로 kubeconfig 생성
#   MODE=public ./kubeconfig.sh         퍼블릭 IP 직결 (외부 노출이 필요할 때만)
#   CLUSTER=develop ./kubeconfig.sh     develop 클러스터용
#   ./kubeconfig.sh status              현재 상태
#   ./kubeconfig.sh stop                터널 종료
#   ./kubeconfig.sh clean               터널 종료 + kubeconfig 파일 삭제
#
# 환경 변수:
#   MODE         tunnel | public        (기본: tunnel)
#   CLUSTER      platform | develop     (기본: platform)
#   NAME         컨텍스트·파일 이름     (기본: nullus-zadara[-develop])
#   PUBLIC_IP    API 서버 퍼블릭 주소   (기본: BASTION 의 호스트 부분)
#   EXT_PORT     외부 노출 포트         (기본: 36443)
#   LOCAL_PORT   tunnel 모드 로컬 포트  (기본: 16443)
#   TLS_NAME     검증에 쓸 인증서 이름  (기본: kubernetes)
#   SSH_KEY      SSH 키 경로            (기본: nullus-key.pem 자동 탐색)
#   BASTION      bastion 접속 대상      (필수, .env 또는 환경변수)
#   KUBECONFIG_OUT 출력 경로            (기본: ~/.kube/<NAME>.conf)
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

MODE="${MODE:-tunnel}"
CLUSTER="${CLUSTER:-platform}"
BASTION="${BASTION:-}"
LOCAL_PORT="${LOCAL_PORT:-16443}"
PUBLIC_IP="${PUBLIC_IP:-${BASTION#*@}}"
# 퍼블릭 IP 는 인증서 SAN 에 없다. 검증에 쓸 이름을 SAN 에 있는 값으로 고정한다.
TLS_NAME="${TLS_NAME:-kubernetes}"

case "$CLUSTER" in
  platform) API_HOST="${API_HOST:-172.31.0.10}"; DEFAULT_NAME="nullus-zadara" ;;
  develop)  API_HOST="${API_HOST:-172.31.1.20}"; DEFAULT_NAME="nullus-zadara-develop" ;;
  *)        printf '✘ CLUSTER 는 platform 또는 develop 이어야 합니다: %s\n' "$CLUSTER" >&2; exit 1 ;;
esac
API_PORT="${API_PORT:-6443}"
# 외부 노출 포트. expose-apiserver.sh 가 노드에서 EXT_PORT → API_PORT 로 변환하고
# 허용 대역 밖은 DROP 한다. 6443 은 외부에 열지 않는다.
EXT_PORT="${EXT_PORT:-36443}"
NAME="${NAME:-$DEFAULT_NAME}"
KUBECONFIG_OUT="${KUBECONFIG_OUT:-$HOME/.kube/${NAME}.conf}"
CTRL_SOCK="${TMPDIR:-/tmp}/nullus-zadara-kube-${CLUSTER}.sock"

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

# kubeconfig 원본은 어느 모드에서든 bastion 에서 가져온다. 터널이 열려 있으면
# 그 마스터 세션을 재사용하고, 아니면 키로 직접 붙는다.
bastion_ssh() {
  if tunnel_up; then
    ssh -S "$CTRL_SOCK" "$BASTION" "$@"
  else
    ssh -i "$(find_key)" -o StrictHostKeyChecking=accept-new "$BASTION" "$@"
  fi
}

# public 모드 사전 점검 — 6443 이 외부에서 닿아야 한다.
check_public_reachable() {
  local probe
  if command -v nc >/dev/null 2>&1; then
    nc -z -G 5 "$PUBLIC_IP" "$EXT_PORT" >/dev/null 2>&1 && return 0
  else
    probe="$(curl -s -o /dev/null -w '%{http_code}' -k --max-time 8 \
             "https://${PUBLIC_IP}:${EXT_PORT}/readyz" || true)"
    [[ "$probe" != "000" ]] && return 0
  fi
  return 1
}

open_tunnel() {
  local key="$1"
  tunnel_up && { info "터널이 이미 열려 있습니다 (localhost:${LOCAL_PORT})"; return; }
  port_in_use "$LOCAL_PORT" && \
    die "localhost:${LOCAL_PORT} 가 이미 사용 중입니다. LOCAL_PORT=<다른포트> 로 실행하세요."
  info "터널 생성 localhost:${LOCAL_PORT} → ${BASTION} → ${API_HOST}:${API_PORT}"
  ssh -i "$key" -o StrictHostKeyChecking=accept-new -o ExitOnForwardFailure=yes \
      -M -S "$CTRL_SOCK" -fN \
      -L "${LOCAL_PORT}:${API_HOST}:${API_PORT}" "$BASTION" \
    || die "SSH 터널 생성 실패"
  tunnel_up || die "터널이 올라오지 않았습니다"
  ok "터널 열림"
}

# -----------------------------------------------------------------------------
# bastion 의 admin.conf 에서 해당 컨텍스트만 뽑아, 이름과 서버 주소를 로컬용으로
# 바꿔 쓴다. 클라이언트 인증서는 그대로 들고 온다.
# -----------------------------------------------------------------------------
write_kubeconfig() {
  local raw; raw="$(mktemp)"
  # shellcheck disable=SC2064  # 지금 값으로 고정해 지우는 것이 의도다
  trap "rm -f '$raw'" RETURN

  bastion_ssh "kubectl config view --raw --minify --context=${CLUSTER}" > "$raw" \
    || die "bastion 에서 kubeconfig 를 가져오지 못했습니다"
  [[ -s "$raw" ]] || die "가져온 kubeconfig 가 비어 있습니다 (컨텍스트: ${CLUSTER})"

  local server tls_name
  if [[ "$MODE" == "public" ]]; then
    server="https://${PUBLIC_IP}:${EXT_PORT}"
    tls_name="$TLS_NAME"     # SAN 에 퍼블릭 IP 가 없어 검증 이름을 고정한다
  else
    server="https://127.0.0.1:${LOCAL_PORT}"
    tls_name=""              # 127.0.0.1 은 SAN 에 있으므로 그대로 검증된다
  fi

  mkdir -p "$(dirname "$KUBECONFIG_OUT")"
  NAME="$NAME" SERVER="$server" TLS_NAME="$tls_name" RAW="$raw" OUT="$KUBECONFIG_OUT" \
  python3 - <<'PY' || die "kubeconfig 변환 실패"
import os, sys, yaml

name     = os.environ['NAME']
server   = os.environ['SERVER']
tls_name = os.environ.get('TLS_NAME') or ''
cfg      = yaml.safe_load(open(os.environ['RAW']))

if not cfg.get('clusters') or not cfg.get('users') or not cfg.get('contexts'):
    sys.exit('kubeconfig 에 clusters/users/contexts 가 없습니다')

user_name = f'{name}-admin'
cfg['clusters'][0]['name'] = name
cfg['clusters'][0]['cluster']['server'] = server
# 어느 모드든 인증서 검증은 유지한다. insecure-skip-tls-verify 는 넣지 않는다.
cfg['clusters'][0]['cluster'].pop('insecure-skip-tls-verify', None)
if tls_name:
    cfg['clusters'][0]['cluster']['tls-server-name'] = tls_name
else:
    cfg['clusters'][0]['cluster'].pop('tls-server-name', None)

cfg['users'][0]['name'] = user_name
cfg['contexts'][0]['name'] = name
cfg['contexts'][0]['context']['cluster'] = name
cfg['contexts'][0]['context']['user'] = user_name
cfg['current-context'] = name

with open(os.environ['OUT'], 'w') as f:
    yaml.safe_dump(cfg, f, default_flow_style=False)
PY

  chmod 600 "$KUBECONFIG_OUT"
  ok "kubeconfig 생성: $KUBECONFIG_OUT  (컨텍스트: $NAME)"
}

verify() {
  command -v kubectl >/dev/null || { warn "로컬에 kubectl 이 없어 검증을 건너뜁니다"; return; }
  local out
  # 포트가 필터링(DROP)된 경우 기본 타임아웃이 매우 길어 그대로 멈춘다.
  if ! out="$(kubectl --kubeconfig "$KUBECONFIG_OUT" --request-timeout=15s get nodes --no-headers 2>&1)"; then
    if [[ "${SKIP_PROBE:-0}" == "1" ]]; then
      warn "검증 생략 — 아직 붙을 수 없습니다: ${out}"
      return
    fi
    die "검증 실패 — kubectl get nodes: ${out}"
  fi
  ok "검증 통과 — 노드 $(printf '%s\n' "$out" | grep -c .) 대"
  printf '%s\n' "$out" | awk '{printf "    %s  %s\n", $1, $2}'
}

do_start() {
  command -v ssh     >/dev/null || die "ssh 가 필요합니다"
  command -v python3 >/dev/null || die "python3 가 필요합니다"
  [[ "$MODE" == "public" || "$MODE" == "tunnel" ]] || die "MODE 는 public 또는 tunnel 이어야 합니다: $MODE"

  local key; key="$(find_key)"

  if [[ "$MODE" == "public" ]]; then
    info "대상 [public] https://${PUBLIC_IP}:${EXT_PORT}  (tls-server-name=${TLS_NAME})"
    if check_public_reachable; then
      ok "${PUBLIC_IP}:${EXT_PORT} 도달 확인"
    elif [[ "${SKIP_PROBE:-0}" == "1" ]]; then
      warn "${PUBLIC_IP}:${EXT_PORT} 미도달 — SKIP_PROBE=1 이라 파일만 미리 만듭니다"
      warn "보안 그룹에서 ${EXT_PORT}/tcp 를 열기 전까지는 이 kubeconfig 로 붙을 수 없습니다"
    else
      warn "${PUBLIC_IP}:${EXT_PORT} 에 닿지 않습니다 — Zadara 콘솔에서 ${EXT_PORT}/tcp 를 열어야 합니다."
      warn "소스는 운영자 IP 로 제한하십시오. ./expose-apiserver.sh open 이 노드 규칙을 함께 넣습니다."
      info "지금 당장 쓰려면: MODE=tunnel $0"
      info "파일만 미리 만들려면: SKIP_PROBE=1 $0"
      die "사전 조건 미충족 — 보안 그룹 ${EXT_PORT}/tcp 미개방"
    fi
  else
    info "대상 [tunnel] localhost:${LOCAL_PORT} → ${API_HOST}:${API_PORT}"
    open_tunnel "$key"
  fi

  write_kubeconfig
  verify
  echo
  info "쓰는 법 — 셋 중 하나"
  echo "    kubectl --kubeconfig $KUBECONFIG_OUT get pods -A"
  echo "    export KUBECONFIG=$KUBECONFIG_OUT && kubectl get pods -A"
  echo "    export KUBECONFIG=\$HOME/.kube/config:$KUBECONFIG_OUT && kubectl config use-context $NAME"
  echo
  if [[ "$MODE" == "public" ]]; then
    warn "kube-apiserver 가 퍼블릭에 열려 있다. 보안 그룹 소스를 운영자 IP 로 제한할 것."
    info "상태: $0 status"
  else
    warn "터널이 닫히면 kubectl 도 끊긴다. 파일만으로는 동작하지 않는다."
    info "종료: $0 stop        상태: $0 status"
  fi
}

do_stop() {
  if tunnel_up; then
    ssh -S "$CTRL_SOCK" -O exit "$BASTION" >/dev/null 2>&1 || true
    ok "터널 종료"
  else
    info "열린 터널이 없습니다"
  fi
  rm -f "$CTRL_SOCK"
}

do_status() {
  if [[ "$MODE" == "public" ]]; then
    if check_public_reachable; then ok "퍼블릭 도달 — ${PUBLIC_IP}:${EXT_PORT}"
    else warn "퍼블릭 미도달 — ${PUBLIC_IP}:${EXT_PORT} (보안 그룹 확인)"; fi
  fi
  if tunnel_up; then ok "터널 열림 — localhost:${LOCAL_PORT} → ${API_HOST}:${API_PORT}"
  else               info "터널 닫힘"; fi

  if [[ -f "$KUBECONFIG_OUT" ]]; then
    ok "kubeconfig 있음: $KUBECONFIG_OUT"
    if command -v kubectl >/dev/null; then
      printf '    server: %s\n' "$(kubectl --kubeconfig "$KUBECONFIG_OUT" config view -o jsonpath='{.clusters[0].cluster.server}')"
      if kubectl --kubeconfig "$KUBECONFIG_OUT" --request-timeout=15s get --raw /readyz >/dev/null 2>&1; then
        ok "API 서버 응답 정상"
      else
        warn "API 서버에 붙지 못했습니다"
      fi
    fi
  else
    info "kubeconfig 없음"
  fi
}

case "${1:-start}" in
  start)  do_start ;;
  stop)   do_stop ;;
  clean)  do_stop; rm -f "$KUBECONFIG_OUT" && ok "kubeconfig 삭제: $KUBECONFIG_OUT" ;;
  status) do_status ;;
  *)      die "사용법: $0 [start|stop|clean|status]   (CLUSTER=platform|develop)" ;;
esac
