# Nullus 로컬 테스트 가이드

> **Windows 사용자**: [Windows 전용 가이드](./Nullus_로컬_테스트_가이드_Windows.md)를 참고하세요. WSL2 + Docker Desktop 환경이 필요합니다.

## 1. 사전 요구사항

| 도구 | 버전 | 비고 |
|------|------|------|
| Docker + Docker Compose | 최신 | 인프라 기동 |
| Go | 1.24+ | 백엔드 빌드/테스트 |
| Node.js | 22+ | npm 포함 |
| golang-migrate CLI | 최신 | `runbook_local.sh up` 시 자동 설치 |
| Playwright Chromium | 최신 | `npx playwright install chromium` |
| kind | 0.20+ | K8s 로컬 테스트 (선택) |

---

## 2. 인프라 기동

```bash
./scripts/runbook_local.sh up
```

이 명령은 다음을 순서대로 실행합니다:

1. Docker Compose로 PostgreSQL, Redis, MinIO, Keycloak 컨테이너 기동
2. `golang-migrate`로 DB 마이그레이션 실행 (18개 파일)
3. Go API 서버 빌드 + 실행 (`:8090`, `ENCRYPTION_KEY` 자동 설정)
4. React 프론트엔드 개발 서버 실행 (`:5173`)

기동 완료 후 접근 가능한 서비스:

| 서비스 | 주소 | 계정 |
|--------|------|------|
| API 서버 | `http://localhost:8090` | - |
| 프론트엔드 | `http://localhost:5173` | 아래 테스트 계정 참조 |
| PostgreSQL 17 | `localhost:5433` | nullus / nullus_dev |
| Keycloak | `http://localhost:8180` | admin / admin |
| MinIO 콘솔 | `http://localhost:9001` | nullus / nullus_dev |
| Redis 7 | `localhost:6380` | - |

kind 클러스터도 함께 시작하려면:

```bash
./scripts/runbook_local.sh up --kind
```

수동으로 각 서비스를 개별 실행하려면:

```bash
make dev                   # Docker 인프라만 기동 + 마이그레이션
make run                   # API 서버만 실행
make web-dev               # 프론트엔드만 실행
./scripts/runbook_local.sh kind-up   # kind 클러스터만 생성
```

---

## 3. 테스트 계정

### 프론트엔드 (Mock Auth, development 모드)

| 이메일 | 비밀번호 | 역할 | 홈 페이지 | 접근 범위 |
|--------|----------|------|-----------|-----------|
| `admin@nullus.dev` | `admin123` | Admin | `/admin/organization` | 전체 |
| `devops@nullus.dev` | `devops123` | DevOps | `/stack/templates` | 스택 + CI/CD + 모니터링 |
| `developer@nullus.dev` | `developer123` | Developer | `/cicd/developer-deploy` | CI/CD + 모니터링(읽기) |

### Keycloak OIDC (production 모드)

| 이메일 | 비밀번호 | 역할 |
|--------|----------|------|
| `admin@nullus.io` | `nullus123!` | admin |
| `devops@nullus.io` | `nullus123!` | devops |
| `dev@nullus.io` | `nullus123!` | developer |

---

## 4. API 엔드포인트 테스트

API 서버: `http://localhost:8090`

### 4.1 Health Check

```bash
curl http://localhost:8090/health
# {"status":"healthy","db":"connected","version":"0.1.0-alpha"}
```

### 4.2 Admin — Organization

```bash
# 현재 Organization 조회 (single-org 모드)
curl http://localhost:8090/api/v1/admin/organization

# Organization 생성
curl -X POST http://localhost:8090/api/v1/admin/orgs \
  -H "Content-Type: application/json" \
  -d '{"name":"My Team","slug":"my-team","domain":"myteam.io"}'

# Organization 수정
curl -X PATCH http://localhost:8090/api/v1/admin/organization \
  -H "Content-Type: application/json" \
  -d '{"name":"My Team Updated"}'
```

### 4.3 Admin — Cluster

```bash
# 클러스터 등록 (kubeconfig은 base64 인코딩)
curl -X POST http://localhost:8090/api/v1/admin/clusters \
  -H "Content-Type: application/json" \
  -d '{"name":"prod-k8s","type":"target","endpoint":"https://k8s.example.com","org_id":"{orgId}","kubeconfig":"<base64-encoded-kubeconfig>"}'

# 클러스터 목록
curl http://localhost:8090/api/v1/admin/clusters

# 클러스터 연결 검증 (실제 K8s API 서버에 접속)
curl -X POST http://localhost:8090/api/v1/admin/clusters/{clusterId}/verify
# {"status":"connected","version":"v1.35.0"}

# 클러스터 수정
curl -X PATCH http://localhost:8090/api/v1/admin/clusters/{clusterId} \
  -H "Content-Type: application/json" \
  -d '{"name":"prod-k8s-updated"}'

# 클러스터 삭제
curl -X DELETE http://localhost:8090/api/v1/admin/clusters/{clusterId}

# 클러스터 네임스페이스 목록 (K8s API 실시간 조회)
curl http://localhost:8090/api/v1/admin/clusters/{clusterId}/namespaces
# {"items":[{"name":"default"},{"name":"production"}]}
```

