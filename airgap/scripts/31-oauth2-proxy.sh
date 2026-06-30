#!/usr/bin/env bash
# 31-oauth2-proxy.sh
# Deploys oauth2-proxy in front of Prometheus and OpenSearch,
# gates both behind Keycloak OIDC SSO via the nullus-gateway HTTPRoutes.
# Idempotent: safe to re-run.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST_DIR="${REPO_ROOT}/deploy/k8s/oauth2-proxy"

REGISTRY="${REGISTRY:-localhost:5001}"
IMAGE_TAG="v7.8.2"
IMAGE="quay.io/oauth2-proxy/oauth2-proxy:${IMAGE_TAG}"
LOCAL_IMAGE="${REGISTRY}/oauth2-proxy/oauth2-proxy:${IMAGE_TAG}"

KC_NAMESPACE="nullus-auth"
KC_SVC="keycloak"
KC_LOCAL_PORT="18180"
KC_REALM="nullus"
KC_ADMIN_USER="${KC_ADMIN_USER:-admin}"
KC_ADMIN_PASS="${KC_ADMIN_PASS:-admin}"

KIND_CLUSTER="${KIND_CLUSTER:-nullus-airgap}"

log() { echo "[31-oauth2-proxy] $*"; }

# ─── 1. Image: pull → push local registry → (optionally kind load) ───────────
log "Checking image in local registry..."
if ! curl -sf "http://${REGISTRY}/v2/oauth2-proxy/oauth2-proxy/tags/list" \
    | grep -q "${IMAGE_TAG}"; then
  log "Image not in local registry — pulling ${IMAGE}..."
  docker pull --platform linux/amd64 "${IMAGE}"
  docker tag "${IMAGE}" "${LOCAL_IMAGE}"
  docker push "${LOCAL_IMAGE}"
  log "Pushed to ${LOCAL_IMAGE}"
else
  log "Image already in local registry, skipping pull."
fi

# ─── 2. Keycloak clients ─────────────────────────────────────────────────────
log "Starting Keycloak port-forward on :${KC_LOCAL_PORT}..."
kubectl port-forward -n "${KC_NAMESPACE}" "svc/${KC_SVC}" "${KC_LOCAL_PORT}:80" \
  &>/tmp/kc-pf-31.log &
KC_PF_PID=$!
trap 'kill ${KC_PF_PID} 2>/dev/null || true' EXIT INT TERM

# Wait for Keycloak to be reachable
for i in $(seq 1 20); do
  if curl -sf "http://localhost:${KC_LOCAL_PORT}/realms/${KC_REALM}" &>/dev/null; then
    break
  fi
  sleep 2
done

KC_TOKEN=$(curl -sf -X POST \
  "http://localhost:${KC_LOCAL_PORT}/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=admin-cli&username=${KC_ADMIN_USER}&password=${KC_ADMIN_PASS}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

create_kc_client() {
  local client_id="$1" secret="$2" redirect_uris="$3"
  # Check if client already exists
  CLIENT_UUID=$(curl -sf \
    "http://localhost:${KC_LOCAL_PORT}/admin/realms/${KC_REALM}/clients?clientId=${client_id}" \
    -H "Authorization: Bearer ${KC_TOKEN}" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['id'] if d else '')" 2>/dev/null || true)

  if [ -n "${CLIENT_UUID}" ]; then
    curl -sf -X PUT "http://localhost:${KC_LOCAL_PORT}/admin/realms/${KC_REALM}/clients/${CLIENT_UUID}" \
      -H "Authorization: Bearer ${KC_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{
        \"clientId\": \"${client_id}\",
        \"enabled\": true,
        \"protocol\": \"openid-connect\",
        \"publicClient\": false,
        \"secret\": \"${secret}\",
        \"redirectUris\": ${redirect_uris},
        \"webOrigins\": [\"*\"],
        \"standardFlowEnabled\": true,
        \"directAccessGrantsEnabled\": false
      }"
    log "Updated Keycloak client '${client_id}'"
  else
    curl -sf -X POST "http://localhost:${KC_LOCAL_PORT}/admin/realms/${KC_REALM}/clients" \
      -H "Authorization: Bearer ${KC_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{
        \"clientId\": \"${client_id}\",
        \"enabled\": true,
        \"protocol\": \"openid-connect\",
        \"publicClient\": false,
        \"secret\": \"${secret}\",
        \"redirectUris\": ${redirect_uris},
        \"webOrigins\": [\"*\"],
        \"standardFlowEnabled\": true,
        \"directAccessGrantsEnabled\": false
      }"
    log "Created Keycloak client '${client_id}'"
  fi
}

create_kc_client \
  "oauth2-proxy-prometheus" \
  "prometheus-proxy-secret-2026" \
  "[\"https://prometheus.nullus.internal/*\", \"https://prometheus.nullus.internal/oauth2/callback\", \"https://prometheus.nullus.internal:8443/*\", \"https://prometheus.nullus.internal:8443/oauth2/callback\"]"

