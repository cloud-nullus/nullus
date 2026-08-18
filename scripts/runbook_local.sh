#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/.runbook-logs"
PID_FILE="$LOG_DIR/pids.txt"
DB_URL="postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

API_PORT=8090
WEB_PORT=5173
POSTGRES_PORT=5433
MINIO_PORT=9000
MINIO_CONSOLE_PORT=9001
REDIS_PORT=6380
KEYCLOAK_PORT=8180
# 로컬 Keycloak 의 부트스트랩 관리자. docker-compose.dev.yaml 의
# KC_BOOTSTRAP_ADMIN_USERNAME / KC_BOOTSTRAP_ADMIN_PASSWORD 와 같아야 한다.
KEYCLOAK_ADMIN_USER="${KEYCLOAK_ADMIN_USER:-admin}"
KEYCLOAK_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin}"
# setup-keycloak.sh 가 테스트 유저에게 심는 비밀번호. 요약 출력에만 쓴다.
DEFAULT_KC_PASSWORD_HINT="${KEYCLOAK_TEST_USER_PASSWORD:-nullus123!}"

ENCRYPTION_KEY="${ENCRYPTION_KEY:-nullus-dev-key-32bytes-padding!!}"
OPENBAO_ADDR="${OPENBAO_ADDR:-http://openbao.nullus.internal}"
OPENBAO_TOKEN="${OPENBAO_TOKEN:-root}"
KIND_CONFIG="$PROJECT_ROOT/scripts/kind-cluster.yaml"
AUTHENTIK_PORT=9090
COMPOSE_AUTH="$PROJECT_ROOT/docker-compose.auth.yaml"

# OIDC provider 선택: --auth=<keycloak|authentik|none> 또는 NULLUS_AUTH_PROVIDER env
AUTH_PROVIDER="${NULLUS_AUTH_PROVIDER:-keycloak}"

validate_auth_provider() {
  case "$AUTH_PROVIDER" in
    keycloak|authentik|none) ;;
    *)
      echo "[nullus] invalid auth provider: '$AUTH_PROVIDER' (allowed: keycloak | authentik | none)"
      exit 1
      ;;
  esac
}

# 공통 --auth 인자 파서: 매칭하면 0, 아니면 1
parse_auth_arg() {
  case "$1" in
    --auth=*) AUTH_PROVIDER="${1#--auth=}"; return 0 ;;
    --authentik) AUTH_PROVIDER="authentik"; return 0 ;;  # deprecated alias
    *) return 1 ;;
  esac
}

kind_cluster_exists() {
  local name="$1"
  kind get clusters 2>/dev/null | grep -q "^${name}$"
}

kind_cluster_names() {
  if [[ -f "$KIND_CONFIG" ]]; then
    awk '/^name:[[:space:]]+/ { print $2 }' "$KIND_CONFIG"
  fi
}

kind_print_status() {
  local has_cluster="false"
  if ! command -v kind >/dev/null 2>&1; then
    return 0
  fi

  while IFS= read -r cluster_name; do
    [[ -z "$cluster_name" ]] && continue
    if kind_cluster_exists "$cluster_name"; then
      has_cluster="true"
      echo "  K8s Cluster   kind-$cluster_name ($(kubectl get nodes --context "kind-$cluster_name" -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}' 2>/dev/null || echo 'unknown'))"
    fi
  done < <(kind_cluster_names)

  [[ "$has_cluster" == "true" ]] && echo ""
  return 0
}

register_kind_cluster_endpoints() {
  command -v kind >/dev/null 2>&1 || return 0

  while IFS= read -r cluster_name; do
    [[ -z "$cluster_name" ]] && continue
    if ! kind_cluster_exists "$cluster_name"; then
      continue
    fi

    local kind_endpoint
    kind_endpoint="$(kubectl config view --context "kind-${cluster_name}" --minify --raw -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null)"
    if [[ -z "$kind_endpoint" ]]; then
      continue
    fi

    echo "[nullus] registering kind cluster endpoint for kind-${cluster_name}: ${kind_endpoint}"
    docker exec draft-postgres-1 psql -U nullus -d nullus -c \
      "UPDATE clusters SET endpoint = '${kind_endpoint}' WHERE name = 'kind-${cluster_name}';" >/dev/null 2>&1 || true
  done < <(kind_cluster_names)
}

auto_register_kind_clusters() {
  local register_script="$PROJECT_ROOT/scripts/register-kind-clusters.sh"
  if [[ ! -x "$register_script" ]]; then
    chmod +x "$register_script" 2>/dev/null || true
  fi

  if [[ -x "$register_script" ]]; then
    echo "[nullus] auto-registering kind clusters in Nullus..."
    NULLUS_API="http://localhost:${API_PORT}" "$register_script" || {
      echo "[nullus] kind cluster auto-registration failed (continuing...)"
    }
  else
    echo "[nullus] register-kind-clusters.sh is not executable; skipping auto-registration"
  fi
}

seed_golden_path_templates_if_needed() {
  local count
  count="$(docker exec draft-postgres-1 psql -U nullus -d nullus -tA -c "SELECT COUNT(*) FROM golden_path_templates;" 2>/dev/null | tr -d '[:space:]' || true)"
  if [[ -z "$count" || "$count" == "0" ]]; then
    echo "[nullus] seeding golden_path_templates..."
    docker exec -i draft-postgres-1 psql -U nullus -d nullus < "$PROJECT_ROOT/db/migrations/000008_seed_templates.up.sql"
    docker exec -i draft-postgres-1 psql -U nullus -d nullus < "$PROJECT_ROOT/db/migrations/000031_seed_empty_template.up.sql"
  else
    echo "[nullus] golden_path_templates already seeded ($count rows)"
  fi
}

seed_cicd_templates_if_needed() {
  local count
  count="$(docker exec draft-postgres-1 psql -U nullus -d nullus -tA -c "SELECT COUNT(*) FROM pipeline_templates;" 2>/dev/null | tr -d '[:space:]' || true)"
  if [[ -z "$count" || "$count" == "0" ]]; then
    echo "[nullus] seeding pipeline_templates..."
    docker exec -i draft-postgres-1 psql -U nullus -d nullus < "$PROJECT_ROOT/db/migrations/000010_seed_cicd_templates.up.sql"
  else
    echo "[nullus] pipeline_templates already seeded ($count rows)"
  fi
}

seed_token_sources_if_needed() {
  local count
  count="$(docker exec draft-postgres-1 psql -U nullus -d nullus -tA -c "SELECT COUNT(*) FROM token_sources WHERE deleted_at IS NULL;" 2>/dev/null | tr -d '[:space:]' || true)"
  if [[ -z "$count" || "$count" == "0" ]]; then
    echo "[nullus] seeding token_sources..."
    bash "$PROJECT_ROOT/scripts/seed-token-sources.sh"
  else
    echo "[nullus] token_sources already seeded ($count rows)"
  fi
}

ensure_golden_path_seed() {
  seed_golden_path_templates_if_needed
  seed_cicd_templates_if_needed
}

KEYCLOAK_CONTAINER=""
keycloak_container() {
  if [[ -z "$KEYCLOAK_CONTAINER" ]]; then
    KEYCLOAK_CONTAINER="$(docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" ps -q keycloak 2>/dev/null | head -1)"
  fi
  printf '%s' "$KEYCLOAK_CONTAINER"
}

# Keycloak realm 의 sslRequired 기본값이 'external' 이라, 호스트 → 컨테이너(:8180)
# 요청을 외부 요청으로 판정해 HTTP 를 거부한다. 그 결과 setup-keycloak.sh 의 admin
# 토큰 발급이 "failed to obtain admin token" 으로 실패한다. 호스트에서는 SSL 을 붙일
# 수 없으므로 컨테이너 안에서 kcadm 으로 요구 조건을 해제한다.
# docker-compose.dev.yaml 의 Keycloak 은 KC_DB=dev-mem(인메모리)이라 컨테이너를
# 재시작하면 초기화된다 — 따라서 up 할 때마다 다시 적용한다.
keycloak_relax_ssl() {
  local realm="$1" cid
  cid="$(keycloak_container)"
  if [[ -z "$cid" ]]; then
    echo "[nullus] keycloak container not found; skipping sslRequired relax ($realm)"
    return 0
  fi

  if ! docker exec "$cid" /opt/keycloak/bin/kcadm.sh config credentials \
      --server http://localhost:8080 --realm master \
      --user "${KEYCLOAK_ADMIN_USER:-admin}" \
      --password "${KEYCLOAK_ADMIN_PASSWORD:-admin}" >/dev/null 2>&1; then
    echo "[nullus] kcadm login failed; skipping sslRequired relax ($realm)"
    return 0
  fi

  if docker exec "$cid" /opt/keycloak/bin/kcadm.sh update "realms/${realm}" \
      -s sslRequired=NONE >/dev/null 2>&1; then
    echo "[nullus] keycloak realm '$realm': sslRequired=NONE"
  else
    echo "[nullus] keycloak realm '$realm': sslRequired update skipped"
  fi
}

