#!/usr/bin/env bash
# =============================================================================
# port-forward-gateway.sh — 스택 도구를 로컬에서 도메인으로 연다
# =============================================================================
# 스택 도구는 게이트웨이 뒤에 있고 Host 헤더로 갈린다(gitlab.<도메인> ...).
# 게이트웨이 Service 는 LoadBalancer 라 LB 연동이 없는 클러스터에서는 외부 주소를
# 영영 받지 못하므로, 로컬에서는 이 스크립트가 그 앞을 대신 세운다.
#
# kind 클러스터면 도커 네트워크 안에 SNI 라우터를 띄운다. 그러면
#   - sudo 가 필요 없다. 443 은 도커가 대신 잡는다
#   - 스택을 여러 개 동시에 열 수 있다. 이름(SNI)으로 갈라 보낸다
#   - NodePort 를 고정하지 않아도 된다. 돌고 있는 게이트웨이에서 찾아낸다
# 그 외(원격 클러스터 등)는 종전대로 kubectl port-forward 로 하나를 연다.
#
# 사용법
#   ./scripts/port-forward-gateway.sh           # 열기(갱신)
#   ./scripts/port-forward-gateway.sh --down    # 라우터 내리기
#   ./scripts/port-forward-gateway.sh --hosts   # /etc/hosts 에 넣을 줄만 출력
#   STACK_NAMESPACE=... ./scripts/port-forward-gateway.sh   # 한 스택만 port-forward
# =============================================================================
set -euo pipefail

# 기본값을 비워 둔다. 예전에는 nullus / nullus-gateway / nullus.internal 이
# 박혀 있었는데 실제 스택은 nullus-<템플릿> 네임스페이스에 <도메인>-gateway 를
# 세우므로, 그냥 실행하면 "네임스페이스가 없습니다" 로 끝났다.
STACK_NAMESPACE="${STACK_NAMESPACE:-}"
GATEWAY_NAME="${GATEWAY_NAME:-}"
LOCAL_HTTP_PORT="${LOCAL_HTTP_PORT:-80}"
REMOTE_HTTP_PORT="${REMOTE_HTTP_PORT:-80}"
LOCAL_HTTPS_PORT="${LOCAL_HTTPS_PORT:-443}"
REMOTE_HTTPS_PORT="${REMOTE_HTTPS_PORT:-443}"
FORWARD_HTTPS="${FORWARD_HTTPS:-true}"
ACCESS_HOST="${ACCESS_HOST:-}"
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/.kube/config}"

ROUTER_CONTAINER="nullus-gateway-router"
ROUTER_NETWORK="${NULLUS_KIND_NETWORK:-kind}"
ROUTER_IMAGE="${NULLUS_ROUTER_IMAGE:-nginx:alpine}"

MODE="auto"
for arg in "$@"; do
  case "$arg" in
    --down)  MODE="down" ;;
    --hosts) MODE="hosts" ;;
    -h|--help) sed -n '2,22p' "$0"; exit 0 ;;
  esac
done

if [[ "$MODE" == "down" ]]; then
  docker rm -f "$ROUTER_CONTAINER" >/dev/null 2>&1 || true
  echo "게이트웨이 라우터를 내렸습니다"
  exit 0
fi

