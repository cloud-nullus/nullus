#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/.runbook-logs"
PID_FILE="$LOG_DIR/pids.txt"
DB_URL="postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable"

API_PORT=8080
WEB_PORT=5173
POSTGRES_PORT=5433
MINIO_PORT=9000
REDIS_PORT=6380
KEYCLOAK_PORT=8180

usage() {
  cat <<'EOF'
Usage:
  ./scripts/runbook_local.sh preflight
  ./scripts/runbook_local.sh up [--seed]
  ./scripts/runbook_local.sh status
  ./scripts/runbook_local.sh smoke
  ./scripts/runbook_local.sh logs [api|web|all]
  ./scripts/runbook_local.sh down
  ./scripts/runbook_local.sh all [--seed]

Commands:
  preflight     Validate toolchain prerequisites
  up [--seed]   Start infra + migrate + API + frontend
  status        Show health of all services
  smoke         Run API smoke tests (health, orgs, stacks, templates)
  logs [svc]    Tail logs for a service (api, web) or all
  down          Stop API, frontend, and docker infra
  all [--seed]  Full lifecycle: up -> smoke -> keep running

Test Accounts (Frontend):
  admin@nullus.dev     / admin123       (admin - full access)
  devops@nullus.dev    / devops123      (devops - stack + cicd)
  developer@nullus.dev / developer123   (developer - cicd only)

Test Accounts (Keycloak OIDC - when auth.mode=oidc):
  admin@nullus.io      / nullus123!     (admin)
  devops@nullus.io     / nullus123!     (devops)
  dev@nullus.io        / nullus123!     (developer)

Infrastructure:
  PostgreSQL  nullus / nullus_dev       (localhost:5433)
  Keycloak    admin / admin             (localhost:8180)
  MinIO       nullus / nullus_dev       (localhost:9000)
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
  nohup bash -lc "cd '$workdir' && exec $cmd" >"$logfile" 2>&1 &
  local pid=$!
  sleep 1
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    echo "[nullus] $name exited immediately; check $logfile"
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

do_preflight() {
  echo "[nullus] checking prerequisites..."
  require_cmd go
  require_cmd node
  require_cmd npm
  require_cmd docker
  require_cmd lsof
  require_cmd curl
  echo "[nullus]   go      $(go version | awk '{print $3}')"
  echo "[nullus]   node    $(node --version)"
  echo "[nullus]   docker  $(docker --version | awk '{print $3}' | tr -d ',')"
  echo "[nullus] preflight OK"
}

do_up() {
  local seed="false"
  for arg in "$@"; do
    case "$arg" in
      --seed) seed="true" ;;
      *) echo "[nullus] unknown option: $arg"; exit 1 ;;
    esac
  done

  ensure_dirs
  do_preflight

  require_port_free "api" "$API_PORT"
  require_port_free "web" "$WEB_PORT"

  : >"$PID_FILE"

  # 1. Docker infra
  echo ""
  echo "[nullus] starting docker infra..."
  docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" up -d postgres redis
  echo "[nullus] waiting for postgres..."
  wait_for_port_listen "$POSTGRES_PORT" 30 || {
    echo "[nullus] postgres did not start"; exit 1
  }
  sleep 2

  # 2. Database migrations
  echo "[nullus] running database migrations..."
  install_migrate
  local MIGRATE
  MIGRATE="$(command -v migrate || echo "$HOME/go/bin/migrate")"
  "$MIGRATE" -path "$PROJECT_ROOT/db/migrations" -database "$DB_URL" up || {
    echo "[nullus] migration failed (may already be applied, continuing...)"
  }

  # 3. Build + start API
  echo ""
  echo "[nullus] building API server..."
  (cd "$PROJECT_ROOT" && go build -o bin/api ./cmd/api)

  echo "[nullus] starting API server on :$API_PORT..."
  run_bg "api" "$PROJECT_ROOT" "./bin/api" "$API_PORT"

  echo "[nullus] waiting for API health..."
  if wait_for_http "http://127.0.0.1:${API_PORT}/health" 30 1; then
    echo "[nullus] API is healthy"
  else
    echo "[nullus] API health check failed; check $LOG_DIR/api.log"
    exit 1
  fi

  # 4. Frontend
  echo ""
  echo "[nullus] installing frontend dependencies..."
  (cd "$PROJECT_ROOT/web" && npm install --silent)

  echo "[nullus] starting frontend dev server on :$WEB_PORT..."
  run_bg "web" "$PROJECT_ROOT/web" "npx vite --port $WEB_PORT" "$WEB_PORT"

  echo "[nullus] waiting for frontend..."
  if wait_for_port_listen "$WEB_PORT" 30; then
    echo "[nullus] frontend is ready"
  else
    echo "[nullus] frontend did not start; check $LOG_DIR/web.log"
    exit 1
  fi

  echo ""
  echo "══════════════════════════════════════════════════"
  echo "  Nullus Local Environment Ready"
  echo "══════════════════════════════════════════════════"
  echo ""
  echo "  Frontend   http://localhost:$WEB_PORT"
  echo "  API        http://localhost:$API_PORT"
  echo "  Health     http://localhost:$API_PORT/health"
  echo "  API Docs   http://localhost:$API_PORT/api/v1/"
  echo ""
  echo "  PostgreSQL localhost:$POSTGRES_PORT  (nullus/nullus_dev)"
  echo "  Redis      localhost:$REDIS_PORT"
  echo ""
  echo "  Logs:      ./scripts/runbook_local.sh logs"
  echo "  Stop:      ./scripts/runbook_local.sh down"
  echo "══════════════════════════════════════════════════"
}

