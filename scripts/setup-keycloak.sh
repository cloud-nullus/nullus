#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8180}"
ADMIN_USER="${KEYCLOAK_ADMIN_USER:-admin}"
ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin}"
REALM="nullus"
CLIENT_ID="nullus-app"
AUDIENCE_MAPPER_NAME="nullus-app-audience"
ORG_MAPPER_NAME="nullus-org-id"
# 시드 마이그레이션이 만드는 조직 (internal/shared/domain.SeededDefaultOrgID 와 동일)
ORG_ID="${NULLUS_DEFAULT_ORG_ID:-11111111-1111-1111-1111-111111111111}"
DEFAULT_PASSWORD="${KEYCLOAK_TEST_USER_PASSWORD:-nullus123!}"

# 응답이 비었거나 JSON 이 아닐 수 있다(예: 인증 실패로 본문이 없는 경우).
# 그럴 때는 빈 문자열을 돌려주고 호출측이 판단하게 한다.
json_get() {
  local json="$1"
  local key="$2"
  python3 -c '
import json,sys
try:
    data = json.loads(sys.argv[1])
except Exception:
    print("")
else:
    print(data.get(sys.argv[2], "") if isinstance(data, dict) else "")
' "$json" "$key"
}

request_admin_token() {
  local response
  response=$(curl -sS -X POST "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    --data-urlencode "grant_type=password" \
    --data-urlencode "client_id=admin-cli" \
    --data-urlencode "username=${ADMIN_USER}" \
    --data-urlencode "password=${ADMIN_PASSWORD}" 2>/dev/null || true)
  json_get "$response" "access_token" 2>/dev/null || true
}

# master realm 의 sslRequired 기본값은 "external" 이라, 도커 포트 매핑을 통해
# 들어온 평문 HTTP 요청은 'HTTPS required' 로 거부된다. 컨테이너 안에서는
# localhost 라 예외가 적용되므로 kcadm 으로 한 번만 낮춰 준다.
relax_master_ssl_required() {
  local container="${KEYCLOAK_CONTAINER:-draft-keycloak-1}"
  if ! command -v docker >/dev/null 2>&1; then
    return 1
  fi
  if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$container"; then
    return 1
  fi

  # get_admin_token 이 명령 치환 안에서 호출되므로 진행 로그는 stderr 로 보낸다.
  # stdout 으로 내보내면 토큰 문자열에 섞여 Authorization 헤더가 깨진다.
  echo "  [keycloak] master realm 의 sslRequired 를 NONE 으로 낮춥니다 (${container})" >&2
  docker exec "$container" /opt/keycloak/bin/kcadm.sh config credentials \
    --server http://localhost:8080 --realm master \
    --user "${ADMIN_USER}" --password "${ADMIN_PASSWORD}" >/dev/null 2>&1 || return 1
  docker exec "$container" /opt/keycloak/bin/kcadm.sh update realms/master \
    -s sslRequired=NONE >/dev/null 2>&1 || return 1
}

get_admin_token() {
  local token
  token=$(request_admin_token)

  # 'HTTPS required' 로 막힌 경우 한 번 완화하고 재시도한다.
  if [[ -z "$token" ]] && relax_master_ssl_required; then
    token=$(request_admin_token)
  fi

  if [[ -z "$token" ]]; then
    echo "failed to obtain admin token" >&2
    exit 1
  fi
  printf '%s' "$token"
}

auth_get() {
  local path="$1"
  curl -sS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${KEYCLOAK_URL}${path}"
}

auth_post() {
  local path="$1"
  local body="$2"
  curl -sS -o /dev/null -w "%{http_code}" -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$body" "${KEYCLOAK_URL}${path}"
}

auth_put() {
  local path="$1"
  local body="$2"
  curl -sS -o /dev/null -w "%{http_code}" -X PUT \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$body" "${KEYCLOAK_URL}${path}"
}

lookup_first_id() {
  local json="$1"
  python3 -c '
import json,sys
try:
    data = json.loads(sys.argv[1])
except Exception:
    print("")
else:
    print(data[0]["id"] if isinstance(data, list) and data else "")
' "$json"
}

ensure_realm() {
  local status
  status=$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${KEYCLOAK_URL}/admin/realms/${REALM}")

  # sslRequired=NONE 이어야 평문 HTTP 로 토큰 발급이 된다 (로컬 개발 전용).
  local realm_payload
  realm_payload=$(cat <<'EOF'
{"realm":"nullus","enabled":true,"sslRequired":"none"}
EOF
)

  if [[ "$status" == "200" ]]; then
    auth_put "/admin/realms/${REALM}" "$realm_payload" >/dev/null
    return
  fi

  auth_post "/admin/realms" "$realm_payload" >/dev/null
}

ensure_client() {
  local clients_json
  clients_json=$(auth_get "/admin/realms/${REALM}/clients?clientId=${CLIENT_ID}")
  local client_id
  client_id=$(lookup_first_id "$clients_json")

  local payload
  payload=$(cat <<'EOF'
{
  "clientId": "nullus-app",
  "enabled": true,
  "publicClient": true,
  "standardFlowEnabled": true,
  "directAccessGrantsEnabled": true,
  "attributes": {
    "pkce.code.challenge.method": "S256"
  },
  "redirectUris": [
    "http://localhost:5173/*"
  ],
  "webOrigins": [
    "http://localhost:5173"
  ]
}
EOF
)

  if [[ -n "$client_id" ]]; then
    auth_put "/admin/realms/${REALM}/clients/${client_id}" "$payload" >/dev/null
  else
    auth_post "/admin/realms/${REALM}/clients" "$payload" >/dev/null
  fi
}

# Keycloak 은 기본적으로 aud=account 로 토큰을 발급한다. API 의 JWT 미들웨어는
# jwt.WithAudience(NULLUS_AUTH_OIDC_AUDIENCE) 로 aud 를 검증하므로, audience
# 매퍼가 없으면 정상 로그인해도 401 로 거부된다.
ensure_audience_mapper() {
  local clients_json client_id mappers_json existing
  clients_json=$(auth_get "/admin/realms/${REALM}/clients?clientId=${CLIENT_ID}")
  client_id=$(lookup_first_id "$clients_json")
  if [[ -z "$client_id" ]]; then
    echo "client ${CLIENT_ID} not found; cannot add audience mapper" >&2
    exit 1
  fi

  mappers_json=$(auth_get "/admin/realms/${REALM}/clients/${client_id}/protocol-mappers/models")
  existing=$(python3 -c '
import json,sys
try:
    mappers = json.loads(sys.argv[1])
except Exception:
    mappers = []
for m in mappers:
    if m.get("name") == sys.argv[2]:
        print(m.get("id",""))
        break
' "$mappers_json" "${AUDIENCE_MAPPER_NAME}")

  local payload
  payload=$(python3 -c '
import json,sys
print(json.dumps({
  "name": sys.argv[1],
  "protocol": "openid-connect",
  "protocolMapper": "oidc-audience-mapper",
  "config": {
    "included.client.audience": sys.argv[2],
    "access.token.claim": "true",
    "id.token.claim": "false",
  },
}))
' "${AUDIENCE_MAPPER_NAME}" "${CLIENT_ID}")

  if [[ -n "$existing" ]]; then
    auth_put "/admin/realms/${REALM}/clients/${client_id}/protocol-mappers/models/${existing}" "$payload" >/dev/null
  else
    auth_post "/admin/realms/${REALM}/clients/${client_id}/protocol-mappers/models" "$payload" >/dev/null
  fi
}

# Keycloak 24+ 의 User Profile 은 선언되지 않은 사용자 속성을 조용히 버린다.
# org_id 를 선언해 두지 않으면 유저에 속성이 저장되지 않고(attributes: null),
# 프로필이 불완전해져 로그인이 'Account is not fully set up' 으로 실패한다.
ensure_user_profile_org_attribute() {
  local profile_json updated
  profile_json=$(auth_get "/admin/realms/${REALM}/users/profile")

  updated=$(python3 -c '
import json,sys
profile = json.loads(sys.argv[1])
name = sys.argv[2]
attrs = profile.setdefault("attributes", [])
if not any(a.get("name") == name for a in attrs):
    attrs.append({
        "name": name,
        "displayName": "Organization ID",
        "multivalued": False,
        "permissions": {"view": ["admin", "user"], "edit": ["admin"]},
        # 관리자만 채우는 값이므로 필수로 두지 않는다 — 필수로 두면
        # 값이 없는 기존 유저가 로그인하지 못한다.
        "validations": {},
    })
print(json.dumps(profile))
' "$profile_json" "org_id")

  auth_put "/admin/realms/${REALM}/users/profile" "$updated" >/dev/null
}

# 사용자 속성 org_id 를 액세스 토큰 클레임으로 실어 보낸다.
# 이 클레임이 없으면 API 가 조직을 알 수 없어 기본 조직으로 폴백하고,
# 스택 생성이 stacks_org_id_fkey FK 위반으로 실패한다.
ensure_org_claim_mapper() {
  local clients_json client_id mappers_json existing
  clients_json=$(auth_get "/admin/realms/${REALM}/clients?clientId=${CLIENT_ID}")
  client_id=$(lookup_first_id "$clients_json")
  if [[ -z "$client_id" ]]; then
    echo "client ${CLIENT_ID} not found; cannot add org claim mapper" >&2
    exit 1
  fi

  mappers_json=$(auth_get "/admin/realms/${REALM}/clients/${client_id}/protocol-mappers/models")
  existing=$(python3 -c '
import json,sys
try:
    mappers = json.loads(sys.argv[1])
except Exception:
    mappers = []
for m in mappers:
    if m.get("name") == sys.argv[2]:
        print(m.get("id",""))
        break
' "$mappers_json" "${ORG_MAPPER_NAME}")

  local payload
  payload=$(python3 -c '
import json,sys
print(json.dumps({
  "name": sys.argv[1],
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "config": {
    "user.attribute": "org_id",
    "claim.name": "org_id",
    "jsonType.label": "String",
    "access.token.claim": "true",
    "id.token.claim": "true",
    "userinfo.token.claim": "true",
    "multivalued": "false",
  },
}))
' "${ORG_MAPPER_NAME}")

  if [[ -n "$existing" ]]; then
    auth_put "/admin/realms/${REALM}/clients/${client_id}/protocol-mappers/models/${existing}" "$payload" >/dev/null
  else
    auth_post "/admin/realms/${REALM}/clients/${client_id}/protocol-mappers/models" "$payload" >/dev/null
  fi
}

ensure_role() {
  local role="$1"
  local status
  status=$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${KEYCLOAK_URL}/admin/realms/${REALM}/roles/${role}")
  if [[ "$status" == "200" ]]; then
    return
  fi
  auth_post "/admin/realms/${REALM}/roles" "{\"name\":\"${role}\"}" >/dev/null
}

urlencode() {
  python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

ensure_user_with_role() {
   local username="$1"
   local role="$2"
   local first_name="${3:-}"
   local last_name="${4:-}"

   local users_json
   users_json=$(auth_get "/admin/realms/${REALM}/users?username=$(urlencode "$username")")
   local user_id
   user_id=$(lookup_first_id "$users_json")

   if [[ -z "$user_id" ]]; then
     local user_payload
     user_payload=$(cat <<EOF
{
  "username": "${username}",
  "email": "${username}",
  "firstName": "${first_name}",
  "lastName": "${last_name}",
  "enabled": true,
  "emailVerified": true,
  "attributes": {
    "org_id": ["${ORG_ID}"]
  },
  "credentials": [
    {
      "type": "password",
      "value": "${DEFAULT_PASSWORD}",
      "temporary": false
    }
  ]
}
EOF
)
     auth_post "/admin/realms/${REALM}/users" "$user_payload" >/dev/null
     users_json=$(auth_get "/admin/realms/${REALM}/users?username=$(urlencode "$username")")
     user_id=$(lookup_first_id "$users_json")
   else
     # 기존 사용자 갱신. PUT 은 표현을 통째로 교체하므로 **유지해야 할 필드를 모두
     # 다시 보내야 한다**. email 을 빠뜨리면 재실행할 때마다 지워지고, API 는 토큰의
     # email 클레임으로 사용자를 찾으므로 로그인 후 조회가 깨진다.
     local update_payload
     update_payload=$(cat <<EOF
{
  "email": "${username}",
  "emailVerified": true,
  "enabled": true,
  "firstName": "${first_name}",
  "lastName": "${last_name}",
  "attributes": {
    "org_id": ["${ORG_ID}"]
  }
}
EOF
)
     auth_put "/admin/realms/${REALM}/users/${user_id}" "$update_payload" >/dev/null
   fi

   # Check if role is already assigned
   local user_roles_json
   user_roles_json=$(auth_get "/admin/realms/${REALM}/users/${user_id}/role-mappings/realm")
   if echo "$user_roles_json" | grep -q "\"name\":\"${role}\""; then
     return
   fi

   local role_json
   role_json=$(auth_get "/admin/realms/${REALM}/roles/${role}")
   local mapping_status
   mapping_status=$(auth_post "/admin/realms/${REALM}/users/${user_id}/role-mappings/realm" "[$role_json]")
   if [[ "$mapping_status" != "204" ]]; then
     echo "failed to assign role ${role} to ${username}" >&2
     exit 1
   fi
}

# Dev fixed client secrets — used by smoke tests and local OSS containers.
GRAFANA_SECRET="${GRAFANA_CLIENT_SECRET:-grafana-dev-secret}"
ARGOCD_SECRET="${ARGOCD_CLIENT_SECRET:-argocd-dev-secret}"
HARBOR_SECRET="${HARBOR_CLIENT_SECRET:-harbor-dev-secret}"

# ensure_oss_client <clientId> <secret> <redirect_uri_1> [<redirect_uri_2> ...]
# Creates or updates a confidential OIDC client for an OSS tool.
ensure_oss_client() {
  local client_id="$1"
  local secret="$2"
  shift 2
  local redirect_uris="$*"   # space-separated list

  # Build JSON array of redirect URIs
  local uris_json
  uris_json=$(python3 -c '
import sys, json
uris = sys.argv[1:]
print(json.dumps(uris))
' $redirect_uris)

  local clients_json
  clients_json=$(auth_get "/admin/realms/${REALM}/clients?clientId=$(urlencode "${client_id}")")
  local internal_id
  internal_id=$(lookup_first_id "$clients_json")

  local payload
  payload=$(python3 -c '
import sys, json
client_id, secret, uris_json = sys.argv[1], sys.argv[2], sys.argv[3]
uris = json.loads(uris_json)
print(json.dumps({
  "clientId": client_id,
  "enabled": True,
  "publicClient": False,
  "standardFlowEnabled": True,
  "directAccessGrantsEnabled": False,
  "secret": secret,
  "redirectUris": uris,
  "webOrigins": ["+"],
}))
' "${client_id}" "${secret}" "${uris_json}")

  if [[ -n "$internal_id" ]]; then
    auth_put "/admin/realms/${REALM}/clients/${internal_id}" "$payload" >/dev/null
  else
    auth_post "/admin/realms/${REALM}/clients" "$payload" >/dev/null
  fi

  echo "  [oss-client] ${client_id}: secret=${secret}"
}

ADMIN_TOKEN=$(get_admin_token)

ensure_realm
ensure_user_profile_org_attribute
ensure_client
ensure_audience_mapper
ensure_org_claim_mapper

ensure_role admin
ensure_role devops
ensure_role developer

ensure_user_with_role admin@nullus.io admin Admin User
ensure_user_with_role devops@nullus.io devops Devops User
ensure_user_with_role dev@nullus.io developer Dev User

echo "Registering OSS confidential clients (grafana / argocd / harbor)..."

ensure_oss_client "grafana" "${GRAFANA_SECRET}" \
  "http://localhost:3000/login/generic_oauth" \
  "https://grafana.nullus.local/login/generic_oauth"

ensure_oss_client "argocd" "${ARGOCD_SECRET}" \
  "http://localhost:8081/auth/callback" \
  "https://argocd.nullus.local/auth/callback"

ensure_oss_client "harbor" "${HARBOR_SECRET}" \
  "http://localhost:8082/c/oidc/callback" \
  "https://harbor.nullus.local/c/oidc/callback"

echo "Keycloak realm '${REALM}' configured."
