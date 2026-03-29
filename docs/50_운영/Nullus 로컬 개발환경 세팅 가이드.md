# Nullus 로컬 개발환경 세팅 가이드 (최신)

**프로젝트**: Nullus Platform (cloud-nullus/draft)  
**최신화일**: 2026-03-30  
**대상**: 개발팀 전원 (Backend / Frontend / QA)

---

## 1) 현재 기준 기술 스택

- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui
- **Backend**: Go 1.24+ (Echo v4) + PostgreSQL 18+
- **Infra(Local)**: Docker Compose + kind(선택) + Helm

> 이 문서는 `README.md`, `Makefile`, `scripts/runbook_local.sh` 동작을 기준으로 작성되었습니다.

---

## 2) 사전 요구사항

- Docker / Docker Compose
- Go 1.24+
- Node.js 22+
- kind (로컬 K8s E2E 시)

권장 확인:

```bash
go version
node -v
docker version
kind version
```

---

## 3) 가장 빠른 시작 (권장)

아래 한 줄이 로컬 개발 시작의 표준입니다.

```bash
./scripts/runbook_local.sh up
```

실행 내용:
1. Docker 인프라 기동 (PostgreSQL/Redis/MinIO/Keycloak)
2. DB 마이그레이션 적용
3. API 서버 기동 (`:8090`)
4. Frontend dev 서버 기동 (`:5173`)

상태 확인:

```bash
./scripts/runbook_local.sh status
./scripts/runbook_local.sh smoke
```

종료:

```bash
./scripts/runbook_local.sh down
```

---

## 4) 환경변수 설정

```bash
cp .env.example .env.dev
```

- `make run`은 `.env.dev`를 자동 로드합니다.
- `ENCRYPTION_KEY`는 **32바이트**여야 하며 kubeconfig 암복호화에 사용됩니다.

예시(기본값):

```bash
ENCRYPTION_KEY="nullus-dev-key-32bytes-padding!!"
```

---

## 5) 수동 실행(분리 실행) 시나리오

### 5.1 인프라 + 마이그레이션만

```bash
make dev-up
make migrate-up
```

### 5.2 API만 실행

```bash
make run
```

### 5.3 Frontend만 실행

```bash
make web-dev
```

---

## 6) 기본 포트 / 접속 정보

| 서비스 | 주소/포트 | 비고 |
|---|---|---|
| Frontend (Vite) | `http://localhost:5173` | dev 서버 |
| API | `http://localhost:8090` | Health: `/health` |
| PostgreSQL | `localhost:5433` | `nullus/nullus_dev` |
| Redis | `localhost:6380` | 로컬 캐시 |
| MinIO API | `localhost:9000` | 오브젝트 저장소 |
| MinIO Console | `http://localhost:9001` | `nullus/nullus_dev` |
| Keycloak | `http://localhost:8180` | `admin/admin` |

---

## 7) 로컬 kind 클러스터 (선택)

생성:

```bash
./scripts/runbook_local.sh kind-up
kind get clusters
```

현재 기본 구성:
- `nullus-platform`
- `nullus-develop`

노드 확인:

```bash
kubectl get nodes --context kind-nullus-platform
kubectl get nodes --context kind-nullus-develop
```

삭제:

```bash
./scripts/runbook_local.sh kind-down
```

---

## 8) 클러스터 등록/검증 (로컬 테스트 필수 흐름)

### 8.1 API로 등록

```bash
kind get kubeconfig --name nullus-platform > /tmp/nullus-platform.kubeconfig

curl -X POST http://localhost:8090/api/v1/admin/clusters \
  -H 'Content-Type: application/json' \
  -d "$(jq -n \
    --arg name 'kind-nullus-platform-fresh' \
    --arg type 'pipeline' \
    --arg org '11111111-1111-1111-1111-111111111111' \
    --arg kubeconfig "$(cat /tmp/nullus-platform.kubeconfig)" \
    '{name:$name,type:$type,org_id:$org,kubeconfig:$kubeconfig}')"
```

### 8.2 연결 확인

```bash
curl -X POST http://localhost:8090/api/v1/admin/clusters/<cluster-id>/verify
```

성공 예시:

```json
{"status":"connected","version":"v1.35.1"}
```

---

## 9) 로컬 테스트 실행

### 9.1 백엔드 테스트

```bash
go test ./... -count=1
```

### 9.2 프론트엔드 단위 테스트

```bash
cd web
npx vitest run
```

### 9.3 Playwright E2E

사전조건: API(`:8090`) + Frontend(`:5173`) 기동

```bash
cd web
npx playwright test --reporter=list
```

### 9.4 Smoke Test

```bash
./scripts/runbook_local.sh smoke
```

---

## 10) 트러블슈팅

### 10.1 포트 충돌

- API 8090 / Web 5173 점유 프로세스 확인 후 종료

```bash
lsof -tiTCP:8090 -sTCP:LISTEN
lsof -tiTCP:5173 -sTCP:LISTEN
```

### 10.2 DB 연결 실패

- 인프라 선기동 여부 확인 (`make dev-up` 또는 `runbook_local.sh up`)
- 필요 시 볼륨 초기화 후 재기동

```bash
make dev-clean
make dev
```

### 10.3 kubeconfig 암복호화 에러

- `.env.dev`의 `ENCRYPTION_KEY` 32바이트 여부 확인
- 키 변경 후 기존 저장 kubeconfig는 재등록 필요

### 10.4 kind endpoint/상태 반영 이슈

- `runbook_local.sh`는 `scripts/kind-cluster.yaml`에 정의된 클러스터를 기준으로 endpoint 상태를 반영합니다.
- 클러스터 생성/삭제 이후 `runbook_local.sh status`로 최종 확인하세요.

---

## 11) 신규 팀원 온보딩 최소 체크리스트

1. 저장소 클론 및 브랜치 전략 확인 (`CONTRIBUTING.md`)
2. `.env.dev` 생성 (`cp .env.example .env.dev`)
3. `./scripts/runbook_local.sh up` 성공 확인
4. `curl http://localhost:8090/health` 확인
5. `./scripts/runbook_local.sh smoke` 통과 확인
6. (선택) `./scripts/runbook_local.sh kind-up` + 클러스터 verify

---

## 12) 관련 문서

- `README.md` (Quick Start / API / 테스트 명령)
- `CONTRIBUTING.md` (개발 워크플로우 / 브랜치 / 테스트 규칙)
- `docs/50_운영/Nullus_로컬_테스트_가이드.md`
- `docs/50_운영/Nullus_로컬_테스트_가이드_Windows.md`
- `docs/50_운영/Nullus_개발자_온보딩_가이드.md`
- `docs/guides/kind-e2e-testing-guide.md`