usage() {
  cat <<'EOF'
Usage:
  ./scripts/runbook_local.sh preflight
  ./scripts/runbook_local.sh up [--seed] [--kind] [--auth=<keycloak|authentik|none>]
     --auth=keycloak   (기본) Keycloak 기동 + realm 셋업
     --auth=authentik  Authentik 스택 기동 + 셋업 (Keycloak 미기동)
     --auth=none       IdP 미기동 (프론트 mock auth 전용)
     --authentik       (deprecated) --auth=authentik 별칭
  환경변수 NULLUS_AUTH_PROVIDER 로도 지정 가능 (플래그가 우선)
  ./scripts/runbook_local.sh status
  ./scripts/runbook_local.sh info
  ./scripts/runbook_local.sh smoke
  ./scripts/runbook_local.sh logs [api|web|all]
  ./scripts/runbook_local.sh refresh [--auth=<keycloak|authentik|none>]
     API/web 만 재빌드·재기동한다. up 과 같은 --auth 를 줘야 인증 구성이 유지된다.

  ./scripts/runbook_local.sh down [--kind] [--auth=<keycloak|authentik|none>] [--volumes]
  ./scripts/runbook_local.sh purge [--keep-kind] [--keep-volumes]
  ./scripts/runbook_local.sh stack-up [--template=<id>] [--name=<n>] [--namespace=<ns>]
                                      [--cluster=<ctx-name>] [--domain=<d>] [--wait]
  ./scripts/runbook_local.sh stack-status <stack-id> [--wait]
  ./scripts/runbook_local.sh stack-down [--all|<stack-id>]
  ./scripts/runbook_local.sh pipeline-down
  ./scripts/runbook_local.sh all [--seed] [--kind] [--auth=<keycloak|authentik|none>]
  ./scripts/runbook_local.sh refresh
  ./scripts/runbook_local.sh kind-up
  ./scripts/runbook_local.sh kind-down

Commands:
  preflight         Validate toolchain prerequisites
  up [--seed]       Start infra (PostgreSQL, Redis, MinIO, Keycloak) + migrate + API + frontend
     [--kind]       Also create a kind K8s cluster
     [--auth=...]   Select OIDC provider: keycloak (default) | authentik | none
  status            Show health of all services (including kind cluster, Authentik)
  info              Show access URLs and credentials
  smoke             Run API smoke tests (13 endpoints)
  logs [svc]        Tail logs for a service (api, web) or all
  down [--kind] [--volumes]  Stop API, frontend, docker infra
        [--auth=authentik] Also stop Authentik services
  purge             전체 초기화: 파이프라인 -> 스택 -> Nullus/백킹 -> kind 삭제
     [--keep-kind]     kind 클러스터는 남긴다 (스택만 지우고 재설치 검증용)
     [--keep-volumes]  DB 볼륨을 남긴다 (기본은 삭제 = 처음 설치 상태)
  stack-up          템플릿으로 스택을 설치한다 (기본: gitea-jenkins-argocd-lite-v1)
                    도구 선택은 템플릿 응답에서 가져오고, 설치는 백엔드 API 가 수행한다
                    자원 계획은 템플릿의 planning_profile 로 백엔드가 계산한다
                    (Lite = local → 8Gi 노드 하나에 들어가는 크기)
  stack-status <id> 스택 상태 조회. --wait 면 completed/failed 까지 폴링한다
  stack-down [id]   설치된 스택을 제품 삭제 경로(helm uninstall + CRD 정리)로 삭제
                    인자 없으면 --all. 삭제 후 남은 helm 릴리스를 보고한다
  pipeline-down     CI/CD 파이프라인 전체 삭제 (스택보다 먼저 지워야 한다)
  all               Full lifecycle: up -> smoke -> keep running
  refresh           Rebuild backend + frontend, run pending migrations, restart
  kind-up           Create kind K8s cluster only
  kind-down         Delete kind K8s cluster only

Test Accounts (Frontend mock auth, development mode):
  admin@nullus.dev     / admin123       (admin)
  devops@nullus.dev    / devops123      (devops)
  developer@nullus.dev / developer123   (developer)

Test Accounts (Keycloak OIDC, production mode):
  admin@nullus.io      / nullus123!     (admin)
  devops@nullus.io     / nullus123!     (devops)
  dev@nullus.io        / nullus123!     (developer)

Infrastructure:
  PostgreSQL  nullus / nullus_dev       (localhost:5433)
  Keycloak    admin / admin             (localhost:8180)
  MinIO       nullus / nullus_dev       (localhost:9000, console :9001)
  Redis       -                         (localhost:6380)
EOF
}

ensure_dirs() {
  mkdir -p "$LOG_DIR"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[nullus] missing required command: $1"
    exit 1
  }
}

wait_for_http() {
  local url="$1" attempts="${2:-30}" delay="${3:-1}" i
  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    sleep "$delay"
  done
  return 1
}

wait_for_port_listen() {
  local port="$1" attempts="${2:-30}" i
  for ((i = 1; i <= attempts; i++)); do
    if lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  return 1
}

wait_for_port_free() {
  local port="$1" attempts="${2:-15}" i
  for ((i = 1; i <= attempts; i++)); do
    if ! lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  return 1
}

require_port_free() {
  local name="$1" port="$2"
  if lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    local pids
    pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null | tr '\n' ',' | sed 's/,$//')"
    echo "[nullus] cannot start $name: port $port in use by pid=$pids"
    echo "[nullus] run 'down' first or free the port"
    exit 1
  fi
}

remove_pid_entry() {
  [[ -f "$PID_FILE" ]] || return 0
  local tmp="$PID_FILE.tmp"
  grep -Ev "^${1}:" "$PID_FILE" >"$tmp" || true
  mv "$tmp" "$PID_FILE"
}

run_bg() {
  local name="$1" workdir="$2" cmd="$3" port="$4"
  local logfile="$LOG_DIR/${name}.log"
  : >"$logfile"
  nohup bash -lc "cd '$workdir' && exec $cmd </dev/null" >"$logfile" 2>&1 &
  local pid=$!
  sleep 3
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    echo "[nullus] $name exited immediately; check $logfile"
    tail -5 "$logfile" 2>/dev/null
    exit 1
  fi
  local tmp="$PID_FILE.tmp"
  cat "$PID_FILE" >"$tmp" 2>/dev/null || true
  echo "$name:$pid" >>"$tmp"
  mv "$tmp" "$PID_FILE"
  printf '[nullus] started %-12s pid=%s log=%s\n' "$name" "$pid" "$logfile"
}

