# 문제 해결 가이드
> 에어갭 kind 배포 중 발생하는 주요 증상별 원인, 해결 방법, 로그 수집 명령

---

## ImagePullBackOff

**증상**

```
kubectl get pods -n nullus
# STATUS: ImagePullBackOff 또는 ErrImagePull
```

**원인 A — containerd mirror 미적용**

`kind-airgap.yaml`의 `containerdConfigPatches`가 반영되지 않아 클러스터 노드가 외부 레지스트리(`ghcr.io`, `docker.io`)에 직접 접근을 시도하다 실패한다.

해결:

```bash
# 1. 클러스터 노드의 containerd 설정 확인
CONTROL_NODE=$(kubectl get nodes --context kind-nullus-airgap \
  -l node-role.kubernetes.io/control-plane \
  --no-headers -o custom-columns=':metadata.name')

docker exec "${CONTROL_NODE}" cat /etc/containerd/config.toml | grep -A3 'mirrors'
# [plugins."io.containerd.grpc.v1.cri".registry.mirrors."ghcr.io"] 항목이 있어야 함

# 2. 미러 항목이 없으면 클러스터를 재생성한다
kind delete cluster --name nullus-airgap
bash airgap/scripts/11-create-cluster.sh
```

**원인 B — localhost:5001 접근 불가**

레지스트리 컨테이너가 kind 도커 네트워크에 연결되지 않아 클러스터 노드 내부에서 레지스트리에 도달하지 못한다.

해결:

```bash
# 1. 레지스트리 실행 상태 확인
docker inspect kind-registry --format '{{.State.Running}}'
# false 이면 재기동
docker start kind-registry

# 2. kind 네트워크 연결 여부 확인
docker inspect kind-registry \
  --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}'
# 'kind' 가 없으면 연결
docker network connect kind kind-registry

# 3. 호스트에서 레지스트리 응답 확인
curl -sf http://localhost:5001/v2/
# 출력: {}
```

**로그 수집**

```bash
docker logs kind-registry
kubectl describe pod <pod-name> -n nullus
kubectl get events -n nullus --sort-by='.lastTimestamp'
docker exec "${CONTROL_NODE}" crictl images
```

---

## helm install timeout

**증상**

```
Error: INSTALLATION FAILED: timed out waiting for the condition
# 또는
helm install 이 수 분 후 오류로 종료됨
```

**원인 A — postgresql initContainer 실패**

`bitnami/os-shell` initContainer가 이미지를 Pull하지 못하거나 DB 초기화에 실패한다.

해결:

```bash
# 1. postgresql 파드 상태 확인
kubectl describe pod -n nullus -l app.kubernetes.io/name=postgresql

# 2. initContainer 로그 확인
kubectl logs -n nullus -l app.kubernetes.io/name=postgresql \
  -c init-chmod-data 2>/dev/null || \
kubectl logs -n nullus -l app.kubernetes.io/name=postgresql \
  --previous 2>/dev/null

# 3. os-shell 이미지가 레지스트리에 있는지 확인
curl -s http://localhost:5001/v2/bitnami/os-shell/tags/list
# {"name":"bitnami/os-shell","tags":["12-debian-12-r49",...]}
```

이미지가 없으면 `12-push-to-registry.sh`를 재실행한다.

**원인 B — PVC Binding 대기**

`nullus` 네임스페이스의 PersistentVolumeClaim이 `Pending` 상태로 남아 파드 기동이 차단된다.

해결:

```bash
# 1. PVC 상태 확인
kubectl get pvc -n nullus

# 2. StorageClass 확인
kubectl get storageclass
# standard (default) 또는 standard-rwo 가 있어야 함

# 3. kind 기본 StorageClass 재확인
kubectl get storageclass standard -o yaml
```

kind는 `rancher.io/local-path` provisioner를 기본으로 제공한다. StorageClass가 없거나 `(default)` 표시가 없으면 kind 클러스터를 재생성한다.

**로그 수집**

```bash
kubectl describe pod <postgresql-pod> -n nullus
kubectl describe pvc -n nullus
kubectl get events -n nullus --sort-by='.lastTimestamp' | tail -20
```

---

## kind create cluster 실패

**증상**

```
ERROR: failed to create cluster: ...
# 또는 kind create cluster 가 오류 없이 종료되었으나 클러스터가 없음
```

**원인 A — 포트 충돌 (5001, 80, 443)**

```bash
# 충돌 포트 확인
lsof -i :5001 -i :80 -i :443    # macOS
ss -tlnp | grep -E ':(5001|80|443)'  # Linux

# 충돌 프로세스 종료 후 재시도
```

80/443 충돌이 레지스트리 컨테이너가 아닌 다른 프로세스 때문이라면 `kind-airgap.yaml`의 `extraPortMappings`를 다른 포트로 변경할 수 있다.

**원인 B — docker 데몬 미기동**

```bash
# 데몬 상태 확인
docker info

# macOS: Docker Desktop 실행
# Linux:
sudo systemctl start docker
sudo systemctl status docker
```

**원인 C — cgroup v2 / cgroup driver 불일치 (Linux)**

```bash
# cgroup 버전 확인
stat -fc %T /sys/fs/cgroup/
# tmpfs → cgroup v1
# cgroup2fs → cgroup v2

# docker cgroup driver 확인
docker info | grep 'Cgroup Driver'
```

