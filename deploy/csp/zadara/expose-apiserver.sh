#!/usr/bin/env bash
# =============================================================================
# expose-apiserver.sh — kube-apiserver 를 비표준 포트로, 소스 IP 를 제한해 노출
# =============================================================================
# ⚠ 기본 설계는 이 스크립트를 쓰지 않는 것이다. zadara_cloud_poc.md §1.2·§1.3 은
#   "외부에서는 node-10 만 SSH(22/tcp) 접근 가능" 을 전제로 하고, kubectl 은 §7 대로
#   node-10 안에서 쓴다. 로컬 kubectl 이 필요하면 kubeconfig.sh 의 터널 모드로 충분하다
#   — 보안 그룹을 건드리지 않는다. 이 스크립트는 터널로 감당이 안 되는 이유가 있을 때만
#   쓰고, 끝나면 close 로 되돌린다.
#
# kube-apiserver(6443)를 인터넷에 그대로 여는 것은 피한다. 대신
#
#   외부 → <공인IP>:36443 → (노드 iptables REDIRECT) → 127.0.0.1:6443
#
# 로 비표준 포트를 쓰고, 노드에서 **소스 IP 를 한 번 더 검사**한다. 보안 그룹이
# 실수로 넓게 열려도 허용 대역 밖 패킷은 노드가 DROP 한다 (이중 방어).
# 6443 자체는 외부에 계속 닫아 둔다.
#
# 왜 노드에서 포트를 바꾸는가: 보안 그룹은 허용/차단만 하고 포트 변환을 못 한다.
# 그래서 포트 변환은 노드의 nat 테이블에서 처리한다.
#
# 이 스크립트가 못 하는 것: **보안 그룹 규칙 자체**. Zadara zCompute 는 EC2 호환
# API 를 제공하지만 이 환경에는 자격증명도 IAM 롤도 없다(메타데이터의
# iam/security-credentials 가 404). 자격증명을 주면 open 시 자동으로 시도하고,
# 없으면 콘솔에서 넣을 규칙을 출력한다.
#
# 사용법:
#   ./expose-apiserver.sh open      규칙 적용 + 보안 그룹 안내(또는 자동 적용)
#   ./expose-apiserver.sh close     규칙 제거
#   ./expose-apiserver.sh status    현재 상태
#
# 환경 변수:
#   ALLOW_CIDR   허용 대역 (기본: 현재 공인 IP/32). 0.0.0.0/0 은 거부한다.
#   EXT_PORT     외부 노출 포트          (기본: 36443)
#   API_PORT     apiserver 실제 포트     (기본: 6443)
#   NODE         적용 대상 노드          (기본: bastion 자신 = control-plane)
#   SSH_KEY      SSH 키 경로             (기본: nullus-key.pem 자동 탐색)
#   BASTION      bastion 접속 대상       (기본: ubuntu@121.78.39.184)
#   PUBLIC_IP    외부 접속 주소          (기본: BASTION 의 호스트 부분)
#   AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / ZADARA_EC2_ENDPOINT / SG_ID
#                주어지면 보안 그룹 규칙을 EC2 호환 API 로 직접 넣는다.
#
# 종료 코드: 0 성공 / 1 사전조건·검증 실패
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

BASTION="${BASTION:-ubuntu@121.78.39.184}"
PUBLIC_IP="${PUBLIC_IP:-${BASTION#*@}}"
EXT_PORT="${EXT_PORT:-36443}"
API_PORT="${API_PORT:-6443}"
CHAIN="NULLUS-EXTAPI"
UNIT="nullus-extapi.service"

if [[ -t 1 ]]; then
  C_OK=$'\033[1;32m'; C_ERR=$'\033[1;31m'; C_WARN=$'\033[1;33m'; C_DIM=$'\033[2m'; C_RST=$'\033[0m'
else
  C_OK=""; C_ERR=""; C_WARN=""; C_DIM=""; C_RST=""