stop_service() {
  local name="$1" port="$2"
  [[ -f "$PID_FILE" ]] || return 0
  local line pid
  line="$(grep -E "^${name}:" "$PID_FILE" | tail -n 1 || true)"
  if [[ -n "$line" ]]; then
    pid="${line#*:}"
    if [[ "$pid" =~ ^[0-9]+$ ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      sleep 2
      kill -0 "$pid" >/dev/null 2>&1 && kill -9 "$pid" >/dev/null 2>&1 || true
      printf '[nullus] stopped %-12s pid=%s\n' "$name" "$pid"
    fi
  fi
  remove_pid_entry "$name"
  local port_pids
  port_pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [[ -n "$port_pids" ]]; then
    kill $port_pids >/dev/null 2>&1 || true
  fi
  wait_for_port_free "$port" 10 || true
}

install_migrate() {
  if ! command -v migrate >/dev/null 2>&1; then
    echo "[nullus] installing golang-migrate..."
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  fi
}

do_kind_up() {
  if ! command -v kind >/dev/null 2>&1; then
    echo "[nullus] kind not found, skipping K8s cluster"
    return 1
  fi

  local preferred_context="kind-nullus-platform"

  if [[ -f "$KIND_CONFIG" ]]; then
    local tmp_dir
    tmp_dir="$(mktemp -d)"
    awk -v outdir="$tmp_dir" '
      BEGIN { doc=0; file="" }
      /^---[[:space:]]*$/ { file=""; next }
      {
        if (file == "") {
          doc++
          file=sprintf("%s/kind-doc-%d.yaml", outdir, doc)
        }
        print >> file
      }
    ' "$KIND_CONFIG"

    local cfg cluster_name
    for cfg in "$tmp_dir"/kind-doc-*.yaml; do
      [[ -f "$cfg" ]] || continue
      cluster_name="$(awk '/^name:[[:space:]]+/ { print $2; exit }' "$cfg")"
      if [[ -z "$cluster_name" ]]; then
        continue
      fi

      if kind_cluster_exists "$cluster_name"; then
        echo "[nullus] kind cluster '$cluster_name' already exists"
        continue
      fi

      echo "[nullus] creating kind cluster '$cluster_name'..."
      kind create cluster --config "$cfg"
      echo "[nullus] kind cluster '$cluster_name' ready"
    done

    rm -rf "$tmp_dir"
  else
    if kind_cluster_exists "nullus-platform"; then
      echo "[nullus] kind cluster 'nullus-platform' already exists"
      return 0
    fi
    echo "[nullus] creating kind cluster 'nullus-platform'..."
    kind create cluster --name "nullus-platform"
    echo "[nullus] kind cluster 'nullus-platform' ready"
  fi

  if command -v kubectl >/dev/null 2>&1 && kind_cluster_exists "nullus-platform"; then
    kubectl config use-context "$preferred_context" >/dev/null 2>&1 || true
    echo "[nullus] kubectl context set to '$preferred_context'"
  fi
}

do_kind_down() {
  if ! command -v kind >/dev/null 2>&1; then
    return 0
  fi

  local found="false"
  while IFS= read -r cluster_name; do
    [[ -z "$cluster_name" ]] && continue
    if kind_cluster_exists "$cluster_name"; then
      found="true"
      echo "[nullus] deleting kind cluster '$cluster_name'..."
      kind delete cluster --name "$cluster_name"
      echo "[nullus] kind cluster '$cluster_name' deleted"
    fi
  done < <(kind_cluster_names)

  if [[ "$found" == "false" ]] && kind_cluster_exists "nullus-platform"; then
    echo "[nullus] deleting kind cluster 'nullus-platform'..."
    kind delete cluster --name "nullus-platform"
    echo "[nullus] kind cluster 'nullus-platform' deleted"
  fi
}

do_preflight() {
  echo "[nullus] checking prerequisites..."
  echo ""

  require_cmd go
  require_cmd node
  require_cmd npm
  require_cmd docker
  require_cmd lsof
  require_cmd curl

  echo "[nullus] toolchain:"
  echo "[nullus]   go      $(go version | awk '{print $3}')"
  echo "[nullus]   node    $(node --version)"
  echo "[nullus]   docker  $(docker --version | awk '{print $3}' | tr -d ',')"
  if command -v kind >/dev/null 2>&1; then
    echo "[nullus]   kind    $(kind version)  (optional)"
  else
    echo "[nullus]   kind    not installed  (optional — brew install kind)"
  fi
  if command -v kubectl >/dev/null 2>&1; then
    echo "[nullus]   kubectl $(kubectl version --client -o json 2>/dev/null | grep -o '"gitVersion":"[^"]*"' | head -1 | cut -d'"' -f4)  (optional)"
  fi
  if command -v helm >/dev/null 2>&1; then
    echo "[nullus]   helm    $(helm version --short 2>/dev/null)  (optional)"
  fi
  echo ""

if ! docker info >/dev/null 2>&1; then
  echo "[nullus] ERROR: Docker daemon is not running."

  os="$(uname -s 2>/dev/null || echo unknown)"
  case "$os" in
    Darwin)
      echo "[nullus]   macOS: Start your Docker runtime (Docker Desktop / Colima / OrbStack / Rancher Desktop)"
      echo "[nullus]   e.g. Docker Desktop: 'open -a Docker', Colima: 'colima start'"
      ;;
    Linux)
      # WSL 감지
      if grep -qi microsoft /proc/version 2>/dev/null; then
        echo "[nullus]   WSL: Start Docker Desktop on Windows and enable WSL integration"
      else
        echo "[nullus]   Linux: Run 'sudo systemctl start docker' (or your distro equivalent)"
      fi
      ;;
    MINGW*|MSYS*|CYGWIN*)
      echo "[nullus]   Windows: Start Docker Desktop"
      echo "[nullus]   (PowerShell as Admin: Start-Service com.docker.service)"
      ;;
    *)
      echo "[nullus]   Start Docker Desktop (or Docker Engine) for your OS"
      ;;
  esac

  exit 1
fi

echo "[nullus] docker daemon: running"

  echo ""
  echo "[nullus] resource requirements:"
  echo "[nullus]   base (postgres+redis+minio+keycloak): ~2GB RAM, 4GB disk"
  echo "[nullus]   with --kind (K8s cluster):            +2GB RAM, +2GB disk"
  echo ""
  echo "[nullus] preflight OK"
}

do_info() {
  echo ""
  echo -e "${BOLD}════════════════════════════════════════════════════════════════════════${NC}"
  echo -e "${BOLD}  Nullus Local Environment — Access Info${NC}"
  echo -e "${BOLD}════════════════════════════════════════════════════════════════════════${NC}"
  echo ""
  echo -e "${BOLD}  Test Accounts (Frontend mock auth, development mode)${NC}"
  echo "  ──────────────────────────────────────────────────────────────────"
  echo "  Email                        Password        Role"
  echo "  ──────────────────────────────────────────────────────────────────"
  echo "  admin@nullus.dev             admin123        admin"
  echo "  devops@nullus.dev            devops123       devops"
  echo "  developer@nullus.dev         developer123    developer"
  echo ""
  echo -e "${BOLD}  Test Accounts (Keycloak OIDC, production mode)${NC}"
  echo "  ──────────────────────────────────────────────────────────────────"
  echo "  Email                        Password        Role"
  echo "  ──────────────────────────────────────────────────────────────────"
  echo "  admin@nullus.io              nullus123!      admin"
  echo "  devops@nullus.io             nullus123!      devops"
  echo "  dev@nullus.io                nullus123!      developer"
  echo ""
  echo -e "${CYAN}  ── Application ──${NC}"
  echo "  Frontend           http://localhost:$WEB_PORT"
  echo "  API                http://localhost:$API_PORT"
  echo "  Health             http://localhost:$API_PORT/health"
  echo ""
  echo -e "${CYAN}  ── Infrastructure ──${NC}"
  echo "  PostgreSQL         localhost:$POSTGRES_PORT  (nullus / nullus_dev)"
  echo "  Keycloak           http://localhost:$KEYCLOAK_PORT  (admin / admin)"
  echo "  MinIO Console      http://localhost:$MINIO_CONSOLE_PORT  (nullus / nullus_dev)"
  echo "  MinIO API          localhost:$MINIO_PORT"
  echo "  Redis              localhost:$REDIS_PORT"
  echo ""
  local printed_k8s="false"
  if command -v kind >/dev/null 2>&1; then
    while IFS= read -r cluster_name; do
      [[ -z "$cluster_name" ]] && continue
      if ! kind_cluster_exists "$cluster_name"; then
        continue
      fi
      if [[ "$printed_k8s" == "false" ]]; then
        echo -e "${CYAN}  ── Kubernetes ──${NC}"
        printed_k8s="true"
      fi
      echo "  Kind Cluster       kind-$cluster_name ($(kubectl get nodes --context "kind-$cluster_name" -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}' 2>/dev/null || echo 'unknown'))"
    done < <(kind_cluster_names)
    [[ "$printed_k8s" == "true" ]] && echo ""
  fi
  echo -e "${CYAN}  ── Commands ──${NC}"
  echo "  Logs               ./scripts/runbook_local.sh logs"
  echo "  Status             ./scripts/runbook_local.sh status"
  echo "  Smoke Test         ./scripts/runbook_local.sh smoke"
  echo "  Stop               ./scripts/runbook_local.sh down"
  echo ""
  echo -e "${BOLD}════════════════════════════════════════════════════════════════════════${NC}"
}

