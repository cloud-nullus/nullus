#!/usr/bin/env bash
# =============================================================================
# build-sonar-scanner-image.sh — 파이프라인 SAST 단계의 스캐너 이미지를 만든다
# =============================================================================
# 공식 sonarsource/sonar-scanner-cli 는 amd64 만 게시한다. arm64 노드(Apple Silicon kind,
# arm64 에어갭)에서 SAST 단계가 서려면 멀티아치로 굽는 이 이미지가 필요하다.
# CD 가 main 마다 ghcr 에 미는 것과 같은 Dockerfile 이다 — 로컬에서는 kind 노드에 적재한다.
#
# 사용법:
#   ./scripts/build-sonar-scanner-image.sh                    # 빌드만
#   ./scripts/build-sonar-scanner-image.sh --kind-load        # kind 클러스터에 적재
#   IMAGE=... TAG=... ./scripts/build-sonar-scanner-image.sh  # 이름 재정의
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${IMAGE:-ghcr.io/cloud-nullus/nullus/nullus-sonar-scanner}"
# 태그는 스캐너 버전-이미지 리비전이다 — 코드 상수(SonarScannerImage)와 같아야 한다.
TAG="${TAG:-$("$PROJECT_ROOT/scripts/sonar-scanner-image-tag.sh")}"
KIND_LOAD="false"
KIND_CLUSTERS=""

for arg in "$@"; do
  case "$arg" in
    --kind-load)     KIND_LOAD="true" ;;
    --cluster=*)     KIND_CLUSTERS="${arg#*=}"; KIND_LOAD="true" ;;
    -h|--help)       sed -n '2,13p' "$0"; exit 0 ;;
    *) echo "[nullus] unknown option: $arg" >&2; exit 1 ;;
  esac
done

REF="${IMAGE}:${TAG}"
echo "[nullus] building ${REF}"

# provenance/sbom 을 끈다 — attestation manifest 가 붙으면 kind load 가 다루지 못한다
# (build-jenkins-image.sh 와 같은 이유).
if docker build --help 2>/dev/null | grep -q -- '--provenance'; then
  docker build --provenance=false --sbom=false -t "$REF" "$PROJECT_ROOT/deploy/images/sonar-scanner"
else
  docker build -t "$REF" "$PROJECT_ROOT/deploy/images/sonar-scanner"
fi

if [[ "$KIND_LOAD" != "true" ]]; then
  echo "[nullus] built ${REF}"
  exit 0
fi

if [[ -z "$KIND_CLUSTERS" ]]; then
  KIND_CLUSTERS="$(kind get clusters 2>/dev/null | tr '\n' ' ')"
fi
for cluster in $KIND_CLUSTERS; do
  echo "[nullus] loading ${REF} into kind cluster ${cluster}"
  # kind v0.24 + Docker 28 에서는 kind load 가 containerd snapshotter 를 못 찾는다 —
  # 노드 containerd 에 직접 import 한다(docs/50_운영 로컬 가이드의 우회책과 같다).
  tmp="$(mktemp -t nullus-sonar-scanner.XXXXXX)"
  docker save -o "$tmp" "$REF"
  for node in $(kind get nodes --name "$cluster"); do
    docker exec -i "$node" ctr --namespace=k8s.io images import --digests --all-platforms - < "$tmp"
  done
  rm -f "$tmp"
done
echo "[nullus] loaded ${REF}"
