#!/usr/bin/env bash
set -euo pipefail

STACK_NAMESPACE="${STACK_NAMESPACE:-nullus}"
GATEWAY_NAME="${GATEWAY_NAME:-nullus-devsecops-stack-gateway}"
LOCAL_HTTP_PORT="${LOCAL_HTTP_PORT:-80}"
REMOTE_HTTP_PORT="${REMOTE_HTTP_PORT:-80}"
LOCAL_HTTPS_PORT="${LOCAL_HTTPS_PORT:-443}"
REMOTE_HTTPS_PORT="${REMOTE_HTTPS_PORT:-443}"
FORWARD_HTTPS="${FORWARD_HTTPS:-true}"
ACCESS_HOST="${ACCESS_HOST:-nullus-devsecops-stack.internal}"
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/.kube/config}"

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
  echo "kubectl context 연결 실패: $KUBE_CONTEXT"
  echo "올바른 컨텍스트를 지정하세요. 예) export KUBE_CONTEXT=kind-nullus-platform"
  kubectl --kubeconfig "$KUBECONFIG_PATH" config get-contexts
  exit 1
fi

GW_SVC=""

# 1) 가장 엄격한 선택: gateway name + namespace 라벨
GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -l "gateway.envoyproxy.io/owning-gateway-name=$GATEWAY_NAME,gateway.envoyproxy.io/owning-gateway-namespace=$STACK_NAMESPACE" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
GW_SVC="$(pick_single_or_empty "$GW_SVC_LIST")"

# 2) namespace 라벨만으로 선택
if [[ -z "$GW_SVC" ]]; then
  GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -l "gateway.envoyproxy.io/owning-gateway-namespace=$STACK_NAMESPACE" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  GW_SVC="$(pick_single_or_empty "$GW_SVC_LIST")"
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
  echo "  GATEWAY_NAME=nullus-devsecops-stack-gateway ./scripts/port-forward-gateway.sh"
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
