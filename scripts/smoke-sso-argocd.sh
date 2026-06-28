#!/usr/bin/env bash
# smoke-sso-argocd.sh — ArgoCD ↔ Keycloak SSO 자동로그인 스모크 테스트
# 구조: smoke-sso.sh (Grafana) 패턴을 그대로 차용
# 실행: bash scripts/smoke-sso-argocd.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KC_URL="http://localhost:8180"
ARGOCD_URL="http://localhost:8081"
REALM="nullus"
TEST_USER="admin@nullus.io"
TEST_PASS="${KEYCLOAK_TEST_USER_PASSWORD:-nullus123!}"
ARGOCD_NS="argocd"
COOKIE_JAR="${TMPDIR:-/tmp}/smoke-sso-argocd-cookies-$$.txt"
REDIRECT_LOG="${TMPDIR:-/tmp}/smoke-sso-argocd-redirects-$$.txt"

cleanup() { rm -f "$COOKIE_JAR" "$REDIRECT_LOG"; }
trap cleanup EXIT

log()  { echo "[SMOKE] $*"; }
fail() { echo "[FAIL]  $*" >&2; exit 1; }
pass() { echo "[PASS]  $*"; }

# ── 0. K8s 사전 확인 ─────────────────────────────────────────────────────────
log "kubectl 연결 확인..."
kubectl cluster-info --request-timeout=5s >/dev/null 2>&1 \
  || fail "kubectl cluster-info 실패: K8s(192.168.56.100:6443) 접근 불가"
log "K8s OK"

# ── 1. K8s argocd 네임스페이스 + ConfigMap + Secret 생성 ─────────────────────
log "argocd 네임스페이스 생성..."
kubectl create namespace "${ARGOCD_NS}" --dry-run=client -o yaml \
  | kubectl apply -f - >/dev/null

log "argocd-cm ConfigMap 적용 (OIDC → Keycloak)..."
kubectl apply -n "${ARGOCD_NS}" -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  # ArgoCD 공개 URL — redirect_uri 구성에 사용 (setup-keycloak.sh 등록값과 일치)
  url: "http://localhost:8081"
  # 직접 OIDC (Dex 비활성) — issuer 정합: localhost:8180 고정
  oidc.config: |
    name: Keycloak
    issuer: http://localhost:8180/realms/nullus
    clientID: argocd
    clientSecret: $oidc.keycloak.clientSecret
    requestedScopes:
      - openid
      - profile
      - email
EOF

log "argocd-rbac-cm RBAC ConfigMap 적용..."
kubectl apply -n "${ARGOCD_NS}" -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-rbac-cm
    app.kubernetes.io/part-of: argocd
data:
  policy.default: role:readonly
  policy.csv: |
    g, admin, role:admin
EOF

log "argocd-secret Secret 적용..."
# admin.password = bcrypt("argocd-admin") base64-encoded
# oidc.keycloak.clientSecret = base64("argocd-dev-secret") → YXJnb2NkLWRldi1zZWNyZXQ=
# server.secretkey = random 32-byte base64
SERVER_SECRET_KEY=$(python3 -c "import base64,os; print(base64.b64encode(os.urandom(32)).decode())")
SERVER_SECRET_B64=$(python3 -c "import base64,sys; print(base64.b64encode(sys.argv[1].encode()).decode())" "$SERVER_SECRET_KEY")
kubectl apply -n "${ARGOCD_NS}" -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: argocd-secret
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-secret
    app.kubernetes.io/part-of: argocd
type: Opaque
data:
  # bcrypt hash of "argocd-admin" — 로컬 어드민 로그인용 (스모크에서는 미사용)
  admin.password: JDJhJDEwJHlnOVEuTjRTUy9nanF4UlJzMlhnLy5JRjZhMnRXZFkzRm5hY0NkMTc1LjVwdjhKTWx2SlNp
  admin.passwordMtime: $(python3 -c "import base64,datetime; print(base64.b64encode(datetime.datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ').encode()).decode())")
  # argocd-cm oidc.config 의 \$oidc.keycloak.clientSecret 참조 값
  oidc.keycloak.clientSecret: YXJnb2NkLWRldi1zZWNyZXQ=
  server.secretkey: ${SERVER_SECRET_B64}
