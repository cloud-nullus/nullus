# Nullus 로컬 테스트 가이드

## 1. 사전 요구사항

| 도구 | 버전 | 비고 |
|------|------|------|
| Docker + Docker Compose | 최신 | 인프라 기동 |
| Go | 1.24+ | `/opt/homebrew/bin/go` |
| Node.js | 22+ | npm 포함 |
| golang-migrate CLI | 최신 | `make dev` 시 자동 설치 |
| Playwright Chromium | 최신 | `npx playwright install chromium` |

---

## 2. 인프라 기동

```bash
make dev
```

`make dev`는 다음을 순서대로 실행합니다.

1. Docker Compose로 PostgreSQL, MinIO, Redis 컨테이너 기동
2. `golang-migrate`로 DB 마이그레이션 실행 (마이그레이션 파일 6개)

기동 완료 후 접근 가능한 서비스:

| 서비스 | 주소 |
|--------|------|
| PostgreSQL 17 | `localhost:5433` |
| MinIO API | `localhost:9000` |
| MinIO 콘솔 | `localhost:9001` |
| Redis 7 | `localhost:6380` |

> **참고**: 호스트 포트가 기본값(5432, 6379)과 다르게 설정된 이유는 기존 로컬 서비스와의 충돌을 피하기 위해서입니다.

---

## 3. 백엔드 API 서버 실행

```bash
make run
```

`make run`은 빌드 후 환경 변수를 자동으로 주입하여 서버를 실행합니다. 기본 포트가 이미 사용 중이라면 포트를 변경할 수 있습니다.

```bash
NULLUS_SERVER_PORT=9090 make run
```

서버 기동 후 확인:

| 항목 | 값 |
|------|-----|
| API 서버 | `http://localhost:8080` |
| Health 엔드포인트 | `GET /health` |
| pprof (개발 모드) | `http://localhost:8080/debug/pprof/` |

Health 응답 예시:

```json
{"status":"healthy","db":"connected","version":"0.1.0-alpha"}
```

---

## 4. 프론트엔드 개발 서버 실행

```bash
make web-dev
```

- URL: `http://localhost:5173`
- Hot Module Replacement(HMR) 지원
- API 프록시 미설정 — mock 데이터로 fallback 동작

---

## 5. API 엔드포인트 테스트 가이드

아래 예시에서 포트는 `9090`을 사용합니다. 기본 포트(`8080`)로 실행한 경우 해당 포트로 변경하세요.

### 5.1 Health Check

```bash
curl http://localhost:9090/health
```

### 5.2 Organization 관리

```bash
# 조직 생성
curl -X POST http://localhost:9090/api/v1/orgs \
  -H "Content-Type: application/json" \
  -d '{"name":"My Team","slug":"my-team","domain":"myteam.io"}'

# 조직 조회
curl http://localhost:9090/api/v1/orgs/{orgId}

# 조직 수정
curl -X PUT http://localhost:9090/api/v1/orgs/{orgId} \
  -H "Content-Type: application/json" \
  -d '{"name":"My Team Updated","domain":"updated.io"}'
```

### 5.3 클러스터 관리

클러스터 타입은 `pipeline` 또는 `target`입니다.

```bash
# 클러스터 등록
curl -X POST http://localhost:9090/api/v1/clusters \
  -H "Content-Type: application/json" \
  -d '{"name":"prod-k8s","type":"pipeline","endpoint":"https://k8s.example.com","org_id":"{orgId}"}'

# 클러스터 목록 (조직 필터)
curl "http://localhost:9090/api/v1/clusters?org_id={orgId}"

# 클러스터 상세 조회
curl http://localhost:9090/api/v1/clusters/{clusterId}

# 클러스터 연결 검증
curl -X POST http://localhost:9090/api/v1/clusters/{clusterId}/verify
```

### 5.4 스택 템플릿

```bash
# 템플릿 목록 (3개 Golden Path)
curl http://localhost:9090/api/v1/templates

# 특정 템플릿 조회
curl http://localhost:9090/api/v1/templates/gitlab-allinone-v1
```

### 5.5 스택 배포

```bash
# 스택 생성
curl -X POST http://localhost:9090/api/v1/stacks \
  -H "Content-Type: application/json" \
  -d '{"name":"my-stack","cluster_id":"{clusterId}","golden_path_id":"gitlab-allinone-v1","config":{}}'

# 스택 배포 (202 Accepted)
curl -X POST http://localhost:9090/api/v1/stacks/{stackId}/deploy

# 배포 상태 확인
curl http://localhost:9090/api/v1/stacks/{stackId}/status
```

### 5.6 호환성 매트릭스