kind 클러스터 등록 예시:

```bash
KUBECONFIG_B64=$(kind get kubeconfig --name nullus-test | base64 | tr -d '\n')
ORG_ID=$(curl -s http://localhost:8090/api/v1/admin/organization | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

curl -X POST http://localhost:8090/api/v1/admin/clusters \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"kind-nullus-test\",\"type\":\"target\",\"endpoint\":\"https://127.0.0.1:PORT\",\"org_id\":\"$ORG_ID\",\"kubeconfig\":\"$KUBECONFIG_B64\"}"
```

> **endpoint / kubeconfig 는 nullus-api 가 닿을 수 있는 주소여야 한다.**
> - **로컬 dev (api 가 호스트에서 실행)**: 위처럼 `https://127.0.0.1:PORT` + 일반 `kind get kubeconfig`.
> - **클러스터 내부 배포 (airgap 등, api 가 파드로 실행)**: 파드에서 `127.0.0.1` 은 자기 자신이라 닿지 않는다.
>   `--internal` kubeconfig 를 쓰고 endpoint 도 control-plane 내부 주소로 등록한다.
>   ```bash
>   CLUSTER=nullus-airgap
>   KUBECONFIG_B64=$(kind get kubeconfig --name "$CLUSTER" --internal | base64 | tr -d '\n')
>   ENDPOINT="https://${CLUSTER}-control-plane:6443"   # 노드 컨테이너 hostname
>   # org_id 는 로그인 사용자의 org (예: 11111111-... Nullus DevOps Team)
>   ```
>
> **사전 조건 — `ENCRYPTION_KEY` 는 정확히 32바이트여야 한다.** 미설정/길이 불일치 시
> 등록이 `500 "ENCRYPTION_KEY must be 32 bytes"` 로 실패한다. airgap 차트는
> `secrets.encryptionKey`(32바이트) 를 `ENCRYPTION_KEY` 환경변수로 주입한다.

### 4.4 Admin — Members

```bash
# 멤버 목록
curl http://localhost:8090/api/v1/admin/organizations/{orgId}/members

# 멤버 초대
curl -X POST http://localhost:8090/api/v1/admin/organizations/{orgId}/members \
  -H "Content-Type: application/json" \
  -d '{"email":"new@nullus.dev","role":"developer"}'

# 멤버 역할 변경
curl -X PATCH http://localhost:8090/api/v1/admin/organizations/{orgId}/members/{memberId} \
  -H "Content-Type: application/json" \
  -d '{"role":"devops"}'

# 기존 사용자 검색 (다른 조직에서 활동 중인 사용자)
curl http://localhost:8090/api/v1/admin/users/search?email=devops@nullus.dev
# {"found":true,"user":{"id":"...","name":"DevOps Engineer","email":"devops@nullus.dev","is_active":true}}
```

### 4.5 Admin — 기타

```bash
# Known Issues
curl http://localhost:8090/api/v1/admin/known-issues

# 감사 로그
curl http://localhost:8090/api/v1/admin/audit-logs

# 알림 설정
curl http://localhost:8090/api/v1/admin/notifications/configs

# 알림 이력
curl http://localhost:8090/api/v1/admin/notifications/history
```

### 4.6 Admin — OpenBao Token Sources

```bash
# token source 목록 조회 (org header 필요)
curl http://localhost:8090/api/v1/admin/token-sources \
  -H "X-Org-ID: 11111111-1111-1111-1111-111111111111"

# rotate
curl -X POST http://localhost:8090/api/v1/admin/token-sources/{tokenSourceId}/rotate \
  -H "Content-Type: application/json" \
  -d '{"reason":"manual trigger"}'

# approve
curl -X POST http://localhost:8090/api/v1/admin/token-sources/{tokenSourceId}/approve \
  -H "Content-Type: application/json" \
  -d '{"reason":"manual approve"}'

# re-auth -> reveal
STEP_UP_TOKEN=$(curl -sS -X POST http://localhost:8090/api/v1/admin/token-sources/{tokenSourceId}/re-auth \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user-1" \
  -d '{"reason":"security"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['step_up_token'])")

curl -X POST http://localhost:8090/api/v1/admin/token-sources/{tokenSourceId}/reveal \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user-1" \
  -d "{\"step_up_token\":\"$STEP_UP_TOKEN\"}"
```

