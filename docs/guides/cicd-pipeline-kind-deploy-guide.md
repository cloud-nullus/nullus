# Nullus Platform — CI/CD 파이프라인 kind 클러스터 배포 가이드

CI/CD 파이프라인 템플릿을 선택하고, kind 클러스터에 배포하는 전 과정을 단계별로 안내합니다.

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
3. Stack Template 페이지로 리다이렉트됩니다

### 2-2. CI/CD Template 페이지 이동

1. 좌측 사이드바에서 **CI/CD** → **CI/CD Template** 클릭
2. 또는 직접 URL 입력: `http://localhost:5173/cicd/templates`

3가지 템플릿이 표시됩니다:

| 템플릿 | 앱 타입 | 파이프라인 스테이지 |
|--------|---------|-------------------|
| Batch Job Pipeline | batch | Build → ImageBuild → CronJobDeploy |
| **Web Backend Pipeline** | backend | Build → Test → ImageBuild → Deploy |
| Web Frontend Pipeline | web | Build → Test → StaticBuild → Deploy |

### 2-3. 템플릿 선택

**Web Backend Pipeline** 카드의 `Use Base Template` 버튼을 클릭합니다.

→ **Pipeline Setup & Developer Deploy** 페이지(`/cicd/developer-deploy`)로 이동합니다.

---

## Step 3: 파이프라인 설정 (5단계 위저드)

Pipeline Setup 페이지는 5단계 위저드와 YAML 미리보기로 구성됩니다.

### 3-1. 앱 템플릿 선택

상단의 앱 템플릿 중 하나를 선택합니다:
- **Go Web API** (go1.24)
- React Vite App (node22)
- Spring Boot Service (java21)

### 3-2. Step 1 — 앱 이름 입력

- 입력: `nullus-api-demo`
- 규칙: 소문자, 숫자, 하이픈만 사용 가능
- 우측 YAML 미리보기에 Deployment + Service 매니페스트가 실시간 반영됩니다
- `다음` 버튼 클릭

### 3-3. Step 2 — Git Repository

- Repository URL: `https://github.com/cloud-nullus/draft`
- Branch: `main`

### 3-4. Step 3 — 클러스터 / 네임스페이스

- 클러스터: `kind-nullus-platform` (Step 1에서 등록한 클러스터)
- 네임스페이스: `default`

### 3-5. Step 4 — 리소스 설정

기본값 사용:
- CPU Request: 100m / Limit: 500m
- Memory Request: 128Mi / Limit: 512Mi
- Replicas: 2

### 3-6. Step 5 — 환경 변수

필요한 환경 변수를 추가합니다 (선택 사항).

---

## Step 4: 파이프라인 생성 및 배포

### 방법 A: UI에서 배포

1. CI/CD List 페이지(`/cicd/list`)에서 생성된 파이프라인의 **Deploy** 버튼 클릭
2. Pipeline Setup 페이지에서 배포 진행

### 방법 B: API로 배포 (권장 — 직접 확인용)

```bash
# 1. 파이프라인 생성
PIPELINE_ID=$(curl -s -X POST http://localhost:8090/api/v1/cicd/pipelines \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "nullus-api-demo",
    "template_id": "web-backend-v1",
    "app_type": "backend",
    "cluster_id": "<cluster-id>",
    "namespace": "default",
    "git_repo": "https://github.com/cloud-nullus/draft",
    "git_branch": "main",
    "image_registry": "harbor.nullus.io",
    "image_name": "nullus-api-demo"
  }' | jq -r '.id')

echo "Pipeline ID: $PIPELINE_ID"
# → pip_xxxxxxxxxx
```

```bash
# 2. 배포 실행
curl -s -X POST "http://localhost:8090/api/v1/cicd/pipelines/$PIPELINE_ID/deploy" \
  -H 'Content-Type: application/json' \
  -d '{"version": "v0.1.0", "image_tag": "latest"}' \
  | python3 -m json.tool
# → {"deploymentId":"dep_xxxxxxxxxx"}
```

```bash
# 3. 배포 이력 확인
curl -s http://localhost:8090/api/v1/cicd/deployments \
  | jq '.items[] | select(.pipeline_id == "'$PIPELINE_ID'")'
# → status: "success"
```

---

## Step 5: 배포 결과 확인

### UI에서 확인

1. **CI/CD List** (`/cicd/list`): 파이프라인 목록에 `nullus-api-demo`가 Active 상태로 표시
2. **CI/CD History** (`/cicd/history`): 배포 이력에 v0.1.0 — **Success** 표시

### API로 확인

```bash
# 파이프라인 목록
curl -s http://localhost:8090/api/v1/cicd/pipelines | jq '.items[0]'

# 배포 이력
curl -s http://localhost:8090/api/v1/cicd/deployments | jq '.items[0]'
```

### kind 클러스터 상태 확인 (Nullus 플랫폼 자체 배포)

```bash
# Nullus 플랫폼이 Helm으로 배포된 경우
kubectl get pods -n nullus-system --context kind-nullus-platform
# NAME                          READY   STATUS    RESTARTS   AGE
# nullus-api-xxx                1/1     Running   0          ...
# nullus-postgresql-0           1/1     Running   0          ...
# nullus-web-xxx                1/1     Running   0          ...

kubectl exec -n nullus-system deploy/nullus-api --context kind-nullus-platform \
  -- wget -qO- http://localhost:8080/health
# → {"db":"connected","status":"healthy","version":"0.1.0-alpha"}
```

---

## 정리

```bash
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
- [ ] CI/CD Template 페이지 → Web Backend Pipeline 선택
- [ ] Pipeline Setup 위저드 5단계 완료
- [ ] 파이프라인 생성 → 배포 → `status: success`
- [ ] CI/CD History 페이지에서 배포 이력 확인

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

### Docker 이미지 빌드 오류 (Go 버전)

`Dockerfile`의 Go 버전이 `go.mod`와 일치해야 합니다 (현재 Go 1.26).

```bash
# Dockerfile 첫 줄 확인
head -1 Dockerfile
# → FROM golang:1.26-alpine AS builder
```
