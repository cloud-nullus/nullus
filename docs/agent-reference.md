# Agent Reference — 시행착오 및 해결 방법

Nullus Platform 개발 과정에서 반복적으로 겪은 문제와 해결 패턴을 정리합니다.
kind 클러스터 배포, Helm 차트, Docker 빌드, 프론트엔드 빌드 전반을 다룹니다.

---

## Helm 차트

### ServiceAccount 템플릿 누락

**증상**: Pod 생성 시 `error looking up service account nullus-system/nullus: serviceaccount "nullus" not found`

**원인**: `deployment.yaml`에서 `serviceAccountName: nullus`를 참조하지만, ServiceAccount를 생성하는 템플릿 파일이 없었음.

**해결**: `deploy/helm/nullus/templates/serviceaccount.yaml` 추가.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "nullus.fullname" . }}
  labels:
    {{- include "nullus.labels" . | nindent 4 }}
automountServiceAccountToken: false
```

**교훈**: `helm template`으로 렌더링한 결과에서 `serviceAccountName`을 grep 해보면, 해당 SA를 생성하는 리소스가 있는지 바로 확인할 수 있다.

```bash
helm template nullus deploy/helm/nullus/ | grep -A5 "kind: ServiceAccount"
```

---

### pre-install Hook이 PostgreSQL보다 먼저 실행됨

**증상**: Migration Job이 `nullus-postgresql` 호스트에 연결 실패. Job이 영원히 재시도.

**원인**: Migration Job이 `helm.sh/hook: pre-install` 어노테이션을 가지고 있어서 PostgreSQL StatefulSet보다 먼저 실행됨. pre-install Hook은 모든 일반 리소스(StatefulSet 포함) 생성 **전에** 실행된다.

**해결 방법 3가지**:

| 방법 | 장점 | 단점 |
|------|------|------|
| Hook을 `post-install`로 변경 | 간단 | `--wait` 없으면 PG가 아직 Ready가 아닐 수 있음 |
| API Deployment에 initContainer로 이동 | Pod 수명과 연결됨, 재시도 자동 | Migration이 Deployment에 결합 |
| 별도 Job + wait-for-db initContainer | 유연함 | ConfigMap/이미지 관리 필요 |

현재 선택: initContainer 방식 (wait-for-db initContainer → API 컨테이너).

```yaml
initContainers:
- name: wait-for-db
  image: busybox:1.37
  command: ['sh', '-c', 'until nc -z nullus-postgresql 5432; do sleep 2; done']
```

---

### Migration Job의 command가 존재하지 않는 서브커맨드 참조

**증상**: Migration Job이 실행되지만 바이너리가 `migrate` 서브커맨드를 지원하지 않아 실패.

**원인**: `cmd/api/main.go`는 순수 웹서버. `api migrate up` 같은 서브커맨드가 구현되어 있지 않음. Migration은 `golang-migrate` CLI로 별도 실행해야 함.

**교훈**: Helm 차트의 Job이 참조하는 커맨드가 실제 바이너리에 존재하는지 반드시 확인할 것. `docker run --rm <image> <command> --help`로 검증 가능.

---

### ConfigMap이 Pod에 마운트되지 않음

**증상**: API Pod가 `CrashLoopBackOff`. 로그: `failed to load config error="open configs/config.yaml: no such file or directory"`

**원인**: `configmap.yaml`은 존재하지만 `deployment.yaml`에 volumeMount가 없었음.

**해결**: Deployment에 ConfigMap 볼륨 마운트 추가.

```yaml
volumeMounts:
- name: config
  mountPath: /configs
  readOnly: true
volumes:
- name: config
  configMap:
    name: {{ include "nullus.fullname" . }}-config
```

**주의**: Go 코드에서 `config.LoadConfig("configs/config.yaml")`처럼 상대경로를 사용하면, 컨테이너의 WORKDIR(기본 `/`)에서 `/configs/config.yaml`으로 해석된다. 마운트 경로가 이와 정확히 일치해야 함.

---

### DB 비밀번호 불일치

**증상**: API health check에서 `"db":"unavailable"`. Pod 자체는 Running.

**원인**: Helm values에서 `secrets.dbPassword`(API가 사용)와 `postgresql.auth.password`(PostgreSQL 초기화용)가 서로 다른 값.

**해결**: 두 값을 일치시키거나, 같은 Secret을 참조하도록 수정.

```bash
helm install nullus deploy/helm/nullus/ \
  --set secrets.dbPassword=nullus \          # API가 사용
  --set postgresql.auth.password=nullus      # PostgreSQL이 사용 (기본값과 동일하면 생략 가능)
