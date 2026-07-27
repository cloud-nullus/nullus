#!/usr/bin/env bash
# =============================================================================
# 25-port-forward.sh — Envoy Gateway 데이터플레인 포트포워딩 (외부 접근 진입점)
# =============================================================================
# 용도: kind 에는 실 LoadBalancer/포트맵(443)이 없으므로, envoy 게이트웨이
#       데이터플레인 서비스를 로컬 포트로 포워딩해 모든 *.nullus.internal /
#       nullus.internal 도메인 접근의 단일 진입점을 만든다.
#       (Gateway 가 호스트네임으로 분기하므로 포워딩은 1개면 충분하다.)
#
# 사용법:
#   ./25-port-forward.sh                # 127.0.0.1:8443 → gateway:443 (비특권, sudo 불필요)
#   PORT=443 ./25-port-forward.sh       # 127.0.0.1:443 (특권포트 → sudo 자동 사용)
#   ADDRESS=0.0.0.0 ./25-port-forward.sh  # 외부 인터페이스에도 바인드(원격 접근)
#   BACKGROUND=1 ./25-port-forward.sh   # 백그라운드 실행(.runlog 로그)
#
# 환경 변수:
#   PORT        로컬 포트 (기본: 8443). 1024 미만이면 sudo 사용.
#   ADDRESS     바인드 주소 (기본: 127.0.0.1)
#   GATEWAY_NS  Gateway 네임스페이스 (기본: nullus)
#   GATEWAY_NAME Gateway 이름 (기본: nullus-gateway)
#   BACKGROUND  1 = 백그라운드 실행
#
# 포그라운드(기본) 실행 시 Ctrl+C 로 종료. 그동안 브라우저로 도메인 접근.
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

PORT="${PORT:-8443}"
ADDRESS="${ADDRESS:-127.0.0.1}"
GATEWAY_NS="${GATEWAY_NS:-nullus}"
GATEWAY_NAME="${GATEWAY_NAME:-nullus-gateway}"
BACKGROUND="${BACKGROUND:-0}"

command -v kubectl >/dev/null || { log_err "kubectl 없음"; exit 1; }
kubectl cluster-info >/dev/null 2>&1 || { log_err "클러스터 접근 불가"; exit 1; }

# envoy 데이터플레인 서비스 탐지 (owning-gateway 라벨)
ENVOY_SVC="$(kubectl get svc -n "$GATEWAY_NS" \
  -l "gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME}" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
if [[ -z "$ENVOY_SVC" ]]; then
  log_err "Gateway 데이터플레인 서비스를 못 찾음 — 먼저 23-setup-gateway.sh 실행 필요"
  exit 1
fi
log_info "데이터플레인: ${GATEWAY_NS}/${ENVOY_SVC}"

# 특권포트면 sudo
SUDO=""
if [[ "$PORT" -lt 1024 && "$(id -u)" != "0" ]]; then
  SUDO="sudo"
  log_info "포트 $PORT 는 특권포트 — sudo 사용 (비밀번호 요구될 수 있음)"
fi

# 기존 동일 포워딩 정리
pkill -f "port-forward.*${ENVOY_SVC}.*${PORT}:443" 2>/dev/null || true

PF_CMD=(kubectl port-forward -n "$GATEWAY_NS" "svc/${ENVOY_SVC}" "${PORT}:443" --address "$ADDRESS")

# 접근 URL 안내
suffix=""; [[ "$PORT" != "443" ]] && suffix=":${PORT}"
echo ""
echo "============================================================"
echo " 외부 접근 진입점: ${ADDRESS}:${PORT} → ${ENVOY_SVC}:443 (HTTPS)"
echo "============================================================"
echo "  포털 : https://nullus.internal${suffix}/"
echo "  ArgoCD: https://argocd.nullus.internal${suffix}/"
echo "  (그 외 등록된 *.nullus.internal 도메인 모두 동일 진입점)"
echo "  ※ 자체서명 인증서: 브라우저에서 최초 접속 시 예외 허용 필요"
echo "============================================================"

if [[ "$BACKGROUND" == "1" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  RUNLOG="${SCRIPT_DIR}/../.runlog"; mkdir -p "$RUNLOG"
  LOG="${RUNLOG}/port-forward-${PORT}.log"
  nohup ${SUDO:+$SUDO} "${PF_CMD[@]}" >"$LOG" 2>&1 &
  log_ok "백그라운드 포워딩 시작 (PID $!), 로그: $LOG"
  log_info "종료:  pkill -f 'port-forward.*${ENVOY_SVC}'"
else
  log_info "포그라운드 실행 — 종료하려면 Ctrl+C"
  exec ${SUDO:+$SUDO} "${PF_CMD[@]}"
fi
