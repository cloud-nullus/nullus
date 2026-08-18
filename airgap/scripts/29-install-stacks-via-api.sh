#!/usr/bin/env bash
# =============================================================================
# 29-install-stacks-via-api.sh — Nullus 백엔드 API 로 스택 설치
# =============================================================================
# 27-install-stacks.sh 는 helm 을 직접 호출해 백엔드를 우회한다. 그 결과
# 설치 구현이 둘로 갈라져 에어갭 경로와 일반 경로가 서로 다른 결과를 낼 수 있다.
#
# 이 스크립트는 백엔드 API 를 호출해 설치 구현을 하나로 통일한다.
# OpenBao init/unseal → Kubernetes Auth → ESO → 시크릿 프로비저닝 → SSO 까지
# 백엔드의 설치 파이프라인이 그대로 수행된다.
#
# 사용법:
#   ./29-install-stacks-via-api.sh
#
# 환경 변수:
#   NULLUS_API        백엔드 주소 (기본: http://127.0.0.1:18080/api/v1)
#   KEYCLOAK_URL      Keycloak 주소 — 부트스트랩 자격 발급에 사용
#   NULLUS_TOKEN      직접 지정 시 부트스트랩 발급을 건너뜀
#   ORG_ID            조직 ID (미지정 시 첫 조직 사용)
#   TEMPLATE_ID       설치할 골든패스 템플릿 (기본: gitea-jenkins-argocd-v1)
#   STACK_NAME        스택 이름 (기본: airgap-stack)
#   STACK_NAMESPACE   설치 네임스페이스 (기본: nullus)
#   ACCESS_DOMAIN     접속 도메인 (기본: nullus.internal)
#   STORAGE_CLASS     PVC StorageClass (미지정 시 클러스터 기본값)
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

NULLUS_API="${NULLUS_API:-http://127.0.0.1:18080/api/v1}"
TEMPLATE_ID="${TEMPLATE_ID:-gitea-jenkins-argocd-v1}"
STACK_NAME="${STACK_NAME:-airgap-stack}"
STACK_NAMESPACE="${STACK_NAMESPACE:-nullus}"
ACCESS_DOMAIN="${ACCESS_DOMAIN:-nullus.internal}"
STORAGE_CLASS="${STORAGE_CLASS:-}"

