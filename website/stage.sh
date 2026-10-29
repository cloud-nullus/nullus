#!/usr/bin/env bash
# 레포 안의 단독 HTML 가이드·발표 장표를 문서 사이트로 복사한다.
# 원본은 docs/ 에 하나만 두고, 사이트는 빌드할 때마다 그걸 가져간다 — 사본을
# 커밋하면 둘이 갈라진다.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SITE="$ROOT/website/docs"

copy() {
  local src="$ROOT/$1" dest="$SITE/$2"
  if [[ ! -f "$src" ]]; then
    echo "[stage] 원본이 없습니다: $1" >&2
    return 1
  fi
  mkdir -p "$(dirname "$dest")"
  cp "$src" "$dest"
  echo "[stage] $2 <- $1"
}

copy "docs/50_운영/Nullus_로컬_E2E_가이드.html" "guides/local-e2e.html"
copy "docs/20_아키텍처/Nullus_백업복구_아키텍처.html" "guides/backup-architecture.html"

# 발표 장표는 발표자 노트를 뺀 배포용만 올린다. 시연 영상(mp4)은 올리지 않는다 —
# 장표가 같은 폴더에서 영상을 못 찾으면 유튜브를 새 탭으로 연다.
copy "docs/presentations/Nullus_3차기술검증_발표장표_v0.4_배포용.html" "presentations/stp3-v0.4.html"
