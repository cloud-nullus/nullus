#!/usr/bin/env bash
# =============================================================================
# 인클러스터 백업 리허설 — 설계 Q9(egress) 검증
#
# 묻는 것은 하나다: **클러스터 안 파드가 클러스터 밖 오브젝트 스토리지에
# 실제로 쓸 수 있는가.** 설계 §4.2.2 가 "미확인" 으로 남겨 둔 항목이다.
#
# internal/backup/rehearsal 의 Go 리허설과 역할이 다르다:
#   - Go 리허설  → 정지·복원 **메커니즘** (코드를 호스트에서 돌린다)
#   - 이 스크립트 → **위상** (플랫폼이 파드로 뜨고, 목적지는 클러스터 밖)
#
# 목적지 MinIO 는 kind 도커 네트워크의 별도 컨테이너다. 클러스터 안이 아니라
# **밖**이면서 노드에서 도달 가능한 위치이며, 운영의 "조직 내부망 오브젝트
# 스토리지" 와 같은 자리다. 클러스터 안에 두면 설계 §4.2 와 어긋난다 —
# 클러스터가 죽을 때 백업본도 같이 죽는다.
#
# 사용법:
#   ./scripts/backup-rehearsal-incluster.sh [--cluster nullus-develop] [--keep]
#
# 필요한 것: docker, kind, kubectl, helm
# =============================================================================
set -euo pipefail

CLUSTER="${CLUSTER:-nullus-develop}"
NS="${NS:-nullus-rehearsal}"
STORE="nullus-backup-store"
BUCKET="nullus-backup"
STORE_USER="nullus-admin"
STORE_PASS="nullus-minio-secret"
IMAGE_TAG="backup-rehearsal"
IMAGE="ghcr.io/cloud-nullus/nullus/nullus-api:${IMAGE_TAG}"
KEEP="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cluster) CLUSTER="$2"; shift 2 ;;
    --keep)    KEEP="true"; shift ;;
    -h|--help) sed -n '2,25p' "$0"; exit 0 ;;
    *) echo "알 수 없는 인자: $1" >&2; exit 1 ;;
  esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
KUBECONFIG_FILE="$WORK/kubeconfig"
export KUBECONFIG="$KUBECONFIG_FILE"

log()  { echo -e "\033[1;34m[리허설]\033[0m $*"; }
ok()   { echo -e "\033[1;32m[  OK  ]\033[0m $*"; }
fail() { echo -e "\033[1;31m[ 실패 ]\033[0m $*" >&2; exit 1; }

