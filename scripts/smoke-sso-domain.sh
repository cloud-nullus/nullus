#!/usr/bin/env bash
# smoke-sso-domain.sh — 도메인 기반 Grafana ↔ Keycloak SSO 스모크 테스트
# 접속: http://grafana.nullus.internal (포트 80 nginx proxy 경유)
# 전제: /etc/hosts 에 127.0.0.1 *.nullus.internal 매핑 존재
# 실행: bash scripts/smoke-sso-domain.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KC_URL="http://keycloak.nullus.internal"
GF_URL="http://grafana.nullus.internal"
# Keycloak Admin API는 내부 포트 직접 접근 (healthcheck 등)
KC_INTERNAL="http://localhost:8180"
REALM="nullus"
TEST_USER="admin@nullus.io"
TEST_PASS="${KEYCLOAK_TEST_USER_PASSWORD:-nullus123!}"
COOKIE_JAR="${TMPDIR:-/tmp}/smoke-sso-domain-cookies-$$.txt"
REDIRECT_LOG="${TMPDIR:-/tmp}/smoke-sso-domain-redirects-$$.txt"

cleanup() { rm -f "$COOKIE_JAR" "$REDIRECT_LOG"; }
trap cleanup EXIT

log()  { echo "[SMOKE] $*"; }
fail() {
  echo "[FAIL]  $*" >&2
  echo ""
  echo "── Grafana 로그 (마지막 30줄) ───────────────────────────────────────────────"
  docker compose \
    -f "${REPO_ROOT}/docker-compose.dev.yaml" \
    -f "${REPO_ROOT}/docker-compose.sso-domain.yaml" \
    logs --tail=30 grafana 2>/dev/null || true
  echo ""
  echo "── Keycloak 로그 (마지막 20줄) ──────────────────────────────────────────────"
  docker compose \
    -f "${REPO_ROOT}/docker-compose.dev.yaml" \
    -f "${REPO_ROOT}/docker-compose.sso-domain.yaml" \
    logs --tail=20 keycloak 2>/dev/null || true
  echo ""
  echo "── reverseproxy 로그 (마지막 20줄) ──────────────────────────────────────────"
  docker compose \
    -f "${REPO_ROOT}/docker-compose.dev.yaml" \
    -f "${REPO_ROOT}/docker-compose.sso-domain.yaml" \
    logs --tail=20 reverseproxy 2>/dev/null || true
  exit 1
}
pass() { echo "[PASS]  $*"; }

# ── 0. 기존 스택 완전 정리 (keycloak fresh 보장) ─────────────────────────────
log "기존 SSO 스택 정리 (fresh Keycloak 보장)..."
docker compose \
  -f "${REPO_ROOT}/docker-compose.dev.yaml" \
  -f "${REPO_ROOT}/docker-compose.sso.yaml" \
  -f "${REPO_ROOT}/docker-compose.sso-domain.yaml" \
  down --remove-orphans 2>/dev/null || true

# ── 1. 도메인 오버레이로 기동 ─────────────────────────────────────────────────
log "docker compose up (dev + sso-domain overlay)..."
docker compose \
  -f "${REPO_ROOT}/docker-compose.dev.yaml" \
  -f "${REPO_ROOT}/docker-compose.sso-domain.yaml" \
  up -d

# ── 2. Keycloak health 대기 ────────────────────────────────────────────────────
log "Keycloak health 대기 (최대 120s, 내부 포트 8180)..."
WAIT=0
until curl -sf "${KC_INTERNAL}/realms/master" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 120 ]] && fail "Keycloak 기동 타임아웃"
done
log "Keycloak OK (${WAIT}s)"

# ── 2b. proxy 경유 keycloak.nullus.internal 도달 확인 ────────────────────────
log "proxy 경유 keycloak.nullus.internal 도달 확인..."
WAIT=0
until curl -sf "${KC_URL}/realms/master" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 60 ]] && fail "keycloak.nullus.internal proxy 경유 타임아웃"
done
log "keycloak.nullus.internal OK (proxy 경유, ${WAIT}s)"

# ── 3. Grafana health 대기 ─────────────────────────────────────────────────────
log "Grafana health 대기 (최대 90s, proxy 경유)..."
WAIT=0
until curl -sf "${GF_URL}/api/health" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 90 ]] && fail "Grafana 기동 타임아웃"
done
log "Grafana OK (${WAIT}s)"

# ── 4. Keycloak realm + grafana client + 테스트 유저 등록 ─────────────────────
log "setup-keycloak.sh 실행..."
KEYCLOAK_URL="${KC_INTERNAL}" \
KEYCLOAK_ADMIN_USER=admin \
KEYCLOAK_ADMIN_PASSWORD=admin \
GRAFANA_CLIENT_SECRET="${GRAFANA_CLIENT_SECRET:-grafana-dev-secret}" \
  bash "${REPO_ROOT}/scripts/setup-keycloak.sh"
log "Keycloak 프로비저닝 완료"

