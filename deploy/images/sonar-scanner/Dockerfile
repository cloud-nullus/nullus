# =============================================================================
# Nullus SonarScanner CLI — 멀티아치(linux/amd64 · linux/arm64) 스캐너 이미지
# =============================================================================
# 공식 sonarsource/sonar-scanner-cli 는 amd64 빌드만 게시한다(Docker Hub 전 태그 실측,
# 2026-10). arm64 노드에서는 에뮬레이션 없이는 파이프라인 SAST 단계가 서지 않고,
# 에어갭 arm64 번들은 그 이미지를 아예 건너뛴다(01-pull-images.sh 의 skipped-platform).
#
# 스캐너 CLI 의 zip 은 플랫폼 독립(순수 Java)이고, 베이스로 쓰는 amazoncorretto 는
# 두 아키텍처를 모두 낸다. 공식 Dockerfile(SonarSource/sonar-scanner-cli-docker, LGPL-3.0)
# 의 구조를 그대로 따르되 플랫폼마다 굽는다 — 빌드 단계는 TARGETARCH 와 무관하다.
#
# 태그는 릴리즈 버전이 아니라 아래 ARG 의 스캐너 버전이다(nullus-jenkins 와 같은 방침).
# CD(.github/workflows/cd.yml)가 이 값을 단일 출처로 읽고, 코드의 상수
# (internal/shared/domain/runtime_images.go)는 테스트가 같은 값으로 묶는다.
# =============================================================================
FROM alpine:3.21 AS builder

ARG SONAR_SCANNER_HOME=/opt/sonar-scanner
ARG SONAR_SCANNER_VERSION=8.1.0.6389
ENV HOME=/tmp \
    XDG_CONFIG_HOME=/tmp \
    SONAR_SCANNER_HOME=${SONAR_SCANNER_HOME} \
    SCANNER_BINARIES=https://binaries.sonarsource.com/Distribution/sonar-scanner-cli
ENV SCANNER_ZIP_URL="${SCANNER_BINARIES}/sonar-scanner-cli-${SONAR_SCANNER_VERSION}.zip"

WORKDIR /opt

ADD ${SCANNER_ZIP_URL} /opt/sonar-scanner-cli.zip
ADD ${SCANNER_ZIP_URL}.asc /opt/sonar-scanner-cli.zip.asc

# 서명을 검증한다 — 공식 이미지와 같은 SonarSource 공개키다.
RUN set -eux; \
    apk add --no-cache --virtual build-dependencies gnupg unzip; \
    gpg --batch --keyserver hkps://keyserver.ubuntu.com --recv-keys 679F1EE92B19609DE816FDE81DB198F93525EC1A; \
    gpg --verify /opt/sonar-scanner-cli.zip.asc /opt/sonar-scanner-cli.zip; \
    unzip -q sonar-scanner-cli.zip; \
    rm sonar-scanner-cli.zip sonar-scanner-cli.zip.asc; \
    mv "sonar-scanner-${SONAR_SCANNER_VERSION}" "${SONAR_SCANNER_HOME}"; \
    apk del --purge build-dependencies


# amazoncorretto 는 linux/amd64·linux/arm64 를 모두 낸다.
FROM amazoncorretto:21-al2023

ARG SONAR_SCANNER_HOME=/opt/sonar-scanner
ENV HOME=/tmp \
    XDG_CONFIG_HOME=/tmp \
    SONAR_SCANNER_HOME=${SONAR_SCANNER_HOME} \
    SONAR_USER_HOME=${SONAR_SCANNER_HOME}/.sonar \
    PATH=${SONAR_SCANNER_HOME}/bin:${PATH} \
    SRC_PATH=/usr/src \
    SCANNER_WORKDIR_PATH=/tmp/.scannerwork \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

COPY --from=builder /opt/sonar-scanner /opt/sonar-scanner

# 파이프라인 SAST 단계가 쓰는 것만 둔다: git(스캐너의 blame), curl(리포트 — sast.go 의
# sastReportLine 이 SonarQube API 를 curl 로 읽는다), tar(GitHub Actions 컨테이너 잡).
# 공식 이미지의 node·python·poetry 는 싣지 않는다 — JS/TS·Python 분석은 스캐너가 내장한
# 분석기가 하고, 그 런타임은 스택의 파이프라인이 쓰지 않는다. 필요해지면 그때 더한다.
RUN set -eux; \
    dnf install -y git tar curl-minimal; \
    dnf clean all; \
    groupadd --system --gid 1000 scanner-cli; \
    useradd --system -d "${HOME}" --uid 1000 --gid scanner-cli scanner-cli; \
    mkdir -p "${SRC_PATH}" "${SONAR_USER_HOME}" "${SONAR_USER_HOME}/cache" "${SCANNER_WORKDIR_PATH}"; \
    chown -R scanner-cli:scanner-cli "${SONAR_SCANNER_HOME}" "${SRC_PATH}" "${SCANNER_WORKDIR_PATH}"; \
    chmod -R 555 "${SONAR_SCANNER_HOME}"; \
    chmod -R 754 "${SRC_PATH}" "${SONAR_USER_HOME}" "${SCANNER_WORKDIR_PATH}"; \
    dnf upgrade -y --releasever=latest --security; \
    dnf clean all

COPY --chown=scanner-cli:scanner-cli bin/entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod 555 /usr/bin/entrypoint.sh

USER 1000:1000

WORKDIR ${SRC_PATH}

ENTRYPOINT ["/usr/bin/entrypoint.sh"]

CMD ["sonar-scanner"]
