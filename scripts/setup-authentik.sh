#!/usr/bin/env bash
# Authentik 초기 설정 스크립트
# Nullus 플랫폼용 Application, OAuth2 Provider, 그룹, 테스트 사용자를 자동 생성합니다.
#
# 사전 조건:
#   docker compose -f docker-compose.dev.yaml -f docker-compose.auth.yaml up -d
#   Authentik이 http://localhost:9090 에서 응답할 때까지 대기
#
# 사용법:
#   ./scripts/setup-authentik.sh

set -euo pipefail

AUTHENTIK_URL="${AUTHENTIK_URL:-http://localhost:9090}"
API_TOKEN="${AUTHENTIK_BOOTSTRAP_TOKEN:-nullus-authentik-bootstrap-token}"
DEFAULT_PASSWORD="${AUTHENTIK_TEST_USER_PASSWORD:-nullus123!}"

# ─── helpers ───

api_get() {
  local path="$1"
  curl -sS -H "Authorization: Bearer ${API_TOKEN}" "${AUTHENTIK_URL}/api/v3${path}"
}

api_post() {
  local path="$1" body="$2"
  curl -sS -X POST \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$body" "${AUTHENTIK_URL}/api/v3${path}"
}

api_patch() {
  local path="$1" body="$2"
  curl -sS -X PATCH \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$body" "${AUTHENTIK_URL}/api/v3${path}"
}

json_get() {
  python3 -c 'import json,sys; data=json.loads(sys.argv[1]); print(data.get(sys.argv[2],""))' "$1" "$2"
}

json_get_nested() {
  python3 -c '
import json,sys
data=json.loads(sys.argv[1])
results=data.get("results",[])
print(results[0]["pk"] if results else "")
' "$1"
}

wait_for_authentik() {
  echo "[authentik] waiting for Authentik at ${AUTHENTIK_URL} ..."
  local i
  for ((i = 1; i <= 60; i++)); do
    if curl -fsS "${AUTHENTIK_URL}/-/health/ready/" >/dev/null 2>&1; then
      echo "[authentik] Authentik is ready"
      return 0
    fi
    sleep 3
  done
  echo "[authentik] ERROR: Authentik did not become ready within 180s"
  exit 1
}

# ─── setup functions ───

ensure_certificate_keypair() {
  # OAuth2 Provider에 필요한 signing keypair 확인
  local result
  result=$(api_get "/crypto/certificatekeypairs/?name=authentik%20Self-signed%20Certificate")
  local pk
  pk=$(json_get_nested "$result")
  if [[ -n "$pk" ]]; then
    echo "[authentik] signing keypair exists: $pk"
    SIGNING_KEY_PK="$pk"
    return
  fi

  echo "[authentik] creating self-signed keypair..."
  local resp
  resp=$(api_post "/crypto/certificatekeypairs/generate/" '{"common_name":"authentik Self-signed Certificate","validity_days":365}')
  SIGNING_KEY_PK=$(json_get "$resp" "pk")
  echo "[authentik] keypair created: $SIGNING_KEY_PK"
}

ensure_scope_mapping() {
  # groups 스코프 매핑이 있는지 확인 (Authentik 기본 제공)
  local result
  result=$(api_get "/propertymappings/scope/?scope_name=profile")
  PROFILE_SCOPE_PK=$(json_get_nested "$result")

  result=$(api_get "/propertymappings/scope/?scope_name=email")
  EMAIL_SCOPE_PK=$(json_get_nested "$result")

  result=$(api_get "/propertymappings/scope/?scope_name=openid")
  OPENID_SCOPE_PK=$(json_get_nested "$result")

  echo "[authentik] scope mappings: openid=$OPENID_SCOPE_PK, profile=$PROFILE_SCOPE_PK, email=$EMAIL_SCOPE_PK"
}