# API 프로세스에 인증 관련 환경을 넘긴다. do_up 과 do_refresh 가 같은 값을 써야
# 한다 — 예전에는 do_refresh 가 이 블록을 빠뜨려, refresh 한 번에 API 가 조용히
# 무인증 구성으로 되돌아갔다.
export_api_auth_env() {
  [[ "$AUTH_PROVIDER" == "none" ]] && return 0

  export NULLUS_AUTH_OIDC_PROVIDER="$AUTH_PROVIDER"
  case "$AUTH_PROVIDER" in
    keycloak)  export NULLUS_AUTH_OIDC_ISSUER_URL="http://localhost:${KEYCLOAK_PORT}/realms/nullus" ;;
    authentik) export NULLUS_AUTH_OIDC_ISSUER_URL="http://localhost:${AUTHENTIK_PORT}/application/o/nullus/" ;;
  esac

  # 스택이 설치하는 OSS(GitLab/Grafana/ArgoCD/Harbor/MinIO)의 OIDC 클라이언트를
  # 자동 등록하는 provisioning_sso 단계가 이 자격을 쓴다. 주소를 안 주면 그 단계는
  # 로그 한 줄만 남기고 통과해서, 설치는 초록불인데 OSS 는 전부 로컬 admin 계정으로
  # 뜬다. Authentik 은 이 프로비저너가 없으므로 Keycloak 일 때만 넣는다.
  if [[ "$AUTH_PROVIDER" == "keycloak" ]]; then
    export NULLUS_KEYCLOAK_ADMIN_URL="http://localhost:${KEYCLOAK_PORT}"
    export NULLUS_KEYCLOAK_REALM=nullus
    export NULLUS_KEYCLOAK_ADMIN_USER="$KEYCLOAK_ADMIN_USER"
    export NULLUS_KEYCLOAK_ADMIN_PASSWORD="$KEYCLOAK_ADMIN_PASSWORD"
  fi
}

# 프런트엔드를 선택한 provider 에 맞춰 놓는다.
#
# web/.env.development 는 git 에 추적되는 파일이라 여기서 덮어쓰면 실행할 때마다
# 워킹트리가 더러워진다. Vite 는 .env.development.local 을 더 높은 우선순위로
# 읽고 이 파일은 gitignore 대상이므로, 생성물은 그쪽에 쓴다.
sync_web_oidc_env() {
  local target="$PROJECT_ROOT/web/.env.development.local"

  if [[ "$AUTH_PROVIDER" == "none" ]]; then
    if [[ -f "$target" ]]; then
      rm -f "$target"
      echo "[nullus] removed web/.env.development.local (mock auth 로 되돌림)"
    fi
    return 0
  fi

  local authority
  case "$AUTH_PROVIDER" in
    keycloak)  authority="http://localhost:${KEYCLOAK_PORT}/realms/nullus" ;;
    authentik) authority="http://localhost:${AUTHENTIK_PORT}/application/o/nullus/" ;;
  esac

  cat >"$target" <<EOF
# runbook_local.sh 가 생성한 파일이다 — 직접 수정하지 말 것(실행 때마다 덮어쓴다).
# Vite 는 이 파일을 .env.development 보다 먼저 적용한다.
# mock 로그인으로 돌아가려면: ./scripts/runbook_local.sh up --auth=none
VITE_AUTH_MODE=oidc
VITE_OIDC_PROVIDER=${AUTH_PROVIDER}
VITE_OIDC_AUTHORITY=${authority}
# setup-keycloak.sh 가 만드는 클라이언트이자 API 의 audience 기본값이다.
VITE_OIDC_CLIENT_ID=nullus-app
EOF
  echo "[nullus] wrote web/.env.development.local (VITE_AUTH_MODE=oidc, provider=$AUTH_PROVIDER)"
}

do_up() {
  local seed="false" with_kind="false"
  for arg in "$@"; do
    if parse_auth_arg "$arg"; then continue; fi
    case "$arg" in
      --seed) seed="true" ;;
      --kind) with_kind="true" ;;
      *) echo "[nullus] unknown option: $arg"; exit 1 ;;
    esac
  done
  validate_auth_provider

  ensure_dirs
  do_preflight

  require_port_free "api" "$API_PORT"
  require_port_free "web" "$WEB_PORT"

  : >"$PID_FILE"

  # 1. Docker infra (auth provider에 따라 서비스 선택)
  echo ""
  local infra_services=(postgres redis minio)
  [[ "$AUTH_PROVIDER" == "keycloak" ]] && infra_services+=(keycloak)
  echo "[nullus] starting docker infra (${infra_services[*]}) [auth=$AUTH_PROVIDER]..."
  docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" up -d "${infra_services[@]}"
  echo "[nullus] waiting for postgres..."
  wait_for_port_listen "$POSTGRES_PORT" 30 || {
    echo "[nullus] postgres did not start"; exit 1
  }
  sleep 2

  if [[ "$AUTH_PROVIDER" == "keycloak" ]]; then
    echo "[nullus] waiting for keycloak..."
    if wait_for_http "http://localhost:${KEYCLOAK_PORT}" 60 2; then
      echo "[nullus] keycloak is ready, running realm setup..."
      keycloak_relax_ssl master
      if "$PROJECT_ROOT/scripts/setup-keycloak.sh"; then
        # nullus realm 은 setup 이 만든 뒤에야 존재하므로 생성 후 별도로 해제한다
        # (해제 전에는 호스트에서 direct grant 토큰 요청이 실패한다).
        keycloak_relax_ssl nullus
      else
        echo "[nullus] keycloak realm setup failed (non-blocking, run scripts/setup-keycloak.sh manually)"
      fi
    else
      echo "[nullus] keycloak did not start (non-blocking, continuing...)"
    fi
  fi

  # 2. Database migrations
  echo "[nullus] running database migrations..."
  install_migrate
  local MIGRATE
  MIGRATE="$(command -v migrate || echo "$HOME/go/bin/migrate")"
  "$MIGRATE" -path "$PROJECT_ROOT/db/migrations" -database "$DB_URL" up || {
    echo "[nullus] migration failed (may already be applied, continuing...)"
  }

  register_kind_cluster_endpoints

  # 3. Build + start API (with ENCRYPTION_KEY)
  echo ""
  # OCI 차트(envoy gateway)는 helm CLI 로 폴백한다. API 프로세스의 PATH 에
  # helm 이 없으면 게이트웨이 설치만 "executable file not found" 로 실패하는데,
  # 원인이 설치 로그 깊은 곳에만 남아 찾기 어렵다. 여기서 미리 드러낸다.
  if ! command -v helm >/dev/null 2>&1; then
    echo "[nullus] ERROR: helm 을 찾을 수 없습니다. OCI 차트 설치(envoy gateway)가 실패합니다."
    echo "[nullus]        brew install helm 후 다시 실행하세요."
    return 1
  fi

  echo "[nullus] building API server..."
  (cd "$PROJECT_ROOT" && go build -o bin/api ./cmd/api)

  echo "[nullus] starting API server on :$API_PORT..."
  export ENCRYPTION_KEY
  export OPENBAO_ADDR
  export OPENBAO_TOKEN
  export NULLUS_DATABASE_HOST=localhost
  export NULLUS_DATABASE_PORT="$POSTGRES_PORT"
  export NULLUS_SERVER_MODE=development
  export_api_auth_env
  run_bg "api" "$PROJECT_ROOT" "./bin/api" "$API_PORT"

  echo "[nullus] waiting for API health (up to 60s)..."
  if wait_for_http "http://localhost:${API_PORT}/health" 60 2; then
    echo "[nullus] API is healthy"
  else
    echo "[nullus] API health check failed after 60s; check $LOG_DIR/api.log"
    tail -10 "$LOG_DIR/api.log" 2>/dev/null
    exit 1
  fi

  echo ""
  sync_web_oidc_env

  echo "[nullus] installing frontend dependencies..."
  (cd "$PROJECT_ROOT/web" && npm ci --legacy-peer-deps --silent 2>/dev/null || npm ci --legacy-peer-deps)

  echo "[nullus] starting frontend dev server on :$WEB_PORT..."
  run_bg "web" "$PROJECT_ROOT/web" "npx vite --port $WEB_PORT" "$WEB_PORT"

  echo "[nullus] waiting for frontend (up to 30s)..."
  if wait_for_port_listen "$WEB_PORT" 30; then
    echo "[nullus] frontend is ready"
  else
    echo "[nullus] frontend did not start; check $LOG_DIR/web.log"
    tail -10 "$LOG_DIR/web.log" 2>/dev/null
    exit 1
  fi

  # 5. kind cluster (optional)
  if [[ "$with_kind" == "true" ]]; then
    echo ""
    do_kind_up || true
    auto_register_kind_clusters
    seed="true"
  fi

  if [[ "$seed" == "true" ]]; then
    echo ""
    ensure_golden_path_seed
    seed_token_sources_if_needed
  fi

  # 6. Authentik OIDC provider (optional)
  if [[ "$AUTH_PROVIDER" == "authentik" ]]; then
    echo ""
    echo "[nullus] starting Authentik OIDC provider..."
    docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" -f "$COMPOSE_AUTH" up -d \
      authentik-db authentik-redis authentik-server authentik-worker
    echo "[nullus] waiting for Authentik (up to 180s)..."
    if wait_for_http "http://localhost:${AUTHENTIK_PORT}/-/health/ready/" 60 3; then
      echo "[nullus] Authentik is ready, running setup..."
      "$PROJECT_ROOT/scripts/setup-authentik.sh"
    else
      echo "[nullus] Authentik did not start (non-blocking, continuing...)"
      echo "[nullus] check: docker compose -f docker-compose.dev.yaml -f docker-compose.auth.yaml logs authentik-server"
    fi
  fi

  echo ""
  echo "══════════════════════════════════════════════════"
  echo "  Nullus Local Environment Ready"
  echo "══════════════════════════════════════════════════"
  echo ""
  echo "  Frontend      http://localhost:$WEB_PORT"
  echo "  API           http://localhost:$API_PORT"
  echo "  Health        http://localhost:$API_PORT/health"
  echo ""
  echo "  PostgreSQL    localhost:$POSTGRES_PORT  (nullus/nullus_dev)"
  case "$AUTH_PROVIDER" in
    keycloak)  echo "  Keycloak      http://localhost:$KEYCLOAK_PORT  (admin/admin)" ;;
    authentik) echo "  Authentik     http://localhost:$AUTHENTIK_PORT  (admin@nullus.io/nullus123!)" ;;
    none)      echo "  Auth          none (frontend mock auth: VITE_AUTH_MODE=mock)" ;;
  esac
  if [[ "$AUTH_PROVIDER" != "none" ]]; then
    echo ""
    echo "  Web auth      OIDC (web/.env.development.local 자동 생성됨)"
    echo "                포털 접속 시 $AUTH_PROVIDER 로그인 화면으로 바로 이동한다."
    if [[ "$AUTH_PROVIDER" == "keycloak" ]]; then
      echo "  SSO 프로비저닝  ON — 설치되는 OSS 의 OIDC 클라이언트를 Keycloak 에 자동 등록"
      echo "                (스택 설치 시 authentication.provider=openbao 인 경우)"
    fi
    echo ""
    echo "  Login         admin@nullus.io / devops@nullus.io / dev@nullus.io  (pw: $DEFAULT_KC_PASSWORD_HINT)"
  fi
  echo "  MinIO         http://localhost:$MINIO_CONSOLE_PORT  (nullus/nullus_dev)"
  echo "  Redis         localhost:$REDIS_PORT"
  echo ""
  kind_print_status
  echo "  Logs:         ./scripts/runbook_local.sh logs"
  echo "  Stop:         ./scripts/runbook_local.sh down"
  echo "══════════════════════════════════════════════════"
}

