#!/usr/bin/env bash
# =============================================================================
# 00-provision.sh — OpenTofu 로 Kakao Cloud VM 프로비저닝
# =============================================================================
# 사용법:
#   ./scripts/00-provision.sh              # tofu apply (자동 승인 없음)
#   ./scripts/00-provision.sh -auto-approve
#   ./scripts/00-provision.sh -destroy     # 리소스 삭제
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOFU_DIR="${SCRIPT_DIR}/../opentofu"

# tofu 우선, 없으면 terraform 사용
if command -v tofu >/dev/null 2>&1; then
  TF_CMD="tofu"
elif command -v terraform >/dev/null 2>&1; then
  TF_CMD="terraform"
  echo "[WARN] tofu 를 찾을 수 없어 terraform 으로 대체합니다." >&2
else
  echo "[ERR] tofu 또는 terraform 이 설치되어 있지 않습니다." >&2
  exit 1
fi

echo "[INFO] 사용 커맨드: ${TF_CMD}" >&2
echo "[INFO] 작업 디렉토리: ${TOFU_DIR}" >&2

cd "${TOFU_DIR}"

echo "[INFO] ${TF_CMD} init ..." >&2
"${TF_CMD}" init

echo "[INFO] ${TF_CMD} apply $* ..." >&2
"${TF_CMD}" apply "$@"

echo "[OK] 프로비저닝 완료." >&2
echo "[INFO] 다음 단계: scripts/10-build-on-builder.sh" >&2
