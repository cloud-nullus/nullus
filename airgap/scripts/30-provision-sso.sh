#!/usr/bin/env bash
# =============================================================================
# 30-provision-sso.sh — Nullus 플랫폼 Keycloak SSO 프로비저닝 (in-cluster)
# =============================================================================
# 목적: kind-nullus-airgap 클러스터에서 모든 스택(Grafana, ArgoCD, Harbor,
#       MinIO, GitLab)이 realm=nullus / issuer=http://keycloak.nullus.internal
#       를 통해 SSO 인증하도록 idempotent 하게 구성한다.
#
# 수행 작업:
#   1. CoreDNS — *.nullus.internal rewrite rule 패치
#   2. Keycloak — KC_HOSTNAME/KC_HOSTNAME_PORT env var helm upgrade
#   3. Keycloak — realm nullus + 5개 OIDC client 프로비저닝 (Admin REST API)
#   4. GitLab  — gitlab-oidc-provider k8s Secret 생성/갱신
#   5. Helm    — Grafana(kps), ArgoCD, MinIO values upgrade
#   6. Harbor  — Admin REST API로 oidc_auth 설정
#   7. 검증    — 각 앱 curl redirect 확인
#
# 사용법:
#   ./30-provision-sso.sh                        # 전체 실행
#   SKIP_VERIFY=1 ./30-provision-sso.sh          # 검증 단계 생략
#   KC_PORT_FWD=8180 ./30-provision-sso.sh       # 커스텀 KC port-forward 포트
#
# 환경 변수:
#   KC_ADMIN         Keycloak 관리자 계정   (기본: admin)
#   KC_ADMIN_PASS    Keycloak 관리자 비밀번호 (기본: admin)
#   KC_PORT_FWD      로컬 Keycloak port-forward 포트 (기본: 18180)
#   GW_PORT_FWD      Envoy Gateway 로컬 포트 (기본: 8088)
#   HARBOR_ADMIN_PASS Harbor 관리자 비밀번호 (기본: Harbor12345)
#   HARBOR_CORE_PORT Harbor-core 직접 port-forward 포트 (기본: 18085)
#   SKIP_VERIFY      1 이면 curl 검증 단계 생략
#
# 종료 코드: 0=성공, 1=실패
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
VALUES_DIR="${ROOT_DIR}/helm/stack-values"

KC_ADMIN="${KC_ADMIN:-admin}"
KC_ADMIN_PASS="${KC_ADMIN_PASS:-admin}"
KC_PORT_FWD="${KC_PORT_FWD:-18180}"
GW_PORT_FWD="${GW_PORT_FWD:-8088}"
HARBOR_ADMIN_PASS="${HARBOR_ADMIN_PASS:-Harbor12345}"
HARBOR_CORE_PORT="${HARBOR_CORE_PORT:-18085}"
SKIP_VERIFY="${SKIP_VERIFY:-0}"

REALM="nullus"
KC_BASE="http://keycloak.nullus.internal/realms/${REALM}"
KC_LOCAL="http://127.0.0.1:${KC_PORT_FWD}"

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_OK=$'\033[1;32m'; CL_WARN=$'\033[1;33m'
  CL_ERR=$'\033[1;31m';  CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_OK=""; CL_WARN=""; CL_ERR=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }

command -v kubectl >/dev/null || { log_err "kubectl not found"; exit 1; }
command -v helm    >/dev/null || { log_err "helm not found";    exit 1; }
command -v curl    >/dev/null || { log_err "curl not found";    exit 1; }
command -v python3 >/dev/null || { log_err "python3 not found"; exit 1; }