토큰 소스 테스트 데이터는 아래 런북으로 초기화할 수 있습니다.

```bash
./scripts/runbook_local.sh down --kind --volumes
OPENBAO_ADDR=http://127.0.0.1:8200 OPENBAO_TOKEN=root ./scripts/runbook_local.sh up --kind --seed
```

### 4.6 Stack

```bash
# Golden Path 템플릿 (3개)
curl http://localhost:8090/api/v1/stacks/templates

# 스택 목록
curl http://localhost:8090/api/v1/stacks

# 호환성 매트릭스
curl http://localhost:8090/api/v1/stacks/compatibility

# 스택 생성 (namespace 지정 가능)
curl -X POST http://localhost:8090/api/v1/stacks \
  -H "Content-Type: application/json" \
  -H "X-Org-ID: {orgId}" \
  -d '{"name":"my-stack","cluster_id":"{clusterId}","namespace":"my-namespace","golden_path_id":"tpl_minimal","config":{}}'

# 스택 배포
curl -X POST http://localhost:8090/api/v1/stacks/{stackId}/deploy

# 배포 상태 확인
curl http://localhost:8090/api/v1/stacks/{stackId}/status

# 스택 삭제 (Helm uninstall 포함)
curl -X DELETE http://localhost:8090/api/v1/stacks/{stackId}
```

### 4.7 CI/CD

```bash
# CI/CD 파이프라인 템플릿
curl http://localhost:8090/api/v1/cicd/templates

# 앱 템플릿 (Developer Self-Service)
curl http://localhost:8090/api/v1/cicd/app-templates

# 파이프라인 생성
curl -X POST http://localhost:8090/api/v1/cicd/pipelines \
  -H "Content-Type: application/json" \
  -d '{"name":"my-pipeline","template_id":"web-backend-v1","cluster_id":"{clusterId}","namespace":"default","app_type":"backend","git_repo_url":"https://gitlab.example.com/my-app"}'

# 파이프라인 배포
curl -X POST http://localhost:8090/api/v1/cicd/pipelines/{pipelineId}/deploy \
  -H "Content-Type: application/json" \
  -d '{"version":"v1.0.0","deployed_by":"devops@nullus.dev"}'

# 배포 이력
curl http://localhost:8090/api/v1/cicd/deployments
```

### 4.8 Observability

```bash
# 모니터링 대시보드
curl http://localhost:8090/api/v1/observability/dashboard

# 알림 규칙 목록
curl http://localhost:8090/api/v1/observability/alert-rules

# 알림 규칙 생성
curl -X POST http://localhost:8090/api/v1/observability/alert-rules \
  -H "Content-Type: application/json" \
  -d '{"name":"High CPU Alert","condition":"cpu_usage > threshold","threshold":80.0,"channel":"slack","enabled":true}'

# 알림 규칙 수정
curl -X PATCH http://localhost:8090/api/v1/observability/alert-rules/{ruleId} \
  -H "Content-Type: application/json" \
  -d '{"threshold":90.0}'

# 알림 규칙 삭제
curl -X DELETE http://localhost:8090/api/v1/observability/alert-rules/{ruleId}

# 알림 이력
curl http://localhost:8090/api/v1/observability/alert-history
```

---

## 5. 프론트엔드 UI 테스트

### 5.1 로그인

URL: `http://localhost:5173/login`

위 테스트 계정으로 로그인 후 역할별 홈 페이지로 리다이렉트되는지 확인합니다.

### 5.2 역할별 워크플로우 확인

**Admin** (admin@nullus.dev / admin123):
1. `/admin/organization` — 조직 정보 폼 필드 확인
2. `/admin/users` — 사용자 목록 테이블
3. `/admin/clusters` — 클러스터 목록 + "Verify Connection" 버튼
4. `/admin/known-issues` — Known Issues 목록

**DevOps** (devops@nullus.dev / devops123):
1. `/stack/templates` — 3개 Golden Path 카드
2. `/stack/install` — 5개 탭 (Artifacts ~ Resources)
3. `/stack/list` — 스택 목록 테이블
4. `/observability/monitoring` — KPI 카드 4개 + 차트

**Developer** (developer@nullus.dev / developer123):
1. `/cicd/developer-deploy` — 앱 배포 위자드
2. `/observability/monitoring` — 대시보드 (읽기 전용)
3. 사이드바에서 DevSecOps Stack, Admin 메뉴 숨김 확인

### 5.3 주요 페이지 체크리스트

