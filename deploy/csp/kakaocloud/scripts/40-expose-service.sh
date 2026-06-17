#!/usr/bin/env bash
# =============================================================================
# 40-expose-service.sh — Airgap VM 에서 Nullus 서비스 영구 외부 노출 (systemd)
# =============================================================================
# kind 클러스터는 80/443 을 호스트에 매핑하지 않으므로(127.0.0.1 바인딩),
# ClusterIP 서비스를 `kubectl port-forward --address 0.0.0.0` 로 VM 공인 IP 에 노출한다.
# 일회성 port-forward 는 재부팅/프로세스 종료 시 끊기므로 systemd 서비스로 고정한다
# (Restart=always). 외부 접근 제한은 보안그룹(web_allowed_cidr)에서 담당한다.
#
# 사용법:
#   AIRGAP_IP=<ip> SSH_KEY=<path> ./scripts/40-expose-service.sh
#
# 환경 변수:
#   AIRGAP_IP     Airgap VM 공인 IP (필수)
#   SSH_KEY       SSH 개인키 경로 (필수)
#   NAMESPACE     대상 네임스페이스 (기본: nullus)
#   SERVICE       대상 서비스 (기본: nullus-web)
#   LISTEN_PORT   VM 외부 노출 포트 (기본: 80 — 보안그룹 허용 포트와 일치해야 함)
#   TARGET_PORT   서비스 포트 (기본: 80)
#   KUBECONFIG_REMOTE  Airgap VM 의 kubeconfig 경로 (기본: /home/ubuntu/.kube/config)
#   UNIT_NAME     systemd 유닛 이름 (기본: nullus-expose)
# =============================================================================
set -euo pipefail

AIRGAP_IP="${AIRGAP_IP:?AIRGAP_IP 환경 변수가 필요합니다}"
SSH_KEY="${SSH_KEY:?SSH_KEY 환경 변수가 필요합니다}"
NAMESPACE="${NAMESPACE:-nullus}"
SERVICE="${SERVICE:-nullus-web}"
LISTEN_PORT="${LISTEN_PORT:-80}"
TARGET_PORT="${TARGET_PORT:-80}"
KUBECONFIG_REMOTE="${KUBECONFIG_REMOTE:-/home/ubuntu/.kube/config}"
UNIT_NAME="${UNIT_NAME:-nullus-expose}"

SSH_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=30"
SCP_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=no"

echo "[INFO] Airgap VM (${AIRGAP_IP}) 에 ${SERVICE} → 0.0.0.0:${LISTEN_PORT} 영구 노출 설정 ..." >&2

# 1) 번들 kubectl 을 안정 경로(/usr/local/bin)로 설치 (systemd 가 참조)
echo "[INFO] kubectl 안정 경로 설치 ..." >&2
ssh ${SSH_OPTS} ubuntu@"${AIRGAP_IP}" \
  'KB=$(ls -1 ~/nullus-airgap-bundle-*/airgap/bin/linux-amd64/kubectl 2>/dev/null | head -1); \
   if [ -z "$KB" ]; then echo "[ERR] 번들 kubectl 을 찾을 수 없음 — 30-install 먼저 실행"; exit 1; fi; \
   sudo install -m 0755 "$KB" /usr/local/bin/kubectl; \
   echo "  kubectl: $(/usr/local/bin/kubectl version --client 2>/dev/null | head -1)"'

# 2) systemd 유닛 파일 생성 (로컬 임시 → scp)
UNIT_TMP="$(mktemp -t ${UNIT_NAME}.XXXXXX)"
trap 'rm -f "${UNIT_TMP}"' EXIT
cat > "${UNIT_TMP}" <<UNIT
[Unit]
Description=Nullus expose ${NAMESPACE}/${SERVICE} via kubectl port-forward (0.0.0.0:${LISTEN_PORT})
After=docker.service network-online.target
Wants=network-online.target

[Service]
Environment=KUBECONFIG=${KUBECONFIG_REMOTE}
# kind apiserver(127.0.0.1:16443) 가 준비될 때까지 재시도
ExecStartPre=/bin/sh -c 'until /usr/local/bin/kubectl get ns ${NAMESPACE} >/dev/null 2>&1; do sleep 5; done'
ExecStart=/usr/local/bin/kubectl port-forward -n ${NAMESPACE} svc/${SERVICE} ${LISTEN_PORT}:${TARGET_PORT} --address 0.0.0.0
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
UNIT

echo "[INFO] systemd 유닛 전송 + 설치 ..." >&2
scp ${SCP_OPTS} "${UNIT_TMP}" ubuntu@"${AIRGAP_IP}":/tmp/"${UNIT_NAME}".service

# 3) 기존 수동 port-forward 정리 후 유닛 설치/기동
ssh ${SSH_OPTS} ubuntu@"${AIRGAP_IP}" \
  "sudo mv /tmp/${UNIT_NAME}.service /etc/systemd/system/${UNIT_NAME}.service; \
   sudo systemctl daemon-reload; \
   sudo pkill -f 'kubectl port-forwar[d]' 2>/dev/null || true; \
   sudo systemctl enable --now ${UNIT_NAME}.service; \
   sleep 5; \
   echo -n '  systemd 상태: '; systemctl is-active ${UNIT_NAME}.service; \
   echo -n '  리스닝: '; sudo ss -tlnp | grep \":${LISTEN_PORT} \" | head -1 || echo '미바인딩'"

echo "[OK] 영구 노출 설정 완료." >&2
echo "[INFO] 외부 접근: http://${AIRGAP_IP}:${LISTEN_PORT}/  (보안그룹 web_allowed_cidr 허용 대상만)" >&2
echo "[INFO] 상태:   ssh ... 'systemctl status ${UNIT_NAME}'" >&2
echo "[INFO] 중지:   ssh ... 'sudo systemctl disable --now ${UNIT_NAME}'" >&2
