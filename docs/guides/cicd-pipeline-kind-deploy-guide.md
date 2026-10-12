# Nullus Platform — CI/CD 파이프라인 kind 클러스터 배포 가이드

CI/CD 파이프라인 템플릿을 선택하고, kind 클러스터에 배포하는 전 과정을 단계별로 안내합니다.

- **최종 갱신**: 2026-10-12 — 요청 본문·템플릿 목록·실행 모드를 지금 코드(`internal/cicd/adapter/handler/pipeline_handler.go`, `internal/cicd/usecase/create_pipeline.go`)와 대조해 맞췄다. 스택 설치부터 파이프라인·백업까지 실제로 돌려 본 순서는 [로컬 E2E 가이드](../50_운영/Nullus_로컬_E2E_가이드.html)가 더 자세하다.

## 사전 요구사항

- Docker Desktop 실행 중
- kind, kubectl, helm 설치
- Go 1.26+, Node.js 22+

```bash
# 버전 확인
docker info --format '{{.ServerVersion}}'
kind version
kubectl version --client --short
helm version --short
go version
node --version
```

---

## Step 0: 로컬 환경 기동

### 0-1. 인프라 + API + 프론트엔드 한 번에 기동

```bash
./scripts/runbook_local.sh up
```

PostgreSQL(:5433), Redis(:6380), MinIO(:9000), Keycloak(:8180)이 기동되고,
DB 마이그레이션 실행 후 API(:8090)와 프론트엔드(:5173) 개발 서버가 시작됩니다.

### 0-2. kind 클러스터 생성

```bash
./scripts/runbook_local.sh kind-up
```

`nullus-platform`(control-plane + worker)과 `nullus-develop`(control-plane + worker) 2개 클러스터가 생성됩니다.

```bash
# 확인
kind get clusters
kubectl get nodes --context kind-nullus-platform
kubectl get nodes --context kind-nullus-develop
```

### 0-3. 상태 확인

```bash
# API 건강 확인
curl -s http://localhost:8090/health | python3 -m json.tool
# → {"db":"connected","status":"healthy","version":"0.1.0-alpha"}

# 프론트엔드 확인
curl -s -o /dev/null -w "%{http_code}" http://localhost:5173
# → 200
```

---

## Step 1: kind 클러스터 등록 및 연결 검증

API에 kind-nullus-platform 클러스터를 등록합니다.

### 1-1. kubeconfig 추출

```bash
kind get kubeconfig --name nullus-platform > /tmp/nullus-platform.kubeconfig
```

### 1-2. 클러스터 등록 (API)

```bash
curl -s -X POST http://localhost:8090/api/v1/admin/clusters \
  -H 'Content-Type: application/json' \
  -d "$(jq -n \
    --arg name 'kind-nullus-platform' \
    --arg type 'pipeline' \
    --arg org '11111111-1111-1111-1111-111111111111' \
    --arg kubeconfig "$(cat /tmp/nullus-platform.kubeconfig)" \
    '{name:$name,type:$type,org_id:$org,kubeconfig:$kubeconfig}')" \
  | python3 -m json.tool
```

요청 필드는 `name` · `type`(또는 여러 역할이면 `types: ["pipeline","target"]`) · `org_id` · `kubeconfig` 이고, `cloud_provider` · `endpoint` 는 선택이다 (`internal/admin/adapter/handler/cluster_handler.go` 의 `registerClusterRequest`). kind 클러스터는 런북 `up`/`refresh` 가 자동으로 등록·갱신하므로 보통 이 단계는 확인만 한다 (`GET /api/v1/admin/clusters`).

응답에서 `id` 값을 복사합니다 (예: `11097a85-140d-42c9-9bcd-68d3d96d9fb9`).

### 1-3. 연결 검증

```bash
# <cluster-id>를 위에서 받은 id로 교체
curl -s -X POST http://localhost:8090/api/v1/admin/clusters/<cluster-id>/verify \
  | python3 -m json.tool
# → {"status":"connected","version":"v1.35.1"}
```

`status: connected`가 나오면 클러스터 등록 완료입니다.

> **UI로 등록하려면**: `http://localhost:5173` → admin@nullus.dev / admin123 로그인 → 사이드바 Admin → Cluster Management → Register Cluster

---

## Step 2: CI/CD 템플릿 선택 (UI)

### 2-1. 로그인

