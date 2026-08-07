# Zadara Cloud PoC — Nullus 배포 런북

Zadara Cloud 위에 구축된 Kubernetes 클러스터에 Nullus Platform을 배포하는 절차.
클러스터 **구축** 절차는 `docs/50_운영/zadara_cloud_poc.md`에 있고, 이 문서는 그
`§11 다음 단계`에 해당하는 **배포** 절차를 다룬다.

- 작업 위치: **nullus-node-10 (bastion)** — 외부에서 SSH 가능한 유일한 노드
- 배포 대상: **platform 클러스터** (node-10 control-plane + node-11 worker)
- 배포 버전: `v0.3.0-alpha` (첫 정식 릴리즈)

---

## 1. 클러스터 현황 (2026-07-28 확인)

| 항목 | platform | develop |
|---|---|---|
| Control plane | nullus-node-10 (172.31.0.10) | nullus-node-20 (172.31.1.20) |
| Worker | nullus-node-11 (172.31.0.11) | nullus-node-21 (172.31.1.21) |
| Kubernetes | v1.34.3 | v1.34.3 |
| 런타임 / CNI | containerd 2.2.1 / Calico | containerd 2.2.1 / Calico |
| OS | Ubuntu 24.04.3 LTS | Ubuntu 24.04.3 LTS |

> 구축 가이드는 Ubuntu 22.04를 가정하지만 실제 노드는 24.04.3이다.

---

## 2. 완료된 사전작업

아래 3건은 이미 적용되어 있다. 클러스터를 재생성했다면 다시 수행할 것.

### 2.1 kubeconfig 컨텍스트 분리

Kubespray가 만든 두 `admin.conf`가 **모두 `cluster.local`이라는 동일한 식별자**를
써서, 그대로 병합하면 cluster 항목이 하나로 합쳐진다. 그 결과
`kubectl --context=develop`이 **platform 클러스터로 연결되어** 의도하지 않은
클러스터에 배포될 수 있다.

```bash
cp ~/.kube/config ~/.kube/config.bak.$(date +%Y%m%d-%H%M%S)

for c in platform develop; do
  sed "s/cluster\.local/$c/g" ~/kubespray/inventory/$c/artifacts/admin.conf > /tmp/$c.conf
  kubectl --kubeconfig /tmp/$c.conf config rename-context "kubernetes-admin-$c@$c" "$c"
done

KUBECONFIG=/tmp/platform.conf:/tmp/develop.conf kubectl config view --flatten > /tmp/merged.conf
install -m 600 /tmp/merged.conf ~/.kube/config
kubectl config use-context platform
rm -f /tmp/platform.conf /tmp/develop.conf /tmp/merged.conf
```

검증 — cluster 항목이 2개이고 서버 주소가 서로 달라야 한다.

```bash
kubectl config view -o jsonpath='{range .clusters[*]}{.name}{"\t"}{.cluster.server}{"\n"}{end}'
# develop   https://172.31.1.20:6443
# platform  https://172.31.0.10:6443
```

### 2.2 StorageClass — local-path-provisioner

Zadara 볼륨 CSI 연동이 아직 없어 PoC 한정으로 노드 로컬 디스크를 쓴다.
**StorageClass가 없으면 PostgreSQL PVC가 Pending에서 멈춰 배포가 진행되지 않는다.**

```bash
kubectl --context=platform apply -f \
  https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.31/deploy/local-path-storage.yaml

kubectl --context=platform patch storageclass local-path \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

> `WaitForFirstConsumer` 모드라 PVC는 파드가 스케줄된 노드의 로컬 디스크에 바인딩된다.
> **노드를 재생성하면 데이터가 사라진다.** 프로덕션 전환 시 Zadara 볼륨 CSI로 교체할 것.

### 2.3 Ingress — ingress-nginx (NodePort)

Zadara에는 LoadBalancer 연동이 없다. MetalLB는 VPC 여유 IP 대역이 확정되지 않아
쓰지 않고, NodePort로 고정한다.

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm --kube-context=platform upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx --create-namespace \
  --set controller.service.type=NodePort \
  --set controller.service.nodePorts.http=30080 \
  --set controller.service.nodePorts.https=30443 \
  --set controller.replicaCount=1 \
  --wait --timeout 300s
```

