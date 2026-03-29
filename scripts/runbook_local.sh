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

ENCRYPTION_KEY="${ENCRYPTION_KEY:-nullus-dev-key-32bytes-padding!!}"
KIND_CONFIG="$PROJECT_ROOT/scripts/kind-cluster.yaml"
AUTHENTIK_PORT=9090
COMPOSE_AUTH="$PROJECT_ROOT/docker-compose.auth.yaml"

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

usage() {
  cat <<'EOF'
Usage:
  ./scripts/runbook_local.sh preflight
  ./scripts/runbook_local.sh up [--seed] [--kind] [--authentik]
  ./scripts/runbook_local.sh status
  ./scripts/runbook_local.sh info
  ./scripts/runbook_local.sh smoke
  ./scripts/runbook_local.sh logs [api|web|all]
  ./scripts/runbook_local.sh down [--kind] [--authentik]
  ./scripts/runbook_local.sh all [--seed] [--kind] [--authentik]
  ./scripts/runbook_local.sh kind-up
  ./scripts/runbook_local.sh kind-down

Commands:
  preflight         Validate toolchain prerequisites
  up [--seed]       Start infra (PostgreSQL, Redis, MinIO, Keycloak) + migrate + API + frontend
     [--kind]       Also create a kind K8s cluster
     [--authentik]  Also start Authentik OIDC provider (localhost:9090) and run setup
  status            Show health of all services (including kind cluster, Authentik)
  info              Show access URLs and credentials
  smoke             Run API smoke tests (13 endpoints)
  logs [svc]        Tail logs for a service (api, web) or all
  down [--kind]     Stop API, frontend, docker infra
       [--authentik] Also stop Authentik services
  all               Full lifecycle: up -> smoke -> keep running
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
  nohup bash -lc "cd '$workdir' && exec $cmd" >"$logfile" 2>&1 &
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
    echo "[nullus]   Start Docker Desktop or run 'sudo systemctl start docker'"
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

do_up() {
  local seed="false" with_kind="false" with_authentik="false"
  for arg in "$@"; do
    case "$arg" in
      --seed) seed="true" ;;
      --kind) with_kind="true" ;;
      --authentik) with_authentik="true" ;;
      *) echo "[nullus] unknown option: $arg"; exit 1 ;;
    esac
  done

  ensure_dirs
  do_preflight

  require_port_free "api" "$API_PORT"
  require_port_free "web" "$WEB_PORT"

  : >"$PID_FILE"

  # 1. Docker infra (PostgreSQL, Redis, MinIO, Keycloak)
  echo ""
  echo "[nullus] starting docker infra (postgres, redis, minio, keycloak)..."
  docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" up -d
  echo "[nullus] waiting for postgres..."
  wait_for_port_listen "$POSTGRES_PORT" 30 || {
    echo "[nullus] postgres did not start"; exit 1
  }
  sleep 2

  echo "[nullus] waiting for keycloak..."
  if wait_for_http "http://localhost:${KEYCLOAK_PORT}" 60 2; then
    echo "[nullus] keycloak is ready"
  else
    echo "[nullus] keycloak did not start (non-blocking, continuing...)"
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
  echo "[nullus] building API server..."
  (cd "$PROJECT_ROOT" && go build -o bin/api ./cmd/api)

  echo "[nullus] starting API server on :$API_PORT..."
  export ENCRYPTION_KEY
  export NULLUS_DATABASE_HOST=localhost
  export NULLUS_DATABASE_PORT="$POSTGRES_PORT"
  export NULLUS_SERVER_MODE=development
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
  echo "[nullus] installing frontend dependencies..."
  (cd "$PROJECT_ROOT/web" && npm install --legacy-peer-deps --silent 2>/dev/null || npm install --legacy-peer-deps)

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
  fi

  # 6. Authentik OIDC provider (optional)
  if [[ "$with_authentik" == "true" ]]; then
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
  echo "  Keycloak      http://localhost:$KEYCLOAK_PORT  (admin/admin)"
  echo "  MinIO         http://localhost:$MINIO_CONSOLE_PORT  (nullus/nullus_dev)"
  echo "  Redis         localhost:$REDIS_PORT"
  echo ""
  kind_print_status
  if [[ "$with_authentik" == "true" ]]; then
    echo "  Authentik     http://localhost:$AUTHENTIK_PORT  (admin@nullus.io/nullus123!)"
    echo ""
  fi
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

do_down() {
  local with_kind="false" with_authentik="false"
  for arg in "$@"; do
    case "$arg" in
      --kind) with_kind="true" ;;
      --authentik) with_authentik="true" ;;
    esac
  done

  echo "[nullus] stopping services..."
  if [[ -f "$PID_FILE" ]]; then
    stop_service "web" "$WEB_PORT"
    stop_service "api" "$API_PORT"
    rm -f "$PID_FILE"
  fi

  if [[ "$with_authentik" == "true" ]]; then
    echo "[nullus] stopping Authentik services..."
    docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" -f "$COMPOSE_AUTH" down 2>/dev/null || true
  else
    echo "[nullus] stopping docker infra..."
    docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" down 2>/dev/null || true
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

do_all() {
  local extra_args=()
  for arg in "$@"; do
    case "$arg" in
      --seed|--kind|--authentik) extra_args+=("$arg") ;;
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
    smoke) do_smoke ;;
    logs) do_logs "${1:-all}" ;;
    down) do_down "$@" ;;
    all) do_all "$@" ;;
    kind-up) do_kind_up ;;
    kind-down) do_kind_down ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