cgroup v2 환경에서 docker cgroup driver가 `cgroupfs`이면 kind가 실패할 수 있다. `/etc/docker/daemon.json`에서 `"exec-opts": ["native.cgroupdriver=systemd"]`로 변경 후 docker를 재시작한다.

**로그 수집**

```bash
docker info
kind create cluster --name nullus-airgap --config airgap/kind/kind-airgap.yaml --verbosity 5
journalctl -u docker --since "5 minutes ago"  # Linux
```

---

## docker load 실패

**증상**

```bash
docker load -i airgap/bundle/images.tar.gz
# Error response from daemon: invalid tar header
# 또는 종료 코드 1
```

**원인 A — SHA-256 불일치 (파일 손상)**

```bash
# 무결성 검증
cd airgap/bundle
sha256sum -c images.tar.gz.sha256    # Linux
shasum -a 256 -c images.tar.gz.sha256  # macOS
```

`FAILED` 출력 시 파일이 전송 중 손상된 것이다. 번들을 재반입한다.

**원인 B — 디스크 공간 부족**

```bash
df -h /var/lib/docker  # Linux docker 기본 경로
df -h ~/Library/Containers/com.docker.docker  # macOS Docker Desktop

# 불필요한 이미지 정리
docker image prune -f
docker system df
```

**로그 수집**

```bash
docker system df
df -h /
gzip -t airgap/bundle/images.tar.gz && echo "gzip integrity OK"
```

---

## helm dep update 차단 (방화벽)

**증상**

```
Error: failed to download "https://charts.bitnami.com/bitnami/postgresql-16.7.21.tgz"
# 또는 helm dep update 가 네트워크 오류로 실패
```

**원인**: 오프라인 환경에서 `helm dep update`를 실행하면 외부 레포지토리에 접근을 시도하다 실패한다.

**해결 A — 번들에 포함된 차트 사용 (권장)**

Agent 3이 제공하는 `airgap/helm/nullus-*.tgz`는 postgresql 의존성이 이미 패키징되어 있다. 별도로 `helm dep update`를 실행하지 않는다.

```bash
# 번들 차트로 직접 설치
helm install nullus airgap/helm/nullus-*.tgz \
  -n nullus \
  --create-namespace \
  -f airgap/helm/values-airgap.yaml \
  --context kind-nullus-airgap
```

**해결 B — 수동 `helm pull` (온라인 머신에서 준비)**

```bash
# 온라인 머신에서 의존 차트를 수동으로 Pull
helm repo add bitnami https://charts.bitnami.com/bitnami
helm pull bitnami/postgresql --version 16.7.21 --destination deploy/helm/nullus/charts/

# charts/ 디렉토리를 번들에 포함시켜 반입
```

**로그 수집**

```bash
helm dependency list deploy/helm/nullus/
ls deploy/helm/nullus/charts/
```

---

## 레지스트리 Push 인증 오류

**증상**

```
docker push localhost:5001/cloud-nullus/nullus-api:0.1.0-alpha
# unauthorized: authentication required
# 또는 no basic auth credentials
```

**원인**: `registry:2`는 기본적으로 인증 없이 동작하나, 호스트 docker daemon이 해당 레지스트리를 `insecure-registries`에 등록하지 않아 HTTPS를 강제하면 발생할 수 있다.

**해결**

```bash
# 1. docker daemon 설정 확인
cat /etc/docker/daemon.json        # Linux
# macOS: Docker Desktop → Settings → Docker Engine

# 2. insecure-registries 추가
# /etc/docker/daemon.json 또는 Docker Desktop 설정에 아래 추가
{
  "insecure-registries": ["localhost:5001"]
}

# 3. docker 재시작
sudo systemctl restart docker      # Linux
# macOS: Docker Desktop 재시작

# 4. 레지스트리 응답 재확인
curl http://localhost:5001/v2/
```

레지스트리 컨테이너는 `--publish 127.0.0.1:5001:5000`으로 바인딩되어 있으므로 외부에서는 접근할 수 없다. 인증이 필요하면 `registry:2` 컨테이너 환경 변수로 htpasswd 인증을 활성화할 수 있으나, 에어갭 내부 단독 환경에서는 불필요하다.

**로그 수집**

```bash
docker logs kind-registry
docker info | grep -A5 'Insecure Registries'
curl -v http://localhost:5001/v2/
```

---

## 공통 로그 수집 명령

문제 상황을 보고하거나 추가 진단이 필요할 때 아래 명령으로 전체 상태를 수집한다.

```bash
# 클러스터 전체 상태
kubectl get nodes,pods,pvc,svc -A --context kind-nullus-airgap

# 파드 상세 이벤트
kubectl describe pod <pod-name> -n nullus --context kind-nullus-airgap

# 레지스트리 로그
docker logs kind-registry --tail 50

# 레지스트리 카탈로그
curl -s http://localhost:5001/v2/_catalog

# kind 노드 이미지 목록 (crictl)
CONTROL_NODE=$(kubectl get nodes --context kind-nullus-airgap \
  -l node-role.kubernetes.io/control-plane \
  --no-headers -o custom-columns=':metadata.name')
docker exec "${CONTROL_NODE}" crictl images

# containerd 설정 덤프
docker exec "${CONTROL_NODE}" cat /etc/containerd/config.toml

# 클러스터 이벤트 (최근 30개)
kubectl get events -A --sort-by='.lastTimestamp' --context kind-nullus-airgap | tail -30
```

관련 문서: [../README.md](../README.md) | [architecture.md](./architecture.md) | [runbook.md](./runbook.md)
