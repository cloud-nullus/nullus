# 운영 런북
> Nullus Platform 에어갭 배포의 단계별 절차, 재설치/업그레이드/클린업 시나리오

## 단계별 절차

각 단계는 독립적으로 재실행할 수 있도록 멱등성을 보장한다. `DRY_RUN=1`을 설정하면 모든 스크립트가 실제 명령 대신 실행 예정 명령을 출력한다.

---

### 1단계 — 이미지 Pull (온라인 머신)

**목적**: `airgap/images/images.txt`에 나열된 모든 이미지를 로컬 docker daemon으로 Pull한다.

```bash
bash airgap/scripts/01-pull-images.sh

# podman 사용 시
PODMAN=1 bash airgap/scripts/01-pull-images.sh
```

**성공 기준**: 종료 코드 0, 마지막 로그에 `All images pulled successfully.` 출력.

**롤백**: 개별 이미지 Pull 실패 시 스크립트가 목록을 출력한다. 해당 이미지만 수동으로 Pull한 뒤 재실행한다.

```bash
docker pull ghcr.io/cloud-nullus/nullus/nullus-api:main
```

---

### 2단계 — 번들 저장 (온라인 머신)

**목적**: Pull된 이미지를 `bundle/images.tar.gz`로 저장하고 SHA-256 체크섬과 MANIFEST.txt를 생성한다.

```bash
bash airgap/scripts/02-save-bundle.sh

# 번들 저장 위치 변경 시
BUNDLE_DIR=/data/airgap-bundle bash airgap/scripts/02-save-bundle.sh
```

**성공 기준**: `airgap/bundle/images.tar.gz`, `images.tar.gz.sha256`, `MANIFEST.txt` 세 파일이 생성된다.

**롤백**: 디스크 공간 부족이 원인이면 불필요한 이미지를 `docker rmi`로 정리한 뒤 재실행한다. 기존 번들 파일은 재실행 시 덮어쓴다.

---

### 3단계 — 번들 반입

**목적**: 온라인 머신의 번들을 오프라인 머신으로 물리적으로 이동한다.

```bash
# rsync 예시 (같은 네트워크 내 임시 연결 허용 시)
rsync -avz airgap/bundle/ offline-host:/opt/nullus-airgap/bundle/
rsync -avz airgap/ offline-host:/opt/nullus-airgap/

# USB 복사 후 반입 검증
cd airgap/bundle
sha256sum -c images.tar.gz.sha256   # Linux
shasum -a 256 -c images.tar.gz.sha256  # macOS
```

**성공 기준**: `sha256sum -c` 또는 `shasum -c` 결과가 `OK`.

**롤백**: 체크섬 불일치 → 파일 재복사.

---

### 4단계 — 번들 로드 (오프라인 머신)

**목적**: `bundle/images.tar.gz`를 `docker load`로 로컬 daemon에 적재한다.

```bash
# Agent 2가 작성하는 스크립트
bash airgap/scripts/03-load-bundle.sh

# 이미 로드된 경우 건너뜀
SKIP_LOAD=1 bash airgap/scripts/03-load-bundle.sh
```

**성공 기준**: `docker images | grep nullus-api` 등 이미지가 로컬 daemon에 존재한다.

**롤백**: `docker rmi`로 이미지를 제거하고 재실행한다.

---

### 5단계 — 로컬 레지스트리 기동 (오프라인 머신)

**목적**: `registry:2` 컨테이너를 `kind-registry`라는 이름으로 `127.0.0.1:5001`에 바인딩하여 기동한다.

```bash
bash airgap/scripts/10-setup-registry.sh

# 포트 변경 시
REGISTRY_PORT=5002 bash airgap/scripts/10-setup-registry.sh
```

**성공 기준**:

```bash
curl -sf http://localhost:5001/v2/
# 출력: {}
```

**롤백**: `docker stop kind-registry && docker rm kind-registry` 후 재실행한다.

---

### 6단계 — kind 클러스터 생성 (오프라인 머신)

**목적**: `kind/kind-airgap.yaml` 설정으로 `nullus-airgap` 클러스터를 생성하고, kind-registry를 `kind` 도커 네트워크에 연결하며, `registry.yaml` ConfigMap을 적용한다.

```bash
bash airgap/scripts/11-create-cluster.sh

# 클러스터 이름 변경 시
CLUSTER_NAME=my-cluster bash airgap/scripts/11-create-cluster.sh
```

**성공 기준**:

```bash
kind get clusters
# 출력에 nullus-airgap 포함

kubectl cluster-info --context kind-nullus-airgap
# Kubernetes control plane 주소 출력
```

**롤백**: `kind delete cluster --name nullus-airgap` 후 재실행한다. 레지스트리 컨테이너는 그대로 유지된다.

---

### 7단계 — 이미지 리태그 & 레지스트리 푸시 (오프라인 머신)

**목적**: 로컬 daemon의 이미지를 `localhost:5001/<path>:<tag>` 형식으로 리태그한 뒤 로컬 레지스트리에 Push한다.

```bash
bash airgap/scripts/12-push-to-registry.sh
```

**성공 기준**:

```bash
curl -s http://localhost:5001/v2/_catalog
# {"repositories":["cloud-nullus/nullus/nullus-api","cloud-nullus/nullus/nullus-web","bitnamilegacy/postgresql",...]}
```

