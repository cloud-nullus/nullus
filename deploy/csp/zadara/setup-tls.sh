#!/usr/bin/env bash
# =============================================================================
# setup-tls.sh — cert-manager + Let's Encrypt 로 실인증서를 붙인다
# =============================================================================
# HTTPS 가 필요한 이유는 보기 좋아서가 아니다. **로그인이 HTTPS 없이는 동작하지
# 않는다.** OIDC Authorization Code + PKCE 는 code_verifier 해시에 Web Crypto
# (`crypto.subtle`)를 쓰는데, 브라우저는 secure context 에서만 이를 노출한다.
# 평문 HTTP 로 접속하면 로그인 화면이 이렇게 끝난다.
#
#   로그인 오류: Crypto.subtle is available only in secure contexts (HTTPS).
#
# self-signed 인증서로도 secure context 는 성립하지만 브라우저 경고를 매번 넘겨야
# 하고 웹/Keycloak 두 오리진 모두에서 예외를 걸어야 한다. 이 환경은 공인 DNS
# (nip.io)와 80 포트가 열려 있어 Let's Encrypt HTTP-01 이 그대로 통하므로 실인증서를
# 받는다.
#
# 사용법:
#   ./setup-tls.sh install     cert-manager 설치 + ClusterIssuer 생성
#   ./setup-tls.sh status      발급 상태 확인
#   ./setup-tls.sh uninstall   되돌리기
#
# 환경 변수:
#   NAMESPACE      Nullus 네임스페이스   (기본: nullus)
#   KUBE_CONTEXT   kubectl 컨텍스트
#   ACME_EMAIL     만료 알림 수신 주소   (기본: admin@nullus.io)
#   ISSUER         letsencrypt | letsencrypt-staging (기본: letsencrypt)
#                  staging 은 rate limit 이 넉넉하지만 브라우저가 신뢰하지 않는다.
#   CERT_MANAGER_VERSION (기본: v1.16.2)
#
# 종료 코드: 0 성공 / 1 실패
# =============================================================================
set -euo pipefail

NAMESPACE="${NAMESPACE:-nullus}"
ACME_EMAIL="${ACME_EMAIL:-admin@nullus.io}"
ISSUER="${ISSUER:-letsencrypt}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.16.2}"

if [[ -t 1 ]]; then
  C_OK=$'\033[1;32m'; C_ERR=$'\033[1;31m'; C_WARN=$'\033[1;33m'; C_DIM=$'\033[2m'; C_RST=$'\033[0m'
else
  C_OK=""; C_ERR=""; C_WARN=""; C_DIM=""; C_RST=""
fi
ok()   { printf '%s✔%s %s\n' "$C_OK"   "$C_RST" "$*"; }
warn() { printf '%s!%s %s\n' "$C_WARN" "$C_RST" "$*"; }
info() { printf '%s·%s %s\n' "$C_DIM"  "$C_RST" "$*"; }
die()  { printf '%s✘%s %s\n' "$C_ERR"  "$C_RST" "$*" >&2; exit 1; }

K=(kubectl)
[[ -n "${KUBE_CONTEXT:-}" ]] && K+=(--context="$KUBE_CONTEXT")

acme_server() {
  case "$ISSUER" in
    letsencrypt)         echo "https://acme-v02.api.letsencrypt.org/directory" ;;
    letsencrypt-staging) echo "https://acme-staging-v02.api.letsencrypt.org/directory" ;;
    *) die "ISSUER 는 letsencrypt 또는 letsencrypt-staging 이어야 합니다: $ISSUER" ;;
  esac
}

do_install() {
  command -v kubectl >/dev/null || die "kubectl 이 필요합니다"

  if "${K[@]}" get deploy -n cert-manager cert-manager >/dev/null 2>&1; then
    info "cert-manager 이미 설치됨"
  else
    info "cert-manager ${CERT_MANAGER_VERSION} 설치"
    "${K[@]}" apply -f \
      "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml" \
      >/dev/null || die "cert-manager 설치 실패"
    "${K[@]}" -n cert-manager rollout status deploy/cert-manager --timeout=300s >/dev/null
    "${K[@]}" -n cert-manager rollout status deploy/cert-manager-webhook --timeout=300s >/dev/null
    ok "cert-manager 설치 완료"
  fi

  # webhook 이 올라온 직후에는 인증서 리소스 생성이 거부될 수 있다. 준비될 때까지 기다린다.
  local i
  for i in $(seq 1 30); do
    "${K[@]}" get --raw /apis/cert-manager.io/v1 >/dev/null 2>&1 && break
    sleep 5
  done

  info "ClusterIssuer ${ISSUER} 생성 (HTTP-01, ingress class nginx)"
  "${K[@]}" apply -f - >/dev/null <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: ${ISSUER}
spec:
  acme:
    server: $(acme_server)
    email: ${ACME_EMAIL}
    privateKeySecretRef:
      name: ${ISSUER}-account-key
    solvers:
      # 이 환경은 nip.io 로 공인 DNS 가 잡히고 80/tcp 가 열려 있어 HTTP-01 이 통한다.
      # DNS-01 은 nip.io 에 TXT 를 넣을 수 없어 쓸 수 없다.
      - http01:
          ingress:
            class: nginx
EOF
  ok "ClusterIssuer ${ISSUER} 준비"
  echo
  info "다음: values-zadara.yaml 의 ingress.tls 를 켜고 helm upgrade 를 돌리십시오."
  info "발급 확인: $0 status"
}

do_status() {
  printf '· cert-manager ... '
  if "${K[@]}" get deploy -n cert-manager cert-manager >/dev/null 2>&1; then echo "설치됨"; else echo "없음"; return; fi
  echo "· ClusterIssuer"
  "${K[@]}" get clusterissuer -o custom-columns=NAME:.metadata.name,READY:.status.conditions[0].status --no-headers 2>/dev/null | sed 's/^/    /'
  echo "· Certificate (${NAMESPACE})"
  "${K[@]}" -n "$NAMESPACE" get certificate -o custom-columns=NAME:.metadata.name,READY:.status.conditions[0].status,SECRET:.spec.secretName --no-headers 2>/dev/null | sed 's/^/    /'
  local pending
  pending="$("${K[@]}" -n "$NAMESPACE" get challenge --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$pending" != "0" ]]; then
    warn "진행 중인 ACME 챌린지 ${pending}건"
    "${K[@]}" -n "$NAMESPACE" get challenge -o custom-columns=NAME:.metadata.name,STATE:.status.state,REASON:.status.reason --no-headers 2>/dev/null | sed 's/^/    /'
  fi
}

do_uninstall() {
  "${K[@]}" delete clusterissuer letsencrypt letsencrypt-staging --ignore-not-found >/dev/null 2>&1 || true
  "${K[@]}" delete -f \
    "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml" \
    --ignore-not-found >/dev/null 2>&1 || true
  ok "cert-manager 제거 (인증서 시크릿은 네임스페이스에 남는다 — 필요하면 직접 삭제)"
}

case "${1:-status}" in
  install)   do_install ;;
  status)    do_status ;;
  uninstall) do_uninstall ;;
  *)         die "사용법: $0 [install|status|uninstall]" ;;
esac