# PID 파일로 관리하는 port-forward 정리
_PF_PIDS=()
cleanup_pf() {
  for pid in "${_PF_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup_pf EXIT

start_pf() {
  local ns="$1" svc="$2" local_port="$3" remote_port="$4"
  # 이미 열려 있으면 재사용
  if lsof -i ":${local_port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
    log_info "port-forward :${local_port} already open, reusing"
    return 0
  fi
  kubectl port-forward -n "$ns" "svc/$svc" "${local_port}:${remote_port}" \
    &>/tmp/pf-${svc}-${local_port}.log &
  _PF_PIDS+=($!)
  # 최대 10초 대기
  local i=0
  while ! lsof -i ":${local_port}" -sTCP:LISTEN -t >/dev/null 2>&1; do
    sleep 1; i=$((i+1))
    [[ $i -ge 10 ]] && { log_err "port-forward ${svc}:${local_port} 실패"; return 1; }
  done
  log_ok "port-forward ${ns}/${svc} → :${local_port}"
}

# =============================================================================
# 1. CoreDNS — *.nullus.internal rewrite
# =============================================================================
patch_coredns() {
  log_info "── CoreDNS rewrite 패치 ──"

  COREFILE=$(kubectl get configmap coredns -n kube-system \
    -o jsonpath='{.data.Corefile}')

  # 이미 패치된 경우 건너뜀
  if echo "$COREFILE" | grep -q "rewrite name keycloak.nullus.internal"; then
    log_ok "CoreDNS rewrite 이미 적용됨 — 건너뜀"
    return 0
  fi

  REWRITE_BLOCK="        rewrite name keycloak.nullus.internal keycloak.nullus-auth.svc.cluster.local
        rewrite name grafana.nullus.internal kps-grafana.nullus-monitoring.svc.cluster.local
        rewrite name argocd.nullus.internal argo-cd-argocd-server.nullus.svc.cluster.local
        rewrite name harbor.nullus.internal harbor.nullus.svc.cluster.local
        rewrite name minio.nullus.internal nullus-minio-console.nullus.svc.cluster.local
        rewrite name gitlab.nullus.internal gitlab-webservice-default.gitlab.svc.cluster.local"

  NEW_COREFILE=$(echo "$COREFILE" | sed "s|forward . /etc/resolv.conf|${REWRITE_BLOCK}\\n        forward . /etc/resolv.conf|")

  kubectl patch configmap coredns -n kube-system --patch \
    "{\"data\":{\"Corefile\":$(echo "$NEW_COREFILE" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))')}}"

  kubectl rollout restart deployment/coredns -n kube-system
  kubectl rollout status deployment/coredns -n kube-system --timeout=60s >/dev/null
  log_ok "CoreDNS rewrite 패치 완료"
}

# =============================================================================
# 2. Keycloak — KC_HOSTNAME helm upgrade
# =============================================================================
upgrade_keycloak_hostname() {
  log_info "── Keycloak KC_HOSTNAME upgrade ──"

  local chart_dir="${VALUES_DIR}/keycloak.yaml"
  [[ -f "$chart_dir" ]] || { log_err "keycloak.yaml 없음: $chart_dir"; return 1; }

  # KC_HOSTNAME 이미 적용됐는지 확인
  if kubectl get deployment keycloak -n nullus-auth \
      -o jsonpath='{.spec.template.spec.containers[0].env}' 2>/dev/null | \
      grep -q "KC_HOSTNAME"; then
    log_ok "KC_HOSTNAME 이미 설정됨 — helm upgrade 건너뜀"
    return 0
  fi

  local chart
  chart="$(ls "${ROOT_DIR}/helm/charts-catalog"/keycloak-*.tgz 2>/dev/null | head -1 || true)"
  [[ -n "$chart" ]] || { log_err "keycloak chart .tgz 없음"; return 1; }

  helm upgrade keycloak "$chart" \
    -n nullus-auth \
    -f "$chart_dir" \
    --wait --timeout 5m \
    --reuse-values \
    2>&1 | tail -3

  log_ok "Keycloak KC_HOSTNAME upgrade 완료"
}

# =============================================================================
# 3. Keycloak — realm + 5 client 프로비저닝
# =============================================================================
provision_keycloak() {
  log_info "── Keycloak 프로비저닝 (realm + clients) ──"

  start_pf nullus-auth keycloak "$KC_PORT_FWD" 80

  # Admin token
  TOKEN=$(curl -s -X POST \
    "${KC_LOCAL}/realms/master/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "client_id=admin-cli&grant_type=password&username=${KC_ADMIN}&password=${KC_ADMIN_PASS}" | \
    python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')
  [[ -n "$TOKEN" ]] || { log_err "Keycloak admin token 획득 실패"; return 1; }

  KC_API="${KC_LOCAL}/admin/realms"

  # realm 생성 (idempotent)
  REALM_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" "${KC_API}/${REALM}")
  if [[ "$REALM_STATUS" == "200" ]]; then
    log_ok "realm '${REALM}' 이미 존재 — 건너뜀"
  else
    curl -s -X POST "${KC_API}" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"realm\":\"${REALM}\",\"enabled\":true,\"displayName\":\"Nullus\"}" \
      -o /dev/null
    log_ok "realm '${REALM}' 생성"
  fi

  # realm role: admin
  ROLE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" "${KC_API}/${REALM}/roles/admin")
  if [[ "$ROLE_STATUS" != "200" ]]; then
    curl -s -X POST "${KC_API}/${REALM}/roles" \
      -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
      -d '{"name":"admin"}' -o /dev/null
    log_ok "realm role 'admin' 생성"
  fi

  # 5개 confidential OIDC client 프로비저닝
  _provision_client "grafana"  "grafana-dev-secret"  \
    "[\"http://grafana.nullus.internal/login/generic_oauth\"]"

  _provision_client "argocd"   "argocd-dev-secret"   \
    "[\"http://argocd.nullus.internal/auth/callback\"]"

  _provision_client "harbor"   "harbor-dev-secret"   \
    "[\"http://harbor.nullus.internal/c/oidc/callback\"]"

  _provision_client "minio"    "minio-dev-secret"    \
    "[\"http://minio.nullus.internal/oauth_callback\"]"

  _provision_client "gitlab"   "gitlab-dev-secret"   \
    "[\"http://gitlab.nullus.internal/users/auth/openid_connect/callback\"]"

  log_ok "Keycloak 프로비저닝 완료"
}

_provision_client() {
  local cid="$1" secret="$2" redirect_uris="$3"
  local STATUS
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" "${KC_API}/${REALM}/clients?clientId=${cid}")

  # 클라이언트가 이미 있으면 건너뜀 (200 + 비어있지 않은 배열)
  EXISTING=$(curl -s -H "Authorization: Bearer $TOKEN" \
    "${KC_API}/${REALM}/clients?clientId=${cid}" | python3 -c \
    'import sys,json; d=json.load(sys.stdin); print(len(d))' 2>/dev/null || echo "0")

  if [[ "$EXISTING" -gt 0 ]]; then
    log_ok "client '${cid}' 이미 존재 — 건너뜀"
    return 0
  fi

  curl -s -X POST "${KC_API}/${REALM}/clients" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"clientId\": \"${cid}\",
      \"secret\": \"${secret}\",
      \"enabled\": true,
      \"publicClient\": false,
      \"redirectUris\": ${redirect_uris},
      \"webOrigins\": [\"+\"],
      \"standardFlowEnabled\": true,
      \"protocol\": \"openid-connect\",
      \"attributes\": {\"pkce.code.challenge.method\": \"S256\"}
    }" -o /dev/null
  log_ok "client '${cid}' 생성"
}

# =============================================================================
# 4. GitLab — gitlab-oidc-provider Secret
# =============================================================================
create_gitlab_secret() {
  log_info "── GitLab OIDC Provider Secret ──"

  local BASE="http://keycloak.nullus.internal/realms/${REALM}"
  PROVIDER=$(python3 - <<PYEOF
import json
base = "${BASE}"
realm_path = "/realms/${REALM}"
kc_host = "keycloak.nullus.internal"
p = {
  "name": "openid_connect",
  "label": "Keycloak",
  "args": {
    "name": "openid_connect",
    "scope": ["openid", "profile", "email"],
    "response_type": "code",
    "issuer": base,
    "discovery": False,
    "client_auth_method": "query",
    "uid_field": "preferred_username",
    "send_scope_to_token_endpoint": False,
    "pkce": True,
    "client_options": {
      "identifier": "gitlab",
      "secret": "gitlab-dev-secret",
      "redirect_uri": "http://gitlab.nullus.internal/users/auth/openid_connect/callback",
      "scheme": "http",
      "host": kc_host,
      "port": 80,
      "authorization_endpoint": realm_path + "/protocol/openid-connect/auth",
      "token_endpoint": realm_path + "/protocol/openid-connect/token",
      "userinfo_endpoint": realm_path + "/protocol/openid-connect/userinfo",
      "end_session_endpoint": realm_path + "/protocol/openid-connect/logout",
      "jwks_uri": realm_path + "/protocol/openid-connect/certs"
    }
  }
}
print(json.dumps(p))
PYEOF
)

  kubectl create secret generic gitlab-oidc-provider \
    -n gitlab \
    --from-literal=provider="$PROVIDER" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  log_ok "gitlab-oidc-provider secret 적용"
}

# =============================================================================
# 5. Helm upgrade — Grafana(kps), ArgoCD, MinIO
# =============================================================================
upgrade_app_helm() {
  log_info "── 앱 Helm values upgrade ──"

  _helm_upgrade_if_chart() {
    local name="$1" ns="$2" prefix="$3" values_file="$4"
    local chart
    chart="$(ls "${ROOT_DIR}/helm/charts-catalog"/${prefix}-*.tgz 2>/dev/null | head -1 || true)"
    [[ -n "$chart" ]] || { log_warn "${name}: chart .tgz 없음 — 건너뜀"; return 0; }
    helm upgrade "$name" "$chart" -n "$ns" -f "$values_file" \
      --wait --timeout 5m --reuse-values 2>&1 | tail -2
    log_ok "${name} helm upgrade 완료"
  }

  _helm_upgrade_if_chart kps             nullus-monitoring  kube-prometheus-stack \
    "${VALUES_DIR}/prometheus.yaml"
  _helm_upgrade_if_chart argo-cd         nullus             argo-cd \
    "${VALUES_DIR}/argocd.yaml"
  _helm_upgrade_if_chart nullus-minio    nullus             minio \
    "${VALUES_DIR}/minio.yaml"
  _helm_upgrade_if_chart gitlab          gitlab             gitlab \
    "${VALUES_DIR}/gitlab.yaml"
}

# =============================================================================
# 6. Harbor — REST API OIDC 설정
# =============================================================================
configure_harbor_oidc() {
  log_info "── Harbor OIDC 설정 ──"

  start_pf nullus harbor-core "$HARBOR_CORE_PORT" 8080

  CURRENT_MODE=$(curl -s -u "admin:${HARBOR_ADMIN_PASS}" \
    "http://127.0.0.1:${HARBOR_CORE_PORT}/api/v2.0/configurations" | \
    python3 -c 'import sys,json; print(json.load(sys.stdin).get("auth_mode",{}).get("value",""))' \
    2>/dev/null || echo "")

  if [[ "$CURRENT_MODE" == "oidc_auth" ]]; then
    log_ok "Harbor auth_mode 이미 oidc_auth — 건너뜀"
    return 0
  fi

  local KC_ISSUER="http://keycloak.nullus.internal/realms/${REALM}"
  curl -s -X PUT \
    -u "admin:${HARBOR_ADMIN_PASS}" \
    "http://127.0.0.1:${HARBOR_CORE_PORT}/api/v2.0/configurations" \
    -H "Content-Type: application/json" \
    -d "{
      \"auth_mode\": \"oidc_auth\",
      \"oidc_name\": \"Keycloak\",
      \"oidc_endpoint\": \"${KC_ISSUER}\",
      \"oidc_client_id\": \"harbor\",
      \"oidc_client_secret\": \"harbor-dev-secret\",
      \"oidc_scope\": \"openid,profile,email\",
      \"oidc_verify_cert\": false,
      \"oidc_auto_onboard\": true,
      \"oidc_user_claim\": \"preferred_username\"
    }" -o /dev/null
  log_ok "Harbor OIDC 설정 완료"
}

