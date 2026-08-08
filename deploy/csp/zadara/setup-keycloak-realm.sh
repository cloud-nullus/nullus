#!/usr/bin/env bash
# =============================================================================
# setup-keycloak-realm.sh — 배포된 Keycloak 에 nullus realm 을 구성한다
# =============================================================================
# 차트는 Keycloak 을 띄우기만 하고 realm 은 만들지 않는다. realm 이 없으면
# `/realms/nullus/.well-known/openid-configuration` 이 404 라 로그인 화면이
# "Failed to fetch" 로 끝난다.
#
# realm·클라이언트·audience 매퍼·org_id 클레임·롤·사용자 구성은 저장소의
# `scripts/setup-keycloak.sh` 가 이미 관리한다. 그 로직을 여기서 복제하지 않고
# 그대로 호출한 뒤, **이 환경에만 다른 것 하나**를 덧입힌다.
#
#   업스트림은 redirectUris/webOrigins 를 http://localhost:5173 으로 고정한다
#   (로컬 dev 전제). 배포 환경에서는 실제 접속 주소여야 하고, 아니면 Keycloak 이
#   "Invalid parameter: redirect_uri" 로 거부한다.
#
# 업스트림이 만드는 클라이언트는 `nullus-app` 이다. 프런트엔드의 빌드 기본값은
# `nullus-web` 이라 서로 어긋나 있는데, 차트의 audience 기본값도 `nullus-app` 이므로
# 배포에서는 `nullus-app` 으로 통일한다 (values-zadara.yaml 의
# web.auth.oidcClientId / config.auth.oidcAudience).
#
# 사용법:
#   ./setup-keycloak-realm.sh                       구성 + 리다이렉트 URI 교정
#   PUBLIC_URL=https://my.host ./setup-keycloak-realm.sh
#   ./setup-keycloak-realm.sh status                현재 상태만 확인
#
# 환경 변수:
#   NAMESPACE     (기본: nullus)
#   KUBE_CONTEXT  (기본: 현재 컨텍스트)
#   RELEASE       (기본: nullus)  → 파드 <RELEASE>-keycloak-0
#   PUBLIC_URL    SPA 접속 주소      (기본: https://nullus.io)
#                 스킴까지 실제와 같아야 한다 — Keycloak 이 정확히 대조한다.
#   KC_PUBLIC_URL Keycloak 공개 주소 (기본: https://auth.nullus.io)
#   REALM         (기본: nullus)
#   CLIENT_ID     (기본: nullus-app)
#
# 종료 코드: 0 성공 / 1 실패
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"
UPSTREAM="${UPSTREAM:-${REPO_ROOT}/scripts/setup-keycloak.sh}"

NAMESPACE="${NAMESPACE:-nullus}"
RELEASE="${RELEASE:-nullus}"
REALM="${REALM:-nullus}"
CLIENT_ID="${CLIENT_ID:-nullus-app}"
PUBLIC_URL="${PUBLIC_URL:-https://nullus.io}"
KC_PUBLIC_URL="${KC_PUBLIC_URL:-https://auth.nullus.io}"
POD="${RELEASE}-keycloak-0"

if [[ -t 1 ]]; then
  C_OK=$'\033[1;32m'; C_ERR=$'\033[1;31m'; C_WARN=$'\033[1;33m'; C_DIM=$'\033[2m'; C_RST=$'\033[0m'
else
  C_OK=""; C_ERR=""; C_WARN=""; C_DIM=""; C_RST=""
fi
ok()   { printf '%s✔%s %s\n' "$C_OK"   "$C_RST" "$*"; }
warn() { printf '%s!%s %s\n' "$C_WARN" "$C_RST" "$*"; }
info() { printf '%s·%s %s\n' "$C_DIM"  "$C_RST" "$*"; }
die()  { printf '%s✘%s %s\n' "$C_ERR"  "$C_RST" "$*" >&2; exit 1; }

K=(kubectl)
[[ -n "${KUBE_CONTEXT:-}" ]] && K+=(--context="$KUBE_CONTEXT")
K+=(-n "$NAMESPACE")

admin_password() {
  "${K[@]}" get secret "${RELEASE}-keycloak" -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d
}

# kcadm 경로는 이미지 계열마다 다르다. bitnami 계열은 /opt/bitnami/keycloak/bin,
# 공식 quay.io/keycloak 이미지는 /opt/keycloak/bin 이다.
resolve_kcadm() {
  [[ -n "${KCADM:-}" ]] && { printf '%s\n' "$KCADM"; return; }
  local c
  for c in /opt/bitnami/keycloak/bin/kcadm.sh /opt/keycloak/bin/kcadm.sh; do
    if "${K[@]}" exec -i "$POD" -c keycloak -- test -x "$c" 2>/dev/null; then
      printf '%s\n' "$c"; return
    fi
  done
  die "kcadm.sh 를 찾지 못했습니다. KCADM=<경로> 로 지정하세요."
}

kcadm_login() {
  local kcadm="$1" pw="$2"
  "${K[@]}" exec -i "$POD" -c keycloak -- "$kcadm" config credentials \
    --server http://127.0.0.1:8080 --realm master --user admin --password "$pw" \
    --config /tmp/nullus-kcadm.config >/dev/null || die "kcadm 로그인 실패"
}

