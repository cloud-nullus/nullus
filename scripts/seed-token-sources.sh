#!/usr/bin/env bash
set -euo pipefail

DATABASE_URL="${DATABASE_URL:-postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable}"
OPENBAO_ADDR="${OPENBAO_ADDR:-}"
OPENBAO_TOKEN="${OPENBAO_TOKEN:-}"

if command -v psql >/dev/null 2>&1; then
  PSQL=(psql "$DATABASE_URL")
else
  PSQL=(docker exec -i draft-postgres-1 psql -U nullus -d nullus)
fi

ORG_ID="${TOKEN_SOURCE_ORG_ID:-}"
if [[ -z "$ORG_ID" ]] && command -v curl >/dev/null 2>&1; then
  ORG_ID="$(curl -fsS http://localhost:8090/api/v1/admin/organization 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || true)"
fi
ORG_ID="${ORG_ID:-11111111-1111-1111-1111-111111111111}"
MODULE="${TOKEN_SOURCE_MODULE:-artifacts}"
PROVIDER="${TOKEN_SOURCE_PROVIDER:-github}"
TOKEN_PATH="${TOKEN_SOURCE_PATH:-kv/nullus/dev/${ORG_ID}/artifacts/github/token}"
TOKEN_VALUE="${TOKEN_SOURCE_VALUE:-mock-github-token-123}"
TOKEN_TYPE="${TOKEN_SOURCE_TYPE:-reissue}"
STATUS="${TOKEN_SOURCE_STATUS:-healthy}"
SECRET_MANAGER="${TOKEN_SOURCE_SECRET_MANAGER:-openbao}"

if [[ -n "$OPENBAO_ADDR" && -n "$OPENBAO_TOKEN" ]]; then
  # 경로 규약(kv/nullus/...)과 실제 마운트 이름이 일치한다.
  # 부트스트랩 Job 이 KV v2 를 'kv' 로 마운트하므로 재작성하지 않는다.
  openbao_mount="${TOKEN_PATH%%/*}"
  openbao_path="${TOKEN_PATH#*/}"
  # OpenBao 쓰기는 best-effort 다. 실제 시드는 아래 token_sources row 이고,
  # OpenBao 는 로컬(런북)에 없을 수 있다 — 기본값 openbao.nullus.internal 은
  # 클러스터 내부 주소라 호스트에서 DNS 해석이 안 된다. 여기서 죽으면
  # `runbook_local.sh up --seed` 전체가 exit 6 으로 끝나므로 경고만 남기고 계속한다.
  if curl -fsS -X POST "${OPENBAO_ADDR%/}/v1/${openbao_mount}/data/${openbao_path}" \
      -H 'Content-Type: application/json' \
      -H "X-Vault-Token: $OPENBAO_TOKEN" \
      -d "$(printf '{"data":{"token":"%s"}}' "$TOKEN_VALUE")" >/dev/null 2>&1; then
    echo "wrote token to OpenBao: ${OPENBAO_ADDR%/}/v1/${openbao_mount}/data/${openbao_path}"
  else
    echo "warn: OpenBao 쓰기 실패 (${OPENBAO_ADDR}) — DB row 만 시드합니다" >&2
  fi
fi

"${PSQL[@]}" -v ON_ERROR_STOP=1 <<SQL
INSERT INTO token_sources (
  org_id,
  module,
  provider,
  path,
  token_type,
  status,
  metadata,
  updated_at
)
VALUES (
  '$ORG_ID',
  '$MODULE',
  '$PROVIDER',
  '$TOKEN_PATH',
  '$TOKEN_TYPE',
  '$STATUS',
  jsonb_build_object('secret_manager', '$SECRET_MANAGER'),
  now()
)
ON CONFLICT (org_id, provider, path) WHERE deleted_at IS NULL
DO UPDATE SET
  module = EXCLUDED.module,
  token_type = EXCLUDED.token_type,
  status = EXCLUDED.status,
  metadata = EXCLUDED.metadata,
  updated_at = now();
SQL

printf 'seeded token source: %s %s %s\n' "$ORG_ID" "$PROVIDER" "$TOKEN_PATH"