cleanup() {
  if [[ "$KEEP" == "true" ]]; then
    log "--keep 이므로 남겨 둔다: 네임스페이스 $NS, 컨테이너 $STORE"
  else
    kubectl delete ns "$NS" --ignore-not-found --wait=false >/dev/null 2>&1 || true
    docker rm -f "$STORE" >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

# ── 0. 사전 확인 ─────────────────────────────────────────────────────────
for c in docker kind kubectl helm; do
  command -v "$c" >/dev/null || fail "$c 가 필요하다"
done
kind get clusters 2>/dev/null | grep -qx "$CLUSTER" || fail "kind 클러스터 '$CLUSTER' 가 없다"
kind get kubeconfig --name "$CLUSTER" > "$KUBECONFIG_FILE"
ok "클러스터 $CLUSTER"

# ── 1. 클러스터 **밖** 오브젝트 스토리지 ─────────────────────────────────
log "목적지 기동 (클러스터 밖, kind 도커 네트워크)"
docker rm -f "$STORE" >/dev/null 2>&1 || true
docker run -d --name "$STORE" --network kind \
  -e "MINIO_ROOT_USER=$STORE_USER" -e "MINIO_ROOT_PASSWORD=$STORE_PASS" \
  quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z server /data >/dev/null

for i in $(seq 1 30); do
  docker exec "$STORE" mc alias set ext "http://127.0.0.1:9000" "$STORE_USER" "$STORE_PASS" >/dev/null 2>&1 && break
  sleep 2
done
docker exec "$STORE" mc mb "ext/$BUCKET" >/dev/null 2>&1 || true
STORE_IP="$(docker inspect "$STORE" -f '{{.NetworkSettings.Networks.kind.IPAddress}}')"
[[ -n "$STORE_IP" ]] || fail "목적지 IP 를 얻지 못했다"
ok "목적지 $STORE_IP:9000 (버킷 $BUCKET)"

# ── 2. 이미지 반입 ───────────────────────────────────────────────────────
log "API 이미지 빌드"
docker build -q -t "$IMAGE" "$ROOT" >/dev/null
docker run --rm "$IMAGE" pg_dump --version >/dev/null 2>&1 \
  || fail "이미지에 pg_dump 가 없다 — Dockerfile 의 postgresql-client 를 확인하라"
ok "이미지 빌드 (pg_dump 포함 확인)"

log "kind 노드로 반입"
# kind load 는 이 조합(kind 0.24 + Docker 28.x)에서 'failed to detect
# containerd snapshotter' 로 죽는다. containerd 에 직접 넣는다.
docker save -o "$WORK/api.tar" "$IMAGE"
for node in $(kind get nodes --name "$CLUSTER"); do
  docker exec -i "$node" ctr --namespace=k8s.io images import --digests --all-platforms - \
    < "$WORK/api.tar" >/dev/null || fail "$node 반입 실패"
done
ok "노드 반입"

# ── 3. 플랫폼 배포 ───────────────────────────────────────────────────────
cat > "$WORK/values.yaml" <<EOF
api:
  replicaCount: 1
  image: { repository: ghcr.io/cloud-nullus/nullus/nullus-api, tag: "${IMAGE_TAG}", pullPolicy: Never }
web:
  replicaCount: 0
keycloak:
  enabled: false
postgresql:
  enabled: true
  primary:
    initdb:
      scripts:
        01-keycloak-db.sql: |
          CREATE DATABASE keycloak;
secrets:
  dbPassword: nullus
  encryptionKey: "platform-encryption-key-32bytes!"
  backupSealKey: "backup-seal-key-32bytes-padding!"
  backupDestinationSecretKey: "${STORE_PASS}"
config:
  server: { mode: development }
  log: { level: debug, format: text }
  backup:
    enabled: true
    sealKeyId: rehearsal-key
    destination:
      endpoint: "${STORE_IP}:9000"
      bucket: ${BUCKET}
      accessKey: ${STORE_USER}
      region: us-east-1
      useSsl: false
      prefix: ""
    keycloakDatabase:
      host: nullus-postgresql
      port: 5432
      name: keycloak
      user: nullus
      password: nullus
    schedule: { enabled: false, interval: 24h, orgId: "", stackId: "", namespace: "", mode: full }
    retention: { daily: 7, weekly: 4, monthly: 3, maxTotalBytes: 0 }
EOF

log "플랫폼 배포"
kubectl create ns "$NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
helm upgrade --install nullus "$ROOT/deploy/helm/nullus" -n "$NS" \
  -f "$ROOT/deploy/helm/nullus/values-dev.yaml" -f "$WORK/values.yaml" \
  --timeout 8m --wait >/dev/null || fail "helm 설치 실패"
kubectl -n "$NS" rollout status deploy/nullus-api --timeout=180s >/dev/null
ok "플랫폼 기동"

# 백업 모듈이 실제로 조립됐는지 — 여기서 조용히 꺼져 있으면 아래가 전부 무의미하다
kubectl -n "$NS" logs -l app.kubernetes.io/component=api -c api --tail=200 2>/dev/null \
  | grep -q "백업 모듈 준비 완료" \
  || fail "백업 모듈이 켜지지 않았다. 로그를 확인하라: kubectl -n $NS logs -l app.kubernetes.io/component=api"
ok "백업 모듈 준비 확인"

# ── 4. 파드에서 백업 실행 ────────────────────────────────────────────────
kubectl -n "$NS" port-forward svc/nullus-api 18080:8080 >/dev/null 2>&1 &
PF_PID=$!
# 종료 시 job 제어 메시지("Terminated: 15")가 새어 나오지 않게 떼어 둔다.
disown "$PF_PID" 2>/dev/null || true
trap 'kill "$PF_PID" >/dev/null 2>&1 || true; cleanup' EXIT
for i in $(seq 1 20); do curl -sf localhost:18080/health >/dev/null 2>&1 && break; sleep 1; done

ORG="$(curl -s localhost:18080/api/v1/admin/organization \
  | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d.get("id") or d.get("data",{}).get("id",""))')"
[[ -n "$ORG" ]] || fail "조직 ID 를 얻지 못했다"

log "백업 실행 (파드 안에서 돈다)"
RESP="$(curl -sX POST localhost:18080/api/v1/admin/backups \
  -H 'Content-Type: application/json' -H "X-Org-Id: $ORG" -d '{"mode":"platform_only"}')"
STATUS="$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("status",""))')"
BID="$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("id",""))')"
BYTES="$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("total_bytes",0))')"

[[ "$STATUS" == "succeeded" ]] || {
  echo "$RESP" | python3 -m json.tool >&2
  fail "백업 상태가 succeeded 가 아니다: $STATUS"
}
ok "백업 $BID — $BYTES bytes"

# ── 5. Q9 증명 — 산출물이 클러스터 **밖**에 있는가 ───────────────────────
log "목적지 확인 (클러스터 밖에서 조회한다)"
LISTED="$(docker exec "$STORE" mc ls --recursive "ext/$BUCKET/backup-$BID/" 2>/dev/null | wc -l | tr -d ' ')"
[[ "$LISTED" -gt 0 ]] || fail "산출물이 목적지에 없다 — 파드에서 밖으로 나가지 못했다"
docker exec "$STORE" mc ls --recursive "ext/$BUCKET/backup-$BID/" | sed 's/^/         /'
ok "egress 확인 — 파드가 클러스터 밖 스토리지에 $LISTED 건을 썼다"

# ── 6. 읽기 방향도 확인 (verify 는 목적지에서 되읽는다) ──────────────────
VOK="$(curl -sX POST "localhost:18080/api/v1/admin/backups/$BID/verify" -H "X-Org-Id: $ORG" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin).get("ok"))')"
[[ "$VOK" == "True" ]] || fail "verify 실패 — 목적지에서 되읽지 못했다"
ok "읽기 방향 확인 (verify)"

echo
ok "인클러스터 리허설 통과 — Q9(egress) 해소"
echo "     플랫폼: 파드 (네임스페이스 $NS)"
echo "     목적지: $STORE_IP:9000 — 클러스터 밖"
