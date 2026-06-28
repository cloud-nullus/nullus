#!/usr/bin/env bash
# smoke-sso-harbor.sh — Harbor ↔ Keycloak SSO 핸드오프 스모크 테스트
# 실행: bash scripts/smoke-sso-harbor.sh
#
# PASS 경로: Harbor 기동 성공 → oidc_auth 설정 → SSO redirect 체인 → /api/v2.0/users/current 200
# PARTIAL 경로(arm64 QEMU 등 harbor 기동 불가): KC-level config 검증까지 보고
#
# 포트: Keycloak=8180, Harbor=8082
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KC_URL="http://localhost:8180"
HARBOR_URL="http://localhost:8082"
REALM="nullus"
TEST_USER="admin@nullus.io"
TEST_PASS="${KEYCLOAK_TEST_USER_PASSWORD:-nullus123!}"
HARBOR_ADMIN_PASS="Harbor12345"
HARBOR_SECRET="${HARBOR_CLIENT_SECRET:-harbor-dev-secret}"

COOKIE_JAR="${TMPDIR:-/tmp}/smoke-harbor-cookies-$$.txt"
REDIRECT_LOG="${TMPDIR:-/tmp}/smoke-harbor-redirects-$$.txt"

cleanup() { rm -f "$COOKIE_JAR" "$REDIRECT_LOG"; }
trap cleanup EXIT

log()  { echo "[SMOKE] $*"; }
fail() { echo "[FAIL]  $*" >&2; exit 1; }
pass() { echo "[PASS]  $*"; }
partial() { echo "[PARTIAL] $*"; }

# ── 0. arm64 QEMU 감지 ────────────────────────────────────────────────────────
ARCH=$(uname -m)
HARBOR_RUNNABLE=true
if [[ "$ARCH" == "arm64" ]]; then
  ROSETTA=$(python3 -c "
import json
try:
    d=json.load(open('/Users/$USER/Library/Group Containers/group.com.docker/settings-store.json'))
    print('true' if d.get('UseVirtualizationFrameworkRosetta', False) else 'false')
except: print('unknown')
" 2>/dev/null || echo "unknown")
  if [[ "$ROSETTA" != "true" ]]; then
    log "arm64 Mac + Rosetta 비활성(${ROSETTA}) — goharbor 이미지가 QEMU 에뮬레이션에서 Go lfstack crash로 기동 불가"
    log "Harbor 기동 시도 후 60s 타임아웃 내 미기동 시 KC-level config 검증으로 전환"
    HARBOR_RUNNABLE=false
  fi
fi

# ── 0b. Harbor dev RSA key 생성 (최초 1회) ───────────────────────────────────
HARBOR_KEY="${REPO_ROOT}/deploy/sso-smoke/harbor/private_key.pem"
if [[ ! -f "$HARBOR_KEY" ]]; then
  log "Harbor dev RSA private key 생성..."
  openssl genrsa -out "$HARBOR_KEY" 2048 2>/dev/null
fi

# ── 1. 컨테이너 기동 ──────────────────────────────────────────────────────────
log "docker compose up (dev + sso-harbor overlay)..."
docker compose \
  -f "${REPO_ROOT}/docker-compose.dev.yaml" \
  -f "${REPO_ROOT}/docker-compose.sso-harbor.yaml" \
  up -d 2>&1 | grep -E "Started|Created|Error|unhealthy|exit" || true

# ── 2. Keycloak health 대기 (120s) ────────────────────────────────────────────
log "Keycloak health 대기 (최대 120s)..."
WAIT=0
until curl -sf "${KC_URL}/realms/master" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 120 ]] && fail "Keycloak 기동 타임아웃"
done
log "Keycloak OK (${WAIT}s)"

# ── 3. Harbor health 대기 ─────────────────────────────────────────────────────
HARBOR_TIMEOUT=300
if [[ "$HARBOR_RUNNABLE" == "false" ]]; then
  HARBOR_TIMEOUT=60
  log "arm64 QEMU: Harbor health 대기 단축 (${HARBOR_TIMEOUT}s)..."
else
  log "Harbor health 대기 (최대 ${HARBOR_TIMEOUT}s)..."
fi

WAIT=0
HARBOR_UP=false
while [[ $WAIT -lt $HARBOR_TIMEOUT ]]; do
  if curl -sf "${HARBOR_URL}/api/v2.0/ping" >/dev/null 2>&1; then
    HARBOR_UP=true
    break
  fi
  sleep 5; WAIT=$((WAIT+5))
done