do_status() {
  echo "[nullus] docker services"
  docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" ps 2>/dev/null || echo "  (docker compose not running)"
  echo ""

  echo "[nullus] service health"
  if curl -fsS "http://localhost:${API_PORT}/health" 2>/dev/null; then
    echo ""
  else
    echo "  api: unavailable"
  fi

  if lsof -tiTCP:"$WEB_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "  web: listening on :$WEB_PORT"
  else
    echo "  web: not running"
  fi

  if wait_for_http "http://localhost:${KEYCLOAK_PORT}" 3 1 2>/dev/null; then
    echo "  keycloak: listening on :$KEYCLOAK_PORT"
  else
    echo "  keycloak: not running"
  fi

  if lsof -tiTCP:"$MINIO_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "  minio: listening on :$MINIO_PORT (console :$MINIO_CONSOLE_PORT)"
  else
    echo "  minio: not running"
  fi

  if lsof -tiTCP:"$AUTHENTIK_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "  authentik: listening on :$AUTHENTIK_PORT"
  fi
  echo ""

  if command -v kind >/dev/null 2>&1; then
    local printed="false"
    while IFS= read -r cluster_name; do
      [[ -z "$cluster_name" ]] && continue
      if kind_cluster_exists "$cluster_name"; then
        if [[ "$printed" == "false" ]]; then
          echo "[nullus] kind clusters"
          printed="true"
        fi
        echo "  - kind-$cluster_name"
        kubectl get nodes --context "kind-$cluster_name" 2>/dev/null || echo "    kind cluster not reachable"
      fi
    done < <(kind_cluster_names)
    [[ "$printed" == "true" ]] && echo ""
  fi

  if [[ -f "$PID_FILE" ]]; then
    echo "[nullus] managed processes"
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      local name="${line%%:*}" pid="${line#*:}"
      if kill -0 "$pid" >/dev/null 2>&1; then
        echo "  $name: pid=$pid alive"
      else
        echo "  $name: pid=$pid dead"
      fi
    done <"$PID_FILE"
  fi
}

do_smoke() {
  for arg in "$@"; do
    parse_auth_arg "$arg" || true
  done
  validate_auth_provider

  echo "[nullus] running smoke tests..."
  echo ""

  local passed=0 failed=0

  smoke_get() {
    local label="$1" url="$2" expect="${3:-200}"
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || echo "000")"
    if [[ "$code" == "$expect" ]]; then
      printf '  %-45s %s\n' "$label" "OK ($code)"
      ((passed++)) || true
    else
      printf '  %-45s %s\n' "$label" "FAIL (got $code, expected $expect)"
      ((failed++)) || true
    fi
  }

  smoke_get "GET /health"                              "http://localhost:${API_PORT}/health"
  smoke_get "GET /api/v1/admin/organization"           "http://localhost:${API_PORT}/api/v1/admin/organization"
  smoke_get "GET /api/v1/admin/clusters"               "http://localhost:${API_PORT}/api/v1/admin/clusters"
  smoke_get "GET /api/v1/admin/known-issues"           "http://localhost:${API_PORT}/api/v1/admin/known-issues"
  smoke_get "GET /api/v1/admin/audit-logs"             "http://localhost:${API_PORT}/api/v1/admin/audit-logs"
  smoke_get "GET /api/v1/admin/notifications/configs"  "http://localhost:${API_PORT}/api/v1/admin/notifications/configs"
  smoke_get "GET /api/v1/stacks"                       "http://localhost:${API_PORT}/api/v1/stacks"
  smoke_get "GET /api/v1/stacks/templates"             "http://localhost:${API_PORT}/api/v1/stacks/templates"
  smoke_get "GET /api/v1/stacks/compatibility"         "http://localhost:${API_PORT}/api/v1/stacks/compatibility"
  smoke_get "GET /api/v1/cicd/templates"               "http://localhost:${API_PORT}/api/v1/cicd/templates"
  smoke_get "GET /api/v1/cicd/pipelines"               "http://localhost:${API_PORT}/api/v1/cicd/pipelines"
  smoke_get "GET /api/v1/observability/dashboard"      "http://localhost:${API_PORT}/api/v1/observability/dashboard"
  smoke_get "GET /api/v1/observability/alert-rules"    "http://localhost:${API_PORT}/api/v1/observability/alert-rules"
  smoke_get "Frontend reachable"                       "http://localhost:${WEB_PORT}"

  echo ""
  echo "  [auth smoke: provider=$AUTH_PROVIDER]"
  case "$AUTH_PROVIDER" in
    keycloak)
      smoke_get "GET keycloak openid-configuration" \
        "http://localhost:${KEYCLOAK_PORT}/realms/nullus/.well-known/openid-configuration"
      local token
      token="$(curl -sS -X POST \
        "http://localhost:${KEYCLOAK_PORT}/realms/nullus/protocol/openid-connect/token" \
        -d grant_type=password -d client_id=nullus-app \
        -d username=admin@nullus.io -d 'password=nullus123!' 2>/dev/null \
        | python3 -c 'import json,sys; print(json.load(sys.stdin).get("access_token",""))' 2>/dev/null)"
      if [[ -n "$token" ]]; then
        printf '  %-45s %s\n' "POST keycloak token (login smoke)" "OK"
        ((passed++)) || true
      else
        printf '  %-45s %s\n' "POST keycloak token (login smoke)" "FAIL (no access_token)"
        ((failed++)) || true
      fi
      ;;
    authentik)
      smoke_get "GET authentik health" \
        "http://localhost:${AUTHENTIK_PORT}/-/health/ready/"
      smoke_get "GET authentik openid-configuration" \
        "http://localhost:${AUTHENTIK_PORT}/application/o/nullus/.well-known/openid-configuration"
      ;;
    none)
      printf '  %-45s %s\n' "auth=none" "SKIPPED (mock auth)"
      ;;
  esac

  echo ""
  echo "[nullus] smoke: $passed passed, $failed failed"
  [[ "$failed" -eq 0 ]] || exit 1
}

