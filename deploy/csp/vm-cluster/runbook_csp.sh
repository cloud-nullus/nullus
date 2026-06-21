#!/usr/bin/env bash
# =============================================================================
# runbook_csp.sh — Nullus CSP VM 클러스터 배포 스크립트
#
# 대상: CSP VM (Master x3, Worker x3) — Linux 신규 설치 상태
# 역할: 툴체인 설치 → Kubernetes 클러스터 구성 → 인프라 배포 → Nullus 배포
#
# 사용법:
#   ./scripts/runbook_csp.sh bootstrap   # 마스터/워커 공통 툴체인 설치
#   ./scripts/runbook_csp.sh init-master # 마스터 노드 초기화 (m1에서만 실행)
#   ./scripts/runbook_csp.sh join-master # 추가 마스터 노드 조인 (m2, m3에서 실행)
#   ./scripts/runbook_csp.sh join-worker # 워커 노드 조인 (w1~w3에서 실행)
#   ./scripts/runbook_csp.sh deploy      # Nullus 인프라+앱 배포 (m1에서만 실행)
#   ./scripts/runbook_csp.sh status      # 클러스터 및 Nullus 상태 확인
#   ./scripts/runbook_csp.sh upgrade     # Nullus 이미지/차트 업그레이드
#   ./scripts/runbook_csp.sh uninstall   # Nullus 앱 제거 (클러스터는 유지)
#
# 전제 조건:
#   - 모든 VM에 Linux(Ubuntu 22.04+ / RHEL 9+ / Rocky 9+)가 설치되어 있어야 합니다.
#   - VM 간 SSH 통신 및 인터넷 접속이 가능해야 합니다.
#   - m1(첫 번째 마스터) 에서 m2, m3, w1~w3의 join 커맨드를 생성합니다.
#   - 필요한 환경 변수는 스크립트 하단 "Configuration" 섹션을 참조하세요.
# =============================================================================
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration — 배포 환경에 맞게 수정하세요
# ─────────────────────────────────────────────────────────────────────────────

# Kubernetes 버전
K8S_VERSION="${K8S_VERSION:-1.31}"

# 컨테이너 런타임 (containerd | docker)
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-containerd}"

# Pod CIDR (Calico 기본값; Cilium 사용 시 그대로 유지)
POD_CIDR="${POD_CIDR:-192.168.0.0/16}"

# Service CIDR
SERVICE_CIDR="${SERVICE_CIDR:-10.96.0.0/12}"

# CNI 플러그인 (calico | cilium | flannel)
# 기본값 calico — POD_CIDR(192.168.0.0/16) 및 usage 안내와 일치
CNI_PLUGIN="${CNI_PLUGIN:-calico}"

# 마스터 노드 VIP 또는 로드밸런서 IP (HA 구성 시 필수)
# 단일 마스터라면 해당 VM의 IP를 입력
CONTROL_PLANE_ENDPOINT="${CONTROL_PLANE_ENDPOINT:-}"

# Helm 차트 경로 (프로젝트 루트 기준)
CHART_PATH="${CHART_PATH:-./deploy/helm/nullus}"

# Nullus 네임스페이스
NULLUS_NAMESPACE="${NULLUS_NAMESPACE:-nullus}"

# 컨테이너 레지스트리 (ghcr.io 기본)
REGISTRY="${REGISTRY:-ghcr.io/cloud-nullus}"

# 이미지 태그 (CD 파이프라인에서 주입되거나 수동 설정)
IMAGE_TAG="${IMAGE_TAG:-latest}"

# DB 패스워드 (반드시 변경할 것)
DB_PASSWORD="${DB_PASSWORD:-change-me-in-production}"

# Encryption Key (정확히 32바이트)
ENCRYPTION_KEY="${ENCRYPTION_KEY:-change-me-32bytes-production!!}"

# Ingress 호스트명
INGRESS_HOST="${INGRESS_HOST:-nullus.local}"

# Ingress 클래스명 (nginx | haproxy | traefik)
INGRESS_CLASS="${INGRESS_CLASS:-nginx}"

# MetalLB IP 풀 범위 (LoadBalancer 타입 서비스용, CSP 환경에 맞게 조정)
METALLB_IP_RANGE="${METALLB_IP_RANGE:-}"

# join 토큰 파일 경로 (m1에서 생성, 다른 노드와 공유)
JOIN_TOKEN_FILE="${JOIN_TOKEN_FILE:-/tmp/nullus-join.env}"

# API Server 추가 SAN (Subject Alternative Names) 설정
# 공인 IP나 로드밸런서 IP 등을 쉼표로 구분하여 입력
APISERVER_EXTRA_SANS="${APISERVER_EXTRA_SANS:-}"

