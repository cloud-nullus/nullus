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

# provenance/sbom 을 끈다.
#
# BuildKit 은 기본으로 attestation manifest 를 붙여 결과를 manifest list 로
# 만드는데, kind load 가 그것을 다루지 못한다:
#     ERROR: failed to detect containerd snapshotter
# 단일 이미지로 떨어뜨려야 노드에 적재된다. 플래그를 모르는 옛 도커에서는
# 그대로 빌드한다.
BUILD_FLAGS=""
if docker build --help 2>/dev/null | grep -q -- "--provenance"; then
  BUILD_FLAGS="--provenance=false --sbom=false"
fi
# shellcheck disable=SC2086 -- 플래그를 분리해 넘겨야 한다
docker build $BUILD_FLAGS -t "$REF" "${PROJECT_ROOT}/deploy/images/jenkins"
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

# 한 클러스터가 실패해도 나머지는 시도한다. 중간에 멈추면 어떤 클러스터가
# 적재됐는지 알 수 없어, 나중에 "이 클러스터에서만 Jenkins 가 안 뜬다" 가 된다.
# kind load 가 안 되는 조합이 있다. kind CLI 가 노드의 containerd 보다 오래되면
# 스냅샷터를 판별하지 못하고 이렇게 끝난다:
#     ERROR: failed to detect containerd snapshotter
# (실측: kind v0.24.0 + 노드 containerd v2.2.1)
#
# 그때는 노드의 ctr 로 직접 넣는다. kind 의 판별 과정을 건너뛰므로 버전 조합에
# 영향을 받지 않는다.
load_into_cluster() {
  local cluster="$1"
  if kind load docker-image "$REF" --name "$cluster" 2>/dev/null; then
    return 0
  fi

  echo "[nullus]   kind load 실패 — 노드 ctr 로 직접 적재한다"
  local archive="${TMPDIR:-/tmp}/nullus-jenkins-$$.tar"
  docker save "$REF" -o "$archive" || return 1

  local ok=0 node
  for node in $(kind get nodes --name "$cluster" 2>/dev/null); do
    if docker exec -i "$node" ctr --namespace=k8s.io images import --digests=false - <"$archive" >/dev/null 2>&1; then
      echo "[nullus]     $node"
    else
      ok=1
    fi
  done
  rm -f "$archive"
  return "$ok"
}

failed=""
for cluster in $KIND_CLUSTERS; do
  echo "[nullus] loading ${REF} → kind/${cluster}"
  # 노드에 이미지가 있어야 imagePullPolicy 와 무관하게 뜬다. 없으면 파드가
  # ImagePullBackOff 로 멈추고 원인이 "Jenkins 가 안 뜬다" 로만 보인다.
  if ! load_into_cluster "$cluster"; then
    failed="$failed $cluster"
  fi
done

if [[ -n "$failed" ]]; then
  echo "[nullus] 적재 실패:$failed" >&2
  exit 1
fi
echo "[nullus] done"
