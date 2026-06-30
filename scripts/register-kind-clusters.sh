#!/usr/bin/env bash
set -euo pipefail

API_BASE="${NULLUS_API:-http://localhost:8090}"
ORG_ID="${NULLUS_ORG_ID:-}"
CLUSTERS=("nullus-platform" "nullus-develop")

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[register]${NC} $*"; }
warn() { echo -e "${YELLOW}[register]${NC} $*"; }
err()  { echo -e "${RED}[register]${NC} $*" >&2; }

check_prerequisites() {
  for cmd in kind kubectl curl; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      err "$cmd is not installed"
      exit 1
    fi
  done

  if ! curl -fsS "$API_BASE/health" >/dev/null 2>&1; then
    err "Nullus API is not reachable at $API_BASE"
    err "Start the platform first: ./scripts/runbook_local.sh up"
    exit 1
  fi
}

get_org_id() {
  if [[ -n "$ORG_ID" ]]; then
    echo "$ORG_ID"
    return
  fi

  local org_id
  org_id=$(curl -fsS "$API_BASE/api/v1/admin/organization" 2>/dev/null \
    | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")

  if [[ -z "$org_id" ]]; then
    err "Failed to retrieve organization ID from API"
    exit 1
  fi
  echo "$org_id"
}

lookup_cluster_id() {
  local name="$1"
  curl -fsS "$API_BASE/api/v1/admin/clusters" 2>/dev/null \
    | python3 -c "
import sys, json
data = json.load(sys.stdin)
items = data if isinstance(data, list) else data.get('items', [])
for c in items:
    if c.get('name') == '$name':
        print(c.get('id', ''))
        break
" 2>/dev/null || true
}

register_cluster() {
  local cluster_name="$1" org_id="$2"
  local kind_name="${cluster_name#kind-}"

  if ! kind get clusters 2>/dev/null | grep -q "^${kind_name}$"; then
    warn "Kind cluster '$kind_name' does not exist — skipping"
    warn "  Create it first: kind create cluster --name $kind_name --config scripts/kind-cluster.yaml"
    return 1
  fi

  local context="kind-${kind_name}"

  local endpoint kubeconfig
  endpoint=$(kubectl config view --context "$context" --minify --raw \
    -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null)
  kubeconfig=$(kind get kubeconfig --name "$kind_name" 2>/dev/null)

  if [[ -z "$endpoint" || -z "$kubeconfig" ]]; then
    err "Failed to extract endpoint or kubeconfig for $context"
    return 1
  fi

  local cluster_type="pipeline"
  if [[ "$kind_name" == "nullus-develop" ]]; then
    cluster_type="target"
  fi

  local payload
  payload=$(echo "$kubeconfig" | python3 -c "
import json, sys
kc = sys.stdin.read()
print(json.dumps({
    'name': sys.argv[1],
    'type': sys.argv[2],
    'types': [sys.argv[2]],
    'cloud_provider': 'on_premise',
    'endpoint': sys.argv[3],
    'org_id': sys.argv[4],
    'kubeconfig': kc
}))
" "kind-${kind_name}" "${cluster_type}" "${endpoint}" "${org_id}")

  local cluster_id method path response
  cluster_id=$(lookup_cluster_id "$context")
  if [[ -n "$cluster_id" ]]; then
    method="PATCH"
    path="$API_BASE/api/v1/admin/clusters/$cluster_id"
    log "Updating existing $context (id=$cluster_id)..."
  else
    method="POST"
    path="$API_BASE/api/v1/admin/clusters"
    log "Registering $context (type=$cluster_type)..."
  fi

  local response
  response=$(curl -sS -w "\n%{http_code}" -X "$method" "$path" \
    -H "Content-Type: application/json" \
    -d "$payload")

  local http_code body
  http_code=$(echo "$response" | tail -1)
  body=$(echo "$response" | sed '$d')

  if [[ "$http_code" == "201" || "$http_code" == "200" ]]; then
    if [[ -z "$cluster_id" ]]; then
      cluster_id=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
    fi
    if [[ "$http_code" == "201" ]]; then
      log "$context registered successfully (id=$cluster_id)"
    else
      log "$context updated successfully (id=$cluster_id)"
    fi

    log "Verifying connection for $context..."
    local verify_code
    verify_code=$(curl -sS -o /dev/null -w "%{http_code}" \
      -X POST "$API_BASE/api/v1/admin/clusters/$cluster_id/verify" 2>/dev/null)

    if [[ "$verify_code" == "200" ]]; then
      log "$context connection verified"
    else
      warn "$context verification returned HTTP $verify_code (cluster may still work)"
    fi
  else
    err "Failed to register $context (HTTP $http_code)"
    err "$body"
    return 1
  fi
}

main() {
  echo -e "${BOLD}Nullus — Kind Cluster Registration${NC}"
  echo ""

  check_prerequisites

  local org_id
  org_id=$(get_org_id)
  log "Organization ID: $org_id"
  echo ""

  local success=0 failed=0
  for cluster in "${CLUSTERS[@]}"; do
    if register_cluster "$cluster" "$org_id"; then
      ((success++)) || true
    else
      ((failed++)) || true
    fi
    echo ""
  done

  echo -e "${BOLD}Result: ${GREEN}$success registered${NC}, ${RED}$failed failed${NC}"
}

main "$@"