EOF

log "K8s 리소스 적용 완료"

# ── 2. 컨테이너 기동 ──────────────────────────────────────────────────────────
log "docker compose up (dev + sso-argocd overlay)..."
docker compose \
  -f "${REPO_ROOT}/docker-compose.dev.yaml" \
  -f "${REPO_ROOT}/docker-compose.sso-argocd.yaml" \
  up -d

# ── 3. Keycloak health 대기 ────────────────────────────────────────────────────
log "Keycloak health 대기 (최대 120s)..."
WAIT=0
until curl -sf "${KC_URL}/realms/master" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 120 ]] && fail "Keycloak 기동 타임아웃"
done
log "Keycloak OK (${WAIT}s)"

# ── 4. ArgoCD health 대기 ──────────────────────────────────────────────────────
log "ArgoCD health 대기 (최대 120s)..."
WAIT=0
until curl -sf "${ARGOCD_URL}/healthz" >/dev/null 2>&1; do
  sleep 3; WAIT=$((WAIT+3))
  [[ $WAIT -ge 120 ]] && {
    log "ArgoCD 로그 (마지막 50줄):"
    docker compose \
      -f "${REPO_ROOT}/docker-compose.dev.yaml" \
      -f "${REPO_ROOT}/docker-compose.sso-argocd.yaml" \
      logs argocd-server 2>&1 | tail -50
    fail "ArgoCD 기동 타임아웃"
  }
done
log "ArgoCD OK (${WAIT}s)"

# ── 5. Keycloak realm + argocd client + 테스트 유저 등록 ─────────────────────
log "setup-keycloak.sh 실행..."
KEYCLOAK_URL="${KC_URL}" \
KEYCLOAK_ADMIN_USER=admin \
KEYCLOAK_ADMIN_PASSWORD=admin \
ARGOCD_CLIENT_SECRET="${ARGOCD_CLIENT_SECRET:-argocd-dev-secret}" \
  bash "${REPO_ROOT}/scripts/setup-keycloak.sh"
log "Keycloak 프로비저닝 완료"

# ── 5b. Admin token ───────────────────────────────────────────────────────────
ADMIN_TOKEN=$(curl -sS -X POST "${KC_URL}/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=password" \
  --data-urlencode "client_id=admin-cli" \
  --data-urlencode "username=admin" \
  --data-urlencode "password=admin" \
  | python3 -c "import json,sys; print(json.loads(sys.stdin.read()).get('access_token',''))")
[[ -z "$ADMIN_TOKEN" ]] && fail "Admin token 획득 실패"

# ── 5c. VERIFY_PROFILE / VERIFY_EMAIL 비활성화 (smoke-sso.sh 검증된 블록 차용) ─
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

# ── 5d. 테스트 유저 프로필 완성 + requiredActions 클리어 ─────────────────────
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

# ── 6. Keycloak 브라우저 로그인 시뮬레이션 → SSO 쿠키 확보 ──────────────────
# ArgoCD redirect_uri(http://localhost:8081/auth/callback) 기준으로 KC 세션 생성
log "Keycloak 로그인 폼 POST (cookie jar, argocd client)..."
ENC_REDIRECT=$(python3 -c \
  "import urllib.parse; print(urllib.parse.quote('http://localhost:8081/auth/callback'))")
AUTH_URL="${KC_URL}/realms/${REALM}/protocol/openid-connect/auth"
AUTH_PARAMS="client_id=argocd&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=openid&state=smokeargocdtest"

# GET: 로그인 폼 + KC 세션 쿠키 수신
LOGIN_HTML=$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "${AUTH_URL}?${AUTH_PARAMS}")

# 폼 action 추출 (smoke-sso.sh 패턴 동일)
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

log "Cookie jar 내용 (KC 세션 쿠키 확인):"
grep -v "^#\|^$" "$COOKIE_JAR" | awk '{print $6, $7}' || true