# =============================================================================
# 7. 검증 — 각 앱 curl redirect
# =============================================================================
verify_all() {
  [[ "$SKIP_VERIFY" == "1" ]] && { log_warn "검증 단계 건너뜀 (SKIP_VERIFY=1)"; return 0; }

  log_info "── SSO redirect 검증 ──"
  local PASS=0 FAIL=0

  _check_redirect() {
    local app="$1" url="$2" expected_loc="$3" host_hdr="${4:-}"
    local HARGS=()
    [[ -n "$host_hdr" ]] && HARGS=(-H "Host: $host_hdr")
    local REDIR
    REDIR=$(curl -s --max-time 8 "${HARGS[@]}" -o /dev/null \
      -w "%{redirect_url}" "$url" 2>/dev/null || true)
    if echo "$REDIR" | grep -q "$expected_loc"; then
      log_ok "${app}: 302 → ${REDIR:0:80}..."
      PASS=$((PASS+1))
    else
      log_err "${app}: FAIL (redirect='${REDIR:0:80}')"
      FAIL=$((FAIL+1))
    fi
  }

  # Grafana — /login/generic_oauth → Keycloak
  _check_redirect "Grafana" \
    "http://127.0.0.1:${GW_PORT_FWD}/login/generic_oauth?redirectTo=" \
    "keycloak.nullus.internal" \
    "grafana.nullus.internal"

  # ArgoCD — /auth/login → Keycloak
  _check_redirect "ArgoCD" \
    "http://127.0.0.1:${GW_PORT_FWD}/auth/login" \
    "keycloak.nullus.internal" \
    "argocd.nullus.internal"

  # Harbor — /c/oidc/login (through harbor nginx port-forward)
  if lsof -i ":8086" -sTCP:LISTEN -t >/dev/null 2>&1; then
    _check_redirect "Harbor" "http://127.0.0.1:8086/c/oidc/login" "keycloak.nullus.internal"
  else
    log_warn "Harbor: port-forward :8086 없음 — 검증 건너뜀"
  fi

  # MinIO — loginStrategy:redirect API
  STRATEGY=$(curl -s --max-time 5 "http://127.0.0.1:9001/api/v1/login" 2>/dev/null | \
    python3 -c 'import sys,json; print(json.load(sys.stdin).get("loginStrategy","?"))' \
    2>/dev/null || echo "unknown")
  if [[ "$STRATEGY" == "redirect" ]]; then
    log_ok "MinIO: loginStrategy=redirect"
    PASS=$((PASS+1))
  else
    log_err "MinIO: loginStrategy=${STRATEGY} (expected redirect)"
    FAIL=$((FAIL+1))
  fi

  # GitLab — POST /users/auth/openid_connect
  if lsof -i ":8087" -sTCP:LISTEN -t >/dev/null 2>&1; then
    COOKIES=$(mktemp /tmp/gcsso-XXXX)
    HTML=$(curl -s -c "$COOKIES" -b "$COOKIES" \
      -H "Host: gitlab.nullus.internal" "http://127.0.0.1:8087/users/sign_in")
    AUTH_TOKEN=$(echo "$HTML" | grep -o 'name="authenticity_token" value="[^"]*"' | \
      head -1 | sed 's/name="authenticity_token" value="//;s/"//')
    REDIR=$(curl -s -b "$COOKIES" -c "$COOKIES" \
      -H "Host: gitlab.nullus.internal" \
      -X POST --max-time 10 \
      --data-urlencode "authenticity_token=$AUTH_TOKEN" \
      -o /dev/null -w "%{redirect_url}" \
      "http://127.0.0.1:8087/users/auth/openid_connect" 2>/dev/null || true)
    rm -f "$COOKIES"
    if echo "$REDIR" | grep -q "keycloak.nullus.internal"; then
      log_ok "GitLab: 302 → ${REDIR:0:80}..."
      PASS=$((PASS+1))
    else
      log_err "GitLab: FAIL (redirect='${REDIR:0:80}')"
      FAIL=$((FAIL+1))
    fi
  else
    log_warn "GitLab: port-forward :8087 없음 — 검증 건너뜀"
  fi

  echo ""
  log_info "검증 결과: PASS=${PASS}  FAIL=${FAIL}"
  [[ "$FAIL" -gt 0 ]] && return 1 || return 0
}

# =============================================================================
# MAIN
# =============================================================================
main() {
  log_info "====== 30-provision-sso.sh 시작 ======"
  log_info "realm: ${REALM}  issuer: ${KC_BASE}"
  echo ""

  patch_coredns
  echo ""
  upgrade_keycloak_hostname
  echo ""
  provision_keycloak
  echo ""
  create_gitlab_secret
  echo ""
  upgrade_app_helm
  echo ""
  configure_harbor_oidc
  echo ""
  verify_all

  echo ""
  log_ok "====== SSO 프로비저닝 완료 ======"
}

main "$@"
