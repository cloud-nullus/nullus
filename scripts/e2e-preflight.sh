#!/usr/bin/env bash
# E2E 사전 점검 — 시나리오 기록지(docs/60_테스트)의 발견 F5를 스크립트화한 것.
#
# 환경을 등급(tier)으로 판정한다. 회귀 스펙(web/e2e/tour-regression.spec.ts)은
# 화면 상태를 보고 스스로 건너뛰므로 이 스크립트가 없어도 돌지만, "왜 스킵됐는지"
# 는 여기서 먼저 보는 편이 빠르다.
#
#   T0  인프라(compose) + API + web        — 읽기 전용 시나리오(S0~S4·S8·S9)의 전제
#   T1  + kind 듀얼 클러스터               — 클러스터 의존 단정(S2 Connected·S4 선택지)의 전제
#   T2  + 도구 스택(gitea·jenkins·argocd)  — S6 workloads·CI/CD 프로비저닝의 전제
#
# 사용: ./scripts/e2e-preflight.sh
# 포트가 다르면: NULLUS_API_URL=http://localhost:8090 NULLUS_WEB_URL=http://localhost:5173 ./scripts/e2e-preflight.sh
# 종료 코드: T0 충족=0, 미충족=1 (T1·T2 부족은 실패가 아니라 등급으로만 보고)
set -uo pipefail

API_URL="${NULLUS_API_URL:-http://localhost:8091}"
WEB_URL="${NULLUS_WEB_URL:-http://localhost:5174}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dev.yaml}"
KIND_CONTEXT="${KIND_CONTEXT:-kind-nullus-platform}"
STACK_NAMESPACE="${STACK_NAMESPACE:-nullus}"

ok()   { printf '  ✅ %s\n' "$1"; }
bad()  { printf '  ❌ %s\n' "$1"; }
warn() { printf '  ⚠️  %s\n' "$1"; }

t0_fail=0

echo "── T0: 인프라 + API + web ──────────────────────────"

# compose 인프라 4종 — kind 격동 중 docker 데몬 재시작으로 조용히 내려갈 수 있다(F5).
for svc in postgres redis keycloak minio; do
  if docker compose -f "$COMPOSE_FILE" ps "$svc" 2>/dev/null | grep -q "Up"; then
    ok "compose $svc Up"
  else
    bad "compose $svc 내려감 → make dev-up"
    t0_fail=1
  fi
done

health="$(curl -s --max-time 3 "$API_URL/health" 2>/dev/null)"
if printf '%s' "$health" | grep -q '"db":"connected"'; then
  ok "API $API_URL/health db=connected"
else
  bad "API $API_URL 응답 없음/비정상 (${health:-no response}) → 수행 가이드 §2"
  t0_fail=1
fi

if curl -s -o /dev/null -w '%{http_code}' --max-time 3 "$WEB_URL/" 2>/dev/null | grep -q "200"; then
  ok "web $WEB_URL 응답 200"
else
  bad "web $WEB_URL 응답 없음 → cd web && npm run dev -- --port ${WEB_URL##*:} --strictPort"
  t0_fail=1
fi

echo "── T1: kind 클러스터 ───────────────────────────────"

tier=0
kind_clusters="$(kind get clusters 2>/dev/null || true)"
if printf '%s\n' "$kind_clusters" | grep -q "nullus-platform"; then
  ok "kind: $(printf '%s' "$kind_clusters" | tr '\n' ' ')"
  tier=1
else
  warn "kind 클러스터 없음 → ./scripts/runbook_local.sh kind-up (클러스터 의존 단정은 스킵됨)"
  # Rancher Desktop VM 은 inotify 기본값(128)으로는 kind 노드가 부팅하지 못한다(F5).
  # 클러스터를 올리기 전에만 의미 있는 점검이라 T1 부재 시에만 본다.
  if command -v rdctl >/dev/null 2>&1; then
    inotify="$(rdctl shell sysctl -n fs.inotify.max_user_instances 2>/dev/null | tr -d '[:space:]')"
    if [[ -n "$inotify" && "$inotify" -lt 1024 ]]; then
      warn "inotify max_user_instances=$inotify (<1024) — kind-up 전에 상향 필요:"
      warn "  rdctl shell sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576"
    elif [[ -n "$inotify" ]]; then
      ok "inotify max_user_instances=$inotify"
    fi
  fi
fi

echo "── T2: 도구 스택 + port-forward ────────────────────"

if [[ "$tier" -ge 1 ]]; then
  pods="$(kubectl --context "$KIND_CONTEXT" get pods -n "$STACK_NAMESPACE" --no-headers 2>/dev/null || true)"
  tools_running=0
  for tool in gitea jenkins argocd; do
    if printf '%s\n' "$pods" | grep "$tool" | grep -q "Running"; then
      tools_running=$((tools_running + 1))
    fi
  done
  if [[ "$tools_running" -eq 3 ]]; then
    ok "도구 파드 3종(gitea·jenkins·argocd) Running"
    tier=2
  else
    warn "도구 파드 ${tools_running}/3 Running — 스택 미배포면 S6 workloads 단정은 스킵됨"
  fi

  # CI/CD 프로비저닝은 API 가 PF 로 도구에 닿아야 한다 (수행 가이드 §2).
  for pf in "gitea:3100" "jenkins:8480"; do
    name="${pf%%:*}"; port="${pf##*:}"
    if curl -s -o /dev/null --max-time 2 "http://localhost:$port" 2>/dev/null; then
      ok "PF $name localhost:$port 응답"
    else
      warn "PF $name localhost:$port 없음 (CI/CD 프로비저닝 테스트 시에만 필요)"
    fi
  done
else
  warn "kind 부재로 T2 점검 생략"
fi

echo "────────────────────────────────────────────────────"
if [[ "$t0_fail" -ne 0 ]]; then
  echo "TIER=X — T0 미충족: 위 ❌ 를 해소해야 회귀 스펙을 돌릴 수 있다"
  exit 1
fi
echo "TIER=$tier"
