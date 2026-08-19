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
# 하고 웹/Keycloak 두 오리진 모두에서 예외를 걸어야 한다.
#
# 검증 방식이 둘인 이유:
#
#   HTTP-01  공인 DNS 와 80/tcp 만 있으면 되는 가장 단순한 경로. nip.io 호스트가
#            이 경로로 발급된다.
#   DNS-01   **와일드카드는 HTTP-01 로 발급할 수 없다** — ACME 규격이 그렇다.
#            `*.nullus.io` 한 장으로 argocd/grafana/harbor… 를 전부 덮으려면
#            DNS-01 이 유일한 길이다. 도구를 하나 늘릴 때마다 인증서를 새로
#            받는 운영을 없애려는 것이다.
#
# 두 솔버는 공존한다. DNS01_ZONE 에 준 존만 DNS-01 로 가고 나머지는 HTTP-01 이다.
# cert-manager 는 더 구체적인 selector 를 고르므로 서로를 가리지 않는다.
#
# 사용법:
#   ./setup-tls.sh install     cert-manager (+DNS-01 웹훅) 설치 + ClusterIssuer
#   ./setup-tls.sh wildcard    와일드카드 Certificate 생성 (DNS01_ZONE 필요)
#   ./setup-tls.sh status      발급 상태 확인
#   ./setup-tls.sh uninstall   되돌리기
#
# 환경 변수:
#   NAMESPACE      Nullus 네임스페이스   (기본: nullus)
#   KUBE_CONTEXT   kubectl 컨텍스트
#   ACME_EMAIL     만료 알림 수신 주소   (기본: admin@nullus.io)
#   ISSUER         letsencrypt | letsencrypt-staging (기본: letsencrypt)
#                  staging 은 rate limit 이 넉넉하지만 브라우저가 신뢰하지 않는다.
#                  **경로를 처음 뚫을 때는 staging 부터 돌려라** — Let's Encrypt 는
#                  등록 도메인당 주 50건이라 시행착오로 태우면 일주일을 기다린다.
#   CERT_MANAGER_VERSION (기본: v1.16.2)
#
#   DNS01_ZONE     DNS-01 로 검증할 존   (기본: 없음 = 전부 HTTP-01, 종전 동작)
#                  예: nullus.io
#   DNS01_PROVIDER lego 프로바이더 이름  (기본: spaceship)
#   DNS01_SECRET   자격증명 시크릿 이름  (기본: spaceship-dns01, cert-manager ns)
#   LEGO_WEBHOOK_VERSION  lego 웹훅 차트 버전 (기본: 1.4.0)
#
# 종료 코드: 0 성공 / 1 실패
# =============================================================================
set -euo pipefail

NAMESPACE="${NAMESPACE:-nullus}"
ACME_EMAIL="${ACME_EMAIL:-admin@nullus.io}"
ISSUER="${ISSUER:-letsencrypt}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.16.2}"

DNS01_ZONE="${DNS01_ZONE:-}"
DNS01_PROVIDER="${DNS01_PROVIDER:-spaceship}"
DNS01_SECRET="${DNS01_SECRET:-spaceship-dns01}"
# 차트를 고정한다. 떠 있으면 어제 통하던 발급이 오늘 조용히 깨진다.
LEGO_WEBHOOK_VERSION="${LEGO_WEBHOOK_VERSION:-1.4.0}"
LEGO_WEBHOOK_REPO="https://yxwuxuanl.github.io/cert-manager-lego-webhook/"

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