1. 브라우저에서 `http://localhost:5173` 접속
2. 테스트 계정으로 로그인:
   - Email: `devops@nullus.dev`
   - Password: `devops123`

### 2-2. CI/CD Template 페이지 이동

1. 좌측 사이드바에서 **CI/CD** → **CI/CD Template** 클릭
2. 또는 직접 URL 입력: `http://localhost:5173/cicd/templates`

시드된 CI/CD 템플릿은 4개다 (`GET /api/v1/cicd/templates`, 2026-10-12 기준):

| 템플릿 ID | 이름 | 앱 타입 | stages |
|-----------|------|---------|--------|
| `nullus-sample-backend-v1` | **Nullus Sample App — Backend** | backend | GitClone → DockerBuild → ImageLoad → Deploy |
| `nullus-sample-frontend-v1` | Nullus Sample App — Frontend | web | GitClone → DockerBuild → ImageLoad → Deploy |
| `web-backend-v1` | User Custom Pipeline | backend | Build → Deploy |
| `batch-job-v1` | Batch Job Pipeline | batch | Build → Deploy |

> `stages` 는 카드에 보이는 단계 라벨이자 **스택 없이 만든 파이프라인**의 단계 기본값이다. 스택에 붙여 만든 파이프라인은 저장소 프로비저닝이 CI 정의에서 단계(Build · SAST · ImageScan · Deploy 등)를 정한다 — 템플릿의 `stages` 와 다를 수 있다.

### 2-3. 템플릿 선택

**Nullus Sample App — Backend** 카드의 **기본 템플릿 사용**(Use Base Template) 버튼을 클릭한다. → **Pipeline Setup & Developer Deploy** 페이지(`/cicd/developer-deploy`)로 이동한다.

---

## Step 3: 파이프라인 설정 (위저드)

### 3-1. 앱 템플릿 선택

상단의 앱 템플릿(`GET /api/v1/cicd/app-templates`) 중 하나를 선택한다:

| ID | 이름 | 런타임 | 포트 |
|----|------|--------|------|
| `go-web-api` | **Go Web API** | go1.24 | 8080 |
| `react-vite` | React Vite App | node22 | 5173 |
| `spring-boot` | Spring Boot Service | java21 | 8080 |

### 3-2. 앱 이름

- 입력: `nullus-api-demo`
- 규칙: 소문자, 숫자, 하이픈만 사용 가능. 네임스페이스 기본값도 이 이름이다.
- 우측 YAML 미리보기에 Deployment + Service 매니페스트가 실시간 반영된다

### 3-3. 저장소

- **스택을 골랐으면** 저장소는 Nullus 가 스택의 GitLab/Gitea 에 만들어 준다 (스캐폴딩 커밋 포함) — URL 을 입력하지 않는다
- **스택 없이** 만들 때만 Repository URL / Branch 를 직접 넣는다 (예: `https://github.com/cloud-nullus/draft`, `main`)

### 3-4. 클러스터 / 네임스페이스 / 스택

- 클러스터: `kind-nullus-platform` (Step 1에서 등록한 클러스터)
- 네임스페이스: 앱 이름과 같게 (예: `nullus-api-demo`)
- 스택: 그 클러스터에 설치된 스택이 있으면 고른다. **앱은 스택이 설치된 클러스터에 배포된다** (Argo CD 목적지가 `kubernetes.default.svc`) — 화면에서 고른 클러스터가 다르면 모니터링 탭이 비어 보일 수 있다

### 3-5. 리소스 / 환경 변수

기본값: CPU 100m / 500m, Memory 128Mi / 512Mi, Replicas 2 (화면 기본값. API 예시는 1 로 준다). 환경 변수는 선택이다.

---

## Step 4: 파이프라인 생성 및 실행

파이프라인은 **실행 모드**가 둘이다. 어느 쪽인지에 따라 "배포" 가 뜻하는 것이 다르다.

| 실행 모드 | 언제 | 빌드·배포 주체 | `/deploy` 호출 |
|-----------|------|---------------|---------------|
| `stack_integrated` (기본) | `stack_id` 를 준 경우 | 스택의 CI(GitLab CI · Jenkins · GitHub Actions) + Argo CD | **하지 않는다.** 저장소에 커밋이 올라가면 CI 가 돌고 Argo CD 가 배포한다 |
| `emergency_direct` | `stack_id` 없이 만든 경우 | 플랫폼이 직접 이미지를 준비하고 매니페스트를 apply | `POST /pipelines/:id/deploy` |

