#!/usr/bin/env bash
# =============================================================================
# sso-access.sh — kind(nullus-airgap) SSO 브라우저 접속용 게이트웨이 포트포워딩
# =============================================================================
# kind 에는 실 LoadBalancer 가 없어 Envoy Gateway 데이터플레인을 로컬 포트로
# 포워딩해야 브라우저로 *.nullus.internal 에 접근할 수 있다.
#
# ⚠️ SSO(OIDC) 무재인증 로그인은 반드시 :80 이어야 한다.
#    OIDC redirect 가 http://keycloak.nullus.internal (포트 없음 = :80) 으로 가므로
#    :8080 으로 띄우면 포털은 보여도 로그인 버튼에서 연결거부가 난다.
#
# 사용법:
#   sudo ./scripts/sso-access.sh           # :80 (권장, SSO 완전동작)
#   PORT=8080 ./scripts/sso-access.sh      # :8080 (sudo 불필요, 포털 미리보기용; SSO 미완성)
#   /etc/hosts 등록이 안 됐다면 먼저: sudo bash airgap/scripts/24-register-hosts.sh
#
# 환경변수:
#   PORT          로컬 포트 (기본 80). 1024 미만이면 sudo 로 실행해야 함.
#   GATEWAY_NS    게이트웨이 네임스페이스 (기본 nullus)
#   CONTEXT       kubectl context (기본 현재 context)
# =============================================================================
set -euo pipefail

PORT="${PORT:-80}"
GATEWAY_NS="${GATEWAY_NS:-nullus}"
# bash 3.2(macOS) + set -u 에서 빈 배열 확장이 깨지므로 단일 변수로 처리
KCTX="${CONTEXT:+--context=$CONTEXT}"

c_ok=$'\033[1;32m'; c_warn=$'\033[1;33m'; c_err=$'\033[1;31m'; c_rst=$'\033[0m'
[[ -t 1 ]] || { c_ok=""; c_warn=""; c_err=""; c_rst=""; }

command -v kubectl >/dev/null || { echo "${c_err}kubectl 없음${c_rst}" >&2; exit 1; }
kubectl "${KCTX[@]}" cluster-info >/dev/null 2>&1 || { echo "${c_err}클러스터 접근 불가 (kind 클러스터 기동 확인)${c_rst}" >&2; exit 1; }

# 권한 포트 + 비root 면 안내
if [[ "$PORT" -lt 1024 && "$(id -u)" -ne 0 ]]; then
  echo "${c_err}PORT=$PORT 은 특권 포트입니다. sudo 로 실행하세요:${c_rst}" >&2
  echo "  sudo PORT=$PORT $0" >&2
  exit 1
fi

# Envoy Gateway 데이터플레인 svc 자동탐지 (owning-gateway 라벨 → 폴백 이름매칭)
ENVOY_SVC="$(kubectl "${KCTX[@]}" get svc -n "$GATEWAY_NS" \
  -l 'gateway.envoyproxy.io/owning-gateway-name=nullus-gateway' \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
if [[ -z "$ENVOY_SVC" ]]; then
  ENVOY_SVC="$(kubectl "${KCTX[@]}" get svc -n "$GATEWAY_NS" -o name 2>/dev/null \
    | grep -m1 envoy-nullus | sed 's#service/##' || true)"
fi
[[ -z "$ENVOY_SVC" ]] && { echo "${c_err}Envoy Gateway svc 미발견 (게이트웨이 설치 확인: airgap/scripts/23-setup-gateway.sh)${c_rst}" >&2; exit 1; }

cat <<EOF
${c_ok}── Nullus SSO 접속 ─────────────────────────────────${c_rst}
게이트웨이 svc : $ENVOY_SVC (ns=$GATEWAY_NS)
로컬 포트      : $PORT  $( [[ "$PORT" == "80" ]] && echo "(SSO 완전동작)" || echo "${c_warn}(SSO 미완성 — :80 권장)${c_rst}" )

접속 URL$( [[ "$PORT" == "80" ]] || echo " (포트 :$PORT 명시)" ):
  포털     http://nullus.internal$( [[ "$PORT" == "80" ]] || echo ":$PORT" )/
  Grafana  http://grafana.nullus.internal$( [[ "$PORT" == "80" ]] || echo ":$PORT" )/
  ArgoCD   http://argocd.nullus.internal/
  Harbor   http://harbor.nullus.internal/
  GitLab   http://gitlab.nullus.internal/
  MinIO    http://minio.nullus.internal/
  Prom.    http://prometheus.nullus.internal/
  OpenSrch http://opensearch.nullus.internal/
  Keycloak http://keycloak.nullus.internal/   (admin/admin)

테스트 계정 : admin@nullus.io / nullus123!   (또는 dev@nullus.io)

Ctrl+C 로 종료. 이 창은 켜둔 채 브라우저로 접속하세요.
${c_ok}────────────────────────────────────────────────────${c_rst}
EOF

exec kubectl "${KCTX[@]}" port-forward -n "$GATEWAY_NS" "svc/$ENVOY_SVC" "${PORT}:80" --address 127.0.0.1