**롤백**: Push 실패한 이미지만 수동으로 리태그 후 재Push한다. 스크립트 재실행도 안전하다 (멱등).

---

### 8단계 — Helm 설치 (오프라인 머신)

**목적**: `values-airgap.yaml`을 사용해 Nullus Helm 차트를 `nullus` 네임스페이스에 설치한다.

```bash
# Agent 3이 작성하는 스크립트
bash airgap/scripts/21-install-nullus.sh

# 환경 변수 재정의
RELEASE=nullus NAMESPACE=nullus bash airgap/scripts/21-install-nullus.sh
```

**성공 기준**:

```bash
helm status nullus -n nullus
# STATUS: deployed

kubectl get pods -n nullus
# 모든 파드 Running 상태
```

**롤백**: `helm uninstall nullus -n nullus` 후 재설치한다. PVC는 자동 삭제되지 않으므로 데이터 초기화 필요 시 수동 삭제한다.

```bash
kubectl delete pvc -n nullus --all
```

---

### 9단계 — 환경 검증 (오프라인 머신)

**목적**: 클러스터 API 서버 응답, 노드 Ready 상태, 파드 상태, 레지스트리 응답, 노드 이미지 목록을 종합 검증한다.

```bash
bash airgap/scripts/99-verify.sh

# 클러스터 이름 변경 시
CLUSTER_NAME=my-cluster bash airgap/scripts/99-verify.sh
```

**성공 기준**: 마지막 줄에 `최종 결과: PASS — 클러스터 준비 완료` 출력.

**롤백**: 검증 항목별 FAIL 원인을 [troubleshooting.md](./troubleshooting.md)에서 확인한다.

---

## 부트스트랩 원샷

Agent 2가 제공하는 오케스트레이터 스크립트로 4~9단계를 한 번에 실행할 수 있다.

```bash
# Makefile 사용
make -C airgap all

# 직접 스크립트 실행
bash airgap/scripts/bootstrap.sh
```

`bootstrap.sh`는 각 단계를 순서대로 실행하며 중간 실패 시 즉시 중단한다. `DRY_RUN=1`로 전체 흐름을 미리 확인할 수 있다.

```bash
DRY_RUN=1 bash airgap/scripts/bootstrap.sh
```

---

## 재설치 시나리오

기존 클러스터를 완전히 초기화하고 재설치할 때 사용한다.

```bash
# 1. 기존 클러스터 삭제 (레지스트리는 유지)
kind delete cluster --name nullus-airgap

# 2. 클러스터 재생성
bash airgap/scripts/11-create-cluster.sh

# 3. 이미지 재Push (레지스트리가 살아 있으면 건너뜀 가능)
bash airgap/scripts/12-push-to-registry.sh

# 4. Helm 재설치
bash airgap/scripts/21-install-nullus.sh

# 5. 검증
bash airgap/scripts/99-verify.sh
```

레지스트리 컨테이너를 재사용하므로 `03-load-bundle.sh`와 `10-setup-registry.sh`는 불필요하다.

---

## 버전 업그레이드 시나리오

새 버전의 이미지와 차트로 업그레이드할 때 사용한다.

```bash
# 온라인 머신: 새 버전으로 images.txt 수정 후 번들 재생성
vi airgap/images/images.txt   # 태그 변경
bash airgap/scripts/01-pull-images.sh
bash airgap/scripts/02-save-bundle.sh

# 번들 재반입 후 오프라인 머신에서:

# 1. 새 번들 로드
bash airgap/scripts/03-load-bundle.sh

# 2. 새 이미지 Push
bash airgap/scripts/12-push-to-registry.sh

# 3. Helm upgrade
helm upgrade nullus airgap/helm/nullus-*.tgz \
  -n nullus \
  -f airgap/helm/values-airgap.yaml \
  --context kind-nullus-airgap

# 4. 롤아웃 확인
kubectl rollout status deployment/nullus-api -n nullus
kubectl rollout status deployment/nullus-web -n nullus

# 5. 검증
bash airgap/scripts/99-verify.sh
```

업그레이드 실패 시 이전 버전으로 롤백:

```bash
helm rollback nullus -n nullus
```

---

## 클린업

개발 환경을 완전히 제거할 때 사용한다.

```bash
make -C airgap clean
```

`make clean`의 실행 효과:

| 항목 | 처리 |
|------|------|
| kind 클러스터 `nullus-airgap` | 삭제 (`kind delete cluster`) |
| 레지스트리 컨테이너 `kind-registry` | 중지 및 삭제 (`docker stop` + `docker rm`) |
| docker 네트워크 `kind` | 삭제 (kind 삭제 시 자동) |
| `bundle/` 디렉토리 내 파일 | **보존** (재반입 불필요) |
| 로컬 docker 이미지 | **보존** (재Pull 불필요) |

로컬 이미지까지 제거하려면 수동으로 실행한다:

```bash
# images.txt 기반 일괄 제거
while IFS= read -r line; do
  [[ -z "$line" || "$line" == \#* ]] && continue
  docker rmi "$line" 2>/dev/null || true
done < airgap/images/images.txt
```

관련 문서: [../README.md](../README.md) | [architecture.md](./architecture.md) | [troubleshooting.md](./troubleshooting.md)