```bash
# 매트릭스 전체 조회
curl http://localhost:9090/api/v1/compatibility/matrix

# 도구 조합 호환성 검증
curl -X POST http://localhost:9090/api/v1/compatibility/validate \
  -H "Content-Type: application/json" \
  -d '{"tools":{"source_repository":"GitLab CE","ci_platform":"GitLab CI"}}'
```

### 5.7 CI/CD 파이프라인

```bash
# 템플릿 목록 (3개)
curl http://localhost:9090/api/v1/cicd/templates

# 파이프라인 생성
curl -X POST http://localhost:9090/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{"name":"my-pipeline","template_id":"web-backend-v1","cluster_id":"{clusterId}","namespace":"default","app_type":"backend","git_repo_url":"https://gitlab.example.com/my-app"}'

# 파이프라인 배포 (201 Created)
curl -X POST http://localhost:9090/api/v1/pipelines/{pipelineId}/deploy \
  -H "Content-Type: application/json" \
  -d '{"version":"v1.0.0","deployed_by":"devops@nullus.dev"}'

# 배포 이력 조회
curl http://localhost:9090/api/v1/pipelines/{pipelineId}/deployments
```

### 5.8 모니터링 대시보드

```bash
curl http://localhost:9090/api/v1/monitoring/dashboard
```

### 5.9 알림 규칙

```bash
# 알림 규칙 목록
curl http://localhost:9090/api/v1/alerts/rules

# 알림 규칙 생성
curl -X POST http://localhost:9090/api/v1/alerts/rules \
  -H "Content-Type: application/json" \
  -d '{"name":"High CPU Alert","condition":"cpu_usage > threshold","threshold":80.0,"channel":"slack","enabled":true}'

# 알림 이력
curl http://localhost:9090/api/v1/alerts/history
```

---

## 6. 프론트엔드 UI 테스트 가이드

### 6.1 로그인

URL: `http://localhost:5173/login`

테스트 계정:

| 이메일 | 비밀번호 | 역할 | 접근 범위 |
|--------|----------|------|-----------|
| `admin@nullus.dev` | `admin123` | Admin | 조직·사용자·클러스터 관리 전체 |
| `devops@nullus.dev` | `devops123` | DevOps Engineer | 전체 메뉴 |
| `developer@nullus.dev` | `developer123` | Developer | CI/CD + 관측성 |

### 6.2 역할별 화면 확인

각 계정으로 로그인 후 다음 워크플로우를 확인합니다.

- **Admin**: 조직 → 사용자 → 클러스터 관리 페이지 접근 여부
- **DevOps**: 스택 템플릿 선택 → 설치 설정 → 배포 전체 워크플로우
- **Developer**: CI/CD 템플릿 선택 → 파이프라인 생성 → 앱 배포 5단계 위자드

### 6.3 주요 페이지 체크리스트

| 페이지 | URL | 확인 사항 |
|--------|-----|-----------|
| 홈 | `/` | 역할별 CTA 버튼 표시 |
| 스택 템플릿 | `/stack/templates` | 3개 Golden Path 카드 |
| 스택 설치 | `/stack/install` | 6탭 (Artifacts ~ YAML View) |
| 스택 목록 | `/stack/list` | 테이블 렌더링 |
| 스택 이력 | `/stack/history` | 버전 테이블 |
| 호환성 | `/stack/versions` | 매트릭스 표 |
| CI/CD 템플릿 | `/cicd/templates` | 3개 파이프라인 카드 |
| CI/CD 목록 | `/cicd/list` | 파이프라인 테이블 |
| CI/CD 이력 | `/cicd/history` | 배포 이력 |
| 모니터링 | `/observability/monitoring` | KPI 카드 4개 |
| 알림 규칙 | `/observability/alert-rules` | 규칙 테이블 |
| 알림 이력 | `/observability/alert-history` | 이력 테이블 |
| 조직 관리 | `/admin/organization` | 정보 폼 + 멤버 목록 |
| 사용자 관리 | `/admin/users` | 사용자 테이블 |
| 클러스터 관리 | `/admin/clusters` | 리스트 + 상세 |
| Developer 배포 | `/cicd/developer-deploy` | 5단계 위자드 |

### 6.4 다크/라이트 테마

1. 헤더 우측 테마 토글 버튼 클릭
2. 페이지 새로고침 후 테마 유지 여부 확인

### 6.5 다국어 (en/ko)

1. 헤더 우측 언어 드롭다운에서 `en` ↔ `ko` 전환
2. 사이드바 메뉴명이 변경되는지 확인

### 6.6 사이드바 접기/펼치기

- 사이드바 상단 메뉴 아이콘 클릭
- 접힘: 64px / 펼침: 240px

---

## 7. 자동화 테스트 실행

