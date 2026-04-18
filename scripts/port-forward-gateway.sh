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

GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -l "gateway.envoyproxy.io/owning-gateway-name=$GATEWAY_NAME" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')"
GW_SVC="${GW_SVC_LIST%%$'\n'*}"

if [[ -z "$GW_SVC" ]]; then
  GW_SVC_LIST="$(kubectl --kubeconfig "$KUBECONFIG_PATH" --context "$KUBE_CONTEXT" -n "$STACK_NAMESPACE" get svc -l "gateway.envoyproxy.io/owning-gateway-namespace=$STACK_NAMESPACE" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')"
  GW_SVC="${GW_SVC_LIST%%$'\n'*}"
fi

if [[ -z "$GW_SVC" ]]; then
  echo "Gateway 데이터플레인 서비스가 없습니다."
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
    echo "권한 부족: 1024 미만 포트(${LOCAL_HTTP_PORT}${FORWARD_HTTPS:+/${LOCAL_HTTPS_PORT}}) 바인딩에는 root 권한이 필요합니다."
    echo "다음처럼 실행하세요: sudo ./scripts/port-forward-gateway.sh"
    exit 1
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