```

**교훈**: 비밀번호가 2곳 이상에서 설정되는 구조에서는 불일치가 자주 발생. values.yaml에 한 곳에서만 정의하고 helpers로 참조하는 패턴이 안전.

---

### production 모드에서 OIDC 초기화 실패

**증상**: API Pod `CrashLoopBackOff`. 로그: `failed to initialize OIDC provider`

**원인**: `config.server.mode=production`일 때 OIDC provider 설정이 필수. kind 환경에서는 Keycloak이 없으므로 실패.

**해결**: kind 배포 시 `development` 모드로 설정.

```bash
--set config.server.mode=development
```

---

## Docker 빌드

### Dockerfile Go 버전 불일치

**증상**: `go: go.mod requires go >= 1.26.1 (running go 1.24.13; GOTOOLCHAIN=local)`

**원인**: `Dockerfile`의 `FROM golang:1.24-alpine`이 `go.mod`의 Go 1.26.1과 불일치.

**해결**: Dockerfile의 Go 버전을 `go.mod`에 맞춤.

```dockerfile
FROM golang:1.26-alpine AS builder
```

**예방**: CI에서 `go.mod`의 Go 버전을 파싱해 Dockerfile과 비교하는 lint 추가.

```bash
GOMOD_VER=$(grep '^go ' go.mod | awk '{print $2}')
DOCKER_VER=$(grep 'FROM golang:' Dockerfile | sed 's/FROM golang:\([0-9.]*\).*/\1/')
```

---

### web/Dockerfile 빌드 컨텍스트 불일치

**증상**: `COPY package.json: not found` 또는 `COPY web/nginx.conf: not found`

**원인**: Dockerfile 내 `COPY` 경로가 빌드 컨텍스트와 맞지 않음.

| 빌드 명령 | 컨텍스트 | `COPY package.json` | `COPY web/nginx.conf` |
|-----------|---------|---------------------|----------------------|
| `docker build -f web/Dockerfile .` | repo root | **실패** (web/에 있음) | 성공 |
| `docker build -f web/Dockerfile web/` | web/ | 성공 | **실패** (web/web/가 됨) |

**해결**: 빌드 컨텍스트를 `web/`으로 통일하고, COPY 경로를 상대경로로 수정.

```dockerfile
# web/Dockerfile
COPY nginx.conf /etc/nginx/conf.d/default.conf   # web/ 아닌 nginx.conf
```

```bash
docker build -f web/Dockerfile web/
```

---

### npm ci peer dependency 충돌

**증상**: `ERESOLVE could not resolve — peer vite@"^5.2.0 || ^6 || ^7" from @tailwindcss/vite@4.2.1`

**원인**: `vite@8`과 `@tailwindcss/vite@4.2.1`의 peer dependency 범위가 불일치.

**해결**: `--legacy-peer-deps` 플래그 추가.

```dockerfile
RUN npm ci --legacy-peer-deps
```

**근본 해결**: `@tailwindcss/vite`를 vite@8 호환 버전으로 업그레이드하거나, vite를 7.x로 다운그레이드.

---

## kind 클러스터

### 이미지 Pull 실패 (ImagePullBackOff)

**증상**: Pod이 `ImagePullBackOff` 상태. `Failed to pull image "ghcr.io/cloud-nullus/nullus-api:0.1.0-alpha"`

**원인**: kind 클러스터는 로컬 Docker 이미지에 직접 접근할 수 없음. 이미지를 kind에 명시적으로 로드해야 함.

**해결**:

```bash
# 1. 이미지 빌드
docker build -t ghcr.io/cloud-nullus/nullus-api:0.1.0-alpha .
docker build -t ghcr.io/cloud-nullus/nullus-web:0.1.0-alpha -f web/Dockerfile web/