### 7.1 Go 단위 + 통합 테스트

인프라 없이 인메모리 저장소로 실행됩니다.

```bash
make test
```

특정 패키지만 실행:

```bash
go test github.com/cloud-nullus/draft/internal/stack/domain/... -v
```

### 7.2 Go DB 연동 테스트

`make dev`로 인프라가 실행 중이어야 합니다. DB에 접근할 수 없으면 테스트가 자동으로 skip됩니다.

```bash
go test github.com/cloud-nullus/draft/e2e -v -run TestDBIntegration
```

개별 테스트:

```bash
go test github.com/cloud-nullus/draft/e2e -v -run TestDBIntegration_Organizations
go test github.com/cloud-nullus/draft/e2e -v -run TestDBIntegration_Clusters
go test github.com/cloud-nullus/draft/e2e -v -run TestDBIntegration_Stacks
go test github.com/cloud-nullus/draft/e2e -v -run TestDBIntegration_Pipelines
go test github.com/cloud-nullus/draft/e2e -v -run TestDBIntegration_Alerts
```

### 7.3 Go E2E + UAT

인메모리 서버로 실행되므로 인프라 불필요합니다.

```bash
go test github.com/cloud-nullus/draft/e2e -v
```

포함된 시나리오:

| 테스트 함수 | 시나리오 |
|-------------|----------|
| `TestScenario1_OrgAndCluster` | 조직 생성 → 수정 → 클러스터 등록 → 검증 |
| `TestScenario2_StackDeployFlow` | 템플릿 조회 → 스택 생성 → 배포 → 상태 확인 |
| `TestScenario3_CompatibilityMatrix` | 매트릭스 조회 → 호환성 검증 |
| `TestScenario4_CICDPipelineFlow` | 파이프라인 생성 → 배포 → 이력 조회 |
| `TestScenario5_MonitoringAndAlerts` | 대시보드 조회 → 알림 규칙 생성 → 이력 조회 |
| `TestScenario6_HealthCheck` | Health 엔드포인트 확인 |

### 7.4 React 단위/통합 테스트

```bash
make web-test
```

내부적으로 `npx vitest run`을 실행합니다.

### 7.5 Playwright E2E

프론트엔드(`http://localhost:5173`)와 API(`http://localhost:8080`)가 실행 중이어야 합니다.

```bash
cd web && npx playwright test --reporter=list
```

- 총 41개 테스트, 7개 spec 파일 기준으로 실행됩니다.
- spec 파일: `navigation`, `sidebar`, `stack-workflow`, `theme-i18n`, `uat-admin`, `uat-devops`, `uat-developer`

Kind 클러스터 대상 시나리오는 `docs/guides/kind-e2e-testing-guide.md`를 함께 참고하세요.

### 7.6 API Smoke Test

```bash
./scripts/runbook_local.sh smoke
```

- 로컬 API 기준 13개 엔드포인트를 스모크 검증합니다.

### 7.7 Go 벤치마크

```bash
go test github.com/cloud-nullus/draft/internal/stack/domain/ -bench=. -benchmem
```

### 7.8 테스트 커버리지

```bash
# Go 커버리지 (coverage.html 생성)
make test-cover

# 프론트엔드 커버리지
cd /Users/qmin/lifework/cloudbro/draft/web && npm run test:coverage
```

---

## 8. 트러블슈팅

| 문제 | 원인 | 해결 |
|------|------|------|
| 포트 5432 또는 6379 충돌 | 로컬에 PostgreSQL/Redis 실행 중 | `docker-compose.dev.yaml`에서 5433/6380으로 이미 매핑됨. 별도 조치 불필요 |
| API 서버 8080 포트 충돌 | 다른 프로세스가 8080 사용 중 | `NULLUS_SERVER_PORT=9090 make run` |
| `go: command not found` | Go가 PATH에 없음 | `export PATH="/opt/homebrew/bin:$PATH"` |
| DB 마이그레이션 실패 | 볼륨 데이터 충돌 | `make dev-clean && make dev` (볼륨 초기화) |
| Playwright 브라우저 없음 | Chromium 미설치 | `npx playwright install chromium` |
| `golang-migrate` 없음 | PATH에 Go bin 미포함 | `make dev` 실행 시 자동 설치됨. 수동 설치: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |

---

## 9. 인프라 정리

```bash
# 컨테이너 중지 (볼륨 유지)
make dev-down

# 컨테이너 + 볼륨 삭제 (데이터 초기화)
make dev-clean
```

> `make dev-clean` 실행 시 PostgreSQL 데이터가 모두 삭제됩니다. 이후 `make dev`를 다시 실행하면 마이그레이션이 처음부터 적용됩니다.
