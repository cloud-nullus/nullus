#!/bin/bash
# 공식 sonarsource/sonar-scanner-cli 의 entrypoint.sh 를 따른다
# (SonarSource/sonar-scanner-cli-docker, LGPL-3.0). 인자가 없거나 플래그로 시작하면
# sonar-scanner 를 돌리고, SONAR_TOKEN 등 환경변수를 -D 속성으로 옮긴다. CI 잡은
# 보통 자기 스크립트를 넘기므로(script: / sh) 그대로 exec 된다.

set -euo pipefail

declare -a args=()

add_env_var_as_env_prop() {
  if [[ -n "$1" ]]; then
    args+=("-D$2=$1")
  fi
}

# 옛 방식(/tmp/cacerts)의 인증서를 신뢰 저장소로 들인다 — 내부 CA 로 TLS 를 켠 SonarQube 에
# 닿을 때 쓴다.
if [[ -d /tmp/cacerts ]]; then
  # shellcheck disable=SC2312
  if [[ -n "$(ls -A /tmp/cacerts 2>/dev/null)" ]]; then
    mkdir -p "$SONAR_USER_HOME/ssl"
    args+=("-Dsonar.scanner.truststorePassword=changeit")
    export SONAR_SCANNER_OPTS="${SONAR_SCANNER_OPTS:-} -Djavax.net.ssl.trustStore=$SONAR_USER_HOME/ssl/truststore.p12 -Djavax.net.ssl.trustStorePassword=changeit"
    for f in /tmp/cacerts/*; do
      echo "Importing certificate: ${f}"
      keytool -importcert -storetype PKCS12 -file "${f}" -alias "$(basename "${f}")" -keystore "$SONAR_USER_HOME/ssl/truststore.p12" -storepass changeit -trustcacerts -noprompt
    done
  fi
fi

if [[ "$#" == 0 ]]; then
  set -- sonar-scanner
fi

if [[ "${1#-}" != "${1}" ]] || ! command -v "${1}" > /dev/null; then
  set -- sonar-scanner "$@"
fi

if [[ "$1" = 'sonar-scanner' ]]; then
  add_env_var_as_env_prop "${SONAR_TOKEN:-}" "sonar.token"
  add_env_var_as_env_prop "${SONAR_PROJECT_BASE_DIR:-}" "sonar.projectBaseDir"
  add_env_var_as_env_prop "${SCANNER_WORKDIR_PATH:-}" "sonar.working.directory"
  if [[ ${#args[@]} -ne 0 ]]; then
    set -- sonar-scanner "${args[@]}" "${@:2}"
  fi
fi

exec "$@"