# cert-manager 가 내장한 DNS-01 솔버는 Route53/Cloudflare/DigitalOcean/AzureDNS/
# acme-dns/RFC2136 뿐이고 Spaceship 은 없다. 반면 lego 는 v4.22.0 부터 spaceship 을
# 1급 프로바이더로 지원한다(차트 1.4.0 이 lego v4.30.1 을 벤더). 그래서 lego 를
# 감싼 웹훅을 붙인다 — _acme-challenge 를 남의 존으로 위임할 필요도, 네임서버를
# 옮길 필요도 없다.
ensure_dns01_webhook() {
  command -v helm >/dev/null || die "DNS-01 에는 helm 이 필요합니다"

  # 자격증명은 이 스크립트가 만들지 않는다. API 키가 코드나 셸 히스토리, 로그를
  # 통과하지 않게 하려는 것이다. 사람이 직접 넣고, 우리는 있는지만 본다.
  if ! "${K[@]}" -n cert-manager get secret "$DNS01_SECRET" >/dev/null 2>&1; then
    printf '%s✘%s 시크릿 cert-manager/%s 가 없습니다.\n\n' "$C_ERR" "$C_RST" "$DNS01_SECRET" >&2
    printf '  Spaceship > API Manager 에서 domains:read, dnsrecords:read,\n' >&2
    printf '  dnsrecords:write 스코프로 키를 발급한 뒤:\n\n' >&2
    printf '    kubectl -n cert-manager create secret generic %s \\\n' "$DNS01_SECRET" >&2
    printf "      --from-literal=SPACESHIP_API_KEY='...' \\\\\\n" >&2
    printf "      --from-literal=SPACESHIP_API_SECRET='...'\n\n" >&2
    exit 1
  fi

  # 키 **이름**이 lego 환경변수 이름과 정확히 같아야 한다. 웹훅은 시크릿의 키를
  # 그 이름의 환경변수로 그대로 주입하므로, 이름이 다르면 lego 가 자격증명을 못
  # 찾는다. 그때 챌린지는 에러 없이 pending 으로 굳어 원인이 보이지 않는다.
  if [[ "$DNS01_PROVIDER" == "spaceship" ]]; then
    local keys missing=()
    keys="$("${K[@]}" -n cert-manager get secret "$DNS01_SECRET" -o jsonpath='{.data}' 2>/dev/null || echo '')"
    for key in SPACESHIP_API_KEY SPACESHIP_API_SECRET; do
      [[ "$keys" == *"\"${key}\""* ]] || missing+=("$key")
    done
    if [[ ${#missing[@]} -gt 0 ]]; then
      die "시크릿 cert-manager/${DNS01_SECRET} 에 키가 없습니다: ${missing[*]} (키 이름은 lego 환경변수 이름 그대로여야 합니다)"
    fi
  fi

  info "lego DNS-01 웹훅 설치 (chart ${LEGO_WEBHOOK_VERSION})"
  helm repo add cert-manager-lego-webhook "$LEGO_WEBHOOK_REPO" >/dev/null 2>&1 || true
  helm repo update cert-manager-lego-webhook >/dev/null 2>&1 || true
  helm upgrade --install cert-manager-lego-webhook \
    cert-manager-lego-webhook/cert-manager-lego-webhook \
    --version "$LEGO_WEBHOOK_VERSION" \
    --namespace cert-manager \
    --set certManager.namespace=cert-manager \
    --set certManager.serviceAccount.name=cert-manager \
    --wait --timeout 5m >/dev/null || die "lego 웹훅 설치 실패"
  ok "lego DNS-01 웹훅 준비 (provider=${DNS01_PROVIDER}, zone=${DNS01_ZONE})"
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

  # HTTP-01 은 항상 남긴다. nip.io 호스트처럼 와일드카드가 덮지 못하는 이름이
  # 아직 ingress 에 있고, 그것까지 DNS-01 로 보내면 그 존의 TXT 를 우리가 쓸 수
  # 없어 발급이 통째로 막힌다.
  local solvers
  solvers='      - http01:
          ingress:
            class: nginx'

  if [[ -n "$DNS01_ZONE" ]]; then
    ensure_dns01_webhook
    # dnsZones 는 존과 그 하위 이름(와일드카드 포함)에 걸린다. cert-manager 가
    # 더 구체적인 selector 를 우선하므로 위의 HTTP-01 은 나머지 이름에만 남는다.
    solvers="${solvers}
      - dns01:
          webhook:
            groupName: lego.dns-solver
            solverName: lego-solver
            config:
              provider: ${DNS01_PROVIDER}
              envFrom:
                secret:
                  name: ${DNS01_SECRET}
                  namespace: cert-manager
        selector:
          dnsZones:
            - ${DNS01_ZONE}"
    info "ClusterIssuer ${ISSUER} 생성 (${DNS01_ZONE} → DNS-01/${DNS01_PROVIDER}, 그 외 → HTTP-01)"
  else
    info "ClusterIssuer ${ISSUER} 생성 (HTTP-01, ingress class nginx)"
  fi

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
${solvers}
EOF
  ok "ClusterIssuer ${ISSUER} 준비"
  echo
  if [[ -n "$DNS01_ZONE" ]]; then
    info "다음: $0 wildcard 로 *.${DNS01_ZONE} 인증서를 요청하십시오."
  else
    info "다음: values-zadara.yaml 의 ingress.tls 를 켜고 helm upgrade 를 돌리십시오."
  fi
  info "발급 확인: $0 status"
}

# 와일드카드 한 장이 www/auth/argocd/grafana/harbor… 를 전부 덮는다. 호스트마다
# 인증서를 두던 방식은 도구를 하나 늘릴 때마다 발급이 하나 늘고, 그중 하나가
# 실패하면 그 도구만 조용히 접속 불가가 된다.
do_wildcard() {
  [[ -n "$DNS01_ZONE" ]] || die "DNS01_ZONE 이 필요합니다 (예: DNS01_ZONE=nullus.io $0 wildcard)"

  info "Certificate ${NAMESPACE}/nullus-wildcard-tls 요청 (*.${DNS01_ZONE}, ${DNS01_ZONE})"
  "${K[@]}" apply -f - >/dev/null <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: nullus-wildcard
  namespace: ${NAMESPACE}
spec:
  secretName: nullus-wildcard-tls
  issuerRef:
    name: ${ISSUER}
    kind: ClusterIssuer
  dnsNames:
    # 와일드카드는 apex 를 덮지 않는다. 둘 다 적어야 nullus.io 자체도 이 한 장에
    # 들어간다.
    - "*.${DNS01_ZONE}"
    - "${DNS01_ZONE}"
EOF
  ok "Certificate 요청됨 — 발급까지 수 분 걸립니다"
  info "진행 확인: $0 status"
}

do_status() {
  printf '· cert-manager ... '
  if "${K[@]}" get deploy -n cert-manager cert-manager >/dev/null 2>&1; then echo "설치됨"; else echo "없음"; return; fi
  if [[ -n "$DNS01_ZONE" ]]; then
    printf '· lego DNS-01 웹훅 ... '
    if "${K[@]}" -n cert-manager get deploy cert-manager-lego-webhook >/dev/null 2>&1; then echo "설치됨"; else echo "없음"; fi
  fi
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
  if command -v helm >/dev/null; then
    helm uninstall cert-manager-lego-webhook -n cert-manager >/dev/null 2>&1 || true
  fi
  "${K[@]}" delete -f \
    "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml" \
    --ignore-not-found >/dev/null 2>&1 || true
  # 자격증명 시크릿은 남긴다. 재설치 때 다시 발급받게 만들 이유가 없다.
  ok "cert-manager 제거 (인증서·자격증명 시크릿은 남는다 — 필요하면 직접 삭제)"
}

case "${1:-status}" in
  install)   do_install ;;
  wildcard)  do_wildcard ;;
  status)    do_status ;;
  uninstall) do_uninstall ;;
  *)         die "사용법: $0 [install|wildcard|status|uninstall]" ;;
esac
