#!/usr/bin/env bash
# =============================================================================
# setup-local-domain.sh — 로컬에서 도구 간 SSO 가 실제로 넘어가게 배선한다
# =============================================================================
# 왜 필요한가
#
#   Keycloak 의 SSO 세션 쿠키는 호스트 단위로 붙는다. 포털이 로그인한 주소와
#   도구가 브라우저를 보내는 주소가 다르면 쿠키가 실리지 않아 도구마다 다시
#   로그인해야 한다 — SSO 자동화의 목적이 성립하지 않는다.
#
#   게다가 도구(ArgoCD 등)는 issuer 로 JWKS 를 직접 가져온다. 기본값인
#   http://localhost:8180 은 파드 안에서 파드 자신을 가리켜 토큰 검증이 실패한다.
#
#   그래서 브라우저와 파드가 **같은 문자열**로 닿는 이름이 하나 필요하다.
#
#     브라우저  → /etc/hosts        → 127.0.0.1:8180 (공개된 포트)
#     클러스터  → CoreDNS hosts     → host.docker.internal 의 IP
#
# 이 스크립트가 하는 일
#   1. kind 클러스터 CoreDNS 에 keycloak.<도메인> 을 호스트 IP 로 매핑
#   2. /etc/hosts 항목 확인 (없으면 붙여넣을 명령을 출력 — sudo 필요)
#   3. 스택 내부 CA 를 꺼내 신뢰 등록 명령을 출력 (sudo 필요)
#
# 사용법
#   ./scripts/setup-local-domain.sh [--domain=nullus.local] [--context=kind-...]
#   ./scripts/setup-local-domain.sh --apply-hosts   # /etc/hosts 까지 직접 수정(sudo)
# =============================================================================
set -euo pipefail

DOMAIN="nullus.local"
CONTEXT=""
APPLY_HOSTS="false"
KEYCLOAK_PORT="${KEYCLOAK_PORT:-8180}"

for arg in "$@"; do
  case "$arg" in
    --domain=*)  DOMAIN="${arg#*=}" ;;
    --context=*) CONTEXT="${arg#*=}" ;;
    --apply-hosts) APPLY_HOSTS="true" ;;
    -h|--help) sed -n '2,28p' "$0"; exit 0 ;;
    *) echo "[nullus] unknown option: $arg" >&2; exit 1 ;;
  esac
done

if [[ -z "$CONTEXT" ]]; then
  CONTEXT="$(kubectl config current-context 2>/dev/null || true)"
fi
[[ -n "$CONTEXT" ]] || { echo "[nullus] kubectl context 를 찾지 못했습니다 (--context= 로 지정)" >&2; exit 1; }

KC_HOST="keycloak.${DOMAIN}"
TOOL_HOSTS=(argocd grafana harbor gitea jenkins minio gitlab nexus opensearch registry)

echo "[nullus] domain=$DOMAIN context=$CONTEXT"

# ── 1. 노드에서 호스트가 보이는 IP ───────────────────────────────────────────
# Docker Desktop 은 컨테이너에 host.docker.internal 을 넣어 준다. 이 값을 그대로
# 읽는다 — IP 를 박아 두면 Docker 버전이나 네트워크 구성이 바뀔 때 조용히 깨진다.
NODE="$(kubectl get nodes --context "$CONTEXT" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
[[ -n "$NODE" ]] || { echo "[nullus] 클러스터에 노드가 없습니다" >&2; exit 1; }

HOST_IP="$(docker exec "$NODE" getent hosts host.docker.internal 2>/dev/null | awk '{print $1}' | head -1 || true)"
if [[ -z "$HOST_IP" ]]; then
  echo "[nullus] 노드에서 host.docker.internal 을 해석하지 못했습니다." >&2
  echo "[nullus] Docker Desktop 이 아니면 호스트 IP 를 직접 넣어야 합니다:" >&2
  echo "           HOST_IP=<호스트IP> $0 $*" >&2
  exit 1
fi
echo "[nullus] 노드에서 본 호스트 IP: $HOST_IP"

# ── 2. CoreDNS 에 이름 주입 ──────────────────────────────────────────────────
# hosts 플러그인을 Corefile 에 넣는다. fallthrough 를 빼면 나머지 조회가 전부
# 막히므로 반드시 함께 둔다.
CURRENT_COREFILE="$(kubectl -n kube-system --context "$CONTEXT" get cm coredns -o jsonpath='{.data.Corefile}')"

# 이전에 넣은 블록이 있으면 통째로 걷어내고 새로 넣는다. 값만 고쳐 쓰려 하면
# sed 문법이 GNU/BSD 사이에서 갈려 macOS 에서 깨진다(실제로 깨졌다).
# 마커로 우리 블록의 범위를 표시해 두어 남의 설정은 건드리지 않는다.
MARKER="# nullus-local-domain"

CURRENT_COREFILE="$(awk -v marker="$MARKER" -v host="$KC_HOST" '
  # 마커 줄은 버린다.
  index($0, marker) { next }
  # hosts 블록은 통째로 모아 두었다가 우리 이름이 들어 있으면 버린다.
  # 마커가 없던 옛 버전이 남긴 블록도 이렇게 걷힌다.
  /^ *hosts *\{/ && !buffering { buffering = 1; buf = $0 ORS; next }
  buffering {
    buf = buf $0 ORS
    if ($0 ~ /^ *\} *$/) {
      buffering = 0
      if (index(buf, host) == 0) printf "%s", buf
    }
    next
  }
  { print }