NodePort는 전 노드에서 열리므로, 공인 IP가 붙은 node-10으로 들어온 요청이
kube-proxy를 거쳐 node-11의 컨트롤러 파드로 전달된다.

---

## 3. 남은 선행 조건

배포 전에 **반드시** 해결해야 한다.

### 3.1 ghcr 패키지 접근 — **해소됨 (2026-08-07)**

저장소가 public이어도 컨테이너 패키지는 기본 private이다. 3건 모두 private이라
`ImagePullBackOff`가 되던 상태였고, **2026-08-07에 public으로 전환해 해소했다.**
`imagePullSecrets`는 비운 채 두면 된다.

전환 확인 (클러스터에서 pull secret 없이 이미지가 받아져야 한다):

```bash
gh api /orgs/cloud-nullus/packages/container/nullus%2Fnullus-api --jq .visibility   # public
helm pull oci://ghcr.io/cloud-nullus/charts/nullus --version 0.3.0-alpha            # 익명 pull
kubectl --context=platform run ghcr-pull-probe --restart=Never --rm -i \
  --image=ghcr.io/cloud-nullus/nullus/nullus-api:0.3.0-alpha -- echo IMAGE_PULL_OK
```

이미지 2종은 `linux/amd64` + `linux/arm64` 멀티아치다. `skopeo inspect --no-creds`를
macOS(arm64/darwin)에서 돌리면 호스트 플랫폼이 인덱스에 없어
`no image found in image index for architecture`가 나는데, 이는 접근 실패가 아니다.
`--raw`로 인덱스를 직접 보거나 `--override-os linux`를 붙인다.

> 클러스터를 새로 만들거나 패키지를 다시 private으로 돌렸다면 pull secret이 필요하다.
>
> ```bash
> kubectl --context=platform create namespace nullus 2>/dev/null || true
> kubectl --context=platform -n nullus create secret docker-registry ghcr-pull-secret \
>   --docker-server=ghcr.io --docker-username="$GHCR_USER" --docker-password="$GHCR_PAT"
> ```
>
> `GHCR_PAT`는 `read:packages` 스코프면 충분하다. 이후 `values-zadara.yaml`의
> `imagePullSecrets`를 `[{name: ghcr-pull-secret}]`로 바꾼다. 가시성 전환은 REST API로
> 되지 않고 **패키지 설정 UI**에서만 가능하다 — 저장소 Settings가 아니라
> `https://github.com/orgs/cloud-nullus/packages` 아래의 패키지별 Settings다.

### 3.2 차트 패치 — v0.3.0-alpha 태그 그대로는 설치되지 않는다

`v0.3.0-alpha` 태그의 차트에는 이 클러스터에서 실증된 결함 2건이 있다 (CHANGELOG `Unreleased`
`Fixed` 참조). 아직 패치 릴리즈가 나오지 않았다면 **`main`의 차트를 쓰거나** 해당 커밋을
체크아웃해야 한다.

| 결함 | 증상 | 확인 명령 |
|---|---|---|
| `nullus-wildcard-tls` 필수 마운트 | API 파드가 `FailedMount`로 Pending | `kubectl -n nullus describe pod -l app.kubernetes.io/component=api` |
| `bitnami/postgresql` 이미지 소멸 | PostgreSQL 파드 `ImagePullBackOff` | `skopeo inspect --no-creds docker://bitnami/postgresql:17.5.0-debian-12-r20` |

수정된 차트에서는 아래가 참이어야 한다.

```bash
helm template nullus deploy/helm/nullus -f deploy/csp/zadara/values-zadara.yaml \
  --set secrets.dbPassword=x --set secrets.encryptionKey=0123456789abcdef0123456789abcdef \
  | grep -E 'optional: true|bitnamilegacy/postgresql'
```