# ─────────────────────────────────────────────────────────────────────────────
# 색상 / 로거
# ─────────────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()   { echo -e "${CYAN}[nullus]${NC} $*"; }
ok()    { echo -e "${GREEN}[nullus] ✓${NC} $*"; }
warn()  { echo -e "${YELLOW}[nullus] ⚠${NC} $*"; }
error() { echo -e "${RED}[nullus] ✗${NC} $*" >&2; }
die()   { error "$*"; exit 1; }

# ─────────────────────────────────────────────────────────────────────────────
# OS 감지
# ─────────────────────────────────────────────────────────────────────────────
detect_os() {
  if [[ -f /etc/os-release ]]; then
    # shellcheck source=/dev/null
    source /etc/os-release
    OS_ID="${ID:-unknown}"
    OS_VERSION_ID="${VERSION_ID:-0}"
  else
    die "지원되지 않는 OS입니다. /etc/os-release 파일이 없습니다."
  fi

  case "$OS_ID" in
    ubuntu|debian) PKG_MGR="apt" ;;
    rhel|centos|rocky|almalinux|ol) PKG_MGR="dnf" ;;
    *) die "지원하지 않는 배포판: $OS_ID. Ubuntu 22.04+ 또는 RHEL/Rocky 9+를 사용하세요." ;;
  esac

  log "OS 감지: $OS_ID $OS_VERSION_ID (패키지 매니저: $PKG_MGR)"
}

# ─────────────────────────────────────────────────────────────────────────────
# 명령어 존재 여부 확인
# ─────────────────────────────────────────────────────────────────────────────
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "필수 명령어가 없습니다: $1"
}

require_root() {
  [[ $EUID -eq 0 ]] || die "이 단계는 root 권한이 필요합니다. sudo를 사용하세요."
}

# ─────────────────────────────────────────────────────────────────────────────
# do_bootstrap — 마스터/워커 공통 툴체인 설치
# (모든 노드에서 실행)
# ─────────────────────────────────────────────────────────────────────────────
do_bootstrap() {
  require_root
  detect_os

  log "=== Phase 1: 시스템 기본 설정 ==="
  _disable_swap
  _setup_kernel_modules
  _setup_sysctl

  log "=== Phase 2: 컨테이너 런타임 설치 (${CONTAINER_RUNTIME}) ==="
  case "$CONTAINER_RUNTIME" in
    containerd) _install_containerd ;;
    docker)     _install_docker ;;
    *) die "지원하지 않는 런타임: $CONTAINER_RUNTIME" ;;
  esac

  log "=== Phase 3: kubeadm / kubelet / kubectl 설치 ==="
  _install_kubernetes_tools

  ok "bootstrap 완료. 다음 단계:"
  echo "  마스터 1번(m1): sudo ./scripts/runbook_csp.sh init-master"
  echo "  마스터 2~3(m2,m3): sudo ./scripts/runbook_csp.sh join-master"
  echo "  워커(w1~w3): sudo ./scripts/runbook_csp.sh join-worker"
}

_disable_swap() {
  log "스왑 비활성화..."
  swapoff -a
  # 재부팅 후에도 유지
  sed -i '/\sswap\s/s/^/#/' /etc/fstab
  ok "스왑 비활성화 완료"
}

_setup_kernel_modules() {
  log "커널 모듈 설정..."
  cat > /etc/modules-load.d/k8s.conf <<'EOF'
overlay
br_netfilter
EOF
  modprobe overlay
  modprobe br_netfilter
  ok "커널 모듈 로드 완료"
}

_setup_sysctl() {
  log "sysctl 파라미터 설정 (IPv4 포워딩, netfilter)..."
  cat > /etc/sysctl.d/99-kubernetes.conf <<'EOF'
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
# inotify 파일 감시 한계 상향 (Air hot-reload 등에 필요)
fs.inotify.max_user_watches         = 524288
fs.inotify.max_user_instances       = 8192
EOF
  sysctl --system
  ok "sysctl 설정 완료"
}

_install_containerd() {
  if command -v containerd >/dev/null 2>&1; then
    ok "containerd 이미 설치됨: $(containerd --version)"
    return 0
  fi

  log "containerd 설치 중..."
  case "$PKG_MGR" in
    apt)
      apt-get update -qq
      apt-get install -y -qq ca-certificates curl gnupg lsb-release
      install -m 0755 -d /etc/apt/keyrings
      curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
        | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
      chmod a+r /etc/apt/keyrings/docker.gpg
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
        > /etc/apt/sources.list.d/docker.list
      apt-get update -qq
      apt-get install -y -qq containerd.io
      ;;
    dnf)
      dnf config-manager --add-repo \
        https://download.docker.com/linux/centos/docker-ce.repo
      dnf install -y containerd.io
      ;;
  esac

  # containerd 기본 설정 생성 및 SystemdCgroup 활성화
  mkdir -p /etc/containerd
  containerd config default > /etc/containerd/config.toml
  sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

  systemctl enable containerd
  systemctl restart containerd
  ok "containerd 설치 완료: $(containerd --version)"
}