# discover_gateways 는 "호스트이름 노드IP:NodePort" 줄을 만든다.
#
# 노드는 envoy 파드가 도는 것을 고른다. 게이트웨이 Service 의
# externalTrafficPolicy 가 Local 이면 엔드포인트가 없는 노드는 트래픽을 버린다 —
# 아무 노드나 고르면 조용히 닿지 않는다.
discover_gateways() {
  local cluster ctx ns svc node_name node_ip node_port
  while IFS= read -r cluster; do
    [[ -z "$cluster" ]] && continue
    ctx="kind-${cluster}"
    kubectl --context "$ctx" get ns >/dev/null 2>&1 || continue

    while IFS=$'\t' read -r ns svc; do
      [[ -z "$ns" || -z "$svc" ]] && continue
      node_port="$(kubectl --context "$ctx" get svc "$svc" -n "$ns" \
        -o jsonpath='{.spec.ports[?(@.port==443)].nodePort}' 2>/dev/null || true)"
      [[ -n "$node_port" ]] || continue
      node_name="$(kubectl --context "$ctx" get pods -n "$ns" \
        -l gateway.envoyproxy.io/owning-gateway-name \
        -o jsonpath='{.items[0].spec.nodeName}' 2>/dev/null || true)"
      [[ -n "$node_name" ]] || continue
      node_ip="$(docker inspect "$node_name" --format "{{.NetworkSettings.Networks.${ROUTER_NETWORK}.IPAddress}}" 2>/dev/null || true)"
      [[ -n "$node_ip" ]] || continue

      # 이 게이트웨이가 받는 이름은 HTTPRoute 가 알고 있다.
      kubectl --context "$ctx" get httproute -n "$ns" \
        -o jsonpath='{range .items[*]}{range .spec.hostnames[*]}{@}{"\n"}{end}{end}' 2>/dev/null |
        sort -u | while IFS= read -r host; do
          [[ -z "$host" ]] && continue
          printf '%s %s:%s\n' "$host" "$node_ip" "$node_port"
        done
    done < <(kubectl --context "$ctx" get svc --all-namespaces \
      -l gateway.envoyproxy.io/owning-gateway-name \
      -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\n"}{end}' 2>/dev/null)
  done < <(kind get clusters 2>/dev/null)
}

hosts_line() { printf '127.0.0.1'; printf ' %s' $(awk '{print $1}' <<<"$1"); printf '\n'; }

# 스택을 콕 집어 주지 않았고 kind 가 있으면 SNI 라우터로 전부 연다.
if [[ -z "$STACK_NAMESPACE" ]] && command -v kind >/dev/null 2>&1 && command -v docker >/dev/null 2>&1; then
  BACKENDS="$(discover_gateways | sort -u)"
  if [[ -n "$BACKENDS" ]]; then
    if [[ "$MODE" == "hosts" ]]; then hosts_line "$BACKENDS"; exit 0; fi

    ROUTER_CONF="$(mktemp)"
    {
      echo 'events {}'
      echo 'stream {'
      echo '  map $ssl_preread_server_name $nullus_backend {'
      echo '    default "";'
      while IFS=' ' read -r host backend; do
        [[ -z "$host" ]] && continue
        printf '    %s %s;\n' "$host" "$backend"
      done <<<"$BACKENDS"
      echo '  }'
      echo '  server {'
      echo '    listen 443;'
      echo '    ssl_preread on;'
      echo '    proxy_pass $nullus_backend;'
      echo '  }'
      echo '}'
    } >"$ROUTER_CONF"

    docker rm -f "$ROUTER_CONTAINER" >/dev/null 2>&1 || true
    docker run -d --name "$ROUTER_CONTAINER" --network "$ROUTER_NETWORK" \
      -p "${LOCAL_HTTPS_PORT}:443" "$ROUTER_IMAGE" >/dev/null
    # 설정은 컨테이너 안에 둔다. 호스트 임시 파일을 마운트하면 그 파일이 지워진 뒤
    # 컨테이너가 다시 뜨지 못한다.
    docker cp "$ROUTER_CONF" "$ROUTER_CONTAINER:/etc/nginx/nginx.conf" >/dev/null
    docker restart "$ROUTER_CONTAINER" >/dev/null
    rm -f "$ROUTER_CONF"

    echo "게이트웨이 라우터가 :${LOCAL_HTTPS_PORT} 에서 돕니다 (SNI 로 스택을 가른다)"
    while IFS=' ' read -r host backend; do
      [[ -z "$host" ]] && continue
      printf '  https://%-30s → %s\n' "$host" "$backend"
    done <<<"$BACKENDS"
    echo ""
    echo "/etc/hosts 에 아래 한 줄이 필요합니다 (sudo 필요):"
    echo ""
    echo "  echo '$(hosts_line "$BACKENDS")' | sudo tee -a /etc/hosts"
    exit 0
  fi
fi

# 여기부터는 종전 경로다 — 스택을 콕 집었거나 kind 가 아닌 클러스터.
STACK_NAMESPACE="${STACK_NAMESPACE:-nullus}"
ACCESS_HOST="${ACCESS_HOST:-nullus.local}"

if [[ ! -f "$KUBECONFIG_PATH" && "${EUID:-0}" -eq 0 && -n "${SUDO_USER:-}" ]]; then
  SUDO_USER_KUBECONFIG="/Users/${SUDO_USER}/.kube/config"
  if [[ -f "$SUDO_USER_KUBECONFIG" ]]; then
    KUBECONFIG_PATH="$SUDO_USER_KUBECONFIG"
  fi
fi

pick_single_or_empty() {
  local list="$1"
  local count
  count="$(printf '%s\n' "$list" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [[ "$count" == "1" ]]; then
    printf '%s\n' "$list" | sed '/^$/d' | head -n1
    return 0
  fi
  printf ''
  return 1
}

if [[ ! -f "$KUBECONFIG_PATH" ]]; then
  echo "kubeconfig 파일이 없습니다: $KUBECONFIG_PATH"
  exit 1
fi

KUBE_CONTEXT="${KUBE_CONTEXT:-$(kubectl --kubeconfig "$KUBECONFIG_PATH" config current-context 2>/dev/null)}"
if [[ -z "$KUBE_CONTEXT" ]]; then
  echo "kubectl context가 없습니다. 먼저 kubectl config use-context <context> 실행하세요."
  kubectl --kubeconfig "$KUBECONFIG_PATH" config get-contexts
  exit 1
fi

if ! kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" get namespace "$STACK_NAMESPACE" >/dev/null 2>&1; then
  # 연결 자체가 안 되는 것과 네임스페이스가 없는 것은 원인이 전혀 다르다.
  # 둘을 같은 메시지로 뭉뚱그리면 엉뚱한 곳을 고치게 된다.
  if ! kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" version >/dev/null 2>&1; then
    echo "kubectl 컨텍스트에 연결하지 못했습니다: $KUBE_CONTEXT"
    echo "클러스터가 떠 있는지 확인하세요. 예) kind get clusters"
    kubectl --kubeconfig "$KUBECONFIG_PATH" config get-contexts
    exit 1
  fi

  # 연결은 되는데 네임스페이스가 없다 — 다른 컨텍스트에 스택이 있는지 찾아 준다.
  FOUND_CONTEXT=""
  while IFS= read -r ctx; do
    [[ -z "$ctx" || "$ctx" == "$KUBE_CONTEXT" ]] && continue
    if kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$ctx" get namespace "$STACK_NAMESPACE" >/dev/null 2>&1; then
      FOUND_CONTEXT="$ctx"
      break
    fi
  done < <(kubectl --kubeconfig "$KUBECONFIG_PATH" config get-contexts -o name 2>/dev/null)

  echo "컨텍스트 '${KUBE_CONTEXT}' 에 네임스페이스 '${STACK_NAMESPACE}' 가 없습니다."
  if [[ -n "$FOUND_CONTEXT" ]]; then
    echo "'${FOUND_CONTEXT}' 에 있습니다. 다시 실행하세요:"
    echo "  KUBE_CONTEXT=${FOUND_CONTEXT} STACK_NAMESPACE=${STACK_NAMESPACE} ... $0"
  else
    echo "어떤 컨텍스트에서도 찾지 못했습니다. STACK_NAMESPACE 를 확인하세요."
    kubectl --kubeconfig "$KUBECONFIG_PATH" config get-contexts
  fi
  exit 1
fi

GW_SVC=""

# 1) 가장 엄격한 선택: gateway name + namespace 라벨 (이름을 준 경우만)
GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -l "gateway.envoyproxy.io/owning-gateway-name=$GATEWAY_NAME,gateway.envoyproxy.io/owning-gateway-namespace=$STACK_NAMESPACE" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
GW_SVC="$(pick_single_or_empty "$GW_SVC_LIST")"

# 2) namespace 라벨만으로 선택
if [[ -z "$GW_SVC" ]]; then
  GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -l "gateway.envoyproxy.io/owning-gateway-namespace=$STACK_NAMESPACE" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  GW_SVC="$(pick_single_or_empty "$GW_SVC_LIST")"
fi

if [[ "$FORWARD_HTTPS" == "true" ]]; then
  SVC_PORTS="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc "$GW_SVC" -o jsonpath='{range .spec.ports[*]}{.port}{"\n"}{end}' 2>/dev/null || true)"
  if ! printf '%s\n' "$SVC_PORTS" | grep -qx "$REMOTE_HTTPS_PORT"; then
    echo "게이트웨이 서비스($GW_SVC)에 HTTPS 포트($REMOTE_HTTPS_PORT)가 없어 HTTP만 포워딩합니다."
    FORWARD_HTTPS="false"
  fi
fi

# 3) Envoy Gateway 데이터플레인 서비스 패턴 fallback
if [[ -z "$GW_SVC" ]]; then
  GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -l "app.kubernetes.io/managed-by=envoy-gateway,app.kubernetes.io/component=proxy" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  GW_SVC="$(pick_single_or_empty "$GW_SVC_LIST")"
fi

# 4) 최종 fallback: 서비스명 패턴 매칭
if [[ -z "$GW_SVC" ]]; then
  GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null | grep -E '^envoy-.*-gateway-' || true)"
  GW_SVC="$(pick_single_or_empty "$GW_SVC_LIST")"