fi
ok()   { printf '%s✔%s %s\n' "$C_OK"   "$C_RST" "$*"; }
warn() { printf '%s!%s %s\n' "$C_WARN" "$C_RST" "$*"; }
info() { printf '%s·%s %s\n' "$C_DIM"  "$C_RST" "$*"; }
die()  { printf '%s✘%s %s\n' "$C_ERR"  "$C_RST" "$*" >&2; exit 1; }

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

node_ssh() { ssh -i "$(find_key)" -o StrictHostKeyChecking=accept-new "${NODE:-$BASTION}" "$@"; }

# 허용 대역 결정 — 명시가 없으면 현재 공인 IP 를 쓴다. 전체 개방은 막는다.
resolve_cidr() {
  local cidr="${ALLOW_CIDR:-}"
  if [[ -z "$cidr" ]]; then
    local ip
    ip="$(curl -s --max-time 8 https://checkip.amazonaws.com 2>/dev/null | tr -d '[:space:]')"
    [[ -n "$ip" ]] || ip="$(curl -s --max-time 8 https://ifconfig.me 2>/dev/null | tr -d '[:space:]')"
    [[ "$ip" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || die "공인 IP 를 알아내지 못했습니다. ALLOW_CIDR=<x.x.x.x/32> 로 지정하세요."
    cidr="${ip}/32"
  fi
  [[ "$cidr" == "0.0.0.0/0" ]] && die "ALLOW_CIDR=0.0.0.0/0 은 허용하지 않습니다. kube-apiserver 를 전체 공개하지 마십시오."
  printf '%s\n' "$cidr"
}

# -----------------------------------------------------------------------------
# 노드 규칙 — 전용 체인을 만들어 PREROUTING/INPUT 에서 점프시킨다.
# 남의 체인(calico/kube-proxy)을 건드리지 않아 되돌리기 쉽다.
#
#   nat    PREROUTING → NULLUS-EXTAPI : 허용 대역만 EXT_PORT → API_PORT 로 REDIRECT
#   filter INPUT      → NULLUS-EXTAPI : REDIRECT 되지 않은 EXT_PORT 패킷은 DROP
#
# REDIRECT 를 통과한 패킷은 dport 가 API_PORT 로 바뀌므로 filter 의 DROP 에 걸리지
# 않는다. 즉 허용 대역만 통과하고 나머지는 조용히 버려진다(RST 를 주지 않는다).
# -----------------------------------------------------------------------------
remote_apply() {
  local cidr="$1"
  node_ssh "sudo env CIDR='${cidr}' EXT_PORT='${EXT_PORT}' API_PORT='${API_PORT}' CHAIN='${CHAIN}' UNIT='${UNIT}' bash -s" <<'REMOTE'
set -euo pipefail

apply_rules() {
  # 전용 체인 (없으면 만들고, 있으면 비운다 → 멱등)
  iptables -t nat    -N "$CHAIN" 2>/dev/null || iptables -t nat    -F "$CHAIN"
  iptables -t filter -N "$CHAIN" 2>/dev/null || iptables -t filter -F "$CHAIN"

  iptables -t nat    -A "$CHAIN" -p tcp --dport "$EXT_PORT" -s "$CIDR" \
    -m comment --comment "nullus: ext apiserver (allowed source)" -j REDIRECT --to-ports "$API_PORT"
  iptables -t filter -A "$CHAIN" -p tcp --dport "$EXT_PORT" \
    -m comment --comment "nullus: ext apiserver (deny others)" -j DROP

  # PREROUTING/INPUT 맨 앞에 점프를 건다. calico·kube-proxy 체인보다 먼저 평가된다.
  iptables -t nat    -C PREROUTING -j "$CHAIN" 2>/dev/null || iptables -t nat    -I PREROUTING 1 -j "$CHAIN"
  iptables -t filter -C INPUT      -j "$CHAIN" 2>/dev/null || iptables -t filter -I INPUT      1 -j "$CHAIN"
}

apply_rules

# 재부팅·룰 초기화 대비 — 같은 스크립트를 부팅 시 다시 돌린다.
install -d /usr/local/sbin
cat >/usr/local/sbin/nullus-extapi.sh <<EOS
#!/usr/bin/env bash
set -euo pipefail
CIDR='${CIDR}'; EXT_PORT='${EXT_PORT}'; API_PORT='${API_PORT}'; CHAIN='${CHAIN}'
$(declare -f apply_rules)
apply_rules
EOS
chmod 755 /usr/local/sbin/nullus-extapi.sh

cat >/etc/systemd/system/"$UNIT" <<EOS
[Unit]
Description=Nullus - expose kube-apiserver on ${EXT_PORT} for ${CIDR}
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/local/sbin/nullus-extapi.sh

[Install]
WantedBy=multi-user.target
EOS
systemctl daemon-reload
systemctl enable "$UNIT" >/dev/null 2>&1 || true

echo "APPLIED"
iptables -t nat    -S "$CHAIN"
iptables -t filter -S "$CHAIN"
REMOTE
}

remote_remove() {
  node_ssh "sudo env CHAIN='${CHAIN}' UNIT='${UNIT}' bash -s" <<'REMOTE'
set -uo pipefail
iptables -t nat    -D PREROUTING -j "$CHAIN" 2>/dev/null || true
iptables -t filter -D INPUT      -j "$CHAIN" 2>/dev/null || true
iptables -t nat    -F "$CHAIN" 2>/dev/null || true; iptables -t nat    -X "$CHAIN" 2>/dev/null || true
iptables -t filter -F "$CHAIN" 2>/dev/null || true; iptables -t filter -X "$CHAIN" 2>/dev/null || true
systemctl disable --now "$UNIT" >/dev/null 2>&1 || true
rm -f /etc/systemd/system/"$UNIT" /usr/local/sbin/nullus-extapi.sh
systemctl daemon-reload
echo "REMOVED"
REMOTE
}

remote_status() {
  node_ssh "sudo env CHAIN='${CHAIN}' bash -s" <<'REMOTE'
set -uo pipefail
if iptables -t nat -S "$CHAIN" >/dev/null 2>&1; then
  echo "CHAIN_PRESENT"
  iptables -t nat    -S "$CHAIN"
  iptables -t filter -S "$CHAIN"
  iptables -t nat -S PREROUTING | grep -q -- "-j $CHAIN" && echo "JUMP_NAT_OK"   || echo "JUMP_NAT_MISSING"
  iptables -t filter -S INPUT   | grep -q -- "-j $CHAIN" && echo "JUMP_INPUT_OK" || echo "JUMP_INPUT_MISSING"
else
  echo "CHAIN_ABSENT"
fi
REMOTE
}

# -----------------------------------------------------------------------------
# 보안 그룹 — 자격증명이 있으면 EC2 호환 API 로 넣고, 없으면 안내만 한다.
# -----------------------------------------------------------------------------
security_group() {
  local cidr="$1" verb="$2"   # verb: authorize | revoke
  if [[ -z "${AWS_ACCESS_KEY_ID:-}" || -z "${ZADARA_EC2_ENDPOINT:-}" || -z "${SG_ID:-}" ]]; then
    warn "보안 그룹은 자동으로 못 엽니다 — 자격증명/엔드포인트/SG_ID 가 없습니다."
    info "Zadara 콘솔에서 아래 규칙을 넣으십시오 (node-10 이 속한 보안 그룹):"
    printf '    Type: Custom TCP   Port: %s   Source: %s\n' "$EXT_PORT" "$cidr"
    printf '    %s\n' "6443 은 열지 마십시오 — 노드가 ${EXT_PORT} → ${API_PORT} 로 변환합니다."
    info "또는 자격증명을 주고 다시 실행:"
    printf '    export ZADARA_EC2_ENDPOINT=https://<zcompute-api>  SG_ID=sg-xxxx\n'
    printf '    export AWS_ACCESS_KEY_ID=...  AWS_SECRET_ACCESS_KEY=...\n'
    printf '    %s open\n' "$0"
    return 1
  fi
  command -v aws >/dev/null || die "aws CLI 가 필요합니다 (EC2 호환 API 호출용)"
  aws ec2 "${verb}-security-group-ingress" \
      --endpoint-url "$ZADARA_EC2_ENDPOINT" \
      --group-id "$SG_ID" --protocol tcp --port "$EXT_PORT" --cidr "$cidr" \
    && ok "보안 그룹 ${verb} 완료 — ${EXT_PORT}/tcp from ${cidr}"
}

verify_external() {
  local cidr="$1"
  printf '· 외부 도달 확인 %s:%s ... ' "$PUBLIC_IP" "$EXT_PORT"
  if nc -z -G 8 "$PUBLIC_IP" "$EXT_PORT" >/dev/null 2>&1; then
    echo "열림"
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' -k --max-time 10 "https://${PUBLIC_IP}:${EXT_PORT}/readyz" || true)"
    # 클라이언트 인증서 없이 부르면 401 이 정상이다 (익명 접근 거부).
    if [[ "$code" == "401" || "$code" == "403" || "$code" == "200" ]]; then
      ok "apiserver 응답 확인 (HTTP ${code})"
    else
      warn "포트는 열렸으나 apiserver 응답이 이상합니다 (HTTP ${code:-none})"
    fi
    return 0
  fi
  echo "닫힘"
  warn "보안 그룹에서 ${EXT_PORT}/tcp (source ${cidr}) 를 열어야 합니다."
  return 1
}

do_open() {
  command -v ssh >/dev/null || die "ssh 가 필요합니다"
  local cidr; cidr="$(resolve_cidr)"
  info "허용 대역: ${cidr}   외부 포트: ${EXT_PORT} → apiserver ${API_PORT}"

  local out; out="$(remote_apply "$cidr")" || die "노드 규칙 적용 실패"
  grep -q APPLIED <<<"$out" || die "노드 규칙 적용 실패"
  ok "노드 규칙 적용 (부팅 시 재적용되도록 ${UNIT} 등록)"
  printf '%s\n' "$out" | grep -E '^-A' | sed 's/^/    /'

  security_group "$cidr" authorize || true
  echo
  verify_external "$cidr" || true
  echo
  info "kubeconfig 갱신:  API_PORT=${EXT_PORT} ./kubeconfig.sh"
  info "되돌리기:         $0 close"
}

do_close() {
  local out; out="$(remote_remove)" || true
  grep -q REMOVED <<<"$out" && ok "노드 규칙 제거" || warn "노드 규칙 제거 결과를 확인하지 못했습니다"
  if [[ -n "${AWS_ACCESS_KEY_ID:-}" && -n "${ZADARA_EC2_ENDPOINT:-}" && -n "${SG_ID:-}" ]]; then
    security_group "$(resolve_cidr)" revoke || true
  else
    info "보안 그룹에 ${EXT_PORT}/tcp 규칙을 넣었다면 콘솔에서 직접 지우십시오."
  fi
}

do_status() {
  local out; out="$(remote_status)" || true
  if grep -q CHAIN_PRESENT <<<"$out"; then
    ok "노드 규칙 있음"
    printf '%s\n' "$out" | grep -E '^-A' | sed 's/^/    /'
    grep -q JUMP_NAT_OK   <<<"$out" && ok "nat PREROUTING 점프 정상"   || warn "nat PREROUTING 점프 없음"
    grep -q JUMP_INPUT_OK <<<"$out" && ok "filter INPUT 점프 정상"      || warn "filter INPUT 점프 없음"
  else
    info "노드 규칙 없음"
  fi
  printf '· 외부 %s:%s ... ' "$PUBLIC_IP" "$EXT_PORT"
  nc -z -G 8 "$PUBLIC_IP" "$EXT_PORT" >/dev/null 2>&1 && echo "열림" || echo "닫힘 (보안 그룹 확인)"
  printf '· 외부 %s:%s ... ' "$PUBLIC_IP" "$API_PORT"
  nc -z -G 8 "$PUBLIC_IP" "$API_PORT" >/dev/null 2>&1 && echo "열림 — 닫는 것을 권장합니다" || echo "닫힘 (정상)"
}

case "${1:-status}" in
  open)   do_open ;;
  close)  do_close ;;
  status) do_status ;;
  *)      die "사용법: $0 [open|close|status]" ;;
esac