# ── 4b. Admin token ────────────────────────────────────────────────────────────
ADMIN_TOKEN=$(curl -sS -X POST "${KC_INTERNAL}/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=password" \
  --data-urlencode "client_id=admin-cli" \
  --data-urlencode "username=admin" \
  --data-urlencode "password=admin" \
  | python3 -c "import json,sys; print(json.loads(sys.stdin.read()).get('access_token',''))")
[[ -z "$ADMIN_TOKEN" ]] && fail "Admin token 획득 실패"

# ── 4c. grafana client redirect URI에 도메인 추가 ────────────────────────────
log "grafana client에 도메인 redirect URI 추가..."
GF_CLIENT_JSON=$(curl -sS \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KC_INTERNAL}/admin/realms/${REALM}/clients?clientId=grafana")
GF_INTERNAL_ID=$(echo "$GF_CLIENT_JSON" | python3 -c \
  "import json,sys; d=json.loads(sys.stdin.read()); print(d[0]['id'] if d else '')")
[[ -z "$GF_INTERNAL_ID" ]] && fail "grafana client 조회 실패"

# 현재 redirectUris / webOrigins 에 도메인 항목 추가
UPDATED_CLIENT=$(echo "$GF_CLIENT_JSON" | python3 -c "
import json, sys
d = json.loads(sys.stdin.read())[0]
domain_redirect = 'http://grafana.nullus.internal/login/generic_oauth'
domain_origin   = 'http://grafana.nullus.internal'
uris = d.get('redirectUris', [])
origins = d.get('webOrigins', [])
if domain_redirect not in uris:
    uris.append(domain_redirect)
if domain_origin not in origins:
    origins.append(domain_origin)
d['redirectUris'] = uris
d['webOrigins']   = origins
print(json.dumps(d))
")
curl -sS -X PUT \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  "${KC_INTERNAL}/admin/realms/${REALM}/clients/${GF_INTERNAL_ID}" \
  -d "$UPDATED_CLIENT" -o /dev/null
log "grafana client redirect URI 업데이트 완료"

# ── 4d. VERIFY_PROFILE / VERIFY_EMAIL 비활성화 ──────────────────────────────
log "Realm required-actions 비활성화 (VERIFY_PROFILE, VERIFY_EMAIL)..."
for ACTION_ALIAS in VERIFY_PROFILE VERIFY_EMAIL; do
  ACTION_JSON=$(curl -sS \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${KC_INTERNAL}/admin/realms/${REALM}/authentication/required-actions/${ACTION_ALIAS}" 2>/dev/null || echo "{}")
  HAS=$(echo "$ACTION_JSON" | python3 -c \
    "import json,sys; d=json.loads(sys.stdin.read()); print('yes' if 'alias' in d else 'no')" 2>/dev/null || echo "no")
  if [[ "$HAS" == "yes" ]]; then
    PATCHED=$(echo "$ACTION_JSON" | python3 -c \
      "import json,sys; d=json.loads(sys.stdin.read()); d['defaultAction']=False; d['enabled']=False; print(json.dumps(d))")
    curl -sS -X PUT \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H "Content-Type: application/json" \
      "${KC_INTERNAL}/admin/realms/${REALM}/authentication/required-actions/${ACTION_ALIAS}" \
      -d "$PATCHED" -o /dev/null
    log "${ACTION_ALIAS}: enabled=false, defaultAction=false"
  else
    log "${ACTION_ALIAS}: 미존재, 스킵"
  fi
done

# ── 4e. 테스트 유저 프로필 완성 + requiredActions 클리어 ─────────────────────
log "테스트 유저 프로필 완성 및 requiredActions 클리어..."
ENC_USER=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${TEST_USER}'))")
USER_ID=$(curl -sS \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KC_INTERNAL}/admin/realms/${REALM}/users?username=${ENC_USER}" \
  | python3 -c "import json,sys; u=json.loads(sys.stdin.read()); print(u[0]['id'] if u else '')")
[[ -z "$USER_ID" ]] && fail "테스트 유저 '${TEST_USER}' 조회 실패"
log "유저 ID: ${USER_ID}"

curl -sS -X PUT \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  "${KC_INTERNAL}/admin/realms/${REALM}/users/${USER_ID}" \
  -d '{
    "firstName": "Smoke",
    "lastName": "Admin",
    "emailVerified": true,
    "requiredActions": []
  }' -o /dev/null
log "유저 프로필 완성 + requiredActions 클리어 완료"

# ── 5. KC 로그인 → SSO 쿠키 확보 ────────────────────────────────────────────
log "Keycloak 로그인 폼 POST (cookie jar, 도메인 URL)..."
ENC_REDIRECT=$(python3 -c \
  "import urllib.parse; print(urllib.parse.quote('http://grafana.nullus.internal/login/generic_oauth'))")
AUTH_URL="${KC_URL}/realms/${REALM}/protocol/openid-connect/auth"
AUTH_PARAMS="client_id=grafana&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=openid&state=smokessodomain"

# GET: 로그인 폼 + KC 세션 쿠키 수신
LOGIN_HTML=$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "${AUTH_URL}?${AUTH_PARAMS}")