fi

if [[ -z "$GW_SVC" ]]; then
  echo "Gateway 데이터플레인 서비스가 없습니다."
  echo "또는 후보 서비스가 여러 개라 자동 선택이 불가능합니다."
  echo "필요 시 GATEWAY_NAME 환경변수를 명시하세요. 예)"
  echo "  GATEWAY_NAME=nullus-gateway ./scripts/port-forward-gateway.sh"
  echo "확인 항목:"
  echo "  1) Gateway 리소스 존재 여부"
  kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get gateway || true
  echo "  2) owning-gateway 라벨이 붙은 Service 존재 여부"
  kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc --show-labels || true
  exit 1
fi

echo "사용 컨텍스트: $KUBE_CONTEXT"
echo "네임스페이스: $STACK_NAMESPACE"
echo "게이트웨이 서비스: $GW_SVC"
echo "포트포워드(HTTP):  localhost:${LOCAL_HTTP_PORT} -> svc/${GW_SVC}:${REMOTE_HTTP_PORT}"
if [[ "$FORWARD_HTTPS" == "true" ]]; then
  echo "포트포워드(HTTPS): localhost:${LOCAL_HTTPS_PORT} -> svc/${GW_SVC}:${REMOTE_HTTPS_PORT}"
fi

if [[ "$LOCAL_HTTP_PORT" -lt 1024 || ( "$FORWARD_HTTPS" == "true" && "$LOCAL_HTTPS_PORT" -lt 1024 ) ]]; then
  if [[ "$EUID" -ne 0 ]]; then
    echo "1024 미만 포트(${LOCAL_HTTP_PORT}${FORWARD_HTTPS:+/${LOCAL_HTTPS_PORT}}) 사용으로 sudo 권한이 필요합니다."
    echo "sudo 권한으로 재실행합니다..."
    exec sudo -E \
      KUBECONFIG="$KUBECONFIG_PATH" \
      KUBE_CONTEXT="$KUBE_CONTEXT" \
      STACK_NAMESPACE="$STACK_NAMESPACE" \
      GATEWAY_NAME="$GATEWAY_NAME" \
      LOCAL_HTTP_PORT="$LOCAL_HTTP_PORT" \
      REMOTE_HTTP_PORT="$REMOTE_HTTP_PORT" \
      LOCAL_HTTPS_PORT="$LOCAL_HTTPS_PORT" \
      REMOTE_HTTPS_PORT="$REMOTE_HTTPS_PORT" \
      FORWARD_HTTPS="$FORWARD_HTTPS" \
      ACCESS_HOST="$ACCESS_HOST" \
      "$0" "$@"
  fi
fi

if [[ "$LOCAL_HTTP_PORT" == "80" ]]; then
  echo "GitLab 접속 권장 URL: http://${ACCESS_HOST}"
else
  echo "GitLab 접속 권장 URL: http://${ACCESS_HOST}:${LOCAL_HTTP_PORT}"
fi

if [[ "$FORWARD_HTTPS" == "true" ]]; then
  if [[ "$LOCAL_HTTPS_PORT" == "443" ]]; then
    echo "GitLab 접속 권장 URL(HTTPS): https://gitlab.${ACCESS_HOST#gitlab.}"
  else
    echo "GitLab 접속 권장 URL(HTTPS): https://${ACCESS_HOST}:${LOCAL_HTTPS_PORT}"
  fi
fi

if [[ "$FORWARD_HTTPS" == "true" ]]; then
  kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" port-forward "svc/$GW_SVC" "${LOCAL_HTTP_PORT}:${REMOTE_HTTP_PORT}" "${LOCAL_HTTPS_PORT}:${REMOTE_HTTPS_PORT}"
else
  kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" port-forward "svc/$GW_SVC" "${LOCAL_HTTP_PORT}:${REMOTE_HTTP_PORT}"
fi
