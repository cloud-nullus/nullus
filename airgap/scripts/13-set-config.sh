#!/usr/bin/env bash
# =============================================================================
# 13-set-config.sh — kind 클러스터 kubeconfig 설정
# =============================================================================
# 용도: airgap kind 클러스터(`nullus-airgap`)에 대해 ~/.kube/config 의 context
#       를 정리하고, 필요 시 토큰 기반 인증 컨텍스트도 생성한다.
#       Narwhal 의 scripts/common/set-config.sh 와 동일한 UX 를 제공한다.
#
# 사용법:
#   ./13-set-config.sh             # cert (기본) — kind context 를 정리
#   ./13-set-config.sh token       # ServiceAccount 토큰 기반 컨텍스트 생성
#   ./13-set-config.sh internal    # docker network 내부에서 쓸 kubeconfig 출력
#
# 환경 변수:
#   CLUSTER_NAME    kind 클러스터 이름 (기본: nullus-airgap)
#   SA_NAME         token 모드 ServiceAccount 이름 (기본: airgap-admin)
#   SA_NAMESPACE    token 모드 네임스페이스 (기본: kube-system)
#   TOKEN_DURATION  token 모드 토큰 유효기간 (기본: 8760h = 1년)
#   KUBECONFIG_OUT  internal 모드 출력 파일 경로 (기본: ./kubeconfig-internal.yaml)
#   DRY_RUN         1 이면 실제 변경 없이 명령만 출력
#
# 종료 코드:
#   0 — 성공
#   1 — 의존성 또는 클러스터 없음
#   2 — 알 수 없는 인증 모드
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

CLUSTER_NAME="${CLUSTER_NAME:-nullus-airgap}"
SA_NAME="${SA_NAME:-airgap-admin}"
SA_NAMESPACE="${SA_NAMESPACE:-kube-system}"
TOKEN_DURATION="${TOKEN_DURATION:-8760h}"
KUBECONFIG_OUT="${KUBECONFIG_OUT:-./kubeconfig-internal.yaml}"
DRY_RUN="${DRY_RUN:-0}"

AUTH_METHOD="${1:-cert}"
KIND_CONTEXT="kind-${CLUSTER_NAME}"

# -----------------------------------------------------------------------------
# 로깅
# -----------------------------------------------------------------------------
if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_OK=$'\033[1;32m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_WARN=""; CL_ERR=""; CL_OK=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }

run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    printf 'DRY_RUN: %s\n' "$*" >&2
  else
    "$@"
  fi
}

# -----------------------------------------------------------------------------
# 사전 점검
# -----------------------------------------------------------------------------
command -v kind    >/dev/null || { log_err "kind not found";    exit 1; }
command -v kubectl >/dev/null || { log_err "kubectl not found"; exit 1; }

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  log_err "kind 클러스터 '$CLUSTER_NAME' 이(가) 없습니다. 먼저 11-create-cluster.sh 실행."
  exit 1
fi

log_info "=== kind kubeconfig 설정 ==="
log_info "Cluster      : $CLUSTER_NAME"
log_info "Auth Method  : $AUTH_METHOD"
log_info "Kind Context : $KIND_CONTEXT"