do_logs() {
  ensure_dirs
  local service="${1:-all}"
  if [[ "$service" == "all" ]]; then
    ls "$LOG_DIR"/*.log >/dev/null 2>&1 || {
      echo "[nullus] no logs yet"
      return
    }
    tail -f "$LOG_DIR"/*.log
    return
  fi
  local file="$LOG_DIR/${service}.log"
  [[ -f "$file" ]] || { echo "[nullus] no log: $file"; exit 1; }
  tail -f "$file"
}

# ------------------------------------------------------------
# 삭제 경로
#
# kind 클러스터를 통째로 지우면 스택도 같이 사라진다. 하지만 그 길로는 제품이
# 실제로 쓰는 삭제 경로 — DeleteStack usecase 의 helm uninstall + CRD /
# ClusterRoleBinding 정리 — 를 한 번도 밟지 않는다. 커뮤니티 사용자는 클러스터를
# 버리지 않고 스택만 지우므로, 런북도 같은 길을 지나야 삭제가 실제로 도는지
# 확인된다. 그래서 'purge' 는 API 삭제를 먼저 태우고 그다음에 클러스터를 지운다.
# ------------------------------------------------------------

api_base() {
  printf 'http://localhost:%s' "$API_PORT"
}

api_is_up() {
  local code
  code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' "$(api_base)/health" 2>/dev/null || echo "000")"
  [[ "$code" == "200" ]]
}

# 목록 엔드포인트의 items[].id 만 뽑는다. 응답이 비었거나 형태가 달라도
# 삭제 절차 전체를 멈추지 않는다 (set -e 아래에서도 빈 목록으로 수렴).
api_item_ids() {
  local url="$1"
  curl -sS -m 30 "$url" 2>/dev/null | python3 -c '
import json, sys
try:
    data = json.load(sys.stdin)
except Exception:
    sys.exit(0)
items = data.get("items") if isinstance(data, dict) else data
for item in items or []:
    if isinstance(item, dict) and item.get("id"):
        print(item["id"])
' 2>/dev/null || true
}

api_delete() {
  local url="$1"
  curl -sS -X DELETE -m 900 -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || echo "000"
}

# ------------------------------------------------------------
# 설치 경로
#
# 스택 생성도 삭제와 같은 이유로 API 를 탄다 — helm 을 스크립트에서 직접 부르면
# 설치 구현이 백엔드와 스크립트 둘로 갈라져 같은 템플릿이 경로마다 다른 결과를
# 낸다 (airgap/scripts/29-install-stacks-via-api.sh 가 같은 판단을 적어 두었다).
#
# 도구 선택은 템플릿 응답에서 그대로 가져온다. 차트 버전 표를 여기 복사해 두면
# 마이그레이션이 버전을 올릴 때마다 스크립트가 따로 낡는다.
#
# applied_resource_overrides 도 싣지 않는다 — 백엔드가 템플릿의
# planning_profile 을 읽어 직접 계산한다 (internal/stack/domain/planning.go).
# 그래서 Lite 템플릿은 이 명령으로도 8Gi 노드용 크기로 깔린다. 계획을 직접
# 정하고 싶으면 UI 마법사에서 조정한 값을 실어 보내면 되고, 그때는 그 값이 이긴다.
# ------------------------------------------------------------

# 템플릿의 tools[] 를 StackConfig 의 슬롯으로 옮긴다. 고르지 않은 슬롯도
# enabled=false 로 채워야 백엔드가 "미선택"과 "누락"을 구분한다.
stack_create_payload() {
  local template_id="$1" stack_name="$2" cluster_id="$3" namespace="$4" domain="$5"
  curl -sS -m 30 "$(api_base)/api/v1/stacks/templates" 2>/dev/null | \
  TEMPLATE_ID="$template_id" STACK_NAME="$stack_name" CLUSTER_ID="$cluster_id" \
  STACK_NAMESPACE="$namespace" ACCESS_DOMAIN="$domain" python3 -c '
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

config = {"artifacts": {}, "pipeline": {}, "monitoring": {}, "logging": {}}
for section, field in SLOTS.values():
    config[section][field] = {"name": "", "version": "", "enabled": False}

for tool in template.get("tools") or []:
    slot = SLOTS.get(tool.get("category"))
    if slot is None:
        continue
    section, field = slot
    config[section][field] = {
        "name": tool.get("name", ""),
        "version": tool.get("app_version", ""),
        "enabled": True,
    }

config["access_domain"] = os.environ["ACCESS_DOMAIN"]
config["authentication"] = {"provider": "openbao"}

# storage 는 부분만 채우면 검증에서 400 이 난다 — integrated-create 는
# database/object_storage 가 둘 다 mode=create 이어야 하고, create 모드는
# provider_or_engine 과 size(Gi, >0)를 요구한다. 값은 설치 마법사의 기본값과
# 같게 둔다 (web/src/features/stack/stores/stack-config-store.ts 의 storage,
# size 는 stack-normalizers.ts 의 toStorageSizeGi 가 medium 을 옮긴 값).
config["storage"] = {
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

print(json.dumps({
    "name": os.environ["STACK_NAME"],
    "golden_path_id": wanted,
    "cluster_id": os.environ["CLUSTER_ID"],
    "namespace": os.environ["STACK_NAMESPACE"],
    "config": config,
}))
'
}

do_stack_up() {
  local template_id="gitea-jenkins-argocd-lite-v1"
  local stack_name="" namespace="" cluster_name="kind-nullus-platform" domain=""
  local wait_install="false"

  for arg in "$@"; do
    case "$arg" in
      --template=*) template_id="${arg#*=}" ;;
      --name=*)     stack_name="${arg#*=}" ;;
      --namespace=*) namespace="${arg#*=}" ;;
      --cluster=*)  cluster_name="${arg#*=}" ;;
      --domain=*)   domain="${arg#*=}" ;;
      --wait)       wait_install="true" ;;
      *) echo "[nullus] unknown option: $arg"; exit 1 ;;
    esac
  done

  [[ -n "$stack_name" ]] || stack_name="nullus-$(printf '%s' "$template_id" | sed 's/-v1$//' | cut -c1-24)"
  [[ -n "$namespace" ]] || namespace="$stack_name"
  # 기본 접속 도메인. sso_provisioner.go 의 defaultAccessDomain, setup-keycloak.sh 가
  # 등록하는 redirect URI, 차트 ingress 기본값이 모두 nullus.local 이다. 여기만
  # "<스택명>.internal" 로 어긋나 있어 SSO redirect 주소가 맞지 않았다.
  [[ -n "$domain" ]] || domain="nullus.local"

  if ! api_is_up; then
    echo "[nullus] API 가 떠 있지 않습니다 (:$API_PORT). 'up' 후 다시 실행하세요."
    return 1
  fi

  local cluster_id
  cluster_id="$(curl -sS -m 30 "$(api_base)/api/v1/admin/clusters" 2>/dev/null | \
    CLUSTER_NAME="$cluster_name" python3 -c '
import json, os, sys
data = json.load(sys.stdin)
items = data.get("items", data) if isinstance(data, dict) else data
target = os.environ["CLUSTER_NAME"]
for c in items or []:
    if c.get("name") == target:
        print(c.get("id", ""))
        break
' 2>/dev/null || true)"

  if [[ -z "$cluster_id" ]]; then
    echo "[nullus] 클러스터 '$cluster_name' 을 찾지 못했습니다."
    echo "[nullus] ./scripts/register-kind-clusters.sh 로 먼저 등록하세요."
    return 1
  fi

  echo "[nullus] template=$template_id cluster=$cluster_name namespace=$namespace domain=$domain"

  local payload
  payload="$(stack_create_payload "$template_id" "$stack_name" "$cluster_id" "$namespace" "$domain")" || {
    echo "[nullus] 스택 payload 생성 실패"
    return 1
  }

  local stack_id
  stack_id="$(curl -sS -m 60 -X POST -H 'Content-Type: application/json' \
    -d "$payload" "$(api_base)/api/v1/stacks" 2>/dev/null | \
    python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))' 2>/dev/null || true)"

  if [[ -z "$stack_id" ]]; then
    echo "[nullus] 스택 생성 실패 — $LOG_DIR/api.log 확인"
    return 1
  fi
  echo "[nullus] stack created: $stack_id"

  # Pre-Deploy Gate 가 warn 을 내면 명시적 동의 없이는 막힌다. 로컬 검증에서는
  # 동의한 것으로 보내되, 게이트가 block 을 내면 그대로 실패한다.
  local deploy_code
  deploy_code="$(curl -sS -m 120 -X POST -H 'Content-Type: application/json' \
    -d '{"acknowledge_warnings":true}' -o /dev/null -w '%{http_code}' \
    "$(api_base)/api/v1/stacks/$stack_id/deploy" 2>/dev/null || echo "000")"

  if [[ "$deploy_code" != "200" && "$deploy_code" != "202" ]]; then
    echo "[nullus] 배포 요청 실패 (HTTP $deploy_code) — $LOG_DIR/api.log 확인"
    return 1
  fi
  echo "[nullus] deploy accepted ($deploy_code)"

  if [[ "$wait_install" != "true" ]]; then
    echo "[nullus] 진행 상황: ./scripts/runbook_local.sh stack-status $stack_id"
    return 0
  fi

  do_stack_status "$stack_id" --wait
}

# 설치는 수십 분이 걸린다. 상태 폴링을 따로 두어 stack-up 을 기다리지 않고도
# 같은 판정을 다시 볼 수 있게 한다.
do_stack_status() {
  local stack_id="${1:-}" wait_install="false"
  shift || true
  for arg in "$@"; do
    case "$arg" in
      --wait) wait_install="true" ;;
    esac
  done

  if [[ -z "$stack_id" ]]; then
    echo "[nullus] stack id 가 필요합니다"
    return 1
  fi

  local deadline=$(( SECONDS + 3600 ))
  local state last_state=""
  while :; do
    state="$(curl -sS -m 30 "$(api_base)/api/v1/stacks/$stack_id" 2>/dev/null | \
      python3 -c 'import json,sys; print(json.load(sys.stdin).get("state",""))' 2>/dev/null || true)"
    [[ -z "$state" ]] && state="unknown"

    if [[ "$state" != "$last_state" ]]; then
      echo "[nullus] stack $stack_id state: $state"
      last_state="$state"
    fi

    case "$state" in
      completed)
        echo "[nullus] 설치 완료"
        return 0
        ;;
      failed|rolled_back|cancelled)
        echo "[nullus] 설치 실패 상태: $state — $LOG_DIR/api.log 확인"
        return 1
        ;;
    esac

    [[ "$wait_install" == "true" ]] || return 0
    if (( SECONDS > deadline )); then
      echo "[nullus] 60분 안에 완료되지 않았습니다 (마지막 상태: $state)"
      return 1
    fi
    sleep 15
  done
}

# 파이프라인은 스택을 참조한다. 스택을 먼저 지우면 남은 파이프라인이 사라진
# 네임스페이스를 가리켜 화면에 유령 행으로 남는다 — 그래서 파이프라인이 먼저다.
do_pipeline_down() {
  if ! api_is_up; then
    echo "[nullus] API 가 떠 있지 않습니다 (:$API_PORT). 'up' 후 다시 실행하세요."
    return 1
  fi

  local ids
  ids="$(api_item_ids "$(api_base)/api/v1/cicd/pipelines")"
  if [[ -z "$ids" ]]; then
    echo "[nullus] 삭제할 파이프라인이 없습니다"
    return 0
  fi

  local id code failed=0
  while IFS= read -r id; do
    [[ -z "$id" ]] && continue
    echo "[nullus] deleting pipeline $id..."
    code="$(api_delete "$(api_base)/api/v1/cicd/pipelines/$id")"
    if [[ "$code" == "200" || "$code" == "204" ]]; then
      echo "[nullus]   pipeline $id deleted ($code)"
    else
      echo "[nullus]   pipeline $id delete FAILED (HTTP $code)"
      failed=$((failed + 1))
    fi
  done <<<"$ids"

  [[ "$failed" -eq 0 ]]
}

# 스택 삭제는 helm uninstall 을 순차로 돌아 수 분이 걸린다. API 는 동기로
# 응답하므로 curl 타임아웃을 넉넉히 잡는다 (api_delete 의 -m 900).
do_stack_down() {
  local target="${1:---all}"

  if ! api_is_up; then
    echo "[nullus] API 가 떠 있지 않습니다 (:$API_PORT). 'up' 후 다시 실행하세요."
    return 1
  fi

  local ids
  if [[ "$target" == "--all" ]]; then
    ids="$(api_item_ids "$(api_base)/api/v1/stacks")"
  else
    ids="$target"
  fi

  if [[ -z "$ids" ]]; then
    echo "[nullus] 삭제할 스택이 없습니다"
    return 0
  fi

  local id code failed=0
  while IFS= read -r id; do
    [[ -z "$id" ]] && continue
    echo "[nullus] deleting stack $id (helm uninstall + CRD 정리, 수 분 소요)..."
    code="$(api_delete "$(api_base)/api/v1/stacks/$id")"
    if [[ "$code" == "200" || "$code" == "204" ]]; then
      echo "[nullus]   stack $id deleted ($code)"
    else
      echo "[nullus]   stack $id delete FAILED (HTTP $code) — $LOG_DIR/api.log 확인"
      failed=$((failed + 1))
    fi
  done <<<"$ids"

  report_cluster_leftovers
  [[ "$failed" -eq 0 ]]
}

# helm uninstall 은 cluster-scoped 리소스를 남긴다. 남은 것을 보여주지 않으면
# "지웠다"고 믿은 채 다음 설치가 ownership 충돌로 깨진다.
#
# helm 릴리스만 보면 안 된다 — 삭제 경로는 Gateway/HTTPRoute 를 남기고, 살아남은
# Gateway 때문에 Envoy Gateway 가 데이터플레인 Deployment 를 계속 돌린다. 파드가
# 조용히 자원을 먹는데 helm list 는 깨끗하게 나온다(실측 확인).
report_cluster_leftovers() {
  command -v kind >/dev/null 2>&1 || return 0

  local cluster_name remaining
  while IFS= read -r cluster_name; do
    [[ -z "$cluster_name" ]] && continue
    kind_cluster_exists "$cluster_name" || continue

    if command -v helm >/dev/null 2>&1; then
      remaining="$(helm list -A --kube-context "kind-$cluster_name" --short 2>/dev/null || true)"
      if [[ -n "$remaining" ]]; then
        echo "[nullus] kind-$cluster_name 에 남은 helm 릴리스:"
        printf '%s\n' "$remaining" | sed 's/^/[nullus]   - /'
      else
        echo "[nullus] kind-$cluster_name: 남은 helm 릴리스 없음"
      fi
    fi

    command -v kubectl >/dev/null 2>&1 || continue

    # 스택 네임스페이스는 kubectl 로 지워지지 않은 채 남는 일이 있다. 그 안의
    # Gateway 가 살아 있으면 envoy 파드가 계속 뜬다.
    remaining="$(kubectl --context "kind-$cluster_name" get gateways.gateway.networking.k8s.io \
      -A --no-headers -o custom-columns='NS:.metadata.namespace,NAME:.metadata.name' 2>/dev/null || true)"
    if [[ -n "$remaining" ]]; then
      echo "[nullus] kind-$cluster_name 에 남은 Gateway (envoy 데이터플레인이 함께 뜬다):"
      printf '%s\n' "$remaining" | sed 's/^/[nullus]   - /'
    fi

    remaining="$(kubectl --context "kind-$cluster_name" get ns --no-headers -o custom-columns=':.metadata.name' 2>/dev/null \
      | grep -E '^nullus' || true)"
    if [[ -n "$remaining" ]]; then
      echo "[nullus] kind-$cluster_name 에 남은 스택 네임스페이스:"
      printf '%s\n' "$remaining" | sed 's/^/[nullus]   - /'
    fi
  done < <(kind_cluster_names)
}

# 전체 초기화: 파이프라인 → 스택 → Nullus/백킹 → kind 클러스터 순.
# --keep-kind 는 클러스터를 남겨 "스택만 지우고 다시 깔기" 를 검증할 때 쓴다.
do_purge() {
  local keep_kind="false" keep_volumes="false"
  for arg in "$@"; do
    if parse_auth_arg "$arg"; then continue; fi
    case "$arg" in
      --keep-kind) keep_kind="true" ;;
      --keep-volumes) keep_volumes="true" ;;
      *) echo "[nullus] unknown option: $arg"; exit 1 ;;
    esac
  done
  validate_auth_provider

  echo "[nullus] ── purge 1/3: CI/CD 파이프라인 ──"
  if api_is_up; then
    do_pipeline_down || echo "[nullus] 일부 파이프라인 삭제 실패 (계속 진행)"
    echo ""
    echo "[nullus] ── purge 2/3: 스택 ──"
    do_stack_down --all || echo "[nullus] 일부 스택 삭제 실패 (계속 진행)"
  else
    echo "[nullus] API 가 내려가 있어 스택/파이프라인 API 삭제를 건너뜁니다."
    echo "[nullus] 제품 삭제 경로를 검증하려면 'up' 후 'purge' 를 다시 실행하세요."
  fi

  echo ""
  local step3_label="Nullus + 백킹"
  [[ "$keep_kind" == "false" ]] && step3_label="$step3_label + kind"
  echo "[nullus] ── purge 3/3: $step3_label ──"
  local down_args=()
  [[ "$keep_kind" == "false" ]] && down_args+=(--kind)
  [[ "$keep_volumes" == "false" ]] && down_args+=(--volumes)
  down_args+=("--auth=$AUTH_PROVIDER")
  do_down "${down_args[@]}"

  echo ""
  if [[ "$keep_volumes" == "false" ]]; then
    echo "[nullus] purge 완료 — DB 볼륨까지 삭제했습니다. 다음 'up' 은 처음 설치와 같습니다."
  else
    echo "[nullus] purge 완료 — DB 볼륨은 보존했습니다."
  fi
}

do_down() {
  local with_kind="false" with_volumes="false"
  for arg in "$@"; do
    if parse_auth_arg "$arg"; then continue; fi
    case "$arg" in
      --kind) with_kind="true" ;;
      --volumes) with_volumes="true" ;;
    esac
  done
  validate_auth_provider

  echo "[nullus] stopping services..."
  if [[ -f "$PID_FILE" ]]; then
    stop_service "web" "$WEB_PORT"
    stop_service "api" "$API_PORT"
    rm -f "$PID_FILE"
  fi

  if [[ "$AUTH_PROVIDER" == "authentik" ]]; then
    echo "[nullus] stopping docker infra (incl. authentik)..."
    if [[ "$with_volumes" == "true" ]]; then
      docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" -f "$COMPOSE_AUTH" down -v 2>/dev/null || true
    else
      docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" -f "$COMPOSE_AUTH" down 2>/dev/null || true
    fi
  else
    echo "[nullus] stopping docker infra..."
    if [[ "$with_volumes" == "true" ]]; then
      docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" down -v 2>/dev/null || true
    else
      docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" down 2>/dev/null || true
    fi
  fi

  if [[ "$with_kind" == "true" ]]; then
    do_kind_down 2>/dev/null || true
  fi

  echo "[nullus] all stopped"
  if command -v kind >/dev/null 2>&1; then
    local remaining=""
    while IFS= read -r cluster_name; do
      [[ -z "$cluster_name" ]] && continue
      if kind_cluster_exists "$cluster_name"; then
        remaining="$remaining kind-$cluster_name"
      fi
    done < <(kind_cluster_names)

    if [[ -n "$remaining" ]]; then
      echo "[nullus] note: kind cluster(s)$remaining still running (use 'kind-down' or 'down --kind' to remove)"
    fi
  fi
}

do_refresh() {
  # refresh 도 provider 를 알아야 한다. 파싱하지 않으면 기본값(keycloak)이 잡혀,
  # --auth=none 으로 띄운 환경을 refresh 한 번에 OIDC 로 되돌려 버린다.
  for arg in "$@"; do
    parse_auth_arg "$arg" || true
  done
  validate_auth_provider

  ensure_dirs

  echo "[nullus] refreshing backend + frontend..."
  echo ""

  # 1. Stop running API and web
  stop_service "api" "$API_PORT"
  stop_service "web" "$WEB_PORT"

  # 2. Run pending migrations
  echo "[nullus] running pending migrations..."
  install_migrate
  local MIGRATE
  MIGRATE="$(command -v migrate || echo "$HOME/go/bin/migrate")"
  "$MIGRATE" -path "$PROJECT_ROOT/db/migrations" -database "$DB_URL" up 2>/dev/null || true

  register_kind_cluster_endpoints

  # 3. Rebuild + restart API
  echo "[nullus] rebuilding API server..."
  (cd "$PROJECT_ROOT" && go build -o bin/api ./cmd/api)

  echo "[nullus] starting API server on :$API_PORT..."
  export ENCRYPTION_KEY
  export OPENBAO_ADDR
  export OPENBAO_TOKEN
  export NULLUS_DATABASE_HOST=localhost
  export NULLUS_DATABASE_PORT="$POSTGRES_PORT"
  export NULLUS_SERVER_MODE=development
  export_api_auth_env
  run_bg "api" "$PROJECT_ROOT" "./bin/api" "$API_PORT"

  echo "[nullus] waiting for API health (up to 30s)..."
  if wait_for_http "http://localhost:${API_PORT}/health" 30 1; then
    echo "[nullus] API is healthy"
  else
    echo "[nullus] API health check failed; check $LOG_DIR/api.log"
    tail -10 "$LOG_DIR/api.log" 2>/dev/null
    exit 1
  fi

  # 4. Restart frontend
  echo ""
  sync_web_oidc_env
  echo "[nullus] starting frontend dev server on :$WEB_PORT..."
  run_bg "web" "$PROJECT_ROOT/web" "npx vite --port $WEB_PORT" "$WEB_PORT"

  echo "[nullus] waiting for frontend (up to 30s)..."
  if wait_for_port_listen "$WEB_PORT" 30; then
    echo "[nullus] frontend is ready"
  else
    echo "[nullus] frontend did not start; check $LOG_DIR/web.log"
    tail -10 "$LOG_DIR/web.log" 2>/dev/null
    exit 1
  fi

  echo ""
  echo "══════════════════════════════════════════════════"
  echo "  Nullus Refreshed"
  echo "══════════════════════════════════════════════════"
  echo "  Frontend      http://localhost:$WEB_PORT"
  echo "  API           http://localhost:$API_PORT"
  echo "══════════════════════════════════════════════════"
}

do_all() {
  local extra_args=()
  for arg in "$@"; do
    case "$arg" in
      --seed|--kind|--authentik|--auth=*) extra_args+=("$arg") ;;
    esac
  done

  trap 'do_down || true' EXIT INT TERM
  do_up "${extra_args[@]}"
  do_smoke
  trap - EXIT INT TERM
  echo ""
  echo "[nullus] all checks passed. Services are running."
  echo "[nullus] press Ctrl+C or run './scripts/runbook_local.sh down' to stop."
}

main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    preflight) do_preflight ;;
    up) do_up "$@" ;;
    status) do_status ;;
    info) do_info ;;
    smoke) do_smoke "$@" ;;
    logs) do_logs "${1:-all}" ;;
    down) do_down "$@" ;;
    purge) do_purge "$@" ;;
    stack-up) do_stack_up "$@" ;;
    stack-status) do_stack_status "$@" ;;
    stack-down) do_stack_down "${1:---all}" ;;
    pipeline-down) do_pipeline_down ;;
    all) do_all "$@" ;;
    refresh) do_refresh "$@" ;;
    kind-up) do_kind_up ;;
    kind-down) do_kind_down ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