# 폼 action 추출
FORM_ACTION=$(echo "$LOGIN_HTML" | python3 -c "
import sys, re
html = sys.stdin.read()
m = re.search(r'action=\"([^\"]+)\"', html)
if m:
    print(m.group(1).replace('&amp;', '&'))
else:
    raise SystemExit('로그인 폼 action 미발견')
" 2>&1 || true)

if [[ -z "$FORM_ACTION" || "$FORM_ACTION" == *"미발견"* ]]; then
  log "경고: 폼 action 미발견 — KC가 이미 세션을 가지고 있을 수 있음"
  log "LOGIN_HTML 처음 500자: ${LOGIN_HTML:0:500}"
else
  log "Form action: ${FORM_ACTION:0:90}..."

  FORM_HEADERS=$(curl -sS \
    -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X POST "$FORM_ACTION" \
    --data-urlencode "username=${TEST_USER}" \
    --data-urlencode "password=${TEST_PASS}" \
    -D - -o /dev/null)

  KC_REDIRECT=$(echo "$FORM_HEADERS" | grep -i "^location:" | tr -d '\r\n' | sed 's/[Ll]ocation: //')
  log "Keycloak 로그인 Location: ${KC_REDIRECT:0:120}"

  if echo "$KC_REDIRECT" | grep -q "required-action"; then
    fail "Keycloak required-action 차단 남아있음: ${KC_REDIRECT}"
  fi
fi

log "Cookie jar 내용:"
grep -v "^#\|^$" "$COOKIE_JAR" | awk '{print $6, $7}' || true

# ── 6. Grafana SSO 자동로그인 확인 ─────────────────────────────────────────────
log "Grafana SSO redirect 체인 테스트 (도메인 URL)..."
GF_FINAL=$(curl -sS -L \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  --max-redirs 20 \
  -D "$REDIRECT_LOG" \
  -o /dev/null \
  -w "%{http_code} %{url_effective}" \
  "${GF_URL}/login/generic_oauth")

FINAL_CODE=$(echo "$GF_FINAL" | awk '{print $1}')
FINAL_URL=$(echo "$GF_FINAL"  | awk '{print $2}')
log "최종 응답: HTTP ${FINAL_CODE}  URL: ${FINAL_URL}"

echo "── Redirect 체인 ───────────────────────────────────────────────"
grep -E "^HTTP|^[Ll]ocation:" "$REDIRECT_LOG" | head -50 || true
echo "────────────────────────────────────────────────────────────────"

# ── 7. Grafana /api/user 확인 ─────────────────────────────────────────────────
log "GET ${GF_URL}/api/user ..."
API_RAW=$(curl -sS \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -w "\n__STATUS__%{http_code}" \
  "${GF_URL}/api/user")

API_CODE=$(echo "$API_RAW" | python3 -c \
  "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[-1].strip())")
API_BODY=$(echo "$API_RAW" | python3 -c \
  "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[0].rstrip())")

log "/api/user HTTP ${API_CODE}"
log "/api/user body: ${API_BODY}"

# ── 8. issuer 정합 확인 ────────────────────────────────────────────────────────
log "KC issuer 확인 (iss=http://keycloak.nullus.internal 기대)..."
OIDC_META=$(curl -sf "${KC_URL}/realms/${REALM}/.well-known/openid-configuration" 2>/dev/null || echo "{}")
ISSUER=$(echo "$OIDC_META" | python3 -c \
  "import json,sys; print(json.loads(sys.stdin.read()).get('issuer','(unknown)'))" 2>/dev/null || echo "(error)")
log "실제 issuer: ${ISSUER}"
if [[ "$ISSUER" != "http://keycloak.nullus.internal/realms/${REALM}" ]]; then
  log "경고: issuer 불일치 → ${ISSUER} (SSO 실패 원인일 수 있음)"
fi

# ── 9. 결과 판정 ──────────────────────────────────────────────────────────────
if [[ "$API_CODE" == "200" ]]; then
  USER_EMAIL=$(echo "$API_BODY" | python3 -c \
    "import json,sys; d=json.loads(sys.stdin.read()); print(d.get('email','(no email)'))" \
    2>/dev/null || echo "(parse error)")
  pass "도메인 기반 SSO 자동로그인 성공 — Grafana 세션 확보. 사용자: ${USER_EMAIL}"
  echo ""
  echo "브라우저 접속 URL : http://grafana.nullus.internal/"
  echo "계정              : ${TEST_USER} / ${TEST_PASS}"
  echo "Keycloak 콘솔     : http://keycloak.nullus.internal/  (admin / admin)"
  echo ""
  echo "컨테이너 정리 명령 (접속 확인 후):"
  echo "  docker compose -f docker-compose.dev.yaml -f docker-compose.sso-domain.yaml down"
  exit 0
else
  fail "SSO 자동로그인 실패 — /api/user HTTP ${API_CODE}. 위 redirect 체인을 확인하라."
fi