# 관리 API 는 공개 주소(HTTPS)로 붙는다. 비밀번호는 시크릿에서 읽어 환경변수로만
# 넘기고 디스크에 남기지 않는다.
run_upstream() {
  local pw="$1"
  [[ -x "$UPSTREAM" ]] || die "업스트림 스크립트를 찾지 못했습니다: ${UPSTREAM}"
  info "업스트림 실행: ${UPSTREAM#$REPO_ROOT/}"
  KEYCLOAK_URL="$KC_PUBLIC_URL" \
  KEYCLOAK_ADMIN_USER=admin \
  KEYCLOAK_ADMIN_PASSWORD="$pw" \
    "$UPSTREAM" || die "업스트림 스크립트 실패"
}

# 업스트림이 localhost 로 박아 둔 리다이렉트 URI 를 배포 주소로 바꾼다.
# 로컬 dev 주소도 남겨 두어 같은 realm 을 개발에서도 계속 쓸 수 있게 한다.
fix_redirect_uris() {
  local kcadm="$1"
  local uuid
  uuid="$("${K[@]}" exec -i "$POD" -c keycloak -- "$kcadm" get clients -r "$REALM" \
          -q "clientId=${CLIENT_ID}" --fields id --format csv --noquotes \
          --config /tmp/nullus-kcadm.config 2>/dev/null | tr -d '\r' \
          | grep -E '^[0-9a-f-]{36}$' | head -1)"
  [[ -n "$uuid" ]] || die "클라이언트 ${CLIENT_ID} 를 찾지 못했습니다"

  # post.logout.redirect.uris 를 명시하지 않으면 로그아웃 복귀 주소가 거부될 수 있다.
  # "+" 는 redirectUris 를 그대로 쓰겠다는 Keycloak 의 관용 표기다.
  "${K[@]}" exec -i "$POD" -c keycloak -- "$kcadm" update "clients/${uuid}" -r "$REALM" \
    -s "redirectUris=[\"${PUBLIC_URL}/*\",\"http://localhost:5173/*\"]" \
    -s "webOrigins=[\"${PUBLIC_URL}\",\"http://localhost:5173\"]" \
    -s "rootUrl=${PUBLIC_URL}" \
    -s 'attributes."post.logout.redirect.uris"=+' \
    --config /tmp/nullus-kcadm.config >/dev/null || die "리다이렉트 URI 갱신 실패"
  ok "리다이렉트 URI 교정 — ${PUBLIC_URL}/* (+ localhost:5173), post-logout 허용"
}

do_setup() {
  command -v kubectl >/dev/null || die "kubectl 이 필요합니다"
  "${K[@]}" get pod "$POD" >/dev/null 2>&1 || die "Keycloak 파드를 찾지 못했습니다: ${POD}"

  info "대상 ${NAMESPACE}/${POD}"
  info "SPA ${PUBLIC_URL}   Keycloak ${KC_PUBLIC_URL}   realm ${REALM}   client ${CLIENT_ID}"

  local pw; pw="$(admin_password)"
  [[ -n "$pw" ]] || die "Keycloak admin 비밀번호를 읽지 못했습니다 (${RELEASE}-keycloak)"

  run_upstream "$pw"
  local kcadm; kcadm="$(resolve_kcadm)"
  kcadm_login "$kcadm" "$pw"
  fix_redirect_uris "$kcadm"
  echo
  do_status
}

do_status() {
  local disc="${KC_PUBLIC_URL}/realms/${REALM}/.well-known/openid-configuration"
  printf '· discovery ... '
  local code; code="$(curl -s -o /tmp/nullus-oidc.json -w '%{http_code}' --max-time 15 "$disc" || true)"
  if [[ "$code" != "200" ]]; then
    warn "HTTP ${code:-none} — realm 이 없거나 ingress 가 아직 반영되지 않았습니다"
    return
  fi
  ok "HTTP 200"
  python3 -c 'import json;d=json.load(open("/tmp/nullus-oidc.json"));print("    issuer:", d.get("issuer"))' 2>/dev/null || true

  local kcadm; kcadm="$(resolve_kcadm)"
  echo "· 클라이언트 ${CLIENT_ID}"
  "${K[@]}" exec -i "$POD" -c keycloak -- "$kcadm" get clients -r "$REALM" \
    -q "clientId=${CLIENT_ID}" --fields clientId,publicClient,redirectUris \
    --config /tmp/nullus-kcadm.config 2>/dev/null | sed 's/^/    /' || true
  echo "· 사용자"
  "${K[@]}" exec -i "$POD" -c keycloak -- "$kcadm" get users -r "$REALM" \
    --fields username,email,enabled --config /tmp/nullus-kcadm.config 2>/dev/null \
    | python3 -c 'import sys,json
try:
    for u in json.load(sys.stdin): print("    %-24s %s" % (u.get("username"), u.get("email")))
except Exception: pass' || true
}

case "${1:-setup}" in
  setup)  do_setup ;;
  status) do_status ;;
  *)      die "사용법: $0 [setup|status]" ;;
esac
