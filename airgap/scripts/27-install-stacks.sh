#!/usr/bin/env bash
# =============================================================================
# 27-install-stacks.sh — Nullus 카탈로그 스택 일괄 설치 (airgap)
# =============================================================================
# 용도: airgap/helm/charts-catalog/ 에 번들된 chart 로 DevSecOps 카탈로그
#       스택 전체(또는 선택)를 설치한다. 이미 설치된 릴리스는 upgrade 된다
#       (helm upgrade --install — 멱등).
#
# containerd mirror 설정(kind-airgap.yaml)에 의해 docker.io, ghcr.io,
# registry.gitlab.com, registry.k8s.io, quay.io, public.ecr.aws 이미지가
# 모두 kind-registry:5000(localhost:5001) 로 리다이렉트 된다.
# 따라서 차트 기본 이미지 레퍼런스를 그대로 사용하면 로컬 레지스트리에서
# 이미지를 가져온다 — 추가 registry override 불필요.
#
# 사용법:
#   ./27-install-stacks.sh                          # 전체 스택 설치 (순서 보장)
#   ./27-install-stacks.sh argocd harbor keycloak   # 지정 스택만 설치
#   ./27-install-stacks.sh --list                   # 스택 목록 출력 후 종료
#
# 환경 변수:
#   DRY_RUN=1           helm template 으로 렌더만 (클러스터 변경 없음)
#   WAIT=1              --wait 플래그 추가 (배포 완료 대기)
#   NAMESPACE_NULLUS    nullus 네임스페이스     (기본: nullus)
#   NAMESPACE_AUTH      인증 네임스페이스       (기본: nullus-auth)
#   NAMESPACE_OBSERV    모니터링 네임스페이스    (기본: nullus-monitoring)
#   NAMESPACE_GITLAB    GitLab 네임스페이스     (기본: gitlab)
#   NAMESPACE_CERT      cert-manager 네임스페이스 (기본: cert-manager)
#
# 종료 코드:
#   0 — 모든 선택 스택 성공 (또는 --list)
#   1 — 1개 이상 스택 실패 (실패 시에도 이후 스택 계속 진행)
# =============================================================================
set -euo pipefail
IFS=$'\n\t'

# ---------------------------------------------------------------------------
# 경로 해석: 두 가지 레이아웃 지원
#   1) 원본 repo 트리  : <repo>/airgap/scripts/27-install-stacks.sh
#      → AIRGAP_DIR = <repo>/airgap/
#   2) 패키징된 번들   : <bundle>/scripts/27-install-stacks.sh
#      → AIRGAP_DIR = <bundle>/
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# scripts/ 의 부모 디렉토리
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -d "${PARENT_DIR}/helm/charts-catalog" ]]; then
  AIRGAP_DIR="${PARENT_DIR}"
elif [[ -d "${PARENT_DIR}/airgap/helm/charts-catalog" ]]; then
  AIRGAP_DIR="${PARENT_DIR}/airgap"
else
  # fallback: scripts/ 와 나란히 helm/ 이 있는 경우
  AIRGAP_DIR="${SCRIPT_DIR%/scripts}"
fi

CATALOG_DIR="${AIRGAP_DIR}/helm/charts-catalog"
VALUES_DIR="${AIRGAP_DIR}/helm/stack-values"

# ---------------------------------------------------------------------------
# 색상 로그 헬퍼
# ---------------------------------------------------------------------------
if [[ -t 1 ]]; then
  CL_INFO=$'\033[1;34m'; CL_OK=$'\033[1;32m'; CL_WARN=$'\033[1;33m'; CL_ERR=$'\033[1;31m'; CL_RST=$'\033[0m'
else
  CL_INFO=""; CL_OK=""; CL_WARN=""; CL_ERR=""; CL_RST=""
fi
log_info() { printf '%s[INFO]%s %s\n' "$CL_INFO" "$CL_RST" "$*" >&2; }
log_warn() { printf '%s[WARN]%s %s\n' "$CL_WARN" "$CL_RST" "$*" >&2; }
log_err()  { printf '%s[ERR ]%s %s\n' "$CL_ERR"  "$CL_RST" "$*" >&2; }
log_ok()   { printf '%s[ OK ]%s %s\n' "$CL_OK"   "$CL_RST" "$*" >&2; }
hdr() {
  local sep="------------------------------------------------------------"
  printf '%s\n[%s] %s\n%s\n' \
    "${CL_INFO}${sep}${CL_RST}" \
    "${CL_INFO}$1${CL_RST}" "$2" \
    "${CL_INFO}${sep}${CL_RST}" >&2
}