_install_docker() {
  if command -v docker >/dev/null 2>&1; then
    ok "docker 이미 설치됨: $(docker --version)"
    return 0
  fi

  # cri-dockerd 설치가 .deb(ubuntu-jammy) 전용이라 apt 계열만 지원
  if [[ "$PKG_MGR" != "apt" ]]; then
    die "CONTAINER_RUNTIME=docker 는 apt 계열(Ubuntu/Debian)만 지원합니다. RHEL 계열은 CONTAINER_RUNTIME=containerd 를 사용하세요."
  fi

  log "Docker 설치 중..."
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker

  # cri-dockerd 설치 (K8s 1.24+ 필요)
  log "cri-dockerd 설치 중..."
  local arch
  arch=$(dpkg --print-architecture 2>/dev/null || uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
  local cri_ver="0.3.15"
  curl -fsSL "https://github.com/Mirantis/cri-dockerd/releases/download/v${cri_ver}/cri-dockerd_${cri_ver}.3-0.ubuntu-jammy_${arch}.deb" \
    -o /tmp/cri-dockerd.deb
  dpkg -i /tmp/cri-dockerd.deb
  systemctl enable --now cri-docker.socket cri-docker
  ok "Docker + cri-dockerd 설치 완료"
}

_install_kubernetes_tools() {
  if command -v kubeadm >/dev/null 2>&1; then
    ok "Kubernetes 툴 이미 설치됨: $(kubeadm version --output short 2>/dev/null || kubeadm version)"
    return 0
  fi

  log "kubeadm / kubelet / kubectl 설치 중 (버전: ${K8S_VERSION})..."

  case "$PKG_MGR" in
    apt)
      apt-get update -qq
      apt-get install -y -qq apt-transport-https ca-certificates curl gpg
      local keyring_dir="/etc/apt/keyrings"
      mkdir -p "$keyring_dir"
      curl -fsSL "https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/Release.key" \
        | gpg --dearmor -o "${keyring_dir}/kubernetes-apt-keyring.gpg"
      echo "deb [signed-by=${keyring_dir}/kubernetes-apt-keyring.gpg] \
https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/ /" \
        > /etc/apt/sources.list.d/kubernetes.list
      apt-get update -qq
      apt-get install -y -qq kubelet kubeadm kubectl conntrack socat
      apt-mark hold kubelet kubeadm kubectl
      ;;
    dnf)
      cat > /etc/yum.repos.d/kubernetes.repo <<EOF
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/rpm/repodata/repomd.xml.key
exclude=kubelet kubeadm kubectl cri-tools kubernetes-cni
EOF
      dnf install -y --disableexcludes=kubernetes kubelet kubeadm kubectl conntrack socat
      ;;
  esac

  systemctl enable kubelet
  ok "kubeadm / kubelet / kubectl 설치 완료"
}

# ─────────────────────────────────────────────────────────────────────────────
# Helm 설치 (비 root도 가능)
# ─────────────────────────────────────────────────────────────────────────────
_install_helm() {
  if command -v helm >/dev/null 2>&1; then
    ok "helm 이미 설치됨: $(helm version --short)"
    return 0
  fi
  log "Helm 설치 중..."
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
  ok "Helm 설치 완료: $(helm version --short)"
}

# ─────────────────────────────────────────────────────────────────────────────
# do_init_master — 첫 번째 마스터 노드 초기화
# (m1에서만 실행)
# ─────────────────────────────────────────────────────────────────────────────
do_init_master() {
  require_root

  [[ -n "$CONTROL_PLANE_ENDPOINT" ]] || \
    die "CONTROL_PLANE_ENDPOINT가 설정되지 않았습니다.\n  단일 마스터: 이 VM의 IP 주소\n  HA(VIP): 로드밸런서 또는 keepalived VIP"

  log "=== 마스터 노드 초기화 (kubeadm init) ==="
  log "  control-plane-endpoint: ${CONTROL_PLANE_ENDPOINT}"
  log "  pod-network-cidr: ${POD_CIDR}"
  log "  service-cidr: ${SERVICE_CIDR}"

  local extra_args=""
  if [[ "$CONTAINER_RUNTIME" == "docker" ]]; then
    extra_args="--cri-socket unix:///var/run/cri-dockerd.sock"
  fi

  if [[ -n "$APISERVER_EXTRA_SANS" ]]; then
    extra_args="${extra_args} --apiserver-cert-extra-sans=${APISERVER_EXTRA_SANS}"
  fi

  kubeadm init \
    --control-plane-endpoint "${CONTROL_PLANE_ENDPOINT}:6443" \
    --pod-network-cidr "${POD_CIDR}" \
    --service-cidr "${SERVICE_CIDR}" \
    --upload-certs \
    ${extra_args}

  # 현재 사용자(sudo 실행자)의 kubeconfig 설정
  _setup_kubeconfig

  log "=== CNI 플러그인 설치: ${CNI_PLUGIN} ==="
  _install_cni

  log "=== join 커맨드 저장 ==="
  _save_join_tokens

  ok "마스터 초기화 완료!"
  echo ""
  echo -e "${BOLD}  다음 단계:${NC}"
  echo "  1. join 토큰 파일을 다른 노드로 복사:"
  echo "     scp ${JOIN_TOKEN_FILE} <m2_or_w1_ip>:/tmp/"
  echo "  2. 마스터 추가: sudo ./scripts/runbook_csp.sh join-master"
  echo "  3. 워커 추가:   sudo ./scripts/runbook_csp.sh join-worker"
  echo "  4. 배포:        ./scripts/runbook_csp.sh deploy"
  echo ""
}