ensure_oauth2_provider() {
  local result
  result=$(api_get "/providers/oauth2/?name=nullus-oauth2")
  local pk
  pk=$(json_get_nested "$result")

  if [[ -n "$pk" ]]; then
    echo "[authentik] OAuth2 provider exists: $pk"
    PROVIDER_PK="$pk"
    return
  fi

  echo "[authentik] creating OAuth2 provider..."
  local payload
  payload=$(cat <<EOF
{
  "name": "nullus-oauth2",
  "authorization_flow": "",
  "client_type": "public",
  "client_id": "nullus-app",
  "redirect_uris": "http://localhost:5173/*",
  "signing_key": "${SIGNING_KEY_PK}",
  "property_mappings": ["${OPENID_SCOPE_PK}", "${PROFILE_SCOPE_PK}", "${EMAIL_SCOPE_PK}"],
  "include_claims_in_id_token": true,
  "sub_mode": "user_email"
}
EOF
)

  # authorization_flow를 동적으로 조회
  local flow_result
  flow_result=$(api_get "/flows/instances/?designation=authorization&ordering=slug")
  local flow_pk
  flow_pk=$(json_get_nested "$flow_result")

  payload=$(python3 -c "
import json,sys
data=json.loads(sys.argv[1])
data['authorization_flow']=sys.argv[2]
print(json.dumps(data))
" "$payload" "$flow_pk")

  local resp
  resp=$(api_post "/providers/oauth2/" "$payload")
  PROVIDER_PK=$(json_get "$resp" "pk")
  echo "[authentik] OAuth2 provider created: $PROVIDER_PK"
}

ensure_application() {
  local result
  result=$(api_get "/core/applications/?slug=nullus")
  local pk
  pk=$(json_get_nested "$result")

  if [[ -n "$pk" ]]; then
    echo "[authentik] application exists: $pk"
    return
  fi

  echo "[authentik] creating application..."
  local payload
  payload=$(cat <<EOF
{
  "name": "Nullus Platform",
  "slug": "nullus",
  "provider": ${PROVIDER_PK},
  "meta_launch_url": "http://localhost:5173"
}
EOF
)
  api_post "/core/applications/" "$payload" >/dev/null
  echo "[authentik] application 'nullus' created"
}

ensure_group() {
  local name="$1"
  local result
  result=$(api_get "/core/groups/?name=${name}")
  local pk
  pk=$(json_get_nested "$result")

  if [[ -n "$pk" ]]; then
    echo "[authentik] group '${name}' exists: $pk"
    eval "GROUP_${name^^}_PK=$pk"
    return
  fi

  echo "[authentik] creating group '${name}'..."
  local resp
  resp=$(api_post "/core/groups/" "{\"name\":\"${name}\"}")
  pk=$(json_get "$resp" "pk")
  eval "GROUP_${name^^}_PK=$pk"
  echo "[authentik] group '${name}' created: $pk"
}

ensure_user_in_group() {
  local username="$1" email="$2" group_name="$3"

  local group_pk_var="GROUP_${group_name^^}_PK"
  local group_pk="${!group_pk_var}"

  local result
  result=$(api_get "/core/users/?username=${username}")
  local user_pk
  user_pk=$(json_get_nested "$result")

  if [[ -z "$user_pk" ]]; then
    echo "[authentik] creating user '${email}'..."
    local resp
    resp=$(api_post "/core/users/" "{
      \"username\": \"${username}\",
      \"email\": \"${email}\",
      \"name\": \"${username}\",
      \"is_active\": true,
      \"groups\": [\"${group_pk}\"]
    }")
    user_pk=$(json_get "$resp" "pk")

    # set password
    api_post "/core/users/${user_pk}/set_password/" "{\"password\":\"${DEFAULT_PASSWORD}\"}" >/dev/null
    echo "[authentik] user '${email}' created with role '${group_name}'"
  else
    # 기존 사용자에 그룹 추가
    api_patch "/core/users/${user_pk}/" "{\"groups\":[\"${group_pk}\"]}" >/dev/null
    echo "[authentik] user '${email}' updated with group '${group_name}'"
  fi
}

# ─── main ───

wait_for_authentik

echo ""
echo "[authentik] configuring Nullus platform..."
echo ""

ensure_certificate_keypair
ensure_scope_mapping
ensure_oauth2_provider
ensure_application

ensure_group admin
ensure_group devops
ensure_group developer

ensure_user_in_group "admin"     "admin@nullus.io"  "admin"
ensure_user_in_group "devops"    "devops@nullus.io"  "devops"
ensure_user_in_group "developer" "dev@nullus.io"     "developer"

echo ""
echo "════════════════════════════════════════════════════"
echo "  Authentik configured for Nullus Platform"
echo "════════════════════════════════════════════════════"
echo ""
echo "  Admin Console : ${AUTHENTIK_URL}/if/admin/"
echo "  Login         : admin@nullus.io / ${DEFAULT_PASSWORD}"
echo ""
echo "  OAuth2 Provider:"
echo "    Client ID   : nullus-app"
echo "    Issuer      : ${AUTHENTIK_URL}/application/o/nullus/"
echo "    Auth URL    : ${AUTHENTIK_URL}/application/o/authorize/"
echo "    Token URL   : ${AUTHENTIK_URL}/application/o/token/"
echo "    JWKS        : ${AUTHENTIK_URL}/application/o/nullus/jwks/"
echo ""
echo "  Test Accounts:"
echo "    admin@nullus.io     / ${DEFAULT_PASSWORD}  (admin group)"
echo "    devops@nullus.io    / ${DEFAULT_PASSWORD}  (devops group)"
echo "    dev@nullus.io       / ${DEFAULT_PASSWORD}  (developer group)"
echo ""
echo "  To switch backend to Authentik mode:"
echo "    cp configs/config.authentik.yaml configs/config.yaml"
echo "    make run"
echo "════════════════════════════════════════════════════"