# ---------------------------------------------------------------------------
# 환경 변수
# ---------------------------------------------------------------------------
DRY_RUN="${DRY_RUN:-0}"
WAIT="${WAIT:-0}"
NAMESPACE_NULLUS="${NAMESPACE_NULLUS:-nullus}"
NAMESPACE_AUTH="${NAMESPACE_AUTH:-nullus-auth}"
NAMESPACE_OBSERV="${NAMESPACE_OBSERV:-nullus-monitoring}"
NAMESPACE_GITLAB="${NAMESPACE_GITLAB:-gitlab}"
NAMESPACE_CERT="${NAMESPACE_CERT:-cert-manager}"

# ---------------------------------------------------------------------------
# 스택 정의 테이블
# 형식: "key|release|chart-tgz-prefix|namespace|version|values-file-key"
# values-file-key 가 "-" 이면 values 파일 없음
# ---------------------------------------------------------------------------
STACK_TABLE=(
  "certmanager|cert-manager|cert-manager|${NAMESPACE_CERT}|v1.16.3|certmanager"
  "metrics-server|metrics-server|metrics-server|${NAMESPACE_NULLUS}|3.12.2|metrics-server"
  "minio|nullus-minio|minio|${NAMESPACE_NULLUS}|5.4.0|minio"
  "gitlab-postgres|nullus-postgresql|postgresql|${NAMESPACE_GITLAB}|16.7.21|gitlab-postgres"
  "gitlab|gitlab|gitlab|${NAMESPACE_GITLAB}|8.7.2|gitlab"
  "gitlab-runner|gitlab-runner|gitlab-runner|${NAMESPACE_NULLUS}|0.72.0|gitlab-runner"
  "argocd|argo-cd|argo-cd|${NAMESPACE_NULLUS}|7.7.16|argocd"
  "prometheus|kps|kube-prometheus-stack|${NAMESPACE_OBSERV}|69.3.0|prometheus"
  "grafana|grafana|grafana|${NAMESPACE_NULLUS}|8.9.0|grafana"
  "loki|loki|loki|${NAMESPACE_NULLUS}|2.10.3|loki"
  "opensearch|opensearch|opensearch|${NAMESPACE_NULLUS}|2.22.0|opensearch"
  "otel|otel|opentelemetry-collector|${NAMESPACE_NULLUS}|0.75.0|otel"
  "keycloak|keycloak|keycloak|${NAMESPACE_AUTH}|24.4.5|keycloak"
  "harbor|harbor|harbor|${NAMESPACE_NULLUS}|1.15.0|harbor"
  "gateway|eg|gateway-helm|${NAMESPACE_NULLUS}|1.4.3|gateway"
)

# ---------------------------------------------------------------------------
# 헬퍼
# ---------------------------------------------------------------------------
find_chart() {
  local prefix="$1"
  local found
  found="$(ls "${CATALOG_DIR}/${prefix}"-*.tgz 2>/dev/null | head -1 || true)"
  echo "$found"
}

values_file() {
  local key="$1"
  local vf="${VALUES_DIR}/${key}.yaml"
  # 파일이 존재하고, 주석+공백 제거 후 실질 내용이 있을 때만 경로 반환
  if [[ -f "$vf" ]] && grep -qE '^[^#[:space:]]' "$vf" 2>/dev/null; then
    echo "$vf"
  fi
}

# ---------------------------------------------------------------------------
# --list 처리
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--list" ]]; then
  printf '%-16s %-22s %-22s %-20s %s\n' \
    "KEY" "RELEASE" "CHART" "NAMESPACE" "VERSION"
  printf '%s\n' "$(printf '%.0s-' {1..90})"
  for entry in "${STACK_TABLE[@]}"; do
    IFS='|' read -r key release chart_prefix ns ver vkey <<<"$entry"
    printf '%-16s %-22s %-22s %-20s %s\n' "$key" "$release" "${chart_prefix}-*.tgz" "$ns" "$ver"
  done
  exit 0
fi

# ---------------------------------------------------------------------------
# 사전 점검
# ---------------------------------------------------------------------------
command -v helm    >/dev/null || { log_err "helm not found";    exit 1; }
[[ -d "$CATALOG_DIR" ]] || { log_err "카탈로그 디렉토리 없음: $CATALOG_DIR"; exit 1; }

