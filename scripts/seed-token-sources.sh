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

ORG_ID="${TOKEN_SOURCE_ORG_ID:-11111111-1111-1111-1111-111111111111}"
MODULE="${TOKEN_SOURCE_MODULE:-artifacts}"
PROVIDER="${TOKEN_SOURCE_PROVIDER:-github}"
TOKEN_PATH="${TOKEN_SOURCE_PATH:-kv/nullus/dev/11111111-1111-1111-1111-111111111111/artifacts/github/token}"
TOKEN_VALUE="${TOKEN_SOURCE_VALUE:-mock-github-token-123}"
TOKEN_TYPE="${TOKEN_SOURCE_TYPE:-reissue}"
STATUS="${TOKEN_SOURCE_STATUS:-healthy}"
SECRET_MANAGER="${TOKEN_SOURCE_SECRET_MANAGER:-openbao}"

if [[ -n "$OPENBAO_ADDR" && -n "$OPENBAO_TOKEN" ]]; then
  openbao_path="${TOKEN_PATH#kv/}"
  curl -fsS -X POST "${OPENBAO_ADDR%/}/v1/secret/data/${openbao_path}" \
    -H 'Content-Type: application/json' \
    -H "X-Vault-Token: $OPENBAO_TOKEN" \
    -d "$(printf '{"data":{"token":"%s"}}' "$TOKEN_VALUE")" >/dev/null
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
