#!/usr/bin/env bash
# =============================================================================
# 10-build-on-builder.sh — Builder VM 에서 amd64 Air-Gap 번들 빌드
# =============================================================================
# 사용법:
#   BUILDER_IP=<ip> SSH_KEY=<path> ./scripts/10-build-on-builder.sh
#
# 환경 변수:
#   BUILDER_IP      Builder VM 공인 IP (필수)
#   SSH_KEY         SSH 개인키 경로 (필수)
#   REPO_URL        클론할 저장소 URL (기본: https://github.com/cloud-nullus/nullus.git)
#   REPO_BRANCH     체크아웃할 브랜치 (기본: main)
#   BUNDLE_VERSION  번들 버전 태그 (기본: 오늘 날짜 yyyy-mm-dd)
#   GHCR_USER       ghcr / github 사용자명 (기본: dasomel)
#   GHCR_PAT        read:packages + repo(사설 repo 시) 권한 PAT (사설 이미지/repo 접근에 필요)
#
# NOTE: builder VM 은 네이티브 amd64 이므로 crane 크로스풀이 불필요하다.
#   crane 0.21.6 은 일부 ghcr 이미지에서 finalize hang 이 있어 USE_CRANE=0 으로
#   네이티브 docker pull 을 사용한다 (Ubuntu docker-ce classic 스토어라 save/load 안전).
# =============================================================================
set -euo pipefail

BUILDER_IP="${BUILDER_IP:?BUILDER_IP 환경 변수가 필요합니다}"
SSH_KEY="${SSH_KEY:?SSH_KEY 환경 변수가 필요합니다}"
REPO_URL="${REPO_URL:-https://github.com/cloud-nullus/nullus.git}"
REPO_BRANCH="${REPO_BRANCH:-main}"
BUNDLE_VERSION="${BUNDLE_VERSION:-$(date +%F)}"
GHCR_USER="${GHCR_USER:-dasomel}"
GHCR_PAT="${GHCR_PAT:-}"

SSH_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=30"

echo "[INFO] Builder VM (${BUILDER_IP}) 에 연결 중 ..." >&2
echo "[INFO] 저장소: ${REPO_URL} (브랜치: ${REPO_BRANCH})" >&2
echo "[INFO] BUNDLE_VERSION: ${BUNDLE_VERSION}" >&2

# cloud-init 완료 대기 (최대 5분)
echo "[INFO] cloud-init 완료 대기 중 (최대 5분) ..." >&2
for i in $(seq 1 30); do
  if ssh ${SSH_OPTS} ubuntu@"${BUILDER_IP}" "test -f /tmp/cloud-init-done" 2>/dev/null; then
    echo "[OK] cloud-init 완료 확인." >&2
    break
  fi
  echo "[INFO] 대기 중 ... (${i}/30)" >&2
  sleep 10
done

# Builder VM 에서 실행할 스크립트 (heredoc)
# 사설 repo/이미지 인증을 위한 clone URL 구성 (PAT 제공 시 토큰 삽입)
if [[ -n "${GHCR_PAT}" ]]; then
  CLONE_URL="https://${GHCR_USER}:${GHCR_PAT}@github.com/cloud-nullus/nullus.git"
else
  CLONE_URL="${REPO_URL}"
fi

ssh ${SSH_OPTS} ubuntu@"${BUILDER_IP}" bash -s <<EOF
set -euo pipefail

# ghcr 로그인 (사설 이미지 nullus-api/web, dasomel/goharbor pull 에 필요)
if [[ -n "${GHCR_PAT}" ]]; then
  echo "[INFO] ghcr.io 로그인 ..." >&2
  echo "${GHCR_PAT}" | docker login ghcr.io -u "${GHCR_USER}" --password-stdin
fi

# docker 29 는 containerd 이미지 스토어가 기본이라 단일플랫폼 docker save/load 가
# 멀티아치 blob 을 잃어 설치 시 push 가 'does not provide any platform' 으로 깨진다.
# crane 은 일부 ghcr 이미지에서 finalize hang 이 있으므로, 네이티브 amd64 빌드는
# classic overlay2 스토어 + USE_CRANE=0 (네이티브 docker pull) 조합이 가장 안전하다.
if docker info 2>/dev/null | grep -qi 'io.containerd.snapshotter'; then
  echo "[INFO] docker 를 classic overlay2 스토어로 전환 (containerd-snapshotter 비활성) ..." >&2
  sudo mkdir -p /etc/docker
  echo '{"features":{"containerd-snapshotter":false}}' | sudo tee /etc/docker/daemon.json >/dev/null
  sudo systemctl restart docker
  for i in \$(seq 1 15); do docker info >/dev/null 2>&1 && break; sleep 2; done
  echo "[INFO] 전환 후 Storage Driver: \$(docker info 2>/dev/null | grep -i 'Storage Driver')" >&2
  # 스토어 전환으로 ghcr 세션이 초기화될 수 있어 재로그인
  if [[ -n "${GHCR_PAT}" ]]; then echo "${GHCR_PAT}" | docker login ghcr.io -u "${GHCR_USER}" --password-stdin; fi
fi

echo "[INFO] 저장소 클론 / 업데이트 ..." >&2

if [[ -d ~/draft/.git ]]; then
  echo "[INFO] 기존 저장소 업데이트 (git pull)" >&2
  cd ~/draft
  git remote set-url origin "${CLONE_URL}"
  git fetch origin
  git checkout "${REPO_BRANCH}"
  git pull origin "${REPO_BRANCH}"
else
  echo "[INFO] 새로 클론" >&2
  git clone --branch "${REPO_BRANCH}" "${CLONE_URL}" ~/draft
  cd ~/draft
fi
# 토큰이 박힌 remote URL 을 디스크에 남기지 않도록 정리
git remote set-url origin "${REPO_URL}" || true

echo "[INFO] amd64 번들 빌드 시작 (USE_CRANE=0, 네이티브 docker pull) ..." >&2
cd ~/draft/airgap

# USE_CRANE=0: 네이티브 amd64 라 crane 불필요 (crane hang 회피)
# TARGET_PLATFORM=linux/amd64 / PLATFORMS=linux-amd64: amd64 타겟
USE_CRANE=0 \
  TARGET_PLATFORM=linux/amd64 \
  PLATFORMS=linux-amd64 \
  BUNDLE_VERSION="${BUNDLE_VERSION}" \
  ./pre-build.sh

echo "[INFO] 생성된 번들 파일:" >&2
ls -lh ~/draft/airgap/dist/nullus-airgap-bundle-*.tar.gz 2>/dev/null || true
ls -lh ~/draft/airgap/dist/nullus-airgap-bundle-*.tar.gz.sha256 2>/dev/null || true
EOF

echo "[OK] Builder VM 번들 빌드 완료." >&2
echo "[INFO] 다음 단계: scripts/20-transfer-bundle.sh" >&2
