#!/usr/bin/env bash
# =============================================================================
# 20-transfer-bundle.sh — Builder VM → Airgap VM 번들 전송 (에어갭 이관 시뮬레이션)
# =============================================================================
# 사용법:
#   BUILDER_IP=<ip> AIRGAP_IP=<ip> SSH_KEY=<path> ./scripts/20-transfer-bundle.sh
#
# 환경 변수:
#   BUILDER_IP   Builder VM 공인 IP (필수)
#   AIRGAP_IP    Airgap VM 공인 IP (필수)
#   SSH_KEY      SSH 개인키 경로 (필수)
#
# 전송 경로: Builder VM → 로컬 임시 디렉토리 → Airgap VM
# (Builder 와 Airgap 간 직접 SSH 가 불가할 수 있어 경유 방식 사용)
# =============================================================================
set -euo pipefail

BUILDER_IP="${BUILDER_IP:?BUILDER_IP 환경 변수가 필요합니다}"
AIRGAP_IP="${AIRGAP_IP:?AIRGAP_IP 환경 변수가 필요합니다}"
SSH_KEY="${SSH_KEY:?SSH_KEY 환경 변수가 필요합니다}"

SSH_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=30"
SCP_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=no"

LOCAL_TMP="$(mktemp -d)"
trap 'rm -rf "${LOCAL_TMP}"' EXIT

echo "[INFO] Builder VM (${BUILDER_IP}) 에서 최신 번들 경로 조회 ..." >&2

# Builder VM 에서 가장 최신 번들 파일명 가져오기
BUNDLE_NAME=$(ssh ${SSH_OPTS} ubuntu@"${BUILDER_IP}" \
  "ls -t ~/draft/airgap/dist/nullus-airgap-bundle-*.tar.gz 2>/dev/null | head -1 | sed 's#.*/##'")

if [[ -z "${BUNDLE_NAME}" ]]; then
  echo "[ERR] Builder VM 에서 번들 파일을 찾을 수 없습니다. 10-build-on-builder.sh 를 먼저 실행하세요." >&2
  exit 1
fi

echo "[INFO] 전송할 번들: ${BUNDLE_NAME}" >&2

# 1단계: Builder VM → 로컬 임시 디렉토리
echo "[INFO] Builder VM → 로컬 임시 디렉토리 (${LOCAL_TMP}) 복사 중 ..." >&2
scp ${SCP_OPTS} \
  ubuntu@"${BUILDER_IP}":~/draft/airgap/dist/"${BUNDLE_NAME}" \
  "${LOCAL_TMP}/"

# sha256 파일도 존재하면 함께 복사
if ssh ${SSH_OPTS} ubuntu@"${BUILDER_IP}" \
    "test -f ~/draft/airgap/dist/${BUNDLE_NAME}.sha256" 2>/dev/null; then
  scp ${SCP_OPTS} \
    ubuntu@"${BUILDER_IP}":~/draft/airgap/dist/"${BUNDLE_NAME}.sha256" \
    "${LOCAL_TMP}/"
fi

echo "[INFO] 로컬 수신 완료: $(ls -lh "${LOCAL_TMP}"/)" >&2

# 2단계: 로컬 → Airgap VM
echo "[INFO] 로컬 → Airgap VM (${AIRGAP_IP}) 전송 중 ..." >&2
scp ${SCP_OPTS} \
  "${LOCAL_TMP}/${BUNDLE_NAME}" \
  ubuntu@"${AIRGAP_IP}":~/

if [[ -f "${LOCAL_TMP}/${BUNDLE_NAME}.sha256" ]]; then
  scp ${SCP_OPTS} \
    "${LOCAL_TMP}/${BUNDLE_NAME}.sha256" \
    ubuntu@"${AIRGAP_IP}":~/
fi

# Airgap VM 에서 수신 확인
echo "[INFO] Airgap VM 수신 확인 ..." >&2
ssh ${SSH_OPTS} ubuntu@"${AIRGAP_IP}" "ls -lh ~/${BUNDLE_NAME}"

# sha256 체크섬 검증 (파일이 있을 경우)
if ssh ${SSH_OPTS} ubuntu@"${AIRGAP_IP}" \
    "test -f ~/${BUNDLE_NAME}.sha256" 2>/dev/null; then
  echo "[INFO] sha256 체크섬 검증 중 ..." >&2
  ssh ${SSH_OPTS} ubuntu@"${AIRGAP_IP}" \
    "cd ~ && sha256sum -c ${BUNDLE_NAME}.sha256"
  echo "[OK] 체크섬 일치." >&2
fi

echo "[OK] 번들 전송 완료: ~/${BUNDLE_NAME} (Airgap VM)" >&2
echo "[INFO] 다음 단계: BUNDLE_NAME=${BUNDLE_NAME} scripts/30-install-on-airgap.sh" >&2

# 다음 스크립트에서 사용할 수 있도록 번들 이름 출력
echo "${BUNDLE_NAME}"
