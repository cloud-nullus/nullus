#!/usr/bin/env bash
# =============================================================================
# 23-setup-gateway.sh — Envoy Gateway 기반 외부 접근 도메인 구성
# =============================================================================
# 용도: 제품 orchestrator
#         internal/stack/adapter/helm/manifest-builders.go
#         (defaultGatewayBundleManifest / defaultEnvoyGatewayClassManifest)
#       의 Gateway API 스킴을 airgap 환경에 1:1 재현한다.
#
#   - GatewayClass(envoy) + Gateway(*.<domain>:80) 생성
#   - 클러스터에 실제 존재하는 서비스만 탐지하여 HTTPRoute 생성 (부분 설치 대응)
#   - 마지막에 /etc/hosts 항목 + port-forward 명령 + 접근 도메인 목록 출력
#
# 배경: airgap 순수 설치 경로(22-install-platform-stack.sh)는 Keycloak/kps 만
#       설치하고 외부 접근(Gateway/HTTPRoute)을 구성하지 않아 "외부 접근 도메인이
#       보이지 않는" 상태가 된다. 본 스크립트가 그 공백을 메운다.
#
# 사용법:
#   ./23-setup-gateway.sh
#   ACCESS_DOMAIN=nullus.internal GATEWAY_NS=nullus ./23-setup-gateway.sh
#   PRINT_ONLY=1 ./23-setup-gateway.sh   # 적용 없이 접근 정보만 출력
#
# 환경 변수:
#   ACCESS_DOMAIN   기준 도메인 (기본: nullus.internal) — 코드의 `.internal` 컨벤션
#   GATEWAY_NS      Gateway 리소스 네임스페이스 (기본: nullus)
#   GATEWAY_NAME    Gateway 이름 (기본: nullus-gateway)
#   GATEWAY_CLASS   GatewayClass 이름 (기본: envoy)
#   PRINT_ONLY      1 = 리소스 적용 없이 접근 안내만 출력
#
# 종료 코드:
#   0 — 성공 (route 0개여도 성공)
#   1 — 사전조건 실패 (kubectl/CRD/컨트롤러 없음)
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_OK=$'\033[1;32m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_WARN=""; CL_ERR=""; CL_OK=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }

ACCESS_DOMAIN="${ACCESS_DOMAIN:-nullus.internal}"
GATEWAY_NS="${GATEWAY_NS:-nullus}"
GATEWAY_NAME="${GATEWAY_NAME:-nullus-gateway}"
GATEWAY_CLASS="${GATEWAY_CLASS:-envoy}"
PRINT_ONLY="${PRINT_ONLY:-0}"
STACK_LABEL="${ACCESS_DOMAIN%.internal}"
[[ -z "$STACK_LABEL" || "$STACK_LABEL" == "$ACCESS_DOMAIN" ]] && STACK_LABEL="nullus"

# -----------------------------------------------------------------------------
# 라우팅 테이블: "subdomain|namespace|service|port"
#   subdomain == "@" 이면 apex 도메인(ACCESS_DOMAIN 자체)으로 라우팅한다.
# (스택 도구는 manifest-builders.go 의 routeSpec 매핑을 airgap 실제 서비스명/ns 에
#  맞춰 재현, Nullus 포털/API 는 airgap 에서 클러스터 내부에 함께 떠 있으므로 추가)
# -----------------------------------------------------------------------------
ROUTES=(
  "@|nullus|nullus-web|80"
  "api|nullus|nullus-api|8080"
  "argocd|nullus|argo-cd-argocd-server|80"
  "harbor|nullus|harbor|80"
  "minio|nullus|nullus-minio-console|9001"
  "opensearch|nullus|opensearch-cluster-master|9200"
  "gitlab|gitlab|gitlab-webservice-default|8080"
  "grafana|nullus-monitoring|kps-grafana|80"
  "prometheus|nullus-monitoring|kps-kube-prometheus-stack-prometheus|9090"
  "keycloak|nullus-auth|keycloak|80"
)

# -----------------------------------------------------------------------------
# 사전 점검
# -----------------------------------------------------------------------------
command -v kubectl >/dev/null || { log_err "kubectl 없음"; exit 1; }
kubectl cluster-info >/dev/null 2>&1 || { log_err "클러스터 접근 불가 (kubeconfig 확인)"; exit 1; }

if [[ "$PRINT_ONLY" != "1" ]]; then
  kubectl get crd gateways.gateway.networking.k8s.io >/dev/null 2>&1 || {
    log_err "Gateway API CRD 미설치 — 먼저 Gateway API standard-install + envoy gateway-helm 설치 필요"
    exit 1
  }
  if ! kubectl get gatewayclass "$GATEWAY_CLASS" >/dev/null 2>&1; then
    log_warn "GatewayClass '$GATEWAY_CLASS' 없음 — envoy gateway 컨트롤러 미설치 가능성. 계속 시도."
  fi
fi

log_info "ACCESS_DOMAIN : $ACCESS_DOMAIN"
log_info "Gateway       : $GATEWAY_NS/$GATEWAY_NAME (class=$GATEWAY_CLASS)"

# -----------------------------------------------------------------------------
# 적용
# -----------------------------------------------------------------------------
CREATED=()  # "subdomain|ns|svc|port" — 실제 생성된 route 추적

apply_manifest() {
  if [[ "$PRINT_ONLY" == "1" ]]; then return 0; fi
  kubectl apply -f - >/dev/null
}