# ── 7. ArgoCD OIDC 자동로그인 체인 확인 ──────────────────────────────────────
# ArgoCD /auth/login → (302) KC auth → (KC session 존재 → 302 코드 발급) → ArgoCD /auth/callback → 세션 토큰
log "ArgoCD OIDC 로그인 흐름 테스트 (KC session cookie 활용)..."

ARGOCD_FLOW=$(curl -sS -L \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  --max-redirs 20 \
  -D "$REDIRECT_LOG" \
  -o /dev/null \
  -w "%{http_code} %{url_effective}" \
  "${ARGOCD_URL}/auth/login")

FINAL_CODE=$(echo "$ARGOCD_FLOW" | awk '{print $1}')
FINAL_URL=$(echo "$ARGOCD_FLOW" | awk '{print $2}')
log "최종 응답: HTTP ${FINAL_CODE}  URL: ${FINAL_URL}"

echo "── Redirect 체인 ───────────────────────────────────────────────"
grep -E "^HTTP|^[Ll]ocation:" "$REDIRECT_LOG" | head -50 || true
echo "────────────────────────────────────────────────────────────────"

# redirect 체인에서 KC 로그인 폼 재출현 여부 확인 (재인증 = FAIL 조건)
if grep -qi "action=\"${KC_URL}/realms/${REALM}/login-actions/authenticate" "$REDIRECT_LOG" 2>/dev/null; then
  log "ArgoCD 서버 로그 (마지막 30줄):"
  docker compose \
    -f "${REPO_ROOT}/docker-compose.dev.yaml" \
    -f "${REPO_ROOT}/docker-compose.sso-argocd.yaml" \
    logs argocd-server 2>&1 | tail -30
  fail "KC 로그인 폼 재출현 — 무재인증 핸드오프 실패"
fi

# ── 8. argocd.token 으로 /api/v1/session/userinfo 확인 ────────────────────────
log "GET ${ARGOCD_URL}/api/v1/session/userinfo ..."
API_RAW=$(curl -sS \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -w "\n__STATUS__%{http_code}" \
  "${ARGOCD_URL}/api/v1/session/userinfo")

API_CODE=$(echo "$API_RAW" | python3 -c \
  "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[-1].strip())")
API_BODY=$(echo "$API_RAW" | python3 -c \
  "import sys; t=sys.stdin.read(); print(t.split('__STATUS__')[0].rstrip())")

log "/api/v1/session/userinfo HTTP ${API_CODE}"
log "/api/v1/session/userinfo body: ${API_BODY}"

# ── 9. 결과 판정 ──────────────────────────────────────────────────────────────
LOGGED_IN=$(echo "$API_BODY" | python3 -c \
  "import json,sys; d=json.loads(sys.stdin.read()); print(str(d.get('loggedIn','false')).lower())" \
  2>/dev/null || echo "false")

if [[ "$API_CODE" == "200" && "$LOGGED_IN" == "true" ]]; then
  USERNAME=$(echo "$API_BODY" | python3 -c \
    "import json,sys; d=json.loads(sys.stdin.read()); print(d.get('username', d.get('name','(no name)')))" \
    2>/dev/null || echo "(parse error)")
  pass "ArgoCD SSO 핸드오프 성공 — 무재인증 로그인 완료. 사용자: ${USERNAME}"
  echo ""
  echo "컨테이너 정리 명령:"
  echo "  docker compose -f docker-compose.dev.yaml -f docker-compose.sso-argocd.yaml down"
  echo "  kubectl delete namespace argocd"
  exit 0
else
  log "ArgoCD 서버 로그 (마지막 50줄):"
  docker compose \
    -f "${REPO_ROOT}/docker-compose.dev.yaml" \
    -f "${REPO_ROOT}/docker-compose.sso-argocd.yaml" \
    logs argocd-server 2>&1 | tail -50
  fail "ArgoCD SSO 핸드오프 실패 — /api/v1/session/userinfo HTTP ${API_CODE}, loggedIn=${LOGGED_IN}. 위 redirect 체인과 서버 로그를 확인하라."
fi