### 3.3 보안 그룹

현재 외부 → node-10은 **22/tcp만** 열려 있다. 웹 UI에 접근하려면 Zadara 콘솔에서
`30080/tcp`(및 HTTPS 사용 시 `30443/tcp`)를 추가로 연다. 접근 소스는 운영자 IP로
제한하는 것을 권장한다 — PoC 환경에 인증 없이 노출되는 경로가 생긴다.

---

## 4. 배포

```bash
# node-10 에서
git clone https://github.com/cloud-nullus/nullus.git && cd nullus
# 차트 결함 2건(§3.2)이 수정된 리비전을 쓴다. 패치 릴리즈가 나오면 그 태그로 바꿀 것.
git checkout main

export DB_PASSWORD='<강력한_값>'
export ENCRYPTION_KEY='<정확히_32바이트>'   # 예: openssl rand -hex 16

helm dependency update deploy/helm/nullus

helm --kube-context=platform upgrade --install nullus deploy/helm/nullus \
  --namespace nullus --create-namespace \
  -f deploy/csp/zadara/values-zadara.yaml \
  --set secrets.dbPassword="$DB_PASSWORD" \
  --set secrets.encryptionKey="$ENCRYPTION_KEY" \
  --wait --timeout 600s
```

이미지 경로·태그는 차트 기본값(`values.yaml` + `Chart.appVersion`)을 그대로 쓴다.
다른 버전을 배포하려면 `--set api.image.tag=<버전> --set web.image.tag=<버전>`.

> 릴리즈 태그 `v0.3.0-alpha` → 이미지 태그 `0.3.0-alpha` (`v` 접두사 없음).
> 프리릴리즈는 `latest`·`0.3` 같은 rolling 태그를 만들지 않으므로 전체 버전을 써야 한다.

### 4.1 DB 마이그레이션

차트는 스키마를 만들지 않는다. 배포 후 마이그레이션 Job을 실행한다.

```bash
CHART_PATH=./deploy/helm/nullus NULLUS_NAMESPACE=nullus \
  ./deploy/csp/vm-cluster/runbook_csp.sh status   # 배포 상태 확인
```

`runbook_csp.sh deploy`는 MetalLB·ingress 설치까지 포함하므로, 위 2.2/2.3을 이미
수행한 이 환경에서는 **helm 직접 배포 + 마이그레이션만** 사용한다.

---

## 5. 검증

```bash
kubectl --context=platform -n nullus get pods -o wide      # 전부 Running
kubectl --context=platform -n nullus get pvc               # Bound
kubectl --context=platform -n nullus get ingress

# 클러스터 내부에서 Host 헤더를 붙여 확인
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: nullus.zadara.poc' http://172.31.0.10:30080/

# 외부에서 (보안 그룹 개방 후)
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: nullus.zadara.poc' http://<node-10-public-ip>:30080/
```

`nullus.zadara.poc`는 실재하지 않는 이름이므로, 브라우저로 접근하려면 로컬
`/etc/hosts`에 `<node-10-public-ip> nullus.zadara.poc`를 추가한다.

라우팅 구조: ingress는 **모든 경로를 web 서비스로** 보내고, web 컨테이너의 nginx가
`/api/`·`/ws/`를 `http://nullus-api:8080`으로 프록시한다. 따라서 **Helm 릴리즈 이름은
반드시 `nullus`여야 한다** — 이름이 바뀌면 서비스명이 달라져 API 프록시가 깨진다.

---

## 6. 로컬에서 접근하기 (스크립트)

기본 설계는 **외부에서 열리는 포트가 node-10 의 `22/tcp` 하나뿐**이라는 것이다
(`zadara_cloud_poc.md` §1.2·§1.3, §11 "보안 그룹 최소화"). 아래 스크립트는 모두
그 22/tcp 위로 터널을 뚫어, 보안 그룹을 건드리지 않고 로컬에서 쓰게 해 준다.

