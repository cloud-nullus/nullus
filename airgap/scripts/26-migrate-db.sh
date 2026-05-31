#!/usr/bin/env bash
# =============================================================================
# 26-migrate-db.sh — DB 스키마 마이그레이션 적용 (airgap)
# =============================================================================
# 배경: nullus-api 는 부팅 시 자동 마이그레이션을 하지 않으며(golang-migrate CLI
#       또는 CI 로 외부 적용하는 설계), airgap helm 차트에도 migration Job 이
#       없다. 그 결과 설치 직후 DB 스키마가 비어 있어 `users` 등 테이블이 없고
#       로그인/데이터 화면이 전부 실패한다. 본 스크립트가 그 공백을 메운다.
#
# 동작:
#   - 마이그레이션 SQL 은 nullus-api 이미지에 항상 포함된 /etc/nullus/migrations/
#     (Dockerfile: COPY db/migrations/ /etc/nullus/migrations/) 에서 읽는다.
#   - postgres 파드 안에서 psql 로 *.up.sql 을 버전 순서대로 적용한다.
#   - golang-migrate 호환 schema_migrations(version,dirty) 로 진척을 추적하므로
#     멱등하며(이미 적용된 버전은 건너뜀) 재실행해도 안전하다.
#
# 사용법:
#   ./26-migrate-db.sh
#   NAMESPACE=nullus DB_NAME=nullus DB_USER=nullus ./26-migrate-db.sh
#
# 환경 변수:
#   NAMESPACE   nullus 네임스페이스 (기본: nullus)
#   DB_NAME     DB 이름 (기본: nullus)
#   DB_USER     DB 유저 (기본: nullus)
#   DB_PASSWORD DB 비번 (기본: postgres secret 에서 자동 조회, 실패 시 nullus)
#   API_DEPLOY  SQL 소스 deploy (기본: nullus-api)
#
# 종료 코드:
#   0 — 성공 (적용 0건 포함)
#   1 — 사전조건/적용 실패
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

NAMESPACE="${NAMESPACE:-nullus}"
DB_NAME="${DB_NAME:-nullus}"
DB_USER="${DB_USER:-nullus}"
API_DEPLOY="${API_DEPLOY:-nullus-api}"
MIG_DIR="/etc/nullus/migrations"

command -v kubectl >/dev/null || { log_err "kubectl 없음"; exit 1; }
kubectl cluster-info >/dev/null 2>&1 || { log_err "클러스터 접근 불가"; exit 1; }

# -----------------------------------------------------------------------------
# 파드/자격증명 탐지
# -----------------------------------------------------------------------------
# api 파드는 SQL 소스다. helm 설치 직후엔 아직 init(wait-for-db) 중일 수 있으므로
# 컨테이너가 exec 가능해질 때까지 대기한다.
find_api_pod() {
  local p
  p="$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=nullus-api \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  [[ -z "$p" ]] && p="$(kubectl get pods -n "$NAMESPACE" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null | grep -m1 '^nullus-api' || true)"
  echo "$p"
}
API_POD=""
log_info "nullus-api 파드(SQL 소스) 기동 대기..."
for i in $(seq 1 60); do
  API_POD="$(find_api_pod)"
  if [[ -n "$API_POD" ]] && kubectl exec -n "$NAMESPACE" "$API_POD" -- sh -c "ls $MIG_DIR >/dev/null 2>&1" >/dev/null 2>&1; then
    break
  fi
  [[ "$i" == "60" ]] && { log_err "nullus-api 파드 exec 불가 (먼저 21-install-nullus.sh 실행/기동 확인)"; exit 1; }
  sleep 3
done

PG_POD="$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=postgresql \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
[[ -z "$PG_POD" ]] && PG_POD="$(kubectl get pods -n "$NAMESPACE" \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null | grep -m1 'postgresql' || true)"
[[ -z "$PG_POD" ]] && { log_err "postgresql 파드를 찾을 수 없음"; exit 1; }

if [[ -z "${DB_PASSWORD:-}" ]]; then
  DB_PASSWORD="$(kubectl get secret -n "$NAMESPACE" nullus-postgresql \
    -o jsonpath='{.data.password}' 2>/dev/null | base64 -d 2>/dev/null || true)"
  [[ -z "$DB_PASSWORD" ]] && DB_PASSWORD="nullus"
