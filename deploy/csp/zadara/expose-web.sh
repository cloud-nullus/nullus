#!/usr/bin/env bash
# =============================================================================
# expose-web.sh — 웹 UI 를 표준 포트(80/443)로 받도록 노드에 포워딩을 건다
# =============================================================================
# ingress-nginx 는 NodePort 30080/30443 로 떠 있고, apiserver 의 NodePort 허용
# 범위가 30000-32767 이라 서비스에 80 을 직접 줄 수 없다. 그래서 공인 IP 가 붙은
# node-10 에서 포트만 바꿔 넘긴다.
#
#   외부 → <공인IP>:80  → (node-10 REDIRECT) → :30080 → ingress-nginx → web
#   외부 → <공인IP>:443 → (node-10 REDIRECT) → :30443 → ingress-nginx → web
#
# ingress 컨트롤러 파드는 node-11 에 그대로 두고 kube-proxy 가 넘긴다. node-10 은
# 2vCPU/4GB 에 bastion 겸용이라 컨트롤러를 올리지 않는다.
#
# 이 스크립트가 하지 않는 것: **보안 그룹 규칙**. 콘솔에서 80/443 인바운드를 열어야
# 한다. 이 환경에는 zCompute API 자격증명도 IAM 롤도 없다.
#
# apiserver 를 여는 expose-apiserver.sh 와 기법은 같지만 성격이 다르다. 웹 UI 는
# 애초에 외부 공개가 목적(zadara_cloud_poc.md §11 "Ingress Controller 구성")이고,
# apiserver 는 문서상 외부에 열지 않는 것이 기본이다.
#
# 사용법:
#   ./expose-web.sh open                     운영자 IP 만 허용
#   ALLOW_CIDR=0.0.0.0/0 ./expose-web.sh open  전체 공개 (경고를 읽을 것)
#   ./expose-web.sh close                    되돌리기
#   ./expose-web.sh status                   현재 상태
#
# 환경 변수:
#   ALLOW_CIDR   허용 대역 (기본: 현재 공인 IP/32)
#   HTTP_PORT    외부 HTTP 포트   (기본: 80)   → HTTP_NODEPORT
#   HTTPS_PORT   외부 HTTPS 포트  (기본: 443)  → HTTPS_NODEPORT
#   HTTP_NODEPORT / HTTPS_NODEPORT           (기본: 30080 / 30443)
#   HOST_NAME    ingress host     (기본: nullus.zadara.poc)
#   SSH_KEY / BASTION / PUBLIC_IP            (expose-apiserver.sh 와 동일)
#   AWS_PROFILE  ~/.aws 프로파일 이름 — 권장. 키가 히스토리에 남지 않는다.
#   ZADARA_EC2_ENDPOINT / AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
#                프로파일 대신 환경변수로 줄 때. 하나라도 있으면 API 를 시도한다.
#   SG_ID        보안 그룹 ID (미지정 시 node-10 인스턴스에서 조회)
#   AWS_REGION   (기본: symphony)
#
# 종료 코드: 0 성공 / 1 사전조건·검증 실패
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

BASTION="${BASTION:-ubuntu@121.78.39.184}"
PUBLIC_IP="${PUBLIC_IP:-${BASTION#*@}}"
HTTP_PORT="${HTTP_PORT:-80}"
HTTPS_PORT="${HTTPS_PORT:-443}"
HTTP_NODEPORT="${HTTP_NODEPORT:-30080}"
HTTPS_NODEPORT="${HTTPS_NODEPORT:-30443}"
HOST_NAME="${HOST_NAME:-nullus.zadara.poc}"
CHAIN="NULLUS-WEB"
UNIT="nullus-web-expose.service"

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

node_ssh() { ssh -i "$(find_key)" -o StrictHostKeyChecking=accept-new "$BASTION" "$@"; }