create_kc_client \
  "oauth2-proxy-opensearch" \
  "opensearch-proxy-secret-2026" \
  "[\"https://opensearch.nullus.internal/*\", \"https://opensearch.nullus.internal/oauth2/callback\", \"https://opensearch.nullus.internal:8443/*\", \"https://opensearch.nullus.internal:8443/oauth2/callback\"]"

kill "${KC_PF_PID}" 2>/dev/null || true
trap - EXIT INT TERM

# ─── 3. Deploy manifests ──────────────────────────────────────────────────────
log "Applying oauth2-proxy manifests..."
kubectl apply -f "${MANIFEST_DIR}/prometheus-proxy.yaml"
kubectl apply -f "${MANIFEST_DIR}/opensearch-proxy.yaml"

# ─── 4. Repoint HTTPRoutes ────────────────────────────────────────────────────
log "Patching HTTPRoutes to point to oauth2-proxy services..."

PROM_BACKEND=$(kubectl get httproute prometheus-route -n nullus-monitoring \
  -o jsonpath='{.spec.rules[0].backendRefs[0].name}' 2>/dev/null || echo "")
if [ "${PROM_BACKEND}" != "oauth2-proxy-prometheus" ]; then
  kubectl patch httproute prometheus-route -n nullus-monitoring --type='json' -p='[
    {"op": "replace", "path": "/spec/rules/0/backendRefs/0/name", "value": "oauth2-proxy-prometheus"},
    {"op": "replace", "path": "/spec/rules/0/backendRefs/0/port", "value": 4180}
  ]'
  log "prometheus-route → oauth2-proxy-prometheus:4180"
else
  log "prometheus-route already points to oauth2-proxy-prometheus, skipping."
fi

OS_BACKEND=$(kubectl get httproute opensearch-route -n nullus \
  -o jsonpath='{.spec.rules[0].backendRefs[0].name}' 2>/dev/null || echo "")
if [ "${OS_BACKEND}" != "oauth2-proxy-opensearch" ]; then
  kubectl patch httproute opensearch-route -n nullus --type='json' -p='[
    {"op": "replace", "path": "/spec/rules/0/backendRefs/0/name", "value": "oauth2-proxy-opensearch"},
    {"op": "replace", "path": "/spec/rules/0/backendRefs/0/port", "value": 4180}
  ]'
  log "opensearch-route → oauth2-proxy-opensearch:4180"
else
  log "opensearch-route already points to oauth2-proxy-opensearch, skipping."
fi

# ─── 5. Wait for ready ───────────────────────────────────────────────────────
log "Waiting for oauth2-proxy pods to be ready..."
kubectl rollout status deployment/oauth2-proxy-prometheus -n nullus-monitoring --timeout=120s
kubectl rollout status deployment/oauth2-proxy-opensearch -n nullus --timeout=120s

# ─── 6. Quick smoke test via gateway NodePort ─────────────────────────────────
GATEWAY_SVC="envoy-nullus-nullus-gateway-b5828592"
GATEWAY_PORT="18088"

log "Port-forwarding gateway on :${GATEWAY_PORT} for smoke test..."
kubectl port-forward -n nullus "svc/${GATEWAY_SVC}" "${GATEWAY_PORT}:80" \
  &>/tmp/gw-pf-31.log &
GW_PF_PID=$!
sleep 3

check_302() {
  local host="$1" client_id="$2"
  HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" \
    -H "Host: ${host}" "http://127.0.0.1:${GATEWAY_PORT}/" 2>/dev/null || echo "ERR")
  LOCATION=$(curl -sS -D - -o /dev/null \
    -H "Host: ${host}" "http://127.0.0.1:${GATEWAY_PORT}/" 2>/dev/null \
    | grep -i "^location:" | tr -d '\r' || echo "")
  if [ "${HTTP_CODE}" = "302" ] && echo "${LOCATION}" | grep -q "${client_id}"; then
    log "PASS ${host}: ${HTTP_CODE} → ${LOCATION}"
  else
    log "WARN ${host}: HTTP=${HTTP_CODE} Location=${LOCATION}"
  fi
}

check_302 "prometheus.nullus.internal" "oauth2-proxy-prometheus"
check_302 "opensearch.nullus.internal" "oauth2-proxy-opensearch"

kill "${GW_PF_PID}" 2>/dev/null || true

log "Done."
log ""
log "Summary:"
log "  Image pulled : ${IMAGE} (tag ${IMAGE_TAG})"
log "  Local image  : ${LOCAL_IMAGE}"
log "  KC clients   : oauth2-proxy-prometheus, oauth2-proxy-opensearch"
log "  Manifests    : ${MANIFEST_DIR}/"
log "  Routes       : prometheus-route → oauth2-proxy-prometheus:4180"
log "               : opensearch-route → oauth2-proxy-opensearch:4180"
log ""
log "To test manually:"
log "  kubectl port-forward -n nullus svc/${GATEWAY_SVC} 8088:80 &"
log "  curl -sS -D - -H 'Host: prometheus.nullus.internal' http://127.0.0.1:8088/ -o /dev/null | grep location"
log "  curl -sS -D - -H 'Host: opensearch.nullus.internal' http://127.0.0.1:8088/ -o /dev/null | grep location"