fi

log_info "api 파드 : $API_POD ($MIG_DIR)"
log_info "pg 파드  : $PG_POD (db=$DB_NAME user=$DB_USER)"

# postgres 컨테이너 이름 (bitnami=postgresql)
PG_CTR="$(kubectl get pod -n "$NAMESPACE" "$PG_POD" \
  -o jsonpath='{.spec.containers[0].name}' 2>/dev/null || echo postgresql)"

# psql 실행 헬퍼 (stdin 으로 SQL 주입)
psql_exec() {
  kubectl exec -i -n "$NAMESPACE" "$PG_POD" -c "$PG_CTR" -- \
    env PGPASSWORD="$DB_PASSWORD" psql -v ON_ERROR_STOP=1 -qtA -U "$DB_USER" -d "$DB_NAME" "$@"
}

# -----------------------------------------------------------------------------
# DB 준비 대기
# -----------------------------------------------------------------------------
log_info "DB 연결 대기..."
for i in $(seq 1 30); do
  if echo 'SELECT 1;' | psql_exec >/dev/null 2>&1; then break; fi
  [[ "$i" == "30" ]] && { log_err "DB 연결 실패 (비번/기동 확인)"; exit 1; }
  sleep 2
done

# golang-migrate 호환 추적 테이블 보장
echo 'CREATE TABLE IF NOT EXISTS schema_migrations (version bigint NOT NULL, dirty boolean NOT NULL);' | psql_exec >/dev/null

CURRENT="$(echo 'SELECT COALESCE(MAX(version),0) FROM schema_migrations WHERE dirty = false;' | psql_exec 2>/dev/null | head -1)"
CURRENT="${CURRENT:-0}"
log_info "현재 마이그레이션 버전: $CURRENT"

# -----------------------------------------------------------------------------
# up.sql 목록 (api 이미지에서) — 버전 순
# -----------------------------------------------------------------------------
FILES="$(kubectl exec -n "$NAMESPACE" "$API_POD" -- sh -c "ls -1 $MIG_DIR 2>/dev/null" 2>/dev/null \
  | grep -E '\.up\.sql$' | sort -t_ -k1,1n || true)"
[[ -z "$FILES" ]] && { log_err "$MIG_DIR 에 up.sql 없음 (api 이미지 확인)"; exit 1; }
TOTAL="$(printf '%s\n' "$FILES" | grep -c .)"
log_info "마이그레이션 파일 ${TOTAL}개 발견"

applied=0; skipped=0
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  ver="${f%%_*}"; ver="$((10#$ver))"   # 앞자리 0 제거, 10진수
  if [[ "$ver" -le "$CURRENT" ]]; then
    skipped=$((skipped+1)); continue
  fi
  log_info "적용 ▶ $f (v$ver)"
  # dirty 표시 → 적용 → 성공 시 clean 갱신
  echo "DELETE FROM schema_migrations; INSERT INTO schema_migrations(version,dirty) VALUES ($ver,true);" | psql_exec >/dev/null
  if ! kubectl exec -n "$NAMESPACE" "$API_POD" -- sh -c "cat $MIG_DIR/$f" 2>/dev/null \
        | psql_exec --single-transaction >/dev/null; then
    log_err "마이그레이션 실패: $f (schema_migrations dirty=true 로 남음)"
    exit 1
  fi
  echo "DELETE FROM schema_migrations; INSERT INTO schema_migrations(version,dirty) VALUES ($ver,false);" | psql_exec >/dev/null
  applied=$((applied+1))
done <<<"$FILES"

NEWVER="$(echo 'SELECT COALESCE(MAX(version),0) FROM schema_migrations WHERE dirty = false;' | psql_exec 2>/dev/null | head -1)"
log_ok "마이그레이션 완료 — 적용 ${applied}개 / 스킵 ${skipped}개 / 현재 버전 ${NEWVER}"

# 핵심 테이블 sanity
if echo 'SELECT 1 FROM users LIMIT 1;' | psql_exec >/dev/null 2>&1; then
  cnt="$(echo 'SELECT count(*) FROM users;' | psql_exec 2>/dev/null | head -1)"
  log_ok "users 테이블 확인 — 시드 사용자 ${cnt}명"
else
  log_warn "users 테이블 미확인 — 마이그레이션 내용 점검 필요"
fi
