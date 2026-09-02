#!/usr/bin/env bash
# =============================================================================
# 런타임 이미지 에어갭 반입 검증
#
# 묻는 것: **RuntimeImages() 의 이미지들이 에어갭 경로로 실제 반입되고,
# 인터넷 없이 자기 일을 하는가.**
#
# images.txt 에 이름이 올라 있는 것과, 그 이미지가 폐쇄망에서 실제로 도는 것은
# 다른 문제다. 태그가 틀렸거나 아키텍처가 안 맞으면 목록은 통과하고 설치만
# 실패한다 — 그 실패는 설치가 한참 진행된 뒤에 나온다.
#
# 절차:
#   1. RuntimeImages() 를 pull → save → kind 노드로 import (에어갭 번들 경로)
#   2. 각 노드에 실제로 들어갔는지 containerd 로 확인
#   3. mc 이미지로 **실제 버킷 부트스트랩 스크립트**를 돌린다
#      (imagePullPolicy: Never — 노드에 있는 것만 쓴다. 인터넷을 못 쓴다)
#
# 사용법: ./scripts/verify-airgap-runtime-images.sh [--cluster nullus-develop]
# =============================================================================
set -euo pipefail

CLUSTER="${CLUSTER:-nullus-develop}"
NS="${NS:-airgap-image-verify}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cluster) CLUSTER="$2"; shift 2 ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "알 수 없는 인자: $1" >&2; exit 1 ;;
  esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
export KUBECONFIG="$WORK/kubeconfig"

log()  { echo -e "\033[1;34m[검증]\033[0m $*"; }
ok()   { echo -e "\033[1;32m[ OK ]\033[0m $*"; }
fail() { echo -e "\033[1;31m[실패]\033[0m $*" >&2; exit 1; }