log() { printf '[INFO] %s\n' "$*" >&2; }
die() { printf '[ERR ] %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null || die "curl 이 필요합니다"
command -v python3 >/dev/null || die "python3 이 필요합니다"

# --- 0) 부트스트랩 자격 -----------------------------------------------------
# Admin API 는 인증을 요구하는데 무인 설치에는 로그인할 사람이 없다.
# Keycloak service account 를 잠깐 만들어 토큰을 받고, 설치가 끝나면 폐기한다.
#
# 정책은 "폐기 + 멱등 재발급" — 쓰지 않는 admin 자격을 번들·로그에 남기지 않되,
# 재실행 시 마찰 없이 다시 발급된다.
BOOTSTRAP_BIN="${BOOTSTRAP_BIN:-nullus-bootstrap}"
BOOTSTRAP_ISSUED=0

cleanup_bootstrap() {
  if [[ "${BOOTSTRAP_ISSUED}" == "1" ]]; then
    log "부트스트랩 자격 폐기"
    "${BOOTSTRAP_BIN}" revoke || log "폐기 실패 — 수동으로 확인하세요"
  fi
}
trap cleanup_bootstrap EXIT

if [[ -z "${NULLUS_TOKEN:-}" ]]; then
  if command -v "${BOOTSTRAP_BIN}" >/dev/null; then
    [[ -n "${KEYCLOAK_URL:-}" ]] || die "KEYCLOAK_URL 이 필요합니다 (또는 NULLUS_TOKEN 을 직접 지정)"
    log "부트스트랩 자격 발급"
    NULLUS_TOKEN="$("${BOOTSTRAP_BIN}" issue)" || die "부트스트랩 토큰 발급 실패"
    export NULLUS_TOKEN
    BOOTSTRAP_ISSUED=1
  else
    die "${BOOTSTRAP_BIN} 를 찾을 수 없습니다. NULLUS_TOKEN 을 직접 지정하거나 바이너리를 설치하세요"
  fi
fi

api() {
  local method="$1" path="$2" body="${3:-}"
  # 헤더는 배열로 전달한다. ${VAR:+...} 를 비인용 확장하면 단어 분리로
  # "Authorization: Bearer x" 가 여러 인자로 쪼개져 헤더가 깨진다.
  local -a args=(-fsS -X "$method" "${NULLUS_API}${path}")
  if [[ -n "${NULLUS_TOKEN:-}" ]]; then
    args+=(-H "Authorization: Bearer ${NULLUS_TOKEN}")
  fi
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi
  curl "${args[@]}"
}

jsonget() { python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)"; }

# --- 1) 조직 확인 -----------------------------------------------------------
if [[ -z "${ORG_ID:-}" ]]; then
  log "조직 조회"
  ORG_ID="$(api GET /admin/organization | jsonget "['id']")" \
    || die "조직을 찾지 못했습니다. ORG_ID 를 지정하세요"
fi
log "조직: ${ORG_ID}"

# --- 2) 자기 클러스터 등록 --------------------------------------------------
# 에어갭에서는 Nullus 가 자기가 떠 있는 클러스터에 설치한다.
# kubeconfig 업로드 없이 파드의 ServiceAccount 로 등록한다.
log "자기 클러스터 등록"
CLUSTER_ID="$(api POST /admin/clusters/self-register \
  "$(printf '{"name":"nullus-self","org_id":"%s"}' "${ORG_ID}")" \
  | jsonget "['id']")" || die "자기 클러스터 등록 실패"
log "클러스터: ${CLUSTER_ID}"

# --- 3) 스택 생성 -----------------------------------------------------------
log "스택 생성 (템플릿: ${TEMPLATE_ID})"
# 도구 선택은 템플릿 응답에서 가져온다. 여기에 차트 버전 표를 복사해 두면
# 마이그레이션이 버전을 올릴 때마다 에어갭 경로만 낡는다.
TEMPLATES_JSON="$(api GET /stacks/templates)" || die "템플릿 목록 조회 실패"

# heredoc 안의 python 이 환경변수를 읽으므로 먼저 export 한다.
export CLUSTER_ID STACK_NAME STACK_NAMESPACE ACCESS_DOMAIN STORAGE_CLASS TEMPLATE_ID
STACK_PAYLOAD="$(printf '%s' "${TEMPLATES_JSON}" | python3 - <<'PY'
import json, os, sys

SLOTS = {
    "package_registry":         ("artifacts",  "package_registry"),
    "source_repository":        ("artifacts",  "source_repository"),
    "container_registry":       ("artifacts",  "container_registry"),
    "storage_backend":          ("artifacts",  "storage_backend"),
    "ci_platform":              ("pipeline",   "ci_platform"),
    "cd_tool":                  ("pipeline",   "cd_tool"),
    "monitoring_collection":    ("monitoring", "collection"),
    "monitoring_visualization": ("monitoring", "visualization"),
    "log_search":               ("logging",    "search"),
    "trace_layer":              ("logging",    "trace_layer"),
    "agent":                    ("logging",    "trace_exporter"),
}

data = json.load(sys.stdin)
items = data.get("items", data) if isinstance(data, dict) else data
wanted = os.environ["TEMPLATE_ID"]
template = next((t for t in items if t.get("id") == wanted), None)
if template is None:
    sys.stderr.write("template %s not found\n" % wanted)
    sys.exit(1)

cfg = {"artifacts": {}, "pipeline": {}, "monitoring": {}, "logging": {}}
for section, field in SLOTS.values():
    cfg[section][field] = {"name": "", "version": "", "enabled": False}

for tool in template.get("tools") or []:
    slot = SLOTS.get(tool.get("category"))
    if slot is None:
        continue
    section, field = slot
    cfg[section][field] = {
        "name": tool.get("name", ""),
        "version": tool.get("app_version", ""),
        "enabled": True,
    }

cfg["access_domain"] = os.environ["ACCESS_DOMAIN"]
cfg["authentication"] = {"provider": "openbao"}

# storage 는 부분만 채우면 검증에서 400 이 난다 — integrated-create 는
# database/object_storage 가 둘 다 mode=create 이어야 하고, create 모드는
# provider_or_engine 과 size(Gi, >0)를 요구한다.
cfg["storage"] = {
    "plan_mode": "integrated-create",
    "database": {
        "mode": "create",
        "resource_name": "nullus",
        "provider_or_engine": "postgres",
        "version": "17",
        "size": 50,
    },
    "object_storage": {
        "mode": "create",
        "resource_name": "nullus-artifacts",
        "provider_or_engine": "minio",
        "version": "latest",
        "size": 100,
    },
}

sc = os.environ.get("STORAGE_CLASS", "").strip()
if sc:
    cfg["storage"]["storage_class"] = sc

print(json.dumps({
    "name": os.environ["STACK_NAME"],
    "golden_path_id": wanted,
    "cluster_id": os.environ["CLUSTER_ID"],
    "namespace": os.environ["STACK_NAMESPACE"],
    "config": cfg,
}))
PY
)"
STACK_ID="$(api POST /stacks "${STACK_PAYLOAD}" | jsonget "['id']")" \
  || die "스택 생성 실패"
log "스택: ${STACK_ID}"

# --- 4) 배포 시작 -----------------------------------------------------------
# Pre-Deploy Gate 가 warn 을 내면 명시적 동의 없이는 DEPLOY_COMPAT_WARN_UNACK 로
# 막힌다. 무인 설치에는 동의할 사람이 없으므로 동의를 실어 보낸다 — block 판정은
# 이 값과 무관하게 그대로 막힌다.
log "배포 시작 — 백엔드 파이프라인이 OpenBao/ESO/SSO 까지 수행합니다"
api POST "/stacks/${STACK_ID}/deploy" '{"acknowledge_warnings":true}' >/dev/null \
  || die "배포 요청 실패"

log "완료. 진행 상황은 Nullus UI 또는 GET /stacks/${STACK_ID} 로 확인하세요."