if [[ "$DRY_RUN" != "1" ]]; then
  command -v kubectl >/dev/null || { log_err "kubectl not found"; exit 1; }
  kubectl cluster-info >/dev/null 2>&1 || { log_err "클러스터에 접근할 수 없습니다"; exit 1; }
fi

# ---------------------------------------------------------------------------
# 설치할 스택 목록 결정
# ---------------------------------------------------------------------------
SELECTED_KEYS=()
if [[ $# -gt 0 ]]; then
  for arg in "$@"; do
    found=0
    for entry in "${STACK_TABLE[@]}"; do
      IFS='|' read -r key _ <<<"$entry"
      if [[ "$key" == "$arg" ]]; then
        SELECTED_KEYS+=("$arg")
        found=1
        break
      fi
    done
    if [[ "$found" == "0" ]]; then
      log_warn "알 수 없는 스택 키: '$arg' (--list 로 유효한 키 확인)"
    fi
  done
else
  for entry in "${STACK_TABLE[@]}"; do
    IFS='|' read -r key _ <<<"$entry"
    SELECTED_KEYS+=("$key")
  done
fi

if [[ ${#SELECTED_KEYS[@]} -eq 0 ]]; then
  log_err "설치할 스택이 없습니다 (--list 로 유효한 키 확인)"
  exit 1
fi

# ---------------------------------------------------------------------------
# 네임스페이스 생성 헬퍼
# ---------------------------------------------------------------------------
ensure_ns() {
  local ns="$1"
  kubectl get ns "$ns" >/dev/null 2>&1 || kubectl create namespace "$ns" >/dev/null
}

# gitlab 의 object storage(minio) 연결 시크릿. orchestrator 의
# installing_object_storage_secret 단계(helm-values.go: sharedObjectStorageSecretManifest)
# 를 재현. gitlab.yaml 이 global.appConfig.object_store 로 이 시크릿을 참조하므로
# gitlab 설치 전에 gitlab 네임스페이스에 반드시 존재해야 한다(미존재 시 webservice
# 가 FailedMount 로 Init 에서 멈춤).
ensure_object_storage_secret() {
  local ns="$1"
  local endpoint="http://nullus-minio.${NAMESPACE_NULLUS}.svc.cluster.local:9000"
  local conn="provider: AWS
region: us-east-1
aws_access_key_id: nullus-admin
aws_secret_access_key: nullus-minio-secret
endpoint: ${endpoint}
path_style: true"
  ensure_ns "$ns"
  kubectl create secret generic nullus-object-storage -n "$ns" \
    --from-literal=connection="$conn" \
    --from-literal=config="$conn" \
    --dry-run=client -o yaml 2>/dev/null | kubectl apply -f - >/dev/null 2>&1 \
    && log_info "  object storage 시크릿 보장: ${ns}/nullus-object-storage" \
    || log_warn "  object storage 시크릿 생성 실패 — gitlab webservice Init 멈춤 가능"
}

# ---------------------------------------------------------------------------
# 메인 설치 루프
# ---------------------------------------------------------------------------
hdr "스택 설치" "DRY_RUN=${DRY_RUN} WAIT=${WAIT} 스택 수=${#SELECTED_KEYS[@]}"

RESULTS=()
FAILED=0

# STACK_TABLE 순서로 선택된 스택만 처리 (순서 보장)
for entry in "${STACK_TABLE[@]}"; do
  IFS='|' read -r key release chart_prefix ns ver vkey <<<"$entry"

  # 선택 목록에 있는지 확인
  skip=1
  for sel in "${SELECTED_KEYS[@]}"; do
    [[ "$sel" == "$key" ]] && { skip=0; break; }
  done
  [[ "$skip" == "1" ]] && continue

  log_info "── [${key}] ──────────────────────────────────────"
  log_info "  release   : $release"
  log_info "  namespace : $ns"
  log_info "  version   : $ver"

  # chart tgz 탐색
  chart="$(find_chart "$chart_prefix")"
  if [[ -z "$chart" ]]; then
    log_err "  chart 없음: ${CATALOG_DIR}/${chart_prefix}-*.tgz"
    RESULTS+=("${key}|FAIL|chart not found")
    FAILED=1
    continue
  fi
  log_info "  chart     : $(basename "$chart")"

  # values 파일 경로 (없으면 빈 문자열)
  vfile="$(values_file "$vkey")"
  if [[ -n "$vfile" ]]; then
    log_info "  values    : $vfile"
  fi

  if [[ "$DRY_RUN" == "1" ]]; then
    # DRY_RUN: helm template (네임스페이스 플래그는 --namespace)
    cmd=(helm template "$release" "$chart" --namespace "$ns" --version "$ver")
    [[ -n "$vfile" ]] && cmd+=(-f "$vfile")
    log_info "  [DRY_RUN] $(printf '%q ' "${cmd[@]}")"
    if "${cmd[@]}" >/dev/null 2>&1; then
      log_ok "  $key => render OK"
      RESULTS+=("${key}|OK|dry-run")
    else
      out="$("${cmd[@]}" 2>&1 | tail -5)"
      log_err "  $key => render FAIL"
      log_err "  ${out}"
      RESULTS+=("${key}|FAIL|${out}")
      FAILED=1
    fi
  else
    # 실제 설치
    ensure_ns "$ns"
    # gitlab 은 object storage 시크릿이 선결 조건
    [[ "$key" == "gitlab" ]] && ensure_object_storage_secret "$ns"
    cmd=(helm upgrade --install "$release" "$chart"
      --namespace "$ns"
      --create-namespace
      --version "$ver")
    [[ -n "$vfile" ]] && cmd+=(-f "$vfile")
    # gitlab-runner 는 gitlab 이 생성한 실제 등록 토큰이 필요(values 의 더미 대체).
    # gitlab 차트가 gitlab 네임스페이스에 만든 secret 에서 추출한다.
    if [[ "$key" == "gitlab-runner" ]]; then
      rt="$(kubectl get secret -n "${NAMESPACE_GITLAB}" gitlab-gitlab-runner-secret \
        -o jsonpath='{.data.runner-registration-token}' 2>/dev/null | base64 -d 2>/dev/null || true)"
      if [[ -n "$rt" ]]; then
        cmd+=(--set runnerRegistrationToken="$rt")
        log_info "  gitlab 등록 토큰 주입 (gitlab-gitlab-runner-secret)"
      else
        log_warn "  gitlab 등록 토큰 미발견 — runner 등록 실패 가능(gitlab 먼저 설치 필요)"
      fi
    fi
    [[ "$WAIT" == "1" ]] && cmd+=(--wait --timeout 15m)

    log_info "  실행: $(printf '%q ' "${cmd[@]}")"
    if "${cmd[@]}" 2>&1 | sed 's/^/    /' >&2; then
      log_ok "  $key 설치 완료"
      RESULTS+=("${key}|OK|installed")
      # gitlab-postgres 는 다음 단계(gitlab)의 migrations 가 즉시 접속하므로
      # StatefulSet 이 Ready 가 될 때까지 반드시 대기한다 (미대기 시 migrations 실패).
      if [[ "$key" == "gitlab-postgres" ]]; then
        log_info "  gitlab-postgres Ready 대기 (gitlab migrations 선결 조건)..."
        kubectl rollout status "statefulset/${release}" -n "$ns" --timeout=300s 2>&1 \
          | sed 's/^/    /' >&2 || log_warn "  postgres Ready 미확인 — gitlab migrations 실패 가능"
      fi
    else
      log_warn "  $key 설치 실패 — 다음 스택으로 계속 진행"
      RESULTS+=("${key}|FAIL|helm error")
      FAILED=1
    fi
  fi
done

# ---------------------------------------------------------------------------
# 결과 요약 테이블
# ---------------------------------------------------------------------------
log_info ""
log_info "═══════════════════════════════════════"
log_info " 설치 결과 요약"
log_info "═══════════════════════════════════════"
for row in "${RESULTS[@]}"; do
  IFS='|' read -r k status msg <<<"$row"
  if [[ "$status" == "OK" ]]; then
    log_ok "  $(printf '%-16s' "$k") $status"
  else
    log_err "  $(printf '%-16s' "$k") $status  ($msg)"
  fi
done
log_info "═══════════════════════════════════════"

if [[ "$FAILED" == "0" ]]; then
  log_ok "모든 스택 설치 성공"
else
  log_warn "일부 스택 설치 실패 — 위 요약 확인"
fi

# ---------------------------------------------------------------------------
# 사후 안내
# ---------------------------------------------------------------------------
if [[ "$DRY_RUN" != "1" && "$FAILED" == "0" ]]; then
  log_info ""
  log_info "다음 단계:"
  log_info "  HTTPRoute 구성 (신규 서비스 접근 등록):"
  log_info "    bash ${SCRIPT_DIR}/23-setup-gateway.sh"
  log_info "  /etc/hosts 도메인 등록 (sudo):"
  log_info "    sudo bash ${SCRIPT_DIR}/24-register-hosts.sh"
fi

exit "$FAILED"
