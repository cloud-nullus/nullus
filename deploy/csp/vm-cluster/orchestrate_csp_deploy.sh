#!/usr/bin/env bash
set -euo pipefail

log() { echo -e "\n\033[1;36m=== $1 ===\033[0m"; }

M1="172.16.0.163"
M2="172.16.0.28"
M3="172.16.0.242"
W1="172.16.0.246"
W2="172.16.0.30"
W3="172.16.0.6"

ALL_NODES=($M1 $M2 $M3 $W1 $W2 $W3)
OTHER_NODES=($M2 $M3 $W1 $W2 $W3)
OTHER_MASTERS=($M2 $M3)
WORKERS=($W1 $W2 $W3)

log "1. 마스터1 노드(자기자신)에 스크립트 위치 설정"
sudo mkdir -p /scripts
sudo cp ~/runbook_csp.sh /scripts/runbook_csp.sh
sudo chmod +x /scripts/runbook_csp.sh

log "2. 다른 5대 노드에 스크립트 전송"
for NODE in "${OTHER_NODES[@]}"; do
    echo " -> $NODE 전송 중..."
    scp -o StrictHostKeyChecking=no ~/runbook_csp.sh ubuntu@$NODE:/tmp/runbook_csp.sh
    ssh -o StrictHostKeyChecking=no ubuntu@$NODE "sudo mkdir -p /scripts && sudo mv /tmp/runbook_csp.sh /scripts/runbook_csp.sh && sudo chmod +x /scripts/runbook_csp.sh"
done

log "3. 모든 노드에서 bootstrap(툴체인 설치) 병렬 실행 (시간이 걸릴 수 있습니다)"
pids=()
for NODE in "${ALL_NODES[@]}"; do
    if [ "$NODE" == "$M1" ]; then
        sudo /scripts/runbook_csp.sh bootstrap > /tmp/bootstrap_$NODE.log 2>&1 &
        pids+=($!)
    else
        ssh -o StrictHostKeyChecking=no ubuntu@$NODE "sudo /scripts/runbook_csp.sh bootstrap" > /tmp/bootstrap_$NODE.log 2>&1 &
        pids+=($!)
    fi
done

echo " -> 전체 노드 설치를 기다리는 중입니다..."
for pid in "${pids[@]}"; do
    wait $pid || { echo "에러 발생 시 로그 확인 요망: /tmp/bootstrap_*.log"; exit 1; }
done
echo " -> 전체 6대 bootstrap 완료!"

log "4. 첫 번째 마스터($M1) 클러스터 초기화 및 토큰 재생성"
if [ ! -f /etc/kubernetes/admin.conf ]; then
    export CONTROL_PLANE_ENDPOINT="$M1"
    export APISERVER_EXTRA_SANS="61.109.239.220"
    sudo -E /scripts/runbook_csp.sh init-master
else
    log "  -> /etc/kubernetes/admin.conf 가 이미 존재함. 토큰만 재생성합니다."
    sudo /scripts/runbook_csp.sh save-tokens
fi

log "5. 조인 토큰(join token) 파일 다른 노드에 배포"
JOIN_FILE="/tmp/nullus-join.env"
sudo chown ubuntu:ubuntu $JOIN_FILE
for NODE in "${OTHER_NODES[@]}"; do
    echo " -> $NODE 에 조인 환경파일 배포 중..."
    scp -o StrictHostKeyChecking=no $JOIN_FILE ubuntu@$NODE:/tmp/nullus-join.env
done

log "6. 마스터 2, 3 조인 (m2, m3)"
for NODE in "${OTHER_MASTERS[@]}"; do
    if kubectl get nodes -o wide | grep -q "$NODE"; then
        log " -> $NODE 마스터는 이미 클러스터에 존재함. 건너뜀."
    else
        echo " -> $NODE 마스터 합류 중..."
        ssh -o StrictHostKeyChecking=no ubuntu@$NODE "sudo /scripts/runbook_csp.sh join-master"
    fi
done

log "7. 워커 1, 2, 3 조인 (w1, w2, w3)"
for NODE in "${WORKERS[@]}"; do
    if kubectl get nodes -o wide | grep -q "$NODE"; then
        log " -> $NODE 워커는 이미 클러스터에 존재함. 건너뜀."
    else
        echo " -> $NODE 워커 합류 중..."
        ssh -o StrictHostKeyChecking=no ubuntu@$NODE "sudo /scripts/runbook_csp.sh join-worker"
    fi
done

log "8. 클러스터 상태 확인 (모든 노드가 추가될 때까지 10초 대기)"
sleep 10
kubectl get nodes

log "9. Nullus 전체 배포 수행"
export METALLB_IP_RANGE="172.16.2.200-172.16.2.220" # CSP망의 가용 IP 대역 사용
export INGRESS_HOST="nullus.local"
export APISERVER_EXTRA_SANS="61.109.239.220"
export DB_PASSWORD="change-me-in-production"
export ENCRYPTION_KEY="nullus-dev-key-32bytes-padding!!"
export IMAGE_TAG="0.1.0-alpha"
export NULLUS_NAMESPACE="nullus"

# kubeconfig가 ubuntu 권한으로 생성되었으므로 sudo 없이 실행합니다.
/scripts/runbook_csp.sh deploy

log "모든 배포 과정이 완료되었습니다!"