| 스크립트 | 하는 일 | 기본 로컬 포트 |
|---|---|---|
| `tunnel.sh` | 웹 UI 를 브라우저로 연다 | 30080 |
| `kubeconfig.sh` | `kubectl`·`helm` 을 붙인다 | 16443 |
| `expose-apiserver.sh` | apiserver 를 외부에 노출 — **기본적으로 쓰지 않는다** | — |

### 6.1 웹 UI

```bash
./deploy/csp/zadara/tunnel.sh          # direct 모드 — hosts/sudo 불필요
# → http://127.0.0.1:30080
./deploy/csp/zadara/tunnel.sh stop
```

`direct` 는 bastion 에서 `kubectl port-forward svc/nullus-web` 를 띄워 그 포트를 당겨온다.
실제 외부 접근 경로(ingress-nginx NodePort)를 그대로 재현하려면 `MODE=ingress` 를 쓴다 —
이 경우 ingress 가 Host 헤더로 라우팅하므로 `nullus.zadara.poc` hosts 매핑(sudo)이 필요하다.

### 6.2 kubectl / helm

```bash
./deploy/csp/zadara/kubeconfig.sh                  # 터널 + ~/.kube/nullus-zadara.conf 생성
export KUBECONFIG=$HOME/.kube/nullus-zadara.conf
kubectl get pods -A
./deploy/csp/zadara/kubeconfig.sh stop
```

`~/.kube/config` 에 **병합하지 않고 별도 파일로** 쓴다 — §2.1 의 `cluster.local` 충돌과 같은
이유다. cluster/user/context 이름을 모두 `nullus-zadara` 로 다시 지어 넣으므로, 나중에
`KUBECONFIG=a:b` 로 합쳐도 충돌하지 않는다. `develop` 은 `CLUSTER=develop` 으로.

`insecure-skip-tls-verify` 는 쓰지 않는다. API 서버 인증서 SAN 에 `127.0.0.1` 과 `localhost`
가 있어 터널 주소로 붙어도 검증이 통과한다.

### 6.3 apiserver 외부 노출 — 예외 경로

`expose-apiserver.sh` 는 apiserver 를 비표준 포트(36443)로 열고, **노드 iptables 에서 소스 IP
를 한 번 더 검사**한다(보안 그룹이 넓게 열려도 노드가 DROP). 6443 자체는 열지 않는다.

그래도 **문서 설계에 없던 외부 노출면을 만드는 선택**이므로 터널로 감당이 안 될 때만 쓰고,
끝나면 `close` 로 되돌린다. 보안 그룹 규칙 자체는 이 스크립트가 넣지 못한다 — 이 환경에는
zCompute API 자격증명도 IAM 롤도 없다(메타데이터 `iam/security-credentials` 가 404).

```bash
./deploy/csp/zadara/expose-apiserver.sh open    # 노드 규칙 + 보안 그룹 안내
./deploy/csp/zadara/expose-apiserver.sh close   # 되돌리기
```

---

## 7. 알려진 제약

| 항목 | 내용 |
|---|---|
| 스토리지 | local-path — 노드 재생성 시 데이터 소실. 백업 없음 |
| 워커 1대 | `replicaCount: 1` 고정. 2 이상이면 파드가 Pending |
| control-plane | node-10은 2vCPU/4GB이고 bastion 겸용 — 워크로드를 올리지 않는다 |
| TLS | 미구성. HTTP로만 접근한다. SSO(OIDC/PKCE)는 secure context가 필요해 **HTTPS 없이는 동작하지 않는다** |
| SSO 스택 | `deploy/k8s/oauth2-proxy/*`와 `airgap/helm/stack-values/*`에 커밋된 고정 시크릿과 `ssl-insecure-skip-verify` 가 있어 PoC 클라우드에 그대로 적용하면 안 된다 |
| develop 클러스터 | Nullus에 워크로드 클러스터로 등록하는 시나리오는 미수행 |
