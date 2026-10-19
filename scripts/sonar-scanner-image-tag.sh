#!/usr/bin/env bash
# =============================================================================
# sonar-scanner-image-tag.sh — 파이프라인 SAST 스캐너 이미지의 태그를 출력한다
# =============================================================================
# 태그는 "<스캐너 버전>-<이미지 리비전>" 이다(deploy/images/sonar-scanner/Dockerfile 의 두 ARG).
# 같은 스캐너 버전에서 이미지 내용이 바뀌면 리비전을 올린다 — 태그가 그대로면 CD 가 다시 밀어도
# 이미 받아 둔 노드는 옛 이미지를 계속 쓴다(쿠버네티스 기본 pull 정책 IfNotPresent).
#
# CD · 이미지 스캔 · 로컬 빌드가 모두 이 스크립트로 태그를 정한다. 코드 상수
# (internal/shared/domain/runtime_images.go 의 SonarScannerImage)는 테스트가 같은 값으로 묶는다.
# =============================================================================
set -euo pipefail
DOCKERFILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/deploy/images/sonar-scanner/Dockerfile"
version="$(grep -E '^ARG SONAR_SCANNER_VERSION=' "$DOCKERFILE" | cut -d= -f2)"
revision="$(grep -E '^ARG NULLUS_IMAGE_REVISION=' "$DOCKERFILE" | cut -d= -f2)"
if [[ -z "$version" || -z "$revision" ]]; then
  echo "Dockerfile 에서 SONAR_SCANNER_VERSION 또는 NULLUS_IMAGE_REVISION 을 찾지 못했습니다: $DOCKERFILE" >&2
  exit 1
fi
echo "${version}-${revision}"