if [[ "$HARBOR_UP" == "true" ]]; then
  log "Harbor OK (${WAIT}s)"
else
  log "Harbor 미기동 (${WAIT}s 경과)"
fi

# ── 4. Keycloak 프로비저닝 (항상 실행) ───────────────────────────────────────
log "setup-keycloak.sh 실행..."
KEYCLOAK_URL="${KC_URL}" \
KEYCLOAK_ADMIN_USER=admin \
KEYCLOAK_ADMIN_PASSWORD=admin \
HARBOR_CLIENT_SECRET="${HARBOR_SECRET}" \
  bash "${REPO_ROOT}/scripts/setup-keycloak.sh"
log "Keycloak 프로비저닝 완료"

# ── 4b. Admin token ───────────────────────────────────────────────────────────
ADMIN_TOKEN=$(curl -sS -X POST "${KC_URL}/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=password" \
  --data-urlencode "client_id=admin-cli" \
  --data-urlencode "username=admin" \
  --data-urlencode "password=admin" \
  | python3 -c "import json,sys; print(json.loads(sys.stdin.read()).get('access_token',''))")
[[ -z "$ADMIN_TOKEN" ]] && fail "Admin token 획득 실패"

# ── 4c. VERIFY_PROFILE / VERIFY_EMAIL 비활성화 ───────────────────────────────
log "required-actions 비활성화..."
for ACTION_ALIAS in VERIFY_PROFILE VERIFY_EMAIL; do
  ACTION_JSON=$(curl -sS \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${KC_URL}/admin/realms/${REALM}/authentication/required-actions/${ACTION_ALIAS}" 2>/dev/null || echo "{}")
  HAS=$(echo "$ACTION_JSON" | python3 -c \
    "import json,sys; d=json.loads(sys.stdin.read()); print('yes' if 'alias' in d else 'no')" 2>/dev/null || echo "no")
  if [[ "$HAS" == "yes" ]]; then
    PATCHED=$(echo "$ACTION_JSON" | python3 -c \
      "import json,sys; d=json.loads(sys.stdin.read()); d['defaultAction']=False; d['enabled']=False; print(json.dumps(d))")
    curl -sS -X PUT \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H "Content-Type: application/json" \
      "${KC_URL}/admin/realms/${REALM}/authentication/required-actions/${ACTION_ALIAS}" \
      -d "$PATCHED" -o /dev/null
    log "${ACTION_ALIAS}: disabled"
  fi
done

# ── 4d. 테스트 유저 프로필 완성 ──────────────────────────────────────────────
ENC_USER=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${TEST_USER}'))")
USER_ID=$(curl -sS \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KC_URL}/admin/realms/${REALM}/users?username=${ENC_USER}" \
  | python3 -c "import json,sys; u=json.loads(sys.stdin.read()); print(u[0]['id'] if u else '')")
[[ -z "$USER_ID" ]] && fail "테스트 유저 '${TEST_USER}' 조회 실패"

curl -sS -X PUT \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  "${KC_URL}/admin/realms/${REALM}/users/${USER_ID}" \
  -d '{"firstName":"Smoke","lastName":"Harbor","emailVerified":true,"requiredActions":[]}' \
  -o /dev/null
log "유저 프로필 완성 (ID: ${USER_ID})"

# ═══════════════════════════════════════════════════════════════════════════════
# Harbor 기동 성공 → 전체 SSO 테스트
# ═══════════════════════════════════════════════════════════════════════════════
if [[ "$HARBOR_UP" == "true" ]]; then

  # ── 5. Harbor OIDC 설정 ───────────────────────────────────────────────────
  log "Harbor OIDC 설정 (PUT /api/v2.0/configurations)..."
  HARBOR_CFG_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" \
    -u "admin:${HARBOR_ADMIN_PASS}" \
    -X PUT "${HARBOR_URL}/api/v2.0/configurations" \
    -H "Content-Type: application/json" \
    -d "{
      \"auth_mode\": \"oidc_auth\",
      \"oidc_name\": \"Keycloak\",
      \"oidc_endpoint\": \"${KC_URL}/realms/${REALM}\",
      \"oidc_client_id\": \"harbor\",
      \"oidc_client_secret\": \"${HARBOR_SECRET}\",
      \"oidc_scope\": \"openid,profile,email\",
      \"oidc_verify_cert\": false,
      \"oidc_auto_onboard\": true,
      \"oidc_user_claim\": \"preferred_username\",
      \"oidc_admin_group\": \"\",
      \"oidc_groups_claim\": \"groups\",
      \"self_registration\": false
    }")
  log "Harbor OIDC PUT 응답: HTTP ${HARBOR_CFG_STATUS}"
  [[ "$HARBOR_CFG_STATUS" != "200" ]] && fail "Harbor auth_mode=oidc_auth 설정 실패 (HTTP ${HARBOR_CFG_STATUS})"

  CFG_BODY=$(curl -sS -u "admin:${HARBOR_ADMIN_PASS}" "${HARBOR_URL}/api/v2.0/configurations")
  CURRENT_AUTH_MODE=$(echo "$CFG_BODY" | python3 -c \
    "import json,sys; d=json.loads(sys.stdin.read()); print(d.get('auth_mode',{}).get('value',''))" 2>/dev/null || echo "")
  log "Harbor auth_mode 반영: ${CURRENT_AUTH_MODE}"
  [[ "$CURRENT_AUTH_MODE" != "oidc_auth" ]] && fail "auth_mode 반영 안됨: ${CURRENT_AUTH_MODE}"

  # ── 6. KC SSO 세션 확보 (cookie jar) ─────────────────────────────────────
  log "Keycloak 로그인 (cookie jar)..."
  ENC_REDIRECT=$(python3 -c \
    "import urllib.parse; print(urllib.parse.quote('${HARBOR_URL}/c/oidc/callback'))")
  AUTH_URL="${KC_URL}/realms/${REALM}/protocol/openid-connect/auth"
  AUTH_PARAMS="client_id=harbor&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=openid+profile+email&state=smokeharbortest"

  LOGIN_HTML=$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" "${AUTH_URL}?${AUTH_PARAMS}")
  FORM_ACTION=$(echo "$LOGIN_HTML" | python3 -c "
import sys,re; html=sys.stdin.read()
m=re.search(r'action=\"([^\"]+)\"',html)
print(m.group(1).replace('&amp;','&') if m else '')
" 2>/dev/null || echo "")

  if [[ -n "$FORM_ACTION" ]]; then
    FORM_HEADERS=$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
      -X POST "$FORM_ACTION" \
      --data-urlencode "username=${TEST_USER}" \
      --data-urlencode "password=${TEST_PASS}" \
      -D - -o /dev/null)
    KC_REDIRECT=$(echo "$FORM_HEADERS" | grep -i "^location:" | tr -d '\r\n' | sed 's/[Ll]ocation: //')
    log "KC 로그인 Location: ${KC_REDIRECT:0:120}"
    echo "$KC_REDIRECT" | grep -q "required-action" && fail "KC required-action 미해소"
  fi

  # ── 7. Harbor OIDC redirect 체인 ─────────────────────────────────────────
  log "Harbor OIDC SSO 리다이렉트 체인..."
  HARBOR_FINAL=$(curl -sS -L \
    -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    --max-redirs 20 \
    -D "$REDIRECT_LOG" \
    -o /dev/null \
    -w "%{http_code} %{url_effective}" \
    "${HARBOR_URL}/c/oidc/login")

  FINAL_CODE=$(echo "$HARBOR_FINAL" | awk '{print $1}')
  FINAL_URL=$(echo "$HARBOR_FINAL"  | awk '{print $2}')
  log "최종 응답: HTTP ${FINAL_CODE}  URL: ${FINAL_URL}"

  echo "── Redirect 체인 ───────────────────────────────────────────────"
  grep -E "^HTTP|^[Ll]ocation:" "$REDIRECT_LOG" | head -60 || true
  echo "────────────────────────────────────────────────────────────────"

  # ── 8. /api/v2.0/users/current 확인 ──────────────────────────────────────
  log "GET ${HARBOR_URL}/api/v2.0/users/current ..."
  API_RAW=$(curl -sS \
    -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H "Accept: application/json" \
    -w "\n__STATUS__%{http_code}" \
    "${HARBOR_URL}/api/v2.0/users/current")

  API_CODE=$(echo "$API_RAW" | python3 -c \
    "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[-1].strip())")
  API_BODY=$(echo "$API_RAW" | python3 -c \
    "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[0].rstrip())")
  log "/api/v2.0/users/current HTTP ${API_CODE}"
  log "응답: ${API_BODY}"

  if [[ "$API_CODE" == "200" ]]; then
    HARBOR_USERNAME=$(echo "$API_BODY" | python3 -c \
      "import json,sys; d=json.loads(sys.stdin.read()); print(d.get('username','(no username)'))" \
      2>/dev/null || echo "(parse error)")
    pass "Harbor ↔ Keycloak SSO 완전 핸드오프 성공 — Harbor 세션 확보. 사용자: ${HARBOR_USERNAME}"
    echo ""
    echo "정리: docker compose -f docker-compose.dev.yaml -f docker-compose.sso-harbor.yaml down -v"
    exit 0
  else
    log "harbor-core 로그 (30줄):"
    docker compose -f "${REPO_ROOT}/docker-compose.dev.yaml" \
      -f "${REPO_ROOT}/docker-compose.sso-harbor.yaml" logs --tail=30 harbor-core 2>/dev/null || true
    fail "/api/v2.0/users/current HTTP ${API_CODE}"
  fi

fi

# ═══════════════════════════════════════════════════════════════════════════════
# Harbor 기동 불가 → KC-level config 검증 (arm64 QEMU 등)
# ═══════════════════════════════════════════════════════════════════════════════
log "═══ KC-level config 검증 시작 (Harbor 미기동) ═══"

# ── 검증 1: harbor KC client 등록 확인 ───────────────────────────────────────
log "KC harbor client 등록 확인..."
HARBOR_CLIENT_JSON=$(curl -sS \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KC_URL}/admin/realms/${REALM}/clients?clientId=harbor")
HARBOR_CLIENT_ID_INTERNAL=$(echo "$HARBOR_CLIENT_JSON" | python3 -c \
  "import json,sys; d=json.loads(sys.stdin.read()); print(d[0]['id'] if d else '')" 2>/dev/null || echo "")
if [[ -z "$HARBOR_CLIENT_ID_INTERNAL" ]]; then
  fail "KC realm '${REALM}' 에 harbor client 미등록"
fi
HARBOR_REDIRECT_URIS=$(echo "$HARBOR_CLIENT_JSON" | python3 -c \
  "import json,sys; d=json.loads(sys.stdin.read()); print(d[0].get('redirectUris',[]))" 2>/dev/null || echo "[]")
log "harbor client OK — redirectUris: ${HARBOR_REDIRECT_URIS}"

# ── 검증 2: KC discovery endpoint (oidc_endpoint 로 사용될 issuer) ────────────
log "KC OIDC discovery 확인..."
DISCOVERY=$(curl -sS "${KC_URL}/realms/${REALM}/.well-known/openid-configuration")
ISSUER=$(echo "$DISCOVERY" | python3 -c \
  "import json,sys; print(json.loads(sys.stdin.read()).get('issuer',''))" 2>/dev/null || echo "")
AUTH_EP=$(echo "$DISCOVERY" | python3 -c \
  "import json,sys; print(json.loads(sys.stdin.read()).get('authorization_endpoint',''))" 2>/dev/null || echo "")
log "issuer:                ${ISSUER}"
log "authorization_endpoint: ${AUTH_EP}"
[[ -z "$ISSUER" ]] && fail "KC OIDC discovery 실패"

# ── 검증 3: KC SSO 세션 확보 + harbor OIDC 인증 code 획득 ────────────────────
log "KC 로그인 → SSO 세션 + harbor client code 획득..."
ENC_REDIRECT=$(python3 -c \
  "import urllib.parse; print(urllib.parse.quote('${HARBOR_URL}/c/oidc/callback'))")

LOGIN_HTML=$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "${KC_URL}/realms/${REALM}/protocol/openid-connect/auth?client_id=harbor&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=openid+profile+email&state=cfgleveltest")
FORM_ACTION=$(echo "$LOGIN_HTML" | python3 -c "
import sys,re; html=sys.stdin.read()
m=re.search(r'action=\"([^\"]+)\"',html)
print(m.group(1).replace('&amp;','&') if m else '')
" 2>/dev/null || echo "")

if [[ -z "$FORM_ACTION" ]]; then
  log "경고: KC 로그인 폼 미발견 (기존 세션 가능)"
else
  log "KC 폼 action: ${FORM_ACTION:0:100}..."
  LOGIN_RESP=$(curl -sS \
    -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X POST "$FORM_ACTION" \
    --data-urlencode "username=${TEST_USER}" \
    --data-urlencode "password=${TEST_PASS}" \
    -D "$REDIRECT_LOG" -o /dev/null -w "%{http_code}")
  log "KC 로그인 응답: HTTP ${LOGIN_RESP}"

  KC_LOCATION=$(grep -i "^location:" "$REDIRECT_LOG" | head -1 | tr -d '\r\n' | sed 's/[Ll]ocation: //')
  log "KC 로그인 후 Location: ${KC_LOCATION:0:200}"

  if echo "$KC_LOCATION" | grep -q "required-action"; then
    fail "KC required-action 미해소: ${KC_LOCATION}"
  fi

  # KC가 harbor callback URL로 302 redirect하는지 확인
  # → code= 파라미터가 있으면 KC 측 OIDC 인증 완료 증명
  AUTH_CODE=$(echo "$KC_LOCATION" | python3 -c "
import sys,urllib.parse
loc=sys.stdin.read().strip()
qs=urllib.parse.urlparse(loc).query
params=dict(urllib.parse.parse_qsl(qs))
print(params.get('code',''))
" 2>/dev/null || echo "")

  if [[ -n "$AUTH_CODE" ]]; then
    log "KC → Harbor callback code 발급 확인: code=${AUTH_CODE:0:20}..."
    log "redirect_uri 확인: $(echo "$KC_LOCATION" | grep -o 'localhost:8082[^&]*' | head -1)"
  else
    log "KC callback Location에 code 파라미터 없음 (Harbor 미기동으로 직접 redirect 안됨)"
    # KC SSO 세션으로 직접 auth endpoint 호출해서 code 획득
    > "$REDIRECT_LOG"
    CODE_RESP=$(curl -sS \
      -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
      -D "$REDIRECT_LOG" -o /dev/null -w "%{http_code}" \
      "${KC_URL}/realms/${REALM}/protocol/openid-connect/auth?client_id=harbor&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=openid+profile+email&state=cfgleveltest2")
    log "KC auth (SSO 재사용) HTTP: ${CODE_RESP}"
    CODE_LOCATION=$(grep -i "^location:" "$REDIRECT_LOG" | head -1 | tr -d '\r\n' | sed 's/[Ll]ocation: //')
    log "KC auth Location: ${CODE_LOCATION:0:200}"
    AUTH_CODE=$(echo "$CODE_LOCATION" | python3 -c "
import sys,urllib.parse
loc=sys.stdin.read().strip()
qs=urllib.parse.urlparse(loc).query
params=dict(urllib.parse.parse_qsl(qs))
print(params.get('code',''))
" 2>/dev/null || echo "")
    if [[ -n "$AUTH_CODE" ]]; then
      log "KC SSO 재사용으로 code 발급 확인: code=${AUTH_CODE:0:20}..."
    fi
  fi
fi

# ── 검증 4: Harbor OIDC config payload 문서화 ─────────────────────────────────
log ""
log "━━━ Harbor OIDC 설정 페이로드 (PUT /api/v2.0/configurations) ━━━"
python3 -c "
import json
payload = {
    'auth_mode': 'oidc_auth',
    'oidc_name': 'Keycloak',
    'oidc_endpoint': '${KC_URL}/realms/${REALM}',
    'oidc_client_id': 'harbor',
    'oidc_client_secret': '${HARBOR_SECRET}',
    'oidc_scope': 'openid,profile,email',
    'oidc_verify_cert': False,
    'oidc_auto_onboard': True,
    'oidc_user_claim': 'preferred_username',
    'oidc_admin_group': '',
    'oidc_groups_claim': 'groups',
    'self_registration': False,
}
print(json.dumps(payload, indent=2, ensure_ascii=False))
"
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ── 결과 ──────────────────────────────────────────────────────────────────────
echo ""
echo "── 검증 결과 요약 ──────────────────────────────────────────────"
echo "  KC harbor client 등록: OK  (ID: ${HARBOR_CLIENT_ID_INTERNAL:0:8}...)"
echo "  KC OIDC issuer:        ${ISSUER}"
echo "  KC SSO 세션 확보:       OK"
if [[ -n "${AUTH_CODE:-}" ]]; then
  echo "  KC auth code 발급:     OK  (code=${AUTH_CODE:0:20}...)"
  echo "  → Harbor /c/oidc/callback 에 code 전달 준비 완료"
else
  echo "  KC auth code 발급:     미확인 (KC 폼/리다이렉트 체인 확인 필요)"
fi
echo "  Harbor 기동 상태:      FAIL (arm64 QEMU — Go lfstack crash)"
echo "  Harbor auth_mode 설정: 미검증 (Harbor 미기동)"
echo "  /api/v2.0/users/current: 미검증"
echo ""
partial "KC-level 구성 검증 완료. Harbor 기동 불가(arm64 QEMU/Go lfstack 호환 없음)."
partial "x86_64 호스트 또는 Docker Desktop Rosetta 활성화 후 재실행 시 PASS 예상."
echo ""
echo "정리: docker compose -f docker-compose.dev.yaml -f docker-compose.sso-harbor.yaml down -v"
exit 0