_setup_kubeconfig() {
  local target_user="${SUDO_USER:-root}"
  local target_home
  if [[ "$target_user" == "root" ]]; then
    target_home="/root"
  else
    target_home=$(eval echo "~${target_user}")
  fi

  mkdir -p "${target_home}/.kube"
  cp /etc/kubernetes/admin.conf "${target_home}/.kube/config"
  chown "${target_user}:${target_user}" "${target_home}/.kube/config"
  ok "kubeconfig 설정 완료: ${target_home}/.kube/config"
}

_install_cni() {
  # kubeconfig가 설치된 사용자 컨텍스트에서 실행
  local kubectl_cmd="kubectl"
  if [[ -n "${SUDO_USER:-}" ]] && [[ "${SUDO_USER}" != "root" ]]; then
    kubectl_cmd="sudo -u ${SUDO_USER} kubectl"
  fi

  case "$CNI_PLUGIN" in
    calico)
      log "Calico CNI 설치 중..."
      $kubectl_cmd apply -f \
        https://raw.githubusercontent.com/projectcalico/calico/v3.29.1/manifests/calico.yaml
      ok "Calico CNI 설치 완료"
      ;;
    cilium)
      log "Cilium CNI 설치 중..."
      _install_helm  # helm 필요
      helm repo add cilium https://helm.cilium.io/ 2>/dev/null || true
      helm repo update
      helm install cilium cilium/cilium \
        --namespace kube-system \
        --set kubeProxyReplacement=strict \
        --set k8sServiceHost="${CONTROL_PLANE_ENDPOINT}" \
        --set k8sServicePort=6443
      ok "Cilium CNI 설치 완료"
      ;;
    flannel)
      log "Flannel CNI 설치 중..."
      $kubectl_cmd apply -f \
        https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
      ok "Flannel CNI 설치 완료"
      ;;
    *)
      warn "알 수 없는 CNI: ${CNI_PLUGIN}. 수동으로 설치하세요."
      ;;
  esac
}

_save_join_tokens() {
  local master_join_cmd worker_join_cmd cert_key

  # certificate-key upload 및 key 획득
  cert_key=$(kubeadm init phase upload-certs --upload-certs 2>/dev/null | tail -n 1)
  if [[ -z "$cert_key" ]]; then
    cert_key=$(kubeadm certs certificate-key)
  fi

  # 마스터 join 커맨드
  master_join_cmd=$(kubeadm token create --print-join-command --certificate-key "${cert_key}" 2>/dev/null)

  # 워커 join 커맨드
  worker_join_cmd=$(kubeadm token create --print-join-command 2>/dev/null)

  rm -f "${JOIN_TOKEN_FILE}"
  cat > "${JOIN_TOKEN_FILE}" <<EOF
# Nullus Kubernetes Join Tokens
# 생성 시간: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# 이 파일은 24시간 이내에 사용하세요 (토큰 만료)

MASTER_JOIN_CMD="${master_join_cmd}"
WORKER_JOIN_CMD="${worker_join_cmd}"
EOF

  chmod 600 "${JOIN_TOKEN_FILE}"
  ok "join 토큰 저장 완료: ${JOIN_TOKEN_FILE}"
}

# ─────────────────────────────────────────────────────────────────────────────
# do_join_master — 추가 마스터 노드 조인
# (m2, m3에서 실행)
# ─────────────────────────────────────────────────────────────────────────────
do_join_master() {
  require_root

  [[ -f "${JOIN_TOKEN_FILE}" ]] || \
    die "join 토큰 파일이 없습니다: ${JOIN_TOKEN_FILE}\n  m1에서 생성된 파일을 이 노드로 복사하세요:\n  scp m1_ip:${JOIN_TOKEN_FILE} ${JOIN_TOKEN_FILE}"

  # shellcheck source=/dev/null
  source "${JOIN_TOKEN_FILE}"

  [[ -n "${MASTER_JOIN_CMD:-}" ]] || die "MASTER_JOIN_CMD가 join 토큰 파일에 없습니다."

  log "마스터 노드 조인 중..."

  local extra_args=""
  if [[ "$CONTAINER_RUNTIME" == "docker" ]]; then
    extra_args="--cri-socket unix:///var/run/cri-dockerd.sock"
  fi

  eval "${MASTER_JOIN_CMD} --control-plane ${extra_args}"

  _setup_kubeconfig

  ok "마스터 노드 조인 완료"
}