cleanup() {
  kubectl delete ns "$NS" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

for c in docker kind kubectl go; do command -v "$c" >/dev/null || fail "$c 가 필요하다"; done
# `kind get clusters | grep -q` 는 쓰지 않는다. grep -q 가 매치 즉시 파이프를
# 닫으면 kind 가 SIGPIPE 로 죽고, set -o pipefail 이 그 종료 코드를 전파한다 —
# 클러스터가 있는데도 "없다" 가 되고, 타이밍에 따라 갈려 간헐적으로 보인다.
CLUSTERS="$(kind get clusters 2>/dev/null || true)"
printf '%s\n' "$CLUSTERS" | grep -qx "$CLUSTER" || fail "kind 클러스터 '$CLUSTER' 가 없다"
kind get kubeconfig --name "$CLUSTER" > "$KUBECONFIG"

# ── 1. 단일 출처에서 목록을 읽는다 ───────────────────────────────────────
# 스크립트가 목록을 따로 들면 그것도 드리프트 대상이 된다.
# mapfile 은 bash 4+ 다. macOS 기본 bash 는 3.2 라 쓰지 않는다.
IMAGES=()
while IFS= read -r line; do
  [[ -n "$line" ]] && IMAGES+=("$line")
done < <(cd "$ROOT" && go run ./scripts/cmd/runtime-images)
[[ ${#IMAGES[@]} -gt 0 ]] || fail "RuntimeImages() 가 비어 있다"
ok "대상 ${#IMAGES[@]} 종: ${IMAGES[*]}"

# ── 2. pull → save → import (에어갭 번들 경로) ───────────────────────────
NODES=$(kind get nodes --name "$CLUSTER")
FIRST_NODE=$(printf '%s\n' "$NODES" | head -1)
NODE_ARCH=$(kubectl get node "$FIRST_NODE" -o jsonpath='{.status.nodeInfo.architecture}')
[[ -n "$NODE_ARCH" ]] || fail "노드 아키텍처를 확인하지 못했다"
log "노드 아키텍처: linux/$NODE_ARCH"
for img in "${IMAGES[@]}"; do
  log "pull $img"
  # 노드 아키텍처를 맞춘다. 멀티아키 이미지를 그대로 save 하면 매니페스트
  # 리스트만 담기고 다른 플랫폼의 레이어가 없어 import 가 깨진다.
  docker pull -q --platform "linux/$NODE_ARCH" "$img" >/dev/null \
    || fail "$img 를 pull 하지 못했다 — 태그가 틀렸을 수 있다"

  tar="$WORK/img.tar"
  docker save -o "$tar" "$img"
  for node in $NODES; do
    # --all-platforms 는 쓰지 않는다. save 한 tar 에 그 플랫폼만 들어 있어서
    # "content digest not found" 로 깨진다.
    docker exec -i "$node" ctr --namespace=k8s.io images import --digests - < "$tar" >/dev/null \
      || fail "$node 로 $img 반입 실패"
  done
  rm -f "$tar"

  # 노드에 정말 들어갔는지 확인한다. import 가 조용히 실패하는 경우가 있다.
  for node in $NODES; do
    docker exec "$node" ctr --namespace=k8s.io images ls -q | grep -qF "$img" \
      || fail "$node 에 $img 가 없다"
  done
  ok "$img — ${NODES//$'\n'/, } 반입 확인"
done

# ── 3. mc 가 인터넷 없이 실제로 버킷을 만드는가 ──────────────────────────
#
# 목록에 있고 반입돼도, 그 이미지가 코드가 시키는 일을 못하면 의미가 없다.
# imagePullPolicy: Never 라 노드에 있는 것만 쓴다 — 레지스트리로 나가지 못한다.
MC_IMAGE="$(printf '%s\n' "${IMAGES[@]}" | grep '^minio/mc:')"
kubectl create ns "$NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

log "MinIO 기동 (검증 대상)"
kubectl -n "$NS" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata: { name: minio, labels: { app: minio } }
spec:
  containers:
  - name: minio
    image: quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z
    args: ["server", "/data"]
    env:
    - { name: MINIO_ROOT_USER, value: nullus }
    - { name: MINIO_ROOT_PASSWORD, value: nullus-dev-secret }
---
apiVersion: v1
kind: Service
metadata: { name: minio }
spec:
  selector: { app: minio }
  ports: [{ port: 9000, targetPort: 9000 }]
EOF
kubectl -n "$NS" wait --for=condition=ready pod/minio --timeout=180s >/dev/null || fail "MinIO 가 뜨지 않았다"

log "mc 로 버킷 부트스트랩 (imagePullPolicy: Never)"
kubectl -n "$NS" apply -f - >/dev/null <<EOF
apiVersion: batch/v1
kind: Job
metadata: { name: bucket-bootstrap }
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: mc
        image: ${MC_IMAGE}
        imagePullPolicy: Never
        command: ["/bin/sh", "-c"]
        args:
          - |
            set -e
            mc alias set target http://minio.${NS}.svc.cluster.local:9000 nullus nullus-dev-secret
            for b in gitlab-artifacts git-lfs gitlab-uploads; do
              mc mb --ignore-existing "target/\$b"
            done
            mc ls target
EOF

if ! kubectl -n "$NS" wait --for=condition=complete job/bucket-bootstrap --timeout=180s >/dev/null 2>&1; then
  echo "--- Job 로그 ---" >&2
  kubectl -n "$NS" logs job/bucket-bootstrap >&2 || true
  kubectl -n "$NS" describe pod -l job-name=bucket-bootstrap 2>&1 | grep -A 5 "Events:" >&2 || true
  fail "mc 가 버킷을 만들지 못했다"
fi

echo
kubectl -n "$NS" logs job/bucket-bootstrap | sed 's/^/       /'
ok "mc 가 인터넷 없이 버킷 부트스트랩을 수행했다"

echo
ok "런타임 이미지 ${#IMAGES[@]} 종 에어갭 반입 검증 통과"
