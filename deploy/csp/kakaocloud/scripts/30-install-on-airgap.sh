#!/usr/bin/env bash
# =============================================================================
# 30-install-on-airgap.sh — Airgap VM 에서 Nullus 설치
# =============================================================================
# 사용법:
#   AIRGAP_IP=<ip> SSH_KEY=<path> ./scripts/30-install-on-airgap.sh
#   AIRGAP_IP=<ip> SSH_KEY=<path> BUNDLE_NAME=nullus-airgap-bundle-2026-06-08.tar.gz ./scripts/30-install-on-airgap.sh
#
# 환경 변수:
#   AIRGAP_IP     Airgap VM 공인 IP (필수)
#   SSH_KEY       SSH 개인키 경로 (필수)
#   BUNDLE_NAME   번들 파일명 (기본: ~ 에서 최신 파일 자동 탐색)
#   CLUSTER_NAME  kind 클러스터 이름 (기본: nullus-airgap, install.sh 의 기본값)
#   SKIP_VERIFY   1 = 설치 후 검증 건너뜀 (기본: 0)
#   PLATFORM_OVR  플랫폼 수동 지정 (기본: 자동탐지, install.sh 에 위임)
# =============================================================================
set -euo pipefail

AIRGAP_IP="${AIRGAP_IP:?AIRGAP_IP 환경 변수가 필요합니다}"
SSH_KEY="${SSH_KEY:?SSH_KEY 환경 변수가 필요합니다}"
CLUSTER_NAME="${CLUSTER_NAME:-}"
SKIP_VERIFY="${SKIP_VERIFY:-}"
PLATFORM_OVR="${PLATFORM_OVR:-}"

SSH_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=30"

echo "[INFO] Airgap VM (${AIRGAP_IP}) 에 연결 중 ..." >&2

# 번들 파일명 결정 (명시하지 않으면 ~ 에서 최신 파일 자동 탐색)
if [[ -z "${BUNDLE_NAME:-}" ]]; then
  echo "[INFO] BUNDLE_NAME 미지정 — Airgap VM 홈 디렉토리에서 최신 번들 탐색 ..." >&2
  BUNDLE_NAME=$(ssh ${SSH_OPTS} ubuntu@"${AIRGAP_IP}" \
    "ls -t ~/nullus-airgap-bundle-*.tar.gz 2>/dev/null | head -1 | sed 's#.*/##'")
  if [[ -z "${BUNDLE_NAME}" ]]; then
    echo "[ERR] Airgap VM 홈에서 번들 파일을 찾을 수 없습니다. 20-transfer-bundle.sh 를 먼저 실행하세요." >&2
    exit 1
  fi
  echo "[INFO] 발견된 번들: ${BUNDLE_NAME}" >&2
fi

# 환경 변수 조합 (빈 값은 전달하지 않음)
INSTALL_ENV=""
[[ -n "${CLUSTER_NAME}" ]]  && INSTALL_ENV+="CLUSTER_NAME=${CLUSTER_NAME} "
[[ -n "${SKIP_VERIFY}" ]]   && INSTALL_ENV+="SKIP_VERIFY=${SKIP_VERIFY} "
[[ -n "${PLATFORM_OVR}" ]]  && INSTALL_ENV+="PLATFORM_OVR=${PLATFORM_OVR} "

echo "[INFO] 번들: ~/${BUNDLE_NAME}" >&2
echo "[INFO] 설치 환경 변수: ${INSTALL_ENV:-없음}" >&2
echo "[INFO] airgap/install.sh 실행 중 (시간이 걸릴 수 있음) ..." >&2

# Airgap VM 에서 install.sh 실행
# install.sh 는 tar.gz 경로를 받으면 자동 압축 해제 후 내부 install.sh 를 exec 함
ssh ${SSH_OPTS} ubuntu@"${AIRGAP_IP}" bash -s <<EOF
set -euo pipefail

BUNDLE_PATH=~/${BUNDLE_NAME}

if [[ ! -f "\${BUNDLE_PATH}" ]]; then
  echo "[ERR] 번들 파일을 찾을 수 없음: \${BUNDLE_PATH}" >&2
  exit 1
fi

echo "[INFO] install.sh 실행: \${BUNDLE_PATH}" >&2
${INSTALL_ENV}bash "\${BUNDLE_PATH%%.tar.gz}/../install.sh" "\${BUNDLE_PATH}" 2>&1 || \
  ${INSTALL_ENV}bash <(tar -xzf "\${BUNDLE_PATH}" --to-stdout '*/install.sh' 2>/dev/null || true) "\${BUNDLE_PATH}" 2>&1 || \
  { cd ~ && tar -xzf "\${BUNDLE_PATH}" && \
    EXTRACTED_DIR=\$(ls -td nullus-airgap-bundle-* 2>/dev/null | head -1) && \
    ${INSTALL_ENV}bash "\${EXTRACTED_DIR}/install.sh"; }
EOF

echo "[OK] Airgap VM 설치 완료." >&2
echo "[INFO] 접속: ssh -i ${SSH_KEY} ubuntu@${AIRGAP_IP}" >&2
echo "[INFO] 클러스터 확인: kubectl --kubeconfig ~/.kube/config get nodes" >&2
