#!/usr/bin/env bash
# smoke-sso.sh — Grafana ↔ Keycloak SSO 자동로그인 스모크 테스트
# 실행: bash scripts/smoke-sso.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KC_URL="http://localhost:8180"
GF_URL="http://localhost:3000"
REALM="nullus"
TEST_USER="admin@nullus.io"
TEST_PASS="${KEYCLOAK_TEST_USER_PASSWORD:-nullus123!}"
COOKIE_JAR="${TMPDIR:-/tmp}/smoke-sso-cookies-$$.txt"
REDIRECT_LOG="${TMPDIR:-/tmp}/smoke-sso-redirects-$$.txt"

cleanup() { rm -f "$COOKIE_JAR" "$REDIRECT_LOG"; }
trap cleanup EXIT

log()  { echo "[SMOKE] $*"; }
fail() { echo "[FAIL]  $*" >&2; exit 1; }
pass() { echo "[PASS]  $*"; }

# ── 1. 컨테이너 기동 ──────────────────────────────────────────────────────────
log "docker compose up (dev + sso overlay)..."
docker compose \
  -f "${REPO_ROOT}/docker-compose.dev.yaml" \
  -f "${REPO_ROOT}/docker-compose.sso.yaml" \
  up -d

# ── 2. Keycloak health 대기 ────────────────────────────────────────────────────
log "Keycloak health 대기 (최대 120s)..."
WAIT=0
until curl -sf "${KC_URL}/realms/master" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 120 ]] && fail "Keycloak 기동 타임아웃"
done
log "Keycloak OK (${WAIT}s)"

# ── 3. Grafana health 대기 ─────────────────────────────────────────────────────
log "Grafana health 대기 (최대 60s)..."
WAIT=0
until curl -sf "${GF_URL}/api/health" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 60 ]] && fail "Grafana 기동 타임아웃"
done
log "Grafana OK (${WAIT}s)"

# ── 4. Keycloak realm + grafana client + 테스트 유저 등록 ─────────────────────
log "setup-keycloak.sh 실행..."
KEYCLOAK_URL="${KC_URL}" \
KEYCLOAK_ADMIN_USER=admin \
KEYCLOAK_ADMIN_PASSWORD=admin \
GRAFANA_CLIENT_SECRET="${GRAFANA_CLIENT_SECRET:-grafana-dev-secret}" \
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

# ── 4c. VERIFY_PROFILE / VERIFY_EMAIL 비활성화 (enabled=false + defaultAction=false) ─
# KC 26: firstName/lastName 없으면 VERIFY_PROFILE 을 동적으로 주입.
# → required-action 자체를 disabled 로 만들어 트리거 차단
log "Realm required-actions 비활성화 (VERIFY_PROFILE, VERIFY_EMAIL)..."
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
    log "${ACTION_ALIAS}: enabled=false, defaultAction=false"
  else
    log "${ACTION_ALIAS}: 미존재, 스킵"
  fi
done

# ── 4d. 테스트 유저: 완전한 프로필 + requiredActions 클리어 ───────────────────
# KC26 VERIFY_PROFILE 은 firstName/lastName 누락 시 profile 불완전으로 판단 → 동적 주입
# → firstName/lastName 채워서 프로필 완성 + requiredActions=[]
log "테스트 유저 프로필 완성 및 requiredActions 클리어..."
ENC_USER=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${TEST_USER}'))")
USER_ID=$(curl -sS \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KC_URL}/admin/realms/${REALM}/users?username=${ENC_USER}" \
  | python3 -c "import json,sys; u=json.loads(sys.stdin.read()); print(u[0]['id'] if u else '')")
[[ -z "$USER_ID" ]] && fail "테스트 유저 '${TEST_USER}' 조회 실패"
log "유저 ID: ${USER_ID}"

curl -sS -X PUT \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  "${KC_URL}/admin/realms/${REALM}/users/${USER_ID}" \
  -d '{
    "firstName": "Smoke",
    "lastName": "Admin",
    "emailVerified": true,
    "requiredActions": []
  }' -o /dev/null
log "유저 프로필 완성 + requiredActions 클리어 완료"

# ── 5. Keycloak 브라우저 로그인 시뮬레이션 → SSO 쿠키 확보 ──────────────────
log "Keycloak 로그인 폼 POST (cookie jar)..."
ENC_REDIRECT=$(python3 -c \
  "import urllib.parse; print(urllib.parse.quote('http://localhost:3000/login/generic_oauth'))")
AUTH_URL="${KC_URL}/realms/${REALM}/protocol/openid-connect/auth"
AUTH_PARAMS="client_id=grafana&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=openid&state=smokessotest"

# GET: 로그인 폼 + KC_AUTH_STATE 쿠키 수신
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
    # 이미 로그인된 경우 Location 헤더로 리다이렉트될 수 있음
    raise SystemExit('로그인 폼 action 미발견')
" 2>&1 || true)

if [[ -z "$FORM_ACTION" || "$FORM_ACTION" == *"미발견"* ]]; then
  log "경고: 폼 action 미발견 — KC가 이미 세션을 가지고 있을 수 있음"
  log "LOGIN_HTML 처음 500자: ${LOGIN_HTML:0:500}"
else
  log "Form action: ${FORM_ACTION:0:90}..."

  # POST: 자격증명 제출 → KC SSO 세션 생성
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
log "Grafana SSO 자동로그인 테스트 (auto_login + KC session cookie)..."
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

# macOS BSD head -n -1 미지원 → python3 분리
API_CODE=$(echo "$API_RAW" | python3 -c \
  "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[-1].strip())")
API_BODY=$(echo "$API_RAW" | python3 -c \
  "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[0].rstrip())")

log "/api/user HTTP ${API_CODE}"
log "/api/user body: ${API_BODY}"

# ── 8. 결과 판정 ──────────────────────────────────────────────────────────────
if [[ "$API_CODE" == "200" ]]; then
  USER_EMAIL=$(echo "$API_BODY" | python3 -c \
    "import json,sys; d=json.loads(sys.stdin.read()); print(d.get('email','(no email)'))" \
    2>/dev/null || echo "(parse error)")
  pass "SSO 자동로그인 성공 — Grafana 세션 확보. 사용자: ${USER_EMAIL}"
  echo ""
  echo "컨테이너 정리 명령:"
  echo "  docker compose -f docker-compose.dev.yaml -f docker-compose.sso.yaml down"
  exit 0
else
  fail "SSO 자동로그인 실패 — /api/user HTTP ${API_CODE}. 위 redirect 체인을 확인하라."
fi