# ─────────────────────────────────────────────────────────────────────────────
# do_join_worker — 워커 노드 조인
# (w1, w2, w3에서 실행)
# ─────────────────────────────────────────────────────────────────────────────
do_join_worker() {
  require_root

  [[ -f "${JOIN_TOKEN_FILE}" ]] || \
    die "join 토큰 파일이 없습니다: ${JOIN_TOKEN_FILE}\n  m1에서 생성된 파일을 이 노드로 복사하세요:\n  scp m1_ip:${JOIN_TOKEN_FILE} ${JOIN_TOKEN_FILE}"

  # shellcheck source=/dev/null
  source "${JOIN_TOKEN_FILE}"

  [[ -n "${WORKER_JOIN_CMD:-}" ]] || die "WORKER_JOIN_CMD가 join 토큰 파일에 없습니다."

  log "워커 노드 조인 중..."

  local extra_args=""
  if [[ "$CONTAINER_RUNTIME" == "docker" ]]; then
    extra_args="--cri-socket unix:///var/run/cri-dockerd.sock"
  fi

  eval "${WORKER_JOIN_CMD} ${extra_args}"

  ok "워커 노드 조인 완료"
}

# ─────────────────────────────────────────────────────────────────────────────
# do_deploy — Nullus 인프라 + 앱 배포
# (m1에서, bootstrap + init-master 완료 후 실행)
# ─────────────────────────────────────────────────────────────────────────────
do_deploy() {
  _install_helm
  require_cmd kubectl
  require_cmd helm
  _check_cluster_ready

  # 기본 시크릿 값으로 배포 차단 + ENCRYPTION_KEY 32바이트 검증
  if [[ "$DB_PASSWORD" == "change-me-in-production" ]]; then
    die "DB_PASSWORD 가 기본값입니다. 배포 전 'export DB_PASSWORD=<강력한_값>' 으로 변경하세요."
  fi
  if [[ "$ENCRYPTION_KEY" == "change-me-32bytes-production!!" ]]; then
    die "ENCRYPTION_KEY 가 기본값입니다. 배포 전 'export ENCRYPTION_KEY=<32바이트_키>' 로 변경하세요."
  fi
  if [[ ${#ENCRYPTION_KEY} -ne 32 ]]; then
    die "ENCRYPTION_KEY 길이가 ${#ENCRYPTION_KEY}바이트입니다 — 정확히 32바이트여야 합니다."
  fi

  log "=== Nullus 배포 시작 ==="
  log "  Namespace: ${NULLUS_NAMESPACE}"
  log "  Image Tag: ${IMAGE_TAG}"
  log "  Chart:     ${CHART_PATH}"

  _deploy_metallb
  _deploy_ingress_nginx
  _deploy_nullus_app

  ok "=== Nullus 배포 완료 ==="
  do_status
}

_check_cluster_ready() {
  log "클러스터 상태 확인 중..."
  local not_ready
  not_ready=$(kubectl get nodes --no-headers 2>/dev/null | grep -v " Ready" | wc -l || echo "999")
  if [[ "$not_ready" -gt 0 ]]; then
    warn "NotReady 노드가 ${not_ready}개 있습니다. 계속 진행합니다..."
    kubectl get nodes
  else
    ok "모든 노드가 Ready 상태입니다."
    kubectl get nodes
  fi
}

_deploy_metallb() {
  if [[ -z "$METALLB_IP_RANGE" ]]; then
    warn "METALLB_IP_RANGE가 설정되지 않았습니다. MetalLB 설치를 건너뜁니다."
    warn "  CSP 환경의 LoadBalancer IP 범위를 설정해주세요."
    warn "  예: export METALLB_IP_RANGE='10.0.1.200-10.0.1.210'"
    return 0
  fi

  if kubectl get namespace metallb-system >/dev/null 2>&1; then
    ok "MetalLB 이미 설치되어 있습니다."
    return 0
  fi

  log "MetalLB 설치 중..."
  kubectl apply -f \
    https://raw.githubusercontent.com/metallb/metallb/v0.14.9/config/manifests/metallb-native.yaml

  log "MetalLB Controller 준비 대기 (최대 120s)..."
  kubectl wait --namespace metallb-system \
    --for=condition=ready pod \
    --selector=component=controller \
    --timeout=120s

  log "MetalLB IP 풀 설정: ${METALLB_IP_RANGE}"
  cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: nullus-pool
  namespace: metallb-system
spec:
  addresses:
    - ${METALLB_IP_RANGE}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: nullus-l2
  namespace: metallb-system
spec:
  ipAddressPools:
    - nullus-pool
EOF

  ok "MetalLB 설치 및 설정 완료"
}

_deploy_ingress_nginx() {
  if kubectl get namespace ingress-nginx >/dev/null 2>&1; then
    ok "Ingress-Nginx 이미 설치되어 있습니다."
    return 0
  fi

  log "Ingress-Nginx 설치 중..."
  helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx 2>/dev/null || true
  helm repo update

  helm install ingress-nginx ingress-nginx/ingress-nginx \
    --namespace ingress-nginx \
    --create-namespace \
    --set controller.replicaCount=2 \
    --set controller.nodeSelector."kubernetes\.io/os"=linux \
    --set controller.admissionWebhooks.patch.nodeSelector."kubernetes\.io/os"=linux \
    --set defaultBackend.nodeSelector."kubernetes\.io/os"=linux \
    --wait \
    --timeout 120s

  ok "Ingress-Nginx 설치 완료"
}

_deploy_nullus_app() {
  log "Nullus 네임스페이스 생성..."
  kubectl create namespace "${NULLUS_NAMESPACE}" 2>/dev/null || true

  log "컨테이너 레지스트리 Secret 생성 (GHCR 접근용)..."
  # GitHub PAT가 있는 경우 ghcr.io pull secret 생성
  if [[ -n "${GHCR_PAT:-}" ]] && [[ -n "${GHCR_USER:-}" ]]; then
    kubectl create secret docker-registry ghcr-pull-secret \
      --namespace="${NULLUS_NAMESPACE}" \
      --docker-server=ghcr.io \
      --docker-username="${GHCR_USER}" \
      --docker-password="${GHCR_PAT}" \
      --docker-email="${GHCR_USER}@users.noreply.github.com" \
      --dry-run=client -o yaml | kubectl apply -f -
    ok "GHCR pull secret 생성 완료"
  else
    warn "GHCR_PAT / GHCR_USER 가 설정되지 않았습니다."
    warn "  ghcr.io/cloud-nullus 이미지가 public이라면 pull secret 없이 진행됩니다."
    warn "  private 레지스트리라면: export GHCR_USER=<user> GHCR_PAT=<token>"
  fi

  log "Helm 차트 의존성 갱신..."
  helm dependency update "${CHART_PATH}" 2>/dev/null || true

  log "Nullus Helm 차트 배포 중..."
  helm upgrade --install nullus "${CHART_PATH}" \
    --namespace "${NULLUS_NAMESPACE}" \
    --create-namespace \
    --set api.image.repository="${REGISTRY}/nullus-api" \
    --set api.image.tag="${IMAGE_TAG}" \
    --set web.image.repository="${REGISTRY}/nullus-web" \
    --set web.image.tag="${IMAGE_TAG}" \
    --set secrets.dbPassword="${DB_PASSWORD}" \
    --set secrets.encryptionKey="${ENCRYPTION_KEY}" \
    --set config.server.mode=production \
    --set ingress.enabled=true \
    --set ingress.className="${INGRESS_CLASS}" \
    --set "ingress.hosts[0].host=${INGRESS_HOST}" \
    --set "ingress.hosts[0].paths[0].path=/" \
    --set "ingress.hosts[0].paths[0].pathType=Prefix" \
    --wait \
    --timeout 300s

  ok "Nullus Helm 차트 배포 완료"

  log "DB 마이그레이션 Job 실행..."
  _run_migration_job
}

_run_migration_job() {
  # 마이그레이션을 Kubernetes Job으로 실행
  local db_service
  db_service=$(kubectl get svc -n "${NULLUS_NAMESPACE}" -l "app.kubernetes.io/name=postgresql" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "nullus-postgresql")

  cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: nullus-migrate-$(date +%s)
  namespace: ${NULLUS_NAMESPACE}
  labels:
    app.kubernetes.io/name: nullus-migrate
    app.kubernetes.io/managed-by: runbook-csp
spec:
  ttlSecondsAfterFinished: 600
  template:
    spec:
      restartPolicy: OnFailure
      containers:
        - name: migrate
          image: ${REGISTRY}/nullus-api:${IMAGE_TAG}
          command: ["/bin/sh", "-c"]
          args:
            - |
              migrate -path /etc/nullus/migrations \
                -database "postgres://\${NULLUS_DB_USER}:\${NULLUS_DB_PASSWORD}@\${NULLUS_DB_HOST}:\${NULLUS_DB_PORT}/\${NULLUS_DB_NAME}?sslmode=disable" \
                up
          env:
            - name: NULLUS_DB_HOST
              value: "${db_service}"
            - name: NULLUS_DB_PORT
              value: "5432"
            - name: NULLUS_DB_NAME
              value: "nullus"
            - name: NULLUS_DB_USER
              value: "nullus"
            - name: NULLUS_DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: nullus-secrets
                  key: db-password
                  optional: true
EOF

  log "마이그레이션 Job 제출 완료. 결과 확인:"
  echo "  kubectl logs -n ${NULLUS_NAMESPACE} -l app.kubernetes.io/name=nullus-migrate --tail=50"
}

# ─────────────────────────────────────────────────────────────────────────────
# do_status — 클러스터 및 Nullus 상태 확인
# ─────────────────────────────────────────────────────────────────────────────
do_status() {
  require_cmd kubectl

  echo ""
  echo -e "${BOLD}════════════════════════════════════════════════════════════════════${NC}"
  echo -e "${BOLD}  Nullus CSP 클러스터 상태${NC}"
  echo -e "${BOLD}════════════════════════════════════════════════════════════════════${NC}"

  echo ""
  echo -e "${CYAN}── Nodes ──${NC}"
  kubectl get nodes -o wide 2>/dev/null || echo "  (클러스터에 접근할 수 없습니다)"

  echo ""
  echo -e "${CYAN}── Namespaces ──${NC}"
  kubectl get namespaces 2>/dev/null || true

  echo ""
  echo -e "${CYAN}── Nullus Pods (${NULLUS_NAMESPACE}) ──${NC}"
  kubectl get pods -n "${NULLUS_NAMESPACE}" -o wide 2>/dev/null \
    || echo "  (${NULLUS_NAMESPACE} 네임스페이스가 없거나 접근 불가)"

  echo ""
  echo -e "${CYAN}── Nullus Services ──${NC}"
  kubectl get svc -n "${NULLUS_NAMESPACE}" 2>/dev/null || true

  echo ""
  echo -e "${CYAN}── Ingress ──${NC}"
  kubectl get ingress -n "${NULLUS_NAMESPACE}" 2>/dev/null || true

  # Pod 비정상 감지
  local not_running
  not_running=$(kubectl get pods -n "${NULLUS_NAMESPACE}" --no-headers 2>/dev/null \
    | grep -Ev " Running | Completed " | wc -l || echo "0")
  if [[ "$not_running" -gt 0 ]]; then
    echo ""
    warn "${not_running}개의 Pod가 비정상 상태입니다:"
    kubectl get pods -n "${NULLUS_NAMESPACE}" --no-headers 2>/dev/null \
      | grep -Ev " Running | Completed " || true
  fi

  echo ""
  echo -e "${CYAN}── Helm Releases ──${NC}"
  helm list -n "${NULLUS_NAMESPACE}" 2>/dev/null || echo "  (helm을 찾을 수 없거나 배포 없음)"

  echo ""
  echo -e "${BOLD}════════════════════════════════════════════════════════════════════${NC}"

  # 접속 URL 출력
  local ingress_ip
  ingress_ip=$(kubectl get svc -n ingress-nginx \
    -l "app.kubernetes.io/component=controller" \
    -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

  if [[ -n "$ingress_ip" ]]; then
    echo ""
    echo -e "${GREEN}  ✓ 접속 정보${NC}"
    echo "    Frontend:  http://${INGRESS_HOST}  (또는 http://${ingress_ip})"
    echo "    API:       http://${INGRESS_HOST}/api/v1"
    echo "    Health:    http://${INGRESS_HOST}/health"
    if [[ -z "${DNS_CONFIGURED:-}" ]]; then
      echo ""
      warn "DNS 미설정 시 /etc/hosts에 추가하세요:"
      echo "    ${ingress_ip}  ${INGRESS_HOST}"
    fi
  fi
  echo ""
}

# ─────────────────────────────────────────────────────────────────────────────
# do_upgrade — Nullus 이미지/차트 업그레이드
# ─────────────────────────────────────────────────────────────────────────────
do_upgrade() {
  require_cmd kubectl
  require_cmd helm

  [[ -n "${IMAGE_TAG}" ]] || die "IMAGE_TAG 환경 변수를 설정하세요. 예: export IMAGE_TAG=v1.2.0"

  log "Nullus 업그레이드 시작 (태그: ${IMAGE_TAG})..."

  helm upgrade nullus "${CHART_PATH}" \
    --namespace "${NULLUS_NAMESPACE}" \
    --reuse-values \
    --set api.image.tag="${IMAGE_TAG}" \
    --set web.image.tag="${IMAGE_TAG}" \
    --wait \
    --timeout 300s

  ok "Nullus 업그레이드 완료 (태그: ${IMAGE_TAG})"

  log "Rollout 상태 확인..."
  kubectl rollout status deployment/nullus-api -n "${NULLUS_NAMESPACE}" --timeout=120s || true
  kubectl rollout status deployment/nullus-web -n "${NULLUS_NAMESPACE}" --timeout=120s || true

  do_status
}

# ─────────────────────────────────────────────────────────────────────────────
# do_uninstall — Nullus 앱 제거 (클러스터는 유지)
# ─────────────────────────────────────────────────────────────────────────────
do_uninstall() {
  require_cmd helm

  warn "Nullus 앱을 제거합니다. 클러스터는 유지됩니다."
  warn "데이터(PostgreSQL PVC)는 삭제되지 않습니다."
  read -r -p "계속하시겠습니까? (yes 입력): " confirm
  [[ "$confirm" == "yes" ]] || { log "취소되었습니다."; exit 0; }

  helm uninstall nullus -n "${NULLUS_NAMESPACE}" 2>/dev/null || true
  ok "Nullus Helm release 제거 완료"

  # PVC 유지 여부 확인
  local pvcs
  pvcs=$(kubectl get pvc -n "${NULLUS_NAMESPACE}" --no-headers 2>/dev/null | awk '{print $1}' || true)
  if [[ -n "$pvcs" ]]; then
    log "남아있는 PVC (데이터 보존):"
    echo "$pvcs"
    warn "데이터까지 완전히 삭제하려면:"
    echo "  kubectl delete pvc --all -n ${NULLUS_NAMESPACE}"
    echo "  kubectl delete namespace ${NULLUS_NAMESPACE}"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# usage
# ─────────────────────────────────────────────────────────────────────────────
usage() {
  cat <<'EOF'

Nullus CSP VM 클러스터 배포 스크립트

사용법:
  sudo ./scripts/runbook_csp.sh bootstrap     # 모든 노드 — 툴체인 설치 (root 필요)
  sudo ./scripts/runbook_csp.sh init-master   # m1 전용 — 클러스터 초기화 (root 필요)
  sudo ./scripts/runbook_csp.sh join-master   # m2, m3 — 마스터 노드 조인 (root 필요)
  sudo ./scripts/runbook_csp.sh join-worker   # w1~w3 — 워커 노드 조인 (root 필요)
       ./scripts/runbook_csp.sh deploy        # m1 전용 — Nullus 배포
       ./scripts/runbook_csp.sh status        # 클러스터 및 서비스 상태
       ./scripts/runbook_csp.sh upgrade       # Nullus 업그레이드 (IMAGE_TAG 필요)
       ./scripts/runbook_csp.sh uninstall     # Nullus 앱 제거

필수 환경 변수 (init-master / deploy):
  CONTROL_PLANE_ENDPOINT   마스터 VIP 또는 m1 IP
  DB_PASSWORD              DB 비밀번호 (기본값 변경 필수)
  ENCRYPTION_KEY           32바이트 암호화 키 (기본값 변경 필수)
  INGRESS_HOST             서비스 호스트명 (기본: nullus.example.com)

선택 환경 변수:
  K8S_VERSION              K8s 버전 (기본: 1.31)
  CONTAINER_RUNTIME        containerd | docker (기본: containerd)
  CNI_PLUGIN               calico | cilium | flannel (기본: calico)
  METALLB_IP_RANGE         MetalLB IP 범위 (예: 10.0.1.200-10.0.1.210)
  IMAGE_TAG                컨테이너 이미지 태그 (기본: latest)
  GHCR_USER / GHCR_PAT     GHCR private 레지스트리 접근용 자격증명
  JOIN_TOKEN_FILE          join 토큰 파일 경로 (기본: /tmp/nullus-join.env)

배포 순서 (표준 m3/w3 HA 클러스터):
  1. 모든 노드(m1~m3, w1~w3)에서:
       sudo ./scripts/runbook_csp.sh bootstrap

  2. m1에서:
       export CONTROL_PLANE_ENDPOINT=<VIP_or_m1_IP>
       sudo ./scripts/runbook_csp.sh init-master

  3. join 토큰 파일 배포:
       scp /tmp/nullus-join.env m2:/tmp/
       scp /tmp/nullus-join.env m3:/tmp/
       scp /tmp/nullus-join.env w1:/tmp/
       scp /tmp/nullus-join.env w2:/tmp/
       scp /tmp/nullus-join.env w3:/tmp/

  4. m2, m3에서:
       sudo ./scripts/runbook_csp.sh join-master

  5. w1, w2, w3에서:
       sudo ./scripts/runbook_csp.sh join-worker

  6. m1에서:
       export CONTROL_PLANE_ENDPOINT=<VIP_or_m1_IP>
       export METALLB_IP_RANGE='<lb_ip_start>-<lb_ip_end>'
       export INGRESS_HOST='nullus.example.com'
       export DB_PASSWORD='<strong_password>'
       export ENCRYPTION_KEY='<exactly_32_bytes_key>'
       export IMAGE_TAG='latest'  # 또는 특정 버전
       ./scripts/runbook_csp.sh deploy

EOF
}

# ─────────────────────────────────────────────────────────────────────────────
# main
# ─────────────────────────────────────────────────────────────────────────────
main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    bootstrap)   do_bootstrap ;;
    init-master) do_init_master ;;
    save-tokens) _save_join_tokens ;;
    join-master) do_join_master ;;
    join-worker) do_join_worker ;;
    deploy)      do_deploy ;;
    status)      do_status ;;
    upgrade)     do_upgrade ;;
    uninstall)   do_uninstall ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
