#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }

usage() {
  cat <<'EOF'
Nullus DB 마이그레이션 검증 스크립트

Usage:
  ./scripts/verify-db-migration.sh --src <SRC_DSN> --dst <DST_DSN>

Options:
  --src   원본 DB DSN (postgres://...)
  --dst   대상 DB DSN (postgres://...)
  -h, --help  도움말

예시:
  ./scripts/verify-db-migration.sh \
    --src "postgres://nullus:nullus_dev@src-host:5432/nullus?sslmode=disable" \
    --dst "postgres://nullus:nullus_dev@dst-host:5432/nullus?sslmode=disable"

환경변수 방식:
  SRC_DSN="postgres://..." DST_DSN="postgres://..." ./scripts/verify-db-migration.sh
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    fail "required command not found: $1"
    exit 1
  }
}

if ! command -v psql >/dev/null 2>&1 && [[ -x "/opt/homebrew/opt/libpq/bin/psql" ]]; then
  export PATH="/opt/homebrew/opt/libpq/bin:$PATH"
fi

SRC_DSN="${SRC_DSN:-}"
DST_DSN="${DST_DSN:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --src)
      SRC_DSN="${2:-}"
      shift 2
      ;;
    --dst)
      DST_DSN="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$SRC_DSN" || -z "$DST_DSN" ]]; then
  fail "both --src and --dst (or SRC_DSN/DST_DSN env) are required"
  usage
  exit 1
fi

require_cmd psql
require_cmd diff

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

run_query() {
  local dsn="$1"
  local sql="$2"
  local out_file="$3"
  psql "$dsn" -X -v ON_ERROR_STOP=1 -At -F $'\t' -c "$sql" > "$out_file"
}

compare_block() {
  local title="$1"
  local sql="$2"
  local src_file="$WORK_DIR/${title}.src.tsv"
  local dst_file="$WORK_DIR/${title}.dst.tsv"

  info "검증: ${title}"
  run_query "$SRC_DSN" "$sql" "$src_file"
  run_query "$DST_DSN" "$sql" "$dst_file"

  if diff -u "$src_file" "$dst_file" > "$WORK_DIR/${title}.diff"; then
    ok "${title}: 일치"
  else
    fail "${title}: 불일치"
    echo "----- diff (${title}) -----"
    cat "$WORK_DIR/${title}.diff"
    echo "---------------------------"
    return 1
  fi
}

FAILED=0

compare_block "01_table_counts" "
SELECT 'stacks' AS table_name, COUNT(*)::text FROM stacks
UNION ALL
SELECT 'stack_config_versions', COUNT(*)::text FROM stack_config_versions
UNION ALL
SELECT 'golden_path_templates', COUNT(*)::text FROM golden_path_templates
ORDER BY 1;
" || FAILED=1

compare_block "02_stacks_config_keys" "
SELECT
  COUNT(*)::text AS total,
  COUNT(*) FILTER (WHERE config ? 'yaml_overrides')::text AS has_yaml_overrides,
  COUNT(*) FILTER (WHERE config ? 'logging')::text AS has_logging,
  COUNT(*) FILTER (WHERE config ? 'pipeline')::text AS has_pipeline,
  COUNT(*) FILTER (WHERE config ? 'storage')::text AS has_storage
FROM stacks;
" || FAILED=1

compare_block "03_access_domain_typo_count" "
SELECT COUNT(*)::text
FROM stacks
WHERE (config->>'access_domain') ILIKE '%.intenral';
" || FAILED=1

compare_block "04_history_summary" "
SELECT
  stack_id,
  MIN(version)::text AS min_v,
  MAX(version)::text AS max_v,
  COUNT(*)::text AS cnt
FROM stack_config_versions
GROUP BY stack_id
ORDER BY stack_id;
" || FAILED=1

compare_block "05_template_tool_counts" "
SELECT id, name, jsonb_array_length(tools)::text AS tool_count
FROM golden_path_templates
ORDER BY id;
" || FAILED=1

compare_block "06_template_missing_versions" "
SELECT
  t.id,
  elem->>'name' AS tool_name,
  COALESCE(elem->>'helm_version', '') AS helm_version,
  COALESCE(elem->>'app_version', '') AS app_version
FROM golden_path_templates t,
LATERAL jsonb_array_elements(t.tools) elem
WHERE COALESCE(elem->>'helm_version', '') = ''
   OR COALESCE(elem->>'app_version', '') = ''
ORDER BY t.id, tool_name;
" || FAILED=1

compare_block "07_hash_stacks_config" "
SELECT md5(string_agg(id || ':' || md5(config::text), '|' ORDER BY id))
FROM stacks;
" || FAILED=1

compare_block "08_hash_history_config" "
SELECT md5(string_agg(stack_id || ':' || version || ':' || md5(config::text), '|' ORDER BY stack_id, version))
FROM stack_config_versions;
" || FAILED=1

compare_block "09_hash_template_tools" "
SELECT md5(string_agg(id || ':' || md5(tools::text), '|' ORDER BY id))
FROM golden_path_templates;
" || FAILED=1

if [[ "$FAILED" -eq 0 ]]; then
  ok "모든 검증 블록이 일치합니다. 마이그레이션 데이터 무결성 OK"
  exit 0
fi

fail "일부 검증 블록에서 차이가 발견되었습니다. 위 diff를 확인하세요."
exit 2