if [[ "$PRINT_ONLY" != "1" ]]; then
  # 1) GatewayClass (cluster-scoped, idempotent)
  apply_manifest <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: ${GATEWAY_CLASS}
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF

  # 2) Gateway — 와일드카드 리스너 + 크로스 네임스페이스 허용(from: All)
  kubectl get ns "$GATEWAY_NS" >/dev/null 2>&1 || kubectl create namespace "$GATEWAY_NS" >/dev/null
  apply_manifest <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: ${GATEWAY_NAME}
  namespace: ${GATEWAY_NS}
  labels:
    nullus.io/stack-name: ${STACK_LABEL}
spec:
  gatewayClassName: ${GATEWAY_CLASS}
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      hostname: "*.${ACCESS_DOMAIN}"
      allowedRoutes:
        namespaces:
          from: All
    - name: http-apex
      protocol: HTTP
      port: 80
      hostname: "${ACCESS_DOMAIN}"
      allowedRoutes:
        namespaces:
          from: All
EOF
  log_ok "GatewayClass + Gateway 적용 완료"
fi

# 3) HTTPRoute — 존재하는 서비스만 (각 서비스 ns 에 co-locate)
#    sub == "@" 이면 apex 도메인, 그 외엔 "<sub>.<domain>". route 이름도 그에 맞게.
for entry in "${ROUTES[@]}"; do
  IFS='|' read -r sub ns svc port <<<"$entry"
  if [[ "$sub" == "@" ]]; then
    host="${ACCESS_DOMAIN}"; rname="nullus-portal-route"
  else
    host="${sub}.${ACCESS_DOMAIN}"; rname="${sub}-route"
  fi
  if ! kubectl get svc -n "$ns" "$svc" >/dev/null 2>&1; then
    log_warn "건너뜀: $ns/$svc 없음 (${host})"
    continue
  fi
  if [[ "$PRINT_ONLY" != "1" ]]; then
    apply_manifest <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: ${rname}
  namespace: ${ns}
  labels:
    nullus.io/stack-name: ${STACK_LABEL}
spec:
  parentRefs:
    - name: ${GATEWAY_NAME}
      namespace: ${GATEWAY_NS}
  hostnames:
    - ${host}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: ${svc}
          port: ${port}
EOF
    log_ok "HTTPRoute: ${host} -> ${ns}/${svc}:${port}"
  fi
  # CREATED 에는 표시·hosts 용으로 "host|ns|svc|port" 저장
  CREATED+=("${host}|${ns}|${svc}|${port}")
done

# -----------------------------------------------------------------------------
# Gateway Programmed 대기 + 데이터플레인 서비스 탐지
# -----------------------------------------------------------------------------
ENVOY_SVC=""
if [[ "$PRINT_ONLY" != "1" ]]; then
  log_info "Gateway 프로그래밍 대기..."
  kubectl wait --for=condition=Programmed "gateway/${GATEWAY_NAME}" \
    -n "$GATEWAY_NS" --timeout=120s >/dev/null 2>&1 \
    && log_ok "Gateway Programmed" \
    || log_warn "Gateway Programmed 미확인 — envoy 컨트롤러 로그 확인 권장"
fi
# Envoy Gateway 가 자동 생성하는 데이터플레인 서비스 (owning-gateway 라벨)
ENVOY_SVC="$(kubectl get svc -n "$GATEWAY_NS" \
  -l "gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME}" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"

# -----------------------------------------------------------------------------
# 접근 안내 출력 (stdout — 사용자가 그대로 복붙)
# -----------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " 외부 접근 도메인 (${#CREATED[@]}개)"
echo "============================================================"
if [[ ${#CREATED[@]} -eq 0 ]]; then
  echo " (라우팅 가능한 서비스 없음 — 스택 설치 후 재실행)"
else
  for entry in "${CREATED[@]}"; do
    IFS='|' read -r host ns svc port <<<"$entry"
    printf "  http://%-28s -> %s/%s:%s\n" "${host}" "$ns" "$svc" "$port"
  done

  echo ""
  echo "------------------------------------------------------------"
  echo " 1) /etc/hosts 등록 (sudo 필요) — 전용 스크립트 권장:"
  echo "------------------------------------------------------------"
  echo "  sudo bash \"\$(dirname \"\$0\")/24-register-hosts.sh\""
  echo "  (해제: sudo REMOVE=1 bash .../24-register-hosts.sh)"
  echo ""
  echo "  또는 수동으로 /etc/hosts 에 직접 추가:"
  hosts_line="127.0.0.1 "
  for entry in "${CREATED[@]}"; do
    IFS='|' read -r host _ _ _ <<<"$entry"
    hosts_line+="${host} "
  done
  echo "  $hosts_line"

  echo ""
  echo "------------------------------------------------------------"
  echo " 2) Gateway 데이터플레인 포트포워딩 (별도 터미널 유지):"
  echo "------------------------------------------------------------"
  if [[ -n "$ENVOY_SVC" ]]; then
    echo "  sudo kubectl port-forward -n ${GATEWAY_NS} svc/${ENVOY_SVC} 80:80"
  else
    echo "  # 데이터플레인 svc 탐지 실패 — 아래로 확인 후 port-forward:"
    echo "  kubectl get svc -n ${GATEWAY_NS} -l gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME}"
  fi
  echo ""
  echo "  → 이후 브라우저: http://${ACCESS_DOMAIN}/ (Nullus 포털), http://argocd.${ACCESS_DOMAIN}/ 등"
fi
echo "============================================================"

log_ok "외부 접근 도메인 구성 완료 (route ${#CREATED[@]}개)"