`execution_mode` 를 비우면 `stack_id` 유무로 위처럼 정해진다 (`internal/cicd/usecase/create_pipeline.go`).

### 방법 A: UI

Pipeline Setup 페이지에서 생성하면 스택 통합 파이프라인은 저장소 · CI 변수 · pull Secret · Argo CD Application 까지 만들어지고 첫 CI 가 돈다. CI/CD List(`/cicd/list`)에서 실행 기록(History)을 본다.

### 방법 B: API — 스택 통합 파이프라인 (권장)

```bash
API=http://localhost:8090
STACK_ID=$(curl -s $API/api/v1/stacks | jq -r '.items[0].id')          # 설치된 스택
CLUSTER_ID=$(curl -s $API/api/v1/admin/clusters | jq -r '.items[] | select(.name=="kind-nullus-platform") | .id')
APP=nullus-api-demo

# 1. 파이프라인 생성 — 저장소 스캐폴딩·CI 변수·pull Secret·Argo CD Application 까지 만든다
curl -s -X POST $API/api/v1/cicd/pipelines \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\": \"$APP\",
    \"template_id\": \"nullus-sample-backend-v1\",
    \"app_type\": \"backend\",
    \"cluster_id\": \"$CLUSTER_ID\",
    \"stack_id\": \"$STACK_ID\",
    \"namespace\": \"$APP\",
    \"port\": 8080,
    \"replicas\": 1
  }" | tee /tmp/pipeline.json | jq '{id: .pipeline.id, execution_mode: .pipeline.execution_mode, repository_path, argo_application_created, warnings}'

PIPELINE_ID=$(jq -r '.pipeline.id' /tmp/pipeline.json)
# → pip_xxxxxxxxxxxx, execution_mode stack_integrated, repository_path nullus/nullus-api-demo
```

응답은 `pipeline`(생성된 레코드) · `repository_path` · `argo_application_created` · `warnings` 다. `warnings` 가 비어 있지 않으면 만들어졌지만 준비가 덜 된 것이다 — 내용대로 손본다.

```bash
# 2. 실행 기록 — CI 가 돌면 동기화되어 쌓인다 (GitLab 스택은 생성 직후 첫 파이프라인이 돈다)
curl -s "$API/api/v1/cicd/deployments?pipeline_id=$PIPELINE_ID" \
  | jq '.items[] | {id, status, version}'
# → dep_ci_<pipeline>_<n>, status: running → success
```

다시 돌리려면 저장소에 커밋을 올리거나 GitLab 에서 Run pipeline 을 누른다. `[skip ci]` 되커밋의 빈 파이프라인은 실행 기록에 남지 않는다.

### 방법 C: API — 스택 없는 파이프라인 (`emergency_direct`)

스택이 없는 클러스터에 플랫폼이 직접 배포하는 경로다. 저장소·이미지 준비는 사용자가 한다.

```bash
# 1. 생성 — 저장소 URL 을 직접 준다
PIPELINE_ID=$(curl -s -X POST $API/api/v1/cicd/pipelines \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\": \"$APP\",
    \"template_id\": \"web-backend-v1\",
    \"app_type\": \"backend\",
    \"cluster_id\": \"$CLUSTER_ID\",
    \"namespace\": \"$APP\",
    \"git_repo_url\": \"https://github.com/cloud-nullus/draft\",
    \"dockerfile_path\": \"Dockerfile\",
    \"docker_context\": \".\",
    \"port\": 8080,
    \"replicas\": 1
  }" | jq -r '.pipeline.id')

# 2. 배포 실행 — 202 와 deploymentId 를 돌려주고 적용은 비동기로 진행된다
curl -s -X POST "$API/api/v1/cicd/pipelines/$PIPELINE_ID/deploy" \
  -H 'Content-Type: application/json' \
  -d '{"version": "v0.1.0", "deployed_by": "devops@nullus.dev", "manifest_types": ["deployment", "service"]}'
# → {"deploymentId":"dep_xxxxxxxxxxxx"}

# 3. 배포 이력
curl -s "$API/api/v1/cicd/deployments?pipeline_id=$PIPELINE_ID" | jq '.items[] | {id, status, version}'
```

