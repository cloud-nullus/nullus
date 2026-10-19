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
# 태그는 릴리즈 버전이 아니라 아래 ARG 의 스캐너 버전과 이미지 리비전이다(nullus-jenkins 와 같은 방침).
# CD(.github/workflows/cd.yml)가 scripts/sonar-scanner-image-tag.sh 로 이 값을 읽고, 코드의 상수
# (internal/shared/domain/runtime_images.go)는 테스트가 같은 값으로 묶는다.
# =============================================================================
FROM alpine:3.21 AS builder

ARG SONAR_SCANNER_HOME=/opt/sonar-scanner
ARG SONAR_SCANNER_VERSION=8.1.0.6389
# NULLUS_IMAGE_REVISION 은 같은 스캐너 버전에서 이미지 내용이 바뀔 때 올린다. 태그는 "<버전>-<리비전>"
# 이다(scripts/sonar-scanner-image-tag.sh). 태그가 그대로면 CD 가 다시 밀어도 이미 받아 둔 노드는 옛
# 이미지를 계속 쓴다(쿠버네티스 기본 pull 정책 IfNotPresent). 리비전 1 은 접미사 없는 첫 태그였고,
# 2 는 Node.js 를 넣은 것이다. 스캐너 버전을 올리면 1 로 되돌린다.
ARG NULLUS_IMAGE_REVISION=2
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
# sastReportLine 이 SonarQube API 를 curl 로 읽는다), tar(GitHub Actions 컨테이너 잡),
# nodejs22(JS/TS/CSS 분석).
#
# Node.js 가 있어야 한다. SonarQube 의 JS/TS/CSS 분석기는 Node 런타임을 플랫폼별로 내장하는데
# LINUX_ARM64 용은 없어 PATH 의 node 를 찾는다 — 없으면 .js·.css 파일이 하나만 있어도 분석
# 전체가 EXECUTION FAILURE 로 끝난다(arm64 kind, SonarQube 26.9, 전자정부 표준프레임워크
# 샘플의 정적 CSS/JS 로 실측). 그러면 SAST 를 기다리는 deploy 가 돌지 않는다. amd64 에서는
# 내장 런타임을 쓰므로 같은 이미지가 두 아키텍처에서 같게 동작한다.
# 공식 이미지의 python·poetry 는 싣지 않는다 — 스택의 파이프라인이 쓰지 않는다.
RUN set -eux; \
    dnf install -y git tar curl-minimal nodejs22; \
    node --version; \
    dnf clean all; \
    groupadd --system --gid 1000 scanner-cli; \
    useradd --system -d "${HOME}" --uid 1000 --gid scanner-cli scanner-cli; \
    mkdir -p "${SRC_PATH}" "${SONAR_USER_HOME}" "${SONAR_USER_HOME}/cache" "${SCANNER_WORKDIR_PATH}"; \
    chown -R scanner-cli:scanner-cli "${SONAR_SCANNER_HOME}" "${SRC_PATH}" "${SCANNER_WORKDIR_PATH}"; \
    chmod -R 555 "${SONAR_SCANNER_HOME}"; \
    chmod -R 754 "${SRC_PATH}" "${SONAR_USER_HOME}" "${SCANNER_WORKDIR_PATH}"; \
    dnf upgrade -y --releasever=latest --security; \
    dnf clean all

# 저장소 루트의 .gitignore 가 bin/ 을 무시한다(Go 빌드 산출물). 공식 이미지처럼 bin/ 아래 두면
# 커밋되지 않아 CD 빌드가 COPY 에서 죽는다 — 실제로 그랬다(#301 첫 CD). 루트에 둔다.
COPY --chown=scanner-cli:scanner-cli entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod 555 /usr/bin/entrypoint.sh

USER 1000:1000

WORKDIR ${SRC_PATH}

ENTRYPOINT ["/usr/bin/entrypoint.sh"]

CMD ["sonar-scanner"]
