#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8180}"
ADMIN_USER="${KEYCLOAK_ADMIN_USER:-admin}"
ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin}"
REALM="nullus"
CLIENT_ID="nullus-app"
DEFAULT_PASSWORD="${KEYCLOAK_TEST_USER_PASSWORD:-nullus123!}"

json_get() {
  local json="$1"
  local key="$2"
  python3 -c 'import json,sys; data=json.loads(sys.argv[1]); print(data.get(sys.argv[2],""))' "$json" "$key"
}

get_admin_token() {
  local response
  response=$(curl -sS -X POST "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    --data-urlencode "grant_type=password" \
    --data-urlencode "client_id=admin-cli" \
    --data-urlencode "username=${ADMIN_USER}" \
    --data-urlencode "password=${ADMIN_PASSWORD}")

  local token
  token=$(json_get "$response" "access_token")
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
  python3 -c 'import json,sys; data=json.loads(sys.argv[1]); print(data[0]["id"] if data else "")' "$json"
}

ensure_realm() {
  local status
  status=$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${KEYCLOAK_URL}/admin/realms/${REALM}")

  if [[ "$status" == "200" ]]; then
    return
  fi

  local realm_payload
  realm_payload=$(cat <<'EOF'
{"realm":"nullus","enabled":true}
EOF
)
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
  "enabled": true,
  "emailVerified": true,
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
ensure_client

ensure_role admin
ensure_role devops
ensure_role developer

ensure_user_with_role admin@nullus.io admin
ensure_user_with_role devops@nullus.io devops
ensure_user_with_role dev@nullus.io developer

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
