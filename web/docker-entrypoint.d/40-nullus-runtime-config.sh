#!/bin/sh
# =============================================================================
# 40-nullus-runtime-config.sh — /config.js 를 환경변수로 생성한다
# =============================================================================
# nginx 공식 이미지는 기동 전에 /docker-entrypoint.d/*.sh 를 순서대로 실행한다.
# 여기서 만든 /config.js 를 index.html 이 앱 번들보다 먼저 읽어, OIDC 설정을
# 이미지 재빌드 없이 환경마다 바꿀 수 있게 한다.
#
# 값이 비어 있으면 해당 키를 넣지 않는다 — 앱이 빌드 시 값으로 폴백한다.
# =============================================================================
set -eu

# 컨테이너는 readOnlyRootFilesystem 으로 뜨는 것을 전제한다(차트 기본값). 그래서
# 도큐먼트 루트가 아니라 쓰기 가능한 볼륨에 쓰고, nginx 가 /config.js 로 서빙한다.
TARGET="${NULLUS_RUNTIME_CONFIG_PATH:-/var/lib/nullus/config.js}"

if ! mkdir -p "$(dirname "$TARGET")" 2>/dev/null || ! : > "$TARGET" 2>/dev/null; then
  # 쓸 수 없어도 기동은 막지 않는다 — 앱은 빌드 시 값으로 폴백한다.
  echo "[nullus] WARN: ${TARGET} 에 쓸 수 없습니다. 빌드 시 설정으로 폴백합니다." >&2
  exit 0
fi

# JS 문자열 리터럴로 안전하게 넣기 위해 역슬래시·따옴표를 이스케이프한다.
esc() {
  printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

emit() {
  key="$1"
  value="$2"
  [ -z "$value" ] && return 0
  printf '  %s: "%s",\n' "$key" "$(esc "$value")"
}

{
  echo '// 컨테이너 기동 시 생성됨 — 직접 수정하지 말 것'
  echo 'window.__NULLUS_CONFIG__ = {'
  emit authMode      "${NULLUS_AUTH_MODE:-}"
  emit oidcProvider  "${NULLUS_OIDC_PROVIDER:-}"
  emit oidcAuthority "${NULLUS_OIDC_AUTHORITY:-}"
  emit oidcClientId  "${NULLUS_OIDC_CLIENT_ID:-}"
  echo '}'
} > "$TARGET"

echo "[nullus] runtime config written to ${TARGET}"
cat "$TARGET"