resolve_cidr() {
  local cidr="${ALLOW_CIDR:-}"
  if [[ -z "$cidr" ]]; then
    local ip
    ip="$(curl -s --max-time 8 https://checkip.amazonaws.com 2>/dev/null | tr -d '[:space:]')"
    [[ -n "$ip" ]] || ip="$(curl -s --max-time 8 https://ifconfig.me 2>/dev/null | tr -d '[:space:]')"
    [[ "$ip" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || die "공인 IP 를 알아내지 못했습니다. ALLOW_CIDR=<x.x.x.x/32> 로 지정하세요."
    cidr="${ip}/32"
  fi
  printf '%s\n' "$cidr"
}

# -----------------------------------------------------------------------------
# 노드 규칙 — 전용 체인으로 넣어 calico/kube-proxy 체인을 건드리지 않는다.
#
#   nat PREROUTING → NULLUS-WEB : 허용 대역만 80/443 → NodePort 로 REDIRECT
#
# apiserver 때와 달리 filter DROP 은 넣지 않는다. 80/443 은 REDIRECT 되지 않으면
# 어차피 노드에서 받는 프로세스가 없어 연결이 거절된다. 접근 통제의 1차 방어선은
# 보안 그룹이고, 여기서는 소스 대역이 REDIRECT 조건으로 한 번 더 걸린다.
# -----------------------------------------------------------------------------
remote_apply() {
  local cidr="$1"
  node_ssh "sudo env CIDR='${cidr}' HTTP_PORT='${HTTP_PORT}' HTTPS_PORT='${HTTPS_PORT}' \
            HTTP_NODEPORT='${HTTP_NODEPORT}' HTTPS_NODEPORT='${HTTPS_NODEPORT}' \
            CHAIN='${CHAIN}' UNIT='${UNIT}' bash -s" <<'REMOTE'
set -euo pipefail

apply_rules() {
  iptables -t nat -N "$CHAIN" 2>/dev/null || iptables -t nat -F "$CHAIN"

  # REDIRECT 는 nat 체인을 종료시켜 뒤에 오는 KUBE-SERVICES 를 건너뛴다. 그래서
  # kube-proxy 가 NodePort 트래픽에 붙여 주는 masquerade 마크(0x4000)가 빠지고,
  # SNAT 없이 파드까지 간 패킷이 클라이언트 IP 로 직접 응답하려다 끊긴다
  # (ingress 파드는 워커에 있고 워커에는 공인 IP 가 없다).
  # 그래서 REDIRECT 앞에서 같은 마크를 직접 붙인다 — KUBE-POSTROUTING 이 이 마크를
  # 보고 MASQUERADE 하므로, 응답이 node-10 을 거쳐 되돌아온다.
  local p np pair
  for pair in "${HTTP_PORT}:${HTTP_NODEPORT}:http" "${HTTPS_PORT}:${HTTPS_NODEPORT}:https"; do
    p="${pair%%:*}"; np="$(printf '%s' "$pair" | cut -d: -f2)"
    iptables -t nat -A "$CHAIN" -p tcp --dport "$p" -s "$CIDR" \
      -m comment --comment "nullus: web mark-masq" -j MARK --set-xmark 0x4000/0x4000
    iptables -t nat -A "$CHAIN" -p tcp --dport "$p" -s "$CIDR" \
      -m comment --comment "nullus: web redirect" -j REDIRECT --to-ports "$np"
  done

  iptables -t nat -C PREROUTING -j "$CHAIN" 2>/dev/null || iptables -t nat -I PREROUTING 1 -j "$CHAIN"
}

apply_rules

install -d /usr/local/sbin
cat >/usr/local/sbin/nullus-web-expose.sh <<EOS
#!/usr/bin/env bash
set -euo pipefail
CIDR='${CIDR}'; HTTP_PORT='${HTTP_PORT}'; HTTPS_PORT='${HTTPS_PORT}'
HTTP_NODEPORT='${HTTP_NODEPORT}'; HTTPS_NODEPORT='${HTTPS_NODEPORT}'; CHAIN='${CHAIN}'
$(declare -f apply_rules)
apply_rules
EOS
chmod 755 /usr/local/sbin/nullus-web-expose.sh

cat >/etc/systemd/system/"$UNIT" <<EOS
[Unit]
Description=Nullus - forward ${HTTP_PORT}/${HTTPS_PORT} to ingress NodePort for ${CIDR}
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/local/sbin/nullus-web-expose.sh

[Install]
WantedBy=multi-user.target
EOS
systemctl daemon-reload
systemctl enable "$UNIT" >/dev/null 2>&1 || true

echo "APPLIED"
iptables -t nat -S "$CHAIN"
REMOTE
}

remote_remove() {
  node_ssh "sudo env CHAIN='${CHAIN}' UNIT='${UNIT}' bash -s" <<'REMOTE'
set -uo pipefail
iptables -t nat -D PREROUTING -j "$CHAIN" 2>/dev/null || true
iptables -t nat -F "$CHAIN" 2>/dev/null || true
iptables -t nat -X "$CHAIN" 2>/dev/null || true
systemctl disable --now "$UNIT" >/dev/null 2>&1 || true
rm -f /etc/systemd/system/"$UNIT" /usr/local/sbin/nullus-web-expose.sh
systemctl daemon-reload
echo "REMOVED"
REMOTE
}

remote_status() {
  node_ssh "sudo env CHAIN='${CHAIN}' bash -s" <<'REMOTE'
set -uo pipefail
if iptables -t nat -S "$CHAIN" >/dev/null 2>&1; then
  echo "CHAIN_PRESENT"
  iptables -t nat -S "$CHAIN"
  iptables -t nat -S PREROUTING | grep -q -- "-j $CHAIN" && echo "JUMP_OK" || echo "JUMP_MISSING"
else
  echo "CHAIN_ABSENT"
fi
REMOTE
}

# -----------------------------------------------------------------------------
# 보안 그룹 — zCompute 는 EC2 호환 API 를 제공한다. 자격증명이 있으면 직접 넣고,
# 없으면 콘솔에 넣을 규칙만 출력한다.
#
# SG_ID 를 주지 않으면 node-10 인스턴스에 붙은 보안 그룹을 조회해서 쓴다.
# -----------------------------------------------------------------------------
# 자격증명은 두 가지 방식을 받는다.
#   1) AWS_PROFILE — ~/.aws/{credentials,config} 에 둔 프로파일. 키가 셸 히스토리나
#      프로세스 목록에 남지 않으므로 이쪽을 권한다. config 에 endpoint_url 을 넣어
#      두면 ZADARA_EC2_ENDPOINT 도 필요 없다.
#   2) 환경변수 3종 — 일회성으로 쓸 때.
have_creds() {
  [[ -n "${AWS_PROFILE:-}" ]] && return 0
  [[ -n "${ZADARA_EC2_ENDPOINT:-}" && -n "${AWS_ACCESS_KEY_ID:-}" && -n "${AWS_SECRET_ACCESS_KEY:-}" ]]
}

ec2() {
  local args=(ec2)
  [[ -n "${ZADARA_EC2_ENDPOINT:-}" ]] && args+=(--endpoint-url "$ZADARA_EC2_ENDPOINT")
  [[ -n "${AWS_REGION:-}" ]] && args+=(--region "$AWS_REGION")
  aws "${args[@]}" "$@"
}

discover_sg() {
  [[ -n "${SG_ID:-}" ]] && { printf '%s\n' "$SG_ID"; return; }
  local iid; iid="$(node_ssh 'curl -s --max-time 5 http://169.254.169.254/latest/meta-data/instance-id')"
  [[ -n "$iid" ]] || die "instance-id 를 얻지 못했습니다. SG_ID=<sg-...> 로 지정하세요."
  local sg
  sg="$(ec2 describe-instances --instance-ids "$iid" \
        --query 'Reservations[0].Instances[0].SecurityGroups[0].GroupId' --output text 2>/dev/null)"
  [[ -n "$sg" && "$sg" != "None" ]] || die "보안 그룹을 조회하지 못했습니다. SG_ID=<sg-...> 로 지정하세요."
  printf '%s\n' "$sg"
}

security_group() {
  local cidr="$1" verb="$2"   # authorize | revoke
  if ! have_creds; then
    info "Zadara 콘솔에서 node-10 보안 그룹에 아래 인바운드를 넣으십시오:"
    printf '    Custom TCP  %s   Source %s\n' "$HTTP_PORT"  "$cidr"
    printf '    Custom TCP  %s  Source %s\n'  "$HTTPS_PORT" "$cidr"
    info "또는 ~/.aws 에 프로파일을 두고 AWS_PROFILE 로 실행하면 자동으로 넣습니다:"
    printf '    AWS_PROFILE=zadara %s open\n' "$0"
    return 1
  fi
  command -v aws >/dev/null || die "aws CLI 가 필요합니다"
  local sg; sg="$(discover_sg)"
  info "보안 그룹: ${sg}"
  local p
  for p in "$HTTP_PORT" "$HTTPS_PORT"; do
    if ec2 "${verb}-security-group-ingress" --group-id "$sg" \
         --protocol tcp --port "$p" --cidr "$cidr" >/dev/null 2>&1; then
      ok "${verb} ${p}/tcp from ${cidr}"
    else
      # 이미 있는 규칙을 다시 넣으면 중복 오류가 난다 — 멱등하게 넘긴다.
      warn "${verb} ${p}/tcp 실패 또는 이미 존재 — 현재 규칙을 확인합니다"
      ec2 describe-security-groups --group-ids "$sg" \
        --query "SecurityGroups[0].IpPermissions[?FromPort==\`${p}\`]" --output text 2>/dev/null | sed 's/^/      /'
    fi
  done
}

verify_external() {
  local code
  printf '· http://%s:%s  (Host: %s) ... ' "$PUBLIC_IP" "$HTTP_PORT" "$HOST_NAME"
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 -H "Host: ${HOST_NAME}" \
          "http://${PUBLIC_IP}:${HTTP_PORT}/" || true)"
  if [[ "$code" == "200" ]]; then ok "HTTP ${code}"; else echo "HTTP ${code:-none}"; fi
  printf '· https://%s:%s (Host: %s) ... ' "$PUBLIC_IP" "$HTTPS_PORT" "$HOST_NAME"
  code="$(curl -s -o /dev/null -w '%{http_code}' -k --max-time 10 -H "Host: ${HOST_NAME}" \
          "https://${PUBLIC_IP}:${HTTPS_PORT}/" || true)"
  if [[ "$code" == "200" ]]; then ok "HTTPS ${code} (self-signed 인증서)"; else echo "HTTPS ${code:-none}"; fi
}

do_open() {
  local cidr; cidr="$(resolve_cidr)"
  if [[ "$cidr" == "0.0.0.0/0" ]]; then
    warn "전체 공개입니다. 현재 로그인이 동작하지 않아 인증 없는 화면이 인터넷에 그대로 노출됩니다."
    warn "PoC 라면 ALLOW_CIDR=<운영자IP>/32 를 권합니다."
  fi
  info "허용 대역: ${cidr}"
  info "포워딩: ${HTTP_PORT}→${HTTP_NODEPORT}, ${HTTPS_PORT}→${HTTPS_NODEPORT}"

  local out; out="$(remote_apply "$cidr")" || die "노드 규칙 적용 실패"
  grep -q APPLIED <<<"$out" || die "노드 규칙 적용 실패"
  ok "노드 규칙 적용 (부팅 시 재적용되도록 ${UNIT} 등록)"
  printf '%s\n' "$out" | grep -E '^-A' | sed 's/^/    /'

  echo
  security_group "$cidr" authorize || true
  echo
  verify_external
  echo
  info "브라우저: /etc/hosts 에 '${PUBLIC_IP} ${HOST_NAME}' 추가 후 http://${HOST_NAME}"
  info "되돌리기: $0 close"
}

do_close() {
  local out; out="$(remote_remove)" || true
  grep -q REMOVED <<<"$out" && ok "노드 규칙 제거" || warn "제거 결과를 확인하지 못했습니다"
  if have_creds; then
    security_group "$(resolve_cidr)" revoke || true
  else
    info "보안 그룹에 ${HTTP_PORT}/${HTTPS_PORT} 규칙을 넣었다면 콘솔에서 직접 지우십시오."
  fi
}

do_status() {
  local out; out="$(remote_status)" || true
  if grep -q CHAIN_PRESENT <<<"$out"; then
    ok "노드 규칙 있음"
    printf '%s\n' "$out" | grep -E '^-A' | sed 's/^/    /'
    grep -q JUMP_OK <<<"$out" && ok "nat PREROUTING 점프 정상" || warn "nat PREROUTING 점프 없음"
  else
    info "노드 규칙 없음"
  fi
  verify_external
}

case "${1:-status}" in
  open)   do_open ;;
  close)  do_close ;;
  status) do_status ;;
  *)      die "사용법: $0 [open|close|status]" ;;
esac