# -----------------------------------------------------------------------------
# 모드 분기
# -----------------------------------------------------------------------------
case "$AUTH_METHOD" in
  cert)
    # kind 가 자동 생성한 kind-* 컨텍스트를 ~/.kube/config 로 export
    log_info "kind kubeconfig export 중..."
    run kind export kubeconfig --name "$CLUSTER_NAME"

    # 컨텍스트 이름을 kind-<name> → <name> 으로 정리 (이미 있으면 무시)
    if kubectl config get-contexts -o name | grep -qx "$KIND_CONTEXT"; then
      if kubectl config get-contexts -o name | grep -qx "$CLUSTER_NAME"; then
        log_warn "컨텍스트 '$CLUSTER_NAME' 이 이미 존재 — rename 생략"
      else
        log_info "컨텍스트 rename: $KIND_CONTEXT → $CLUSTER_NAME"
        run kubectl config rename-context "$KIND_CONTEXT" "$CLUSTER_NAME"
      fi
    fi

    CONTEXT_NAME="$CLUSTER_NAME"
    ;;

  token)
    # ServiceAccount 토큰 기반 인증 컨텍스트 생성
    log_info "ServiceAccount '$SA_NAME' 준비 (네임스페이스: $SA_NAMESPACE)"
    run kubectl --context "$KIND_CONTEXT" create serviceaccount "$SA_NAME" \
      -n "$SA_NAMESPACE" --dry-run=client -o yaml \
      | run kubectl --context "$KIND_CONTEXT" apply -f - >/dev/null

    log_info "ClusterRoleBinding(cluster-admin) 부여"
    run kubectl --context "$KIND_CONTEXT" create clusterrolebinding "${SA_NAME}-binding" \
      --clusterrole=cluster-admin \
      --serviceaccount="${SA_NAMESPACE}:${SA_NAME}" \
      --dry-run=client -o yaml \
      | run kubectl --context "$KIND_CONTEXT" apply -f - >/dev/null

    log_info "토큰 발급 (유효기간: $TOKEN_DURATION)"
    if [[ "$DRY_RUN" == "1" ]]; then
      TOKEN="<dry-run-token>"
    else
      TOKEN="$(kubectl --context "$KIND_CONTEXT" create token "$SA_NAME" \
        -n "$SA_NAMESPACE" --duration="$TOKEN_DURATION")"
    fi

    # API 서버 / CA 추출
    API_SERVER="$(kubectl --context "$KIND_CONTEXT" config view --minify --raw \
      -o jsonpath='{.clusters[0].cluster.server}')"
    CA_FILE="$(mktemp)"
    trap 'rm -f "$CA_FILE"' EXIT
    kubectl --context "$KIND_CONTEXT" config view --minify --raw \
      -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' \
      | base64 -d > "$CA_FILE"

    log_info "토큰 컨텍스트 등록: ${CLUSTER_NAME}-token"
    run kubectl config set-cluster "$CLUSTER_NAME" \
      --server="$API_SERVER" \
      --certificate-authority="$CA_FILE" \
      --embed-certs=true >/dev/null
    run kubectl config set-credentials "${CLUSTER_NAME}-token" \
      --token="$TOKEN" >/dev/null
    run kubectl config set-context "${CLUSTER_NAME}-token" \
      --cluster="$CLUSTER_NAME" \
      --user="${CLUSTER_NAME}-token" >/dev/null

    CONTEXT_NAME="${CLUSTER_NAME}-token"
    ;;

  internal)
    # docker network 안에서 접근 가능한 kubeconfig (API server: <cluster>-control-plane:6443)
    log_info "internal kubeconfig 출력: $KUBECONFIG_OUT"
    run kind get kubeconfig --internal --name "$CLUSTER_NAME" > "$KUBECONFIG_OUT"
    log_ok "내부 네트워크용 kubeconfig 저장: $KUBECONFIG_OUT"
    log_info "다른 컨테이너에서 사용:"
    log_info "  docker run --network kind -v \$(pwd)/$(basename "$KUBECONFIG_OUT"):/root/.kube/config bitnami/kubectl get nodes"
    exit 0
    ;;

  *)
    log_err "알 수 없는 인증 모드: $AUTH_METHOD"
    log_err "사용법: $0 [cert|token|internal]"
    exit 2
    ;;
esac

# -----------------------------------------------------------------------------
# 컨텍스트 활성화 + 연결 테스트
# -----------------------------------------------------------------------------
log_info "활성 컨텍스트 변경: $CONTEXT_NAME"
run kubectl config use-context "$CONTEXT_NAME" >/dev/null

if [[ "$DRY_RUN" != "1" ]]; then
  log_info "연결 테스트 (kubectl get nodes)"
  if kubectl get nodes 2>&1; then
    log_ok "=== 설정 완료 ==="
    log_info "현재 컨텍스트 : $(kubectl config current-context)"
  else
    log_err "연결 실패 — 클러스터 상태를 확인하세요."
    exit 1
  fi
fi