# 2. kind에 로드
kind load docker-image ghcr.io/cloud-nullus/nullus-api:0.1.0-alpha --name nullus-platform
kind load docker-image ghcr.io/cloud-nullus/nullus-web:0.1.0-alpha --name nullus-platform

# 3. Helm에서 pullPolicy=Never 설정
--set api.image.pullPolicy=Never
--set web.image.pullPolicy=Never
--set postgresql.image.pullPolicy=Never
```

**PostgreSQL도 로드 필요**: Bitnami PostgreSQL 이미지 태그가 Docker Hub에 존재하지 않을 수 있음. `docker pull bitnami/postgresql:latest`로 로컬에 받은 후 `docker tag`로 차트가 기대하는 태그를 붙여서 로드.

```bash
docker pull bitnami/postgresql:latest
docker tag bitnami/postgresql:latest bitnami/postgresql:17.5.0-debian-12-r20
kind load docker-image bitnami/postgresql:17.5.0-debian-12-r20 --name nullus-platform
```

---

### busybox 이미지도 로드 필요

**증상**: initContainer `wait-for-db`가 `ImagePullBackOff`.

**원인**: kind 노드가 `busybox:1.37`을 pull하지 못함 (네트워크 제한 또는 rate limit).

**해결**:

```bash
docker pull busybox:1.37
kind load docker-image busybox:1.37 --name nullus-platform
```

---

### kind 클러스터 재생성 후 kubeconfig 갱신 필요

**증상**: `kubectl get nodes` 실패 — `connection refused` 또는 포트 불일치.

**원인**: kind 클러스터를 삭제/재생성하면 API server 포트가 바뀜. 기존 kubeconfig가 폐기된 포트를 가리킴.

**해결**:

```bash
kind get kubeconfig --name nullus-platform > /tmp/nullus-platform.kubeconfig
export KUBECONFIG=/tmp/nullus-platform.kubeconfig
kubectl get nodes  # 정상 확인
```

API에 등록된 클러스터도 kubeconfig를 갱신해야 함.

---

## Helm 검증 체크리스트

배포 전 아래 순서로 확인하면 대부분의 문제를 사전에 잡을 수 있다.

```bash
# 1. lint — 문법 오류
helm lint deploy/helm/nullus/

# 2. template — 렌더링 확인
helm template nullus deploy/helm/nullus/ --values deploy/helm/nullus/values.yaml > /tmp/rendered.yaml

# 3. ServiceAccount 확인
grep -A5 "kind: ServiceAccount" /tmp/rendered.yaml
grep "serviceAccountName:" /tmp/rendered.yaml

# 4. 이미지 태그 확인
grep "image:" /tmp/rendered.yaml

# 5. 볼륨 마운트 확인
grep -B2 -A3 "mountPath:" /tmp/rendered.yaml

# 6. server-side dry-run — K8s API 검증
helm install nullus deploy/helm/nullus/ --dry-run=server --namespace nullus-system --create-namespace

# 7. 실제 배포
helm install nullus deploy/helm/nullus/ --namespace nullus-system --create-namespace --wait --timeout 5m
```

---

## 빠른 참조

| 문제 | 첫 번째로 확인할 것 |
|------|-------------------|
| Pod `ImagePullBackOff` | `kind load docker-image` 했는지, `pullPolicy: Never` 설정했는지 |
| Pod `CrashLoopBackOff` | `kubectl logs <pod>` — config 파일 경로, DB 연결, 모드(dev/prod) |
| Helm install timeout | `kubectl get pods -n <ns>` → 어떤 Pod이 Ready가 아닌지 확인 |
| ServiceAccount not found | `helm template` 결과에서 SA 리소스 존재 여부 확인 |
| DB connection refused | PG Pod Ready 여부, 비밀번호 일치 여부, initContainer wait-for-db 존재 여부 |
| Config file not found | ConfigMap 볼륨 마운트 존재 여부, 마운트 경로와 코드의 경로 일치 여부 |
| Go build 실패 (Docker) | `go.mod` Go 버전과 Dockerfile Go 이미지 버전 일치 여부 |
| npm ci 실패 | peer dependency 충돌 → `--legacy-peer-deps` 또는 버전 맞춤 |