' <<<"$CURRENT_COREFILE")"

CURRENT_COREFILE="$(awk -v ip="$HOST_IP" -v host="$KC_HOST" -v marker="$MARKER" '
  /^\.:53 \{/ && !inserted {
    print
    print "    " marker
    print "    hosts {"
    print "        " ip " " host
    # fallthrough 를 빼면 여기서 못 찾은 이름의 조회가 전부 막힌다.
    print "        fallthrough"
    print "    }"
    inserted = 1
    next
  }
  { print }
' <<<"$CURRENT_COREFILE")"

TMP_COREFILE="$(mktemp)"
printf '%s\n' "$CURRENT_COREFILE" >"$TMP_COREFILE"
kubectl -n kube-system --context "$CONTEXT" create cm coredns \
  --from-file=Corefile="$TMP_COREFILE" --dry-run=client -o yaml \
  | kubectl --context "$CONTEXT" apply -f - >/dev/null
rm -f "$TMP_COREFILE"
kubectl -n kube-system --context "$CONTEXT" rollout restart deployment/coredns >/dev/null
echo "[nullus] CoreDNS 갱신: $KC_HOST → $HOST_IP"

# ── 3. /etc/hosts ────────────────────────────────────────────────────────────
HOSTS_LINE="127.0.0.1 ${KC_HOST}"
for t in "${TOOL_HOSTS[@]}"; do HOSTS_LINE="$HOSTS_LINE ${t}.${DOMAIN}"; done
HOSTS_LINE="$HOSTS_LINE ${DOMAIN}"

if grep -qF "$KC_HOST" /etc/hosts 2>/dev/null; then
  echo "[nullus] /etc/hosts 에 이미 등록돼 있습니다"
elif [[ "$APPLY_HOSTS" == "true" ]]; then
  # 직접 수정한다. 되돌리려면 이 표식 줄을 지우면 된다.
  printf '\n# nullus local domain (setup-local-domain.sh)\n%s\n' "$HOSTS_LINE" | sudo tee -a /etc/hosts >/dev/null
  echo "[nullus] /etc/hosts 에 추가했습니다"
else
  echo ""
  echo "[nullus] /etc/hosts 에 아래 한 줄이 필요합니다 (sudo 필요):"
  echo ""
  echo "  echo '$HOSTS_LINE' | sudo tee -a /etc/hosts"
  echo ""
  echo "  또는 이 스크립트를 --apply-hosts 로 다시 실행하세요."
fi

# ── 4. 내부 CA 신뢰 ──────────────────────────────────────────────────────────
# 게이트웨이는 스택 설치가 만든 내부 CA 로 서명한 인증서를 쓴다. 브라우저가 그
# CA 를 모르면 도구 접속이 경고로 막힌다.
CA_SECRET="nullus-internal-ca"
CA_NS="$(kubectl get secret --all-namespaces --context "$CONTEXT" \
  -o jsonpath="{range .items[?(@.metadata.name=='${CA_SECRET}')]}{.metadata.namespace}{'\n'}{end}" 2>/dev/null | head -1 || true)"

if [[ -n "$CA_NS" ]]; then
  # TMPDIR 에 쓰면 macOS 에서 /var/folders/... 로 잡혀 경로를 옮겨 적기 어렵고,
  # 주기적으로 청소돼 나중에 다시 신뢰시킬 때 파일이 사라져 있다.
  # 저장소 안의 고정 위치에 둔다(.runbook-logs 는 gitignore 대상).
  CA_DIR="${PROJECT_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}/.runbook-logs"
  mkdir -p "$CA_DIR"
  CA_FILE="${CA_DIR}/nullus-internal-ca.crt"
  kubectl -n "$CA_NS" --context "$CONTEXT" get secret "$CA_SECRET" \
    -o jsonpath='{.data.tls\.crt}' | base64 -d >"$CA_FILE"
  echo ""
  echo "[nullus] 내부 CA 를 꺼냈습니다: $CA_FILE (namespace=$CA_NS)"
  echo "[nullus] 브라우저가 도구 인증서를 신뢰하게 하려면 (sudo 필요):"
  echo ""
  echo "  sudo security add-trusted-cert -d -r trustRoot \\"
  echo "    -k /Library/Keychains/System.keychain '$CA_FILE'"
else
  echo ""
  echo "[nullus] 내부 CA($CA_SECRET)를 찾지 못했습니다 — 스택 설치 후 다시 실행하세요."
fi

# ── 5. 다음 단계 ─────────────────────────────────────────────────────────────
cat <<EOF

══════════════════════════════════════════════════════════════════
  다음 단계
══════════════════════════════════════════════════════════════════

  1) 위 sudo 명령을 실행한다 (/etc/hosts, CA 신뢰)

  2) 같은 issuer 로 스택을 다시 띄운다:

       NULLUS_LOCAL_DOMAIN=$DOMAIN ./scripts/runbook_local.sh up --auth=keycloak

     포털·API·설치되는 도구가 모두
     http://${KC_HOST}:${KEYCLOAK_PORT}/realms/nullus 를 쓰게 된다.

  3) 도구는 443 으로 접근하므로 게이트웨이를 포워딩한다 (sudo, 특권 포트):

       sudo kubectl port-forward -n <스택네임스페이스> svc/<envoy-svc> 443:443

  확인:  https://argocd.$DOMAIN  → Keycloak 재인증 없이 진입
══════════════════════════════════════════════════════════════════
EOF