| 페이지 | URL | 확인 사항 |
|--------|-----|-----------|
| 홈 | `/` | 역할별 CTA 버튼 표시 |
| 스택 템플릿 | `/stack/templates` | 3개 Golden Path 카드 |
| 스택 설치 | `/stack/install` | 5탭 (Artifacts ~ Resources) |
| 스택 목록 | `/stack/list` | 테이블 렌더링 |
| 스택 이력 | `/stack/history` | 버전 테이블 |
| 호환성 | `/stack/versions` | 매트릭스 표 |
| CI/CD 템플릿 | `/cicd/templates` | 3개 파이프라인 카드 |
| CI/CD 목록 | `/cicd/list` | 파이프라인 테이블 |
| 모니터링 | `/observability/monitoring` | KPI 카드 4개 |
| 알림 규칙 | `/observability/alert-rules` | 규칙 테이블 + CRUD |
| 조직 관리 | `/admin/organization` | 정보 폼 + 멤버 목록 |
| 사용자 관리 | `/admin/users` | 사용자 테이블 |
| 클러스터 관리 | `/admin/clusters` | 리스트 + Verify 버튼 |
| Known Issues | `/admin/known-issues` | 이슈 목록 |
| Developer 배포 | `/cicd/developer-deploy` | 앱 배포 위자드 |

### 5.4 다크/라이트 테마

1. 헤더 우측 테마 토글 버튼 클릭
2. 페이지 새로고침 후 테마 유지 여부 확인

### 5.5 다국어 (en/ko)

1. 헤더 우측 언어 드롭다운에서 `en` / `ko` 전환
2. 사이드바 메뉴명이 변경되는지 확인

---

## 6. 자동화 테스트

### 6.1 Go 단위/통합 테스트

```bash
go test ./... -count=1
```

인프라 없이 인메모리 저장소로 실행됩니다.

### 6.2 React 단위 테스트 (Vitest)

```bash
cd web && npx vitest run
```

14개 파일, 125개 테스트 기준.

### 6.3 Playwright E2E

프론트엔드(`:5173`)와 API(`:8090`)가 실행 중이어야 합니다.

```bash
cd web && npx playwright test --reporter=list
```

41개 테스트, 7개 spec 파일:
- `navigation.spec.ts` — 페이지 이동 11개
- `sidebar.spec.ts` — 사이드바 접기/펼치기 4개
- `stack-workflow.spec.ts` — 스택 설치 워크플로우 5개
- `theme-i18n.spec.ts` — 테마/언어 전환 5개
- `uat-admin.spec.ts` — Admin 역할 시나리오 5개
- `uat-devops.spec.ts` — DevOps 역할 시나리오 7개
- `uat-developer.spec.ts` — Developer 역할 시나리오 5개

### 6.4 API Smoke Test

```bash
./scripts/runbook_local.sh smoke
```

14개 API 엔드포인트를 자동 검증합니다.

### 6.5 kind 클러스터 E2E

```bash
./scripts/runbook_local.sh kind-up
```

상세 시나리오: [kind E2E 테스트 가이드](../guides/kind-e2e-testing-guide.md)

---

## 7. 트러블슈팅

| 문제 | 원인 | 해결 |
|------|------|------|
| 포트 5433/6380 충돌 | 로컬 PostgreSQL/Redis 실행 중 | docker-compose에서 이미 비표준 포트 사용. 충돌 시 기존 서비스 중지 |
| API 서버 포트 충돌 | 다른 프로세스가 8090 사용 | `lsof -tiTCP:8090` 로 확인 후 `kill` |
| kubeconfig 암호화 실패 | ENCRYPTION_KEY 미설정 또는 길이 부정확 | `runbook_local.sh`는 기본값 자동 설정. 수동 실행 시 `export ENCRYPTION_KEY="nullus-dev-key-32bytes-padding!!"` (32바이트) |
| Cluster Verify 실패 | kubeconfig이 DB에 없음 | 클러스터 등록 시 `ENCRYPTION_KEY`가 32바이트인지 확인 |
| DB 마이그레이션 실패 | 볼륨 데이터 충돌 | `make dev-clean && make dev` (볼륨 초기화) |
| Playwright 브라우저 없음 | Chromium 미설치 | `npx playwright install chromium` |
| kind 클러스터 안 됨 | kind 미설치 | `brew install kind` |
| ENCRYPTION_KEY 불일치 | 서버 재시작 시 다른 키 사용 | `.env.dev`에서 키 확인. `make run`은 자동 로드 |

---

## 8. 인프라 정리

```bash
# 전체 정리 (API, 프론트엔드, Docker, kind 클러스터)
./scripts/runbook_local.sh down

# Docker 볼륨까지 삭제 (데이터 초기화)
make dev-clean
```
