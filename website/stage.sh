#!/usr/bin/env bash
# 레포 안의 단독 HTML 가이드를 문서 사이트로 복사한다.
# 원본은 docs/ 에 하나만 두고, 사이트는 빌드할 때마다 그걸 가져간다 — 사본을
# 커밋하면 둘이 갈라진다.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$ROOT/website/docs/guides"

mkdir -p "$DEST"

copy() {
  local src="$ROOT/$1" dest="$DEST/$2"
  if [[ ! -f "$src" ]]; then
    echo "[stage] 원본이 없습니다: $1" >&2
    return 1
  fi
  cp "$src" "$dest"
  echo "[stage] $2 <- $1"
}

copy "docs/50_운영/Nullus_로컬_E2E_가이드.html" "local-e2e.html"
copy "docs/20_아키텍처/Nullus_백업복구_아키텍처.html" "backup-architecture.html"