do_status() {
  echo "[nullus] docker services"
  docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" ps 2>/dev/null || echo "  (docker compose not running)"
  echo ""

  echo "[nullus] service health"
  if curl -fsS "http://127.0.0.1:${API_PORT}/health" 2>/dev/null; then
    echo ""
  else
    echo "  api: unavailable"
  fi

  if lsof -tiTCP:"$WEB_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "  web: listening on :$WEB_PORT"
  else
    echo "  web: not running"
  fi
  echo ""

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
      printf '  %-40s %s\n' "$label" "OK ($code)"
      ((passed++)) || true
    else
      printf '  %-40s %s\n' "$label" "FAIL (got $code, expected $expect)"
      ((failed++)) || true
    fi
  }

  smoke_get "GET /health"                              "http://127.0.0.1:${API_PORT}/health"
  smoke_get "GET /api/v1/admin/clusters"               "http://127.0.0.1:${API_PORT}/api/v1/admin/clusters"
  smoke_get "GET /api/v1/admin/known-issues"           "http://127.0.0.1:${API_PORT}/api/v1/admin/known-issues"
  smoke_get "GET /api/v1/admin/audit-logs"             "http://127.0.0.1:${API_PORT}/api/v1/admin/audit-logs"
  smoke_get "GET /api/v1/stacks/templates"             "http://127.0.0.1:${API_PORT}/api/v1/stacks/templates"
  smoke_get "GET /api/v1/stacks"                       "http://127.0.0.1:${API_PORT}/api/v1/stacks"
  smoke_get "GET /api/v1/cicd/templates"               "http://127.0.0.1:${API_PORT}/api/v1/cicd/templates"
  smoke_get "GET /api/v1/cicd/pipelines"               "http://127.0.0.1:${API_PORT}/api/v1/cicd/pipelines"
  smoke_get "GET /api/v1/observability/dashboard"      "http://127.0.0.1:${API_PORT}/api/v1/observability/dashboard"
  smoke_get "GET /api/v1/observability/alert-rules"    "http://127.0.0.1:${API_PORT}/api/v1/observability/alert-rules"
  smoke_get "Frontend reachable"                       "http://127.0.0.1:${WEB_PORT}"

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
  echo "[nullus] stopping services..."
  if [[ -f "$PID_FILE" ]]; then
    stop_service "web" "$WEB_PORT"
    stop_service "api" "$API_PORT"
    rm -f "$PID_FILE"
  fi
  echo "[nullus] stopping docker infra..."
  docker compose -f "$PROJECT_ROOT/docker-compose.dev.yaml" down 2>/dev/null || true
  echo "[nullus] all stopped"
}

do_all() {
  local seed_args=()
  for arg in "$@"; do
    case "$arg" in
      --seed) seed_args+=("--seed") ;;
    esac
  done

  trap 'do_down || true' EXIT INT TERM
  do_up "${seed_args[@]}"
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
    smoke) do_smoke ;;
    logs) do_logs "${1:-all}" ;;
    down) do_down ;;
    all) do_all "$@" ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
