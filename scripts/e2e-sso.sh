#!/usr/bin/env bash
# SSO e2e 검증 스크립트
# 단일 쿠키 jar + --connect-to 로 *.nullus.internal:80 → 127.0.0.1:8080
# KC 1회 로그인 후 무재인증 SSO 확인
#
# 사전조건: kubectl port-forward ... 8080:80 이 실행 중이어야 함
# 사용법: bash scripts/e2e-sso.sh
set -euo pipefail

GW="127.0.0.1:8080"
USER="admin@nullus.io"
PASS="nullus123!"
JAR="$(mktemp)"
trap "rm -f $JAR" EXIT

HOSTS=(
  grafana.nullus.internal keycloak.nullus.internal
  argocd.nullus.internal harbor.nullus.internal
  prometheus.nullus.internal opensearch.nullus.internal
  gitlab.nullus.internal minio.nullus.internal
)

CONN_FLAGS=()
for h in "${HOSTS[@]}"; do CONN_FLAGS+=(--connect-to "${h}:80:${GW}"); done

pass() { echo "[PASS] $*"; }
fail() { echo "[FAIL] $*"; }
info() { echo "  >> $*"; }

# ── 1. Grafana OAuth로 KC 최초 로그인 ────────────────────────────────────────
echo "=== STEP 1: KC 최초 로그인 (Grafana OAuth) ==="
# 1-a Grafana /login/generic_oauth → KC redirect
GF_KC=$(curl -sS -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" -D - -o /dev/null \
  --connect-timeout 10 --max-time 30 \
  "http://grafana.nullus.internal/login/generic_oauth" 2>/dev/null | \
  grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
info "Grafana→KC: ${GF_KC:0:80}..."

# 1-b KC auth 페이지 로드 (폼 추출)
KC_PAGE=$(curl -sS -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" \
  --connect-timeout 10 --max-time 30 "$GF_KC" 2>/dev/null)
ACTION=$(echo "$KC_PAGE" | grep -oE 'action="[^"]*authenticate[^"]*"' | head -1 | \
  sed 's/action="//;s/"//' | sed 's/&amp;/\&/g')
info "KC action: ${ACTION:0:60}..."

# 1-c POST 자격증명
KC_CB=$(curl -sS -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" \
  --connect-timeout 10 --max-time 30 -D - -o /dev/null \
  -X POST "$ACTION" \
  --data-urlencode "username=$USER" --data-urlencode "password=$PASS" \
  -H "Content-Type: application/x-www-form-urlencoded" 2>/dev/null | \
  grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
info "KC→Grafana callback: ${KC_CB:0:80}..."

# KC 세션 확인
if ! grep -q 'KEYCLOAK_SESSION' "$JAR" 2>/dev/null; then
  fail "KEYCLOAK_SESSION 미획득, 중단"
  exit 1
fi
info "KEYCLOAK_SESSION: $(grep KEYCLOAK_SESSION $JAR | awk '{print $NF}' | head -c 20)..."

# 1-d Grafana 콜백 완주
curl -sS -L -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" \
  --max-redirs 10 --connect-timeout 10 --max-time 30 \
  "$KC_CB" > /dev/null 2>&1 || true

# 1-e Grafana 세션 확인
GF_USER=$(curl -sS -L -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" \
  --connect-timeout 10 --max-time 30 -w '\n__S__%{http_code}' \
  "http://grafana.nullus.internal/api/user" 2>/dev/null)
GF_S=$(echo "$GF_USER" | grep '__S__' | sed 's/.*__S__//')
GF_B=$(echo "$GF_USER" | grep -v '__S__')
info "Grafana /api/user: $GF_S | ${GF_B:0:200}"
if [[ "$GF_S" == "200" ]] && echo "$GF_B" | grep -q 'admin@nullus.io'; then
  pass "Grafana — 무재인증 SSO OK (email: admin@nullus.io)"
else
  fail "Grafana — $GF_S | ${GF_B:0:100}"
fi

_curl_h() { curl -sS -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 -D - -o /dev/null "$@" 2>/dev/null; }
_curl_L() { curl -sS -L -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --max-redirs 15 --connect-timeout 10 --max-time 30 -w '\n__S__%{http_code}' "$@" 2>/dev/null; }

echo ""
echo "=== KC 세션 재사용 (무재인증 여부 확인) ==="

# ── 2. Harbor ────────────────────────────────────────────────────────────────
echo "--- Harbor ---"
H_KC=$(_curl_h "http://harbor.nullus.internal/c/oidc/login" | grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
H_CB=$(_curl_h "$H_KC" | grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
[ -n "$H_CB" ] && curl -sS -L -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 "$H_CB" > /dev/null 2>&1
HR=$(_curl_L "http://harbor.nullus.internal/api/v2.0/users/current")
HR_S=$(echo "$HR" | grep '__S__' | sed 's/.*__S__//'); HR_B=$(echo "$HR" | grep -v '__S__')
info "Harbor /api/v2.0/users/current: $HR_S | ${HR_B:0:100}"
if [[ "$HR_S" == "200" ]] && echo "$HR_B" | grep -q '"username"'; then
  pass "Harbor — 무재인증 SSO OK ($(echo $HR_B | grep -oE '"username":"[^"]*"'))"
else
  fail "Harbor — $HR_S | ${HR_B:0:100}"
fi

# ── 3. Prometheus ─────────────────────────────────────────────────────────────
echo "--- Prometheus ---"
PR=$(_curl_L "http://prometheus.nullus.internal/")
PR_S=$(echo "$PR" | grep '__S__' | sed 's/.*__S__//'); PR_B=$(echo "$PR" | grep -v '__S__')
info "HTTP: $PR_S | title: $(echo $PR_B | grep -oE '<title>[^<]*</title>')"
if [[ "$PR_S" == "200" ]] && echo "$PR_B" | grep -qi 'prometheus'; then
  pass "Prometheus — 무재인증 SSO OK (Prometheus UI, _oauth2_proxy 쿠키: $(grep -c _oauth2_proxy $JAR 2>/dev/null)개)"
else
  fail "Prometheus — $PR_S"
fi

# ── 4. OpenSearch ─────────────────────────────────────────────────────────────
echo "--- OpenSearch ---"
OS_ME=$(_curl_L "http://opensearch.nullus.internal/oauth2/userinfo")
OS_S=$(echo "$OS_ME" | grep '__S__' | sed 's/.*__S__//'); OS_B=$(echo "$OS_ME" | grep -v '__S__')
info "OpenSearch /oauth2/userinfo: $OS_S | ${OS_B:0:100}"
OS_API=$(_curl_L "http://opensearch.nullus.internal/")
OS_A_S=$(echo "$OS_API" | grep '__S__' | sed 's/.*__S__//'); OS_A_B=$(echo "$OS_API" | grep -v '__S__')
info "OpenSearch / : $OS_A_S | ${OS_A_B:0:80}"
if [[ "$OS_S" == "200" ]] && echo "$OS_B" | grep -q 'email'; then
  pass "OpenSearch — 무재인증 SSO OK (oauth2-proxy 인증됨: $(echo $OS_B | grep -oE '"email":"[^"]*"')); 단 backend=opensearch API:9200(Dashboards 미배포)"
else
  fail "OpenSearch — oauth2 userinfo $OS_S | ${OS_B:0:100}"
fi

# ── 5. ArgoCD ─────────────────────────────────────────────────────────────────
echo "--- ArgoCD ---"
AG_KC=$(_curl_h "http://argocd.nullus.internal/auth/login" | grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
AG_CB=$(_curl_h "$AG_KC" | grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
info "KC→ArgoCD callback: ${AG_CB:0:100}"
if echo "$AG_CB" | grep -q 'error'; then
  fail "ArgoCD — KC 오류: $AG_CB"
else
  [ -n "$AG_CB" ] && curl -sS -L -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 "$AG_CB" > /dev/null 2>&1
  AG=$(_curl_L "http://argocd.nullus.internal/api/v1/session/userinfo")
  AG_S=$(echo "$AG" | grep '__S__' | sed 's/.*__S__//'); AG_B=$(echo "$AG" | grep -v '__S__')
  info "ArgoCD /api/v1/session/userinfo: $AG_S | $AG_B"
  if [[ "$AG_S" == "200" ]] && echo "$AG_B" | grep -q '"loggedIn":true'; then
    pass "ArgoCD — 무재인증 SSO OK (loggedIn:true)"
  else
    fail "ArgoCD — $AG_S | $AG_B"
  fi
fi

# ── 6. GitLab ─────────────────────────────────────────────────────────────────
echo "--- GitLab ---"
GL_SIGNIN=$(curl -sS -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 "http://gitlab.nullus.internal/users/sign_in" 2>/dev/null)
GL_CSRF=$(echo "$GL_SIGNIN" | grep -oE '<meta name="csrf-token" content="[^"]*"' | head -1 | sed 's/.*content="//;s/"//' || true)
if [ -z "$GL_CSRF" ]; then
  GL_CSRF=$(echo "$GL_SIGNIN" | grep -oE 'name="authenticity_token" value="[^"]*"' | head -1 | sed 's/.*value="//;s/"//' || true)
fi

GL_LOC=$(curl -sS -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 -D - -o /dev/null \
  -X POST "http://gitlab.nullus.internal/users/auth/openid_connect" \
  --data-urlencode "authenticity_token=$GL_CSRF" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "X-CSRF-Token: $GL_CSRF" 2>/dev/null | grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')

info "GitLab OIDC → $GL_LOC"
if echo "$GL_LOC" | grep -q 'keycloak'; then
  GL_CB=$(_curl_h "$GL_LOC" | grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
  [ -n "$GL_CB" ] && curl -sS -L -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 "$GL_CB" > /dev/null 2>&1
  GL=$(_curl_L "http://gitlab.nullus.internal/api/v4/user")
  GL_S=$(echo "$GL" | grep '__S__' | sed 's/.*__S__//'); GL_B=$(echo "$GL" | grep -v '__S__')
  info "GitLab /api/v4/user: $GL_S | ${GL_B:0:100}"
  if [[ "$GL_S" == "200" ]] && echo "$GL_B" | grep -q '"username"'; then
    pass "GitLab — 무재인증 SSO OK"
  else
    fail "GitLab — $GL_S | ${GL_B:0:100}"
  fi
else
  fail "GitLab — OIDC→KC redirect 없음 (→ $GL_LOC). OIDC omniauth 미설정 또는 CSRF 토큰 누락"
fi

# ── 7. MinIO ──────────────────────────────────────────────────────────────────
echo "--- MinIO ---"
MINIO_R=$(curl -sS "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 \
  "http://minio.nullus.internal/api/v1/login" 2>/dev/null | grep -oE '"redirect":"[^"]*"' | sed 's/"redirect":"//;s/"//')
info "MinIO redirect: ${MINIO_R:0:100}..."
MC_CB=$(_curl_h "$MINIO_R" | grep -i '^location:' | tr -d '\r' | sed 's/[Ll]ocation: //')
info "KC→MinIO callback: ${MC_CB:0:100}"
if echo "$MC_CB" | grep -q 'error'; then
  fail "MinIO — KC 오류: $MC_CB"
else
  [ -n "$MC_CB" ] && curl -sS -L -b "$JAR" -c "$JAR" "${CONN_FLAGS[@]}" --connect-timeout 10 --max-time 30 "$MC_CB" > /dev/null 2>&1
  MS=$(_curl_L "http://minio.nullus.internal/api/v1/session")
  MS_S=$(echo "$MS" | grep '__S__' | sed 's/.*__S__//'); MS_B=$(echo "$MS" | grep -v '__S__')
  info "MinIO /api/v1/session: $MS_S | ${MS_B:0:100}"
  if [[ "$MS_S" == "200" ]]; then
    pass "MinIO — 무재인증 SSO OK"
  else
    fail "MinIO — $MS_S | ${MS_B:0:100}"
  fi
fi

echo ""
echo "쿠키 jar 도메인 목록:"
grep -vE '^#|^$' "$JAR" | awk '{print $1}' | sort | uniq