요청 필드 전체는 `internal/cicd/adapter/handler/pipeline_handler.go` 의 `createPipelineRequest` 와 `deployRequest` 다 — `name` · `template_id` · `app_type` · `cluster_id` · `stack_id` · `namespace` · `git_repo_url` · `dockerfile_path` · `docker_context` · `env_vars` · `provision_repository` · `port` · `replicas` · `registry_credentials` · `execution_mode`, 그리고 배포는 `version` · `deployed_by` · `manifest_types`.

---

## Step 5: 결과 확인

### UI

1. **CI/CD List** (`/cicd/list`): 파이프라인이 Active 로 보인다
2. 파이프라인의 **History**: 실행마다 상태(running · success · failed)와 단계가 보인다. SonarQube 가 있는 스택이면 SAST 판정도 붙는다

### API

```bash
curl -s $API/api/v1/cicd/pipelines | jq '.items[] | {id, name, execution_mode, status}'
curl -s "$API/api/v1/cicd/deployments?pipeline_id=$PIPELINE_ID" | jq '.items[] | {id, status}'
```

### 클러스터

```bash
# 스택 통합 파이프라인: Argo CD Application 과 앱 네임스페이스
kubectl --context kind-nullus-platform -n <스택 네임스페이스> get applications.argoproj.io
kubectl --context kind-nullus-platform -n $APP get deploy,pods
# 이미지 태그가 커밋 SHA 로 바뀌면 CI → CD 가 한 바퀴 돈 것이다. :bootstrap 이면 아직이다
kubectl --context kind-nullus-platform -n $APP get deploy $APP -o jsonpath='{.spec.template.spec.containers[0].image}'
```

> kind 노드가 `registry.<도메인>` 을 풀고 내부 CA 를 믿어야 앱 이미지를 받는다. `stack-up --wait` 가 자동으로 하고, 화면에서 설치한 스택은 `./scripts/runbook_local.sh wire` 로 잇는다. 안 돼 있으면 앱이 `ImagePullBackOff` 로 멈춘다.

---

## 정리

```bash
# 파이프라인 삭제 — 부수 리소스는 쿼리로 각각 켠다 (기본은 레코드만)
curl -s -X DELETE "$API/api/v1/cicd/pipelines/$PIPELINE_ID?cluster_resources=true&repository=true&images=true"

# kind 클러스터 삭제
./scripts/runbook_local.sh kind-down

# 전체 서비스 종료
./scripts/runbook_local.sh down
```

---

## 체크리스트

- [ ] `./scripts/runbook_local.sh up` — 로컬 인프라 기동
- [ ] `./scripts/runbook_local.sh kind-up` — kind 클러스터 생성 (2개)
- [ ] API `/health` → `db: connected`
- [ ] kind 클러스터 등록 → `/verify` → `status: connected`
- [ ] (스택 통합 경로) 스택이 `completed` 이고 `wire` 로 노드 배선됨
- [ ] CI/CD Template 페이지 → 템플릿 선택 → 위저드 완료
- [ ] 파이프라인 생성 응답의 `warnings` 가 비어 있음
- [ ] 실행 기록 `status: success`, 앱 이미지 태그가 커밋 SHA

---

## 문제 해결

### kind 클러스터 연결 실패 (Verify 502)

kubeconfig의 API endpoint가 폐기된 포트를 가리키는 경우. 클러스터를 재생성하고 kubeconfig를 다시 등록합니다.

```bash
./scripts/runbook_local.sh kind-down
./scripts/runbook_local.sh kind-up
# kubeconfig 재추출 후 클러스터 재등록
```

### API 서버 기동 실패 (ENCRYPTION_KEY)

`ENCRYPTION_KEY`가 32바이트여야 합니다. `.env.dev` 파일을 확인하세요.

```bash
cp .env.example .env.dev
# ENCRYPTION_KEY=nullus-dev-key-32bytes-padding!! 확인
```

### 앱이 `ImagePullBackOff`

kind 노드가 스택 레지스트리 주소를 못 풀거나 내부 CA 를 믿지 않는다. `./scripts/runbook_local.sh wire` 를 돌린다. 앱 네임스페이스에 pull Secret `nullus-registry` 가 없으면 파이프라인을 다시 만들거나 백업에서 복구한다 (백업이 앱 네임스페이스의 pull Secret 을 담는다).

### Docker 이미지 빌드 오류 (Go 버전)

`Dockerfile`의 Go 버전이 `go.mod`와 일치해야 합니다 (현재 Go 1.26).

```bash
# Dockerfile 첫 줄 확인
head -1 Dockerfile
# → FROM golang:1.26-alpine AS builder
```
