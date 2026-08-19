#!/usr/bin/env bash
# =============================================================================
# build-jenkins-image.sh — 플러그인을 구운 Jenkins 이미지를 만든다
# =============================================================================
# 기본 차트는 파드가 뜰 때마다 플러그인을 내려받는다. SSO 용 oic-auth 를 더하자
# 준비 검사 600초를 넘겨 설치가 실패했고, 에어갭에서는 애초에 불가능하다.
# 느린 다운로드를 매 설치에서 한 번의 빌드로 옮긴다.
#
# 사용법:
#   ./scripts/build-jenkins-image.sh                    # 빌드만
#   ./scripts/build-jenkins-image.sh --kind-load        # kind 클러스터에 적재
#   IMAGE=... TAG=... ./scripts/build-jenkins-image.sh  # 이름 재정의
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${IMAGE:-ghcr.io/cloud-nullus/nullus-jenkins}"
TAG="${TAG:-2.568.2}"
KIND_LOAD="false"
KIND_CLUSTERS=""

for arg in "$@"; do
  case "$arg" in
    --kind-load)     KIND_LOAD="true" ;;
    --cluster=*)     KIND_CLUSTERS="${arg#*=}"; KIND_LOAD="true" ;;
    -h|--help)       sed -n '2,15p' "$0"; exit 0 ;;
    *) echo "[nullus] unknown option: $arg" >&2; exit 1 ;;
  esac
done

REF="${IMAGE}:${TAG}"
echo "[nullus] building ${REF} (플러그인을 굽는 단계라 처음에는 몇 분 걸린다)"
docker build -t "$REF" "${PROJECT_ROOT}/deploy/images/jenkins"
echo "[nullus] built ${REF}"

if [[ "$KIND_LOAD" != "true" ]]; then
  cat <<EOF

  다음 단계:
    배포에서 쓰려면 레지스트리에 push 하거나, 로컬 kind 라면
      $0 --kind-load
EOF
  exit 0
fi

command -v kind >/dev/null || { echo "[nullus] kind 가 없습니다" >&2; exit 1; }

if [[ -z "$KIND_CLUSTERS" ]]; then
  KIND_CLUSTERS="$(kind get clusters 2>/dev/null | tr '\n' ' ')"
fi
[[ -n "${KIND_CLUSTERS// /}" ]] || { echo "[nullus] kind 클러스터가 없습니다" >&2; exit 1; }

for cluster in $KIND_CLUSTERS; do
  echo "[nullus] loading ${REF} → kind/${cluster}"
  # 노드에 이미지가 있어야 imagePullPolicy 와 무관하게 뜬다. 없으면 파드가
  # ImagePullBackOff 로 멈추고 원인이 "Jenkins 가 안 뜬다" 로만 보인다.
  kind load docker-image "$REF" --name "$cluster"
done
echo "[nullus] done"
