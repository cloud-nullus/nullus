# Nullus Platform

Kubernetes 기반 DevSecOps 자동화 오픈소스 플랫폼

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
![Go 1.26+](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)
![Kubernetes 1.34+](https://img.shields.io/badge/Kubernetes-1.34%2B-326CE5?logo=kubernetes&logoColor=white)
![Helm v3](https://img.shields.io/badge/Helm-v3-0F1689?logo=helm&logoColor=white)

**[📖 문서 사이트](https://cloud-nullus.github.io/nullus/)** — 기능별 설명과 사용법, 로컬 E2E 가이드.

**[🚀 라이브 데모](https://cloud-nullus.github.io)** — 설치 없이 화면을 둘러보세요.

**[▶️ 시연 영상](https://www.youtube.com/watch?v=VrTqBKInlNw)** — 클러스터 연결부터 배포까지 여섯 단계를 한 화면에서.

## 개요

Nullus는 DevOps 엔지니어가 검증된 CI/CD 베스트 프랙티스 조합을 선택하고, 웹 UI에서 노코드로 설정한 후 한 번의 버튼 클릭으로 Kubernetes 클러스터에 전체 DevSecOps 스택을 자동 설치할 수 있도록 하는 오픈소스 플랫폼입니다.

Nullus는 "클러스터 준비 -> 스택 선택/설치 -> 배포 관측"의 흐름을 하나의 일관된 사용자 경험으로 제공하는 데 집중합니다. 플랫폼 팀은 표준화된 템플릿과 버전 호환성 정책을 운영하고, 개발 팀은 반복적인 인프라 구성 작업 없이 검증된 파이프라인을 빠르게 사용할 수 있습니다.

사내에서 여러 프로젝트가 병렬로 운영되면 프로젝트별 Kubernetes 클러스터에 파이프라인을 반복 설치해야 하고, 팀마다 설치 절차와 운영 방식이 달라 표준화가 어려워집니다. 또한 GitLab, Argo CD, Prometheus, Grafana, MinIO 등 OSS 조합은 버전 호환성과 의존 관계를 함께 검토해야 해서 "어떤 조합이 실제로 안전하게 동작하는지"를 판단하는 데 많은 시간이 소요됩니다.

Nullus는 이 문제를 해결하기 위해 검증된 OSS 조합을 Golden Path 템플릿으로 제공하고, 설치/연동 절차를 UI 기반 워크플로우로 통일하며, 배포 진행률과 로그를 단일 화면에서 추적할 수 있게 설계되었습니다. 이를 통해 신규 프로젝트 온보딩 시간을 줄이고, 환경별 편차로 인한 장애 가능성을 낮추며, 플랫폼 팀의 반복 작업을 자동화합니다.

> 참고: 스택 템플릿은 11개이고 세 가지 조합을 지원합니다 — **GitLab**(All-in-One · Argo CD ·
> Harbor · Nexus · SonarQube · OTel 조합), **GitHub + Argo CD**(저장소·Actions·GHCR 을 외부
> 서비스로 쓰고 클러스터에는 Argo CD 와 모니터링만 설치), **Gitea + Jenkins + Argo CD**
> (Lite 포함). 가장 많이 검증된 경로는 GitLab 이고, CI/CD 영역은 안정화를 계속 진행 중입니다.

### 주요 기능

- **Golden Path 템플릿 카탈로그**: 검증된 OSS 조합을 템플릿으로 제공해 설치 의사결정 시간을 줄입니다.
- **노코드 스택 설치 워크플로우**: UI 기반 단계형 설정으로 DevSecOps 스택을 배포합니다.
- **스택 상태/이력 가시화**: 스택 목록, 상태, 설치 이력, 핵심 구성 정보를 한 화면에서 확인합니다.
- **실시간 배포 로그 추적**: 단계별 진행률과 로그 스트림으로 설치/장애 상황을 빠르게 파악합니다.
- **클러스터 등록 및 연결 검증**: 다중 Kubernetes 클러스터를 등록하고 API 기반 연결 상태를 검증합니다.
- **버전 호환성 관리**: 검증된 도구 버전 조합을 기준으로 배포 안정성을 높입니다.
- **통합 관측**: 클러스터·스택·앱의 파드·CPU·메모리와 파이프라인 상태를 한 화면에서 봅니다.
- **알림 채널**: 스택 Prometheus 가 올린 심각 경보와 백업 결과를 Telegram·이메일(SMTP)·Slack 으로 보냅니다.
- **파이프라인 보안 게이트**: 시크릿 탐지(gitleaks)·의존성 취약점(Trivy SCA)·정적 분석(SonarQube)·이미지 스캔(Trivy)을 빌드 앞뒤에 끼워 차단합니다.
- **백업/복구**: 플랫폼 DB·Keycloak·금고·클러스터 리소스·볼륨을 암호화해 외부 오브젝트 스토리지에 뜨고 되돌립니다.
- **역할 기반 접근 제어(RBAC)**: Admin/DevOps/Developer 권한 분리로 기능 접근 범위를 제어합니다.

### 핵심 가치

- **Golden Path 템플릿**: 검증된 CI/CD 도구 조합으로 선택의 어려움 제거
- **노코드 설정**: 웹 UI의 체크박스/드롭다운으로 구성 — 인증·아티팩트·파이프라인·관측·보안·스토리지·리소스 등 탭 10개
- **자동 설치**: 한 번의 Deploy로 전체 스택 자동 설치 및 연동
- **버전 호환성 보장**: 테스트 완료된 도구 버전 조합만 제공

## 프로젝트 미리보기

### 1) 메인 대시보드

![Nullus main dashboard](.github/assets/main.png)

- Nullus의 핵심 가치(자동 설치, Golden Path, 모니터링, RBAC)를 한 화면에서 요약합니다.
- 빠른 시작 액션(클러스터 등록, 스택 시작, CI/CD 진입)으로 초기 온보딩 시간을 줄입니다.

### 2) Stack Template 카탈로그

![Nullus stack template](.github/assets/stack_template.png)

- GitLab/Argo CD/Prometheus/Grafana/MinIO 같은 도구 조합을 템플릿으로 표준화합니다.
- 예상 배포 시간, 리소스 요구사항, Helm/App 버전을 함께 보여 의사결정을 돕습니다.

### 3) Stack List 및 설치 결과 요약

![Nullus stack list](.github/assets/stack_list.png)

- 배포된 스택 목록과 상태(Running 등)를 확인하고, 선택한 스택의 상세 정보를 우측 패널에서 즉시 조회합니다.
- OSS 콘솔 링크, 토폴로지, 설치 요약 정보를 제공해 운영 가시성을 높입니다.

### 4) 설치 마법사와 호환성 게이트

![Nullus stack install wizard](.github/assets/stack_install.png)

- 스택 이름/접속 도메인/대상 클러스터부터 인증·아티팩트·리소스·Dry Run까지 탭 기반 마법사로 구성합니다. 순서가 정해진 단계가 아니라 아무 탭에서나 배포할 수 있습니다.
- 선택한 OSS 조합의 리소스 합산과 호환성 대조를 배포 전에 보여 줍니다. 호환성 판정과 리소스 합산은 **브라우저에서** 계산하며, 서버가 배포 요청을 되돌리는 Pre-Deploy 게이트는 아직 켜져 있지 않습니다 — 배포 전 Dry Run 으로 확인하세요.

## 기술 스택

- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui
- **Backend**: Go 1.26+ (Echo v4) + PostgreSQL 17
- **Auth**: Keycloak OIDC + JWT + 3단계 RBAC (Admin/DevOps/Developer)
- **Infrastructure**: Docker, Docker Compose, Helm v3, Kubernetes 1.34+, kind (로컬 테스트 — `kindest/node:v1.35.1`)
- **Object storage**: MinIO 프로토콜. 이미지는 AGPL 포크인 [SILO](https://github.com/pgsty/silo)(`pgsty/silo`)를 씁니다 — MinIO 가 Docker Hub·quay.io 익명 pull 을 막아서입니다. 환경변수·데이터 포맷·`mc` 는 그대로입니다

## Quick Start

### 요구사항

- Docker + Docker Compose
- Go 1.26+
- Node.js 22+
- kind (K8s 로컬 테스트 시)

### 1. 인프라 기동

```bash
./scripts/runbook_local.sh up
```

Docker Compose로 PostgreSQL(:5433), Redis(:6380), MinIO(:9000/:9001), Keycloak(:8180)을 기동하고 DB 마이그레이션을 실행합니다.

### 환경변수

```bash
cp .env.example .env.dev
# .env.dev는 make run 시 자동 로드 (ENCRYPTION_KEY 포함)
```

### 샘플데이터 마이그레이션 (DevSecOps Stack 목업)

`000023_seed_devsecops_stack_mock` 마이그레이션으로 스택 목록/이력 화면 검증용 샘플 데이터를 추가할 수 있습니다.

```bash
# 1) 로컬 인프라 실행 (PostgreSQL 포함)
make dev-up

# 2) 최신 마이그레이션 전체 적용 (000023 포함)
make migrate-up

# 3) 샘플 데이터 확인
docker compose -f docker-compose.dev.yaml exec postgres \
  psql -U nullus -d nullus \
  -c "SELECT id, name, state, namespace FROM stacks WHERE id LIKE 'mock-devsecops-%' ORDER BY id;"
```

롤백이 필요하면 마지막 마이그레이션 1개를 되돌립니다.

```bash
make migrate-down
```

### 2. 백엔드 실행

```bash
make run
```

`.env.dev`에서 환경변수(`ENCRYPTION_KEY` 포함)를 자동 로드합니다.
수동 실행 시:

```bash
ENCRYPTION_KEY="nullus-dev-key-32bytes-padding!!" go run ./cmd/api
```

API 서버: `http://localhost:8090`. `ENCRYPTION_KEY`는 kubeconfig 암호화에 사용되며 32바이트 필수.

설정 파일: `configs/config.yaml`

### 3. 프론트엔드 개발 서버

```bash
cd web && npm run dev
```

Vite 개발 서버가 `http://localhost:5173`에서 실행됩니다.

### 4. K8s 테스트 클러스터 (선택)

기본 클러스터 구조(dual kind):

- `nullus-platform(플랫폼 설치 클러스터)`: control-plane 1 + worker(data-plane) 2
- `nullus-develop(애플리케이션 배포 클러스터)`: control-plane 1 + worker(data-plane) 1
- Kubernetes 버전: `kindest/node:v1.35.1`

생성(권장):

```bash
./scripts/runbook_local.sh kind-up
kind get clusters
```

상태 확인:

```bash
kubectl get nodes --context kind-nullus-platform
kubectl get nodes --context kind-nullus-develop
```

삭제:

```bash
./scripts/runbook_local.sh kind-down
```

직접 kind 명령으로 생성하려면 아래를 사용할 수 있습니다.

```bash
kind create cluster --config scripts/kind-cluster.yaml
```

상세 가이드: [kind E2E 테스트 가이드](./docs/guides/kind-e2e-testing-guide.md)

### 4-1. 스택 설치 (스크립트)

UI 마법사를 거치지 않고 템플릿으로 바로 설치하려면 `stack-up` 을 씁니다. helm 을
직접 부르지 않고 백엔드 API 를 호출하므로 UI 설치와 같은 파이프라인(OpenBao →
ESO → 시크릿 → SSO)을 그대로 탑니다.

```bash
# 기본값: gitea-jenkins-argocd-lite-v1 을 kind-nullus-platform 에 설치
./scripts/runbook_local.sh stack-up --name=nullus-lite --namespace=nullus-lite --wait

# 다른 템플릿 / 다른 클러스터
./scripts/runbook_local.sh stack-up \
  --template=gitea-jenkins-argocd-v1 \
  --cluster=kind-nullus-platform \
  --name=nullus-gj --namespace=nullus-gj

# 진행 상황만 다시 보기
./scripts/runbook_local.sh stack-status stk_abc123 --wait
```

**자원 계획.** 설치 규모는 템플릿의 `planning_profile` 로 백엔드가 계산합니다
(`internal/stack/domain/planning.go`). Lite 템플릿은 `local` 프로파일이라 이
명령으로도 8Gi 노드 하나에 들어가는 크기로 깔립니다 — 예를 들어 Harbor 는
관리자 기본값 2코어/4Gi 대신 0.5코어/1Gi 로 요청합니다.

계획 계산은 설치 마법사와 같은 값을 냅니다. 두 구현이 갈라지면 UI 설치와 API
설치가 다른 크기로 깔리므로, `TestPlanResourceVector_MatchesInstallWizard` 가
마법사의 출력값을 그대로 고정해 두었습니다. 마법사에서 조정한 값을 실어 보내면
그 값이 서버 계산보다 우선합니다.

### 4-2. 설치한 것 되돌리기 (삭제)

클러스터를 버리지 않고 **스택만** 지우려면 `stack-down` 을 씁니다. 제품이 실제로
쓰는 삭제 경로(helm uninstall → CRD / ClusterRoleBinding 정리)를 그대로 타므로,
클러스터를 통째로 지우는 방식과 달리 삭제 로직 자체가 검증됩니다.

```bash
./scripts/runbook_local.sh pipeline-down          # 파이프라인 먼저 (스택을 참조한다)
./scripts/runbook_local.sh stack-down             # 설치된 스택 전체
./scripts/runbook_local.sh stack-down stk_abc123  # 특정 스택만
```

`stack-down` 은 끝나고 클러스터에 남은 helm 릴리스를 출력합니다. 클러스터의
**마지막 스택**을 지우면 스택들이 함께 쓰던 공용 자원(`cert-manager`·내부 CA·
`metrics-server`·Gateway API CRD)도 Nullus 가 설치한 것이면 회수합니다. 클러스터에
원래 있던 것이나 아직 쓰는 곳이 있는 것(사용자 인증서·HPA·Gateway 등)은 남기고,
무엇이 쓰는지 삭제 로그(`deleting_shared`)에 남깁니다.

처음 설치 상태로 완전히 되돌리려면 `purge` 를 씁니다. 파이프라인 → 스택 →
Nullus/백킹 → kind 클러스터 순으로 지우고, DB 볼륨까지 삭제해 다음 `up` 이
빈 DB에서 마이그레이션을 처음부터 돌리게 만듭니다.

```bash
./scripts/runbook_local.sh purge                  # 전부 초기화
./scripts/runbook_local.sh purge --keep-kind      # 클러스터는 남기고 스택만
./scripts/runbook_local.sh purge --keep-volumes   # DB 데이터는 보존
```

> `purge` 의 스택/파이프라인 삭제 단계는 API 를 통합니다. API 가 내려가 있으면
> 그 단계를 건너뛰고 인프라만 정리하므로, 삭제 경로까지 검증하려면 `up` 이
> 떠 있는 상태에서 실행하세요.

### 5. 클러스터 등록/검증 가이드 (팀 공통)

스택 설치 전에 반드시 클러스터를 등록하고 연결 검증(Verify)을 완료하세요.

#### 사전 조건

- API 서버가 실행 중이어야 합니다 (`http://localhost:8090/health` = healthy)
- `ENCRYPTION_KEY`가 **32바이트**로 설정되어 있어야 kubeconfig 저장/복호화가 정상 동작합니다

#### A) UI에서 등록 (권장)

1. `http://localhost:5173` 접속 → **Cluster Management** 이동
2. **Register Cluster** 클릭
3. 아래 값 입력
   - Name: 예) `kind-nullus-platform-fresh`
   - Type: `Kubernetes`(UI) / 백엔드 기준 `pipeline` 또는 `target`
   - kubeconfig: 파일 업로드 또는 YAML 붙여넣기
4. 저장 후 상세 패널에서 **Verify Connection** 실행
5. 상태가 `Connected`로 바뀌면 스택 설치 진행

#### B) API로 등록 (자동화/스크립트용)

kind 클러스터 예시:

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

응답으로 받은 `id`를 사용해 연결 검증:

```bash
curl -X POST http://localhost:8090/api/v1/admin/clusters/<cluster-id>/verify
```

성공 시 응답 예시:

```json
{"status":"connected","version":"v1.35.1"}
```

#### C) 검증 체크리스트

- `GET /api/v1/admin/clusters`에서 `connection_status=connected`
- `GET /api/v1/admin/clusters/:id/namespaces`가 정상 조회
- 스택 생성 시 해당 `cluster_id` 선택 가능

#### D) 운영 팁 / 자주 막히는 지점

- Verify 502/연결 실패: kubeconfig의 API endpoint가 폐기된 kind 포트를 가리키는 경우가 많습니다. 클러스터 재생성 후 kubeconfig를 다시 등록하세요.
- `kubeconfig is not registered for this cluster`: 등록 시 kubeconfig를 비워서 저장한 경우입니다.
- 클러스터 삭제 409(`CLUSTER_IN_USE`): 해당 클러스터를 참조하는 스택/파이프라인을 먼저 삭제해야 합니다.

## 테스트 계정

### 프론트엔드 (Mock Auth, development 모드)

| 역할 | 이메일 | 비밀번호 | 홈 페이지 |
|------|--------|----------|-----------|
| Admin | admin@nullus.dev | admin123 | /admin/organization |
| DevOps | devops@nullus.dev | devops123 | /stack/templates |
| Developer | developer@nullus.dev | developer123 | /cicd/developer-deploy |

### Keycloak OIDC (production 모드)

| 역할 | 이메일 | 비밀번호 |
|------|--------|----------|
| Admin | admin@nullus.io | nullus123! |
| DevOps | devops@nullus.io | nullus123! |
| Developer | dev@nullus.io | nullus123! |

### 인프라 서비스

| 서비스 | URL | 사용자 | 비밀번호 |
|--------|-----|--------|----------|
| PostgreSQL | localhost:5433 | nullus | nullus_dev |
| Keycloak Admin | localhost:8180 | admin | admin |
| MinIO Console | localhost:9001 | nullus | nullus_dev |
| Redis | localhost:6380 | - | - |

오브젝트 스토리지 컨테이너의 이미지는 `pgsty/silo`(MinIO 의 AGPL 포크)입니다. 환경변수와
`mc` 는 MinIO 그대로입니다.

## 테스트

### Go 단위/통합 테스트

```bash
go test ./... -count=1
```

### React 단위 테스트 (Vitest)

```bash
cd web && npx vitest run
```

### E2E 테스트 (Playwright)

프론트엔드 개발 서버(`localhost:5173`)와 API 서버(`localhost:8090`)가 실행 중이어야 합니다.

```bash
cd web && npx playwright test --reporter=list
```

### API Smoke Test

```bash
./scripts/runbook_local.sh smoke
```

### 모바일/반응형 자동 점검

주요 9개 화면을 3개 뷰포트(360/390/768px)로 렌더해 가로 오버플로우와 모바일 사이드바 미collapse 를 자동 감지합니다.

```bash
cd web && npm run responsive:audit          # 리포트 생성 (web/.responsive-audit/)
cd web && npm run responsive:audit:check    # 이슈 발견 시 exit 1 (CI 게이트용)
```

### 테스트 수행 가이드

로컬 E2E(클러스터 등록→스택 설치→파이프라인→배포 검증)·Playwright 수행법·환경 전제와 알려진 함정은
[테스트 수행 가이드](./docs/60_테스트/Nullus_테스트_수행_가이드.md)를 참고하세요.
둘러보기(투어) 여정 기반 기능 검증 결과는 [기능 확인 시나리오](./docs/60_테스트/Nullus_둘러보기_기능확인_시나리오.md)에 기록되어 있습니다.

## API 엔드포인트

API 서버: `http://localhost:8090`

### Admin (`/api/v1/admin`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/admin/organization` | 현재 Organization 조회 |
| PATCH | `/admin/organization` | Organization 수정 |
| POST | `/admin/orgs` | Organization 생성 |
| GET/POST | `/admin/clusters` | 클러스터 목록 / 등록 |
| GET/PATCH/DELETE | `/admin/clusters/:id` | 클러스터 상세 / 수정 / 삭제 |
| POST | `/admin/clusters/:id/verify` | 클러스터 연결 검증 (K8s API 실연동) |
| GET/POST | `/admin/organizations/:orgId/members` | 멤버 목록 / 초대 (기존 사용자 추가 포함) |
| DELETE/PATCH | `/admin/organizations/:orgId/members/:id` | 멤버 제거 / 역할 변경 |
| GET | `/admin/users/search?email=` | 기존 사용자 검색 |
| GET | `/admin/clusters/:id/namespaces` | 클러스터 네임스페이스 목록 (K8s API) |
| GET | `/admin/known-issues` | Known Issues 목록 |
| GET | `/admin/audit-logs` | 감사 로그 |
| GET | `/admin/notifications/events` | 구독할 수 있는 이벤트 5종 |
| GET/POST | `/admin/notifications/channels` | 알림 채널 목록 / 등록 (Telegram·메일·Slack) |
| GET/PATCH/DELETE | `/admin/notifications/channels/:id` | 채널 상세 / 수정 / 삭제 |
| POST | `/admin/notifications/channels/:id/test` | 테스트 발송 (실제 경로로 보냅니다) |
| GET | `/admin/notifications/history` | 발송 이력 |
| GET/POST | `/admin/backups` | 백업 이력 / 실행 (**동기** — 끝날 때까지 응답하지 않습니다) |
| GET | `/admin/backups/:id` | 백업 상세 |
| POST | `/admin/backups/:id/verify` | 산출물 검증 (존재·해시·스키마) |
| POST | `/admin/backups/sets` | 백업 세트 실행 (플랫폼 + 스택별) |
| POST | `/admin/restores` | 복구 실행 (화면 없음 — API 전용) |
| GET | `/admin/restores/:id` | 복구 상세 |

### Stack (`/api/v1/stacks`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET/POST | `/stacks` | 스택 목록 / 생성 (namespace 지정 가능) |
| DELETE | `/stacks/:id` | 스택 삭제 (Helm uninstall 포함) |
| GET | `/stacks/templates` | Golden Path 템플릿 목록 |
| GET | `/stacks/compatibility` | 도구 호환성 매트릭스 |
| GET | `/stacks/resource-defaults` | OSS별 리소스 request/limit 기본값 목록 |
| POST | `/stacks/resource-defaults` | OSS 리소스 request/limit 업서트 (`tool_key` 기준, idempotent) |
| POST | `/stacks/:id/deploy` | 스택 배포 (Helm SDK) |
| GET | `/stacks/:id/status` | 배포 상태 조회 |

### CI/CD (`/api/v1/cicd`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/cicd/templates` | CI/CD 파이프라인 템플릿 |
| GET/POST | `/cicd/pipelines` | 파이프라인 목록 / 생성 |
| POST | `/cicd/pipelines/:id/deploy` | 파이프라인 배포 |
| GET | `/cicd/deployments` | 배포 이력 |
| GET | `/cicd/app-templates` | Developer Self-Service 앱 템플릿 |
| POST | `/cicd/deploy-app` | 앱 배포 |

### Observability (`/api/v1/observability`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/observability/dashboard` | 모니터링 대시보드 |
| GET | `/observability/stack-alerts` | 스택 경보 이력 (알림 이력 화면) |
| GET | `/observability/alert-history` | 알림 발송 이력 |
| GET/POST/PATCH/DELETE | `/observability/alert-rules[/:id]` | 알림 규칙 CRUD. 규칙은 저장되지만 **평가되지 않아** 화면은 내려 두었습니다 |

이 그룹은 **역할 검사가 없습니다**(인증만). 알림 규칙 쓰기까지 인증한 모든 역할이 호출할 수 있습니다.

### 기타

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/health` | 서버 + DB 상태 |
| WS | `/ws/deployments/:id/logs` | 배포 로그 실시간 스트리밍 |

## 기능 구현 현황 (PRD v1.3 Phase 1)

| 기능 | 설명 | 상태 |
|------|------|------|
| F0 | Organization 설정 등록 (Postgres Repository, CRUD API) | 구현됨 |
| F1 | K8S Cluster 등록/검증 (client-go, kubeconfig 암호화) | 구현됨 |
| F2 | 노코드 DevSecOps Stack 설정 UI (React Wizard, RHF+Zod) | 구현됨 |
| F3 | Golden Path 템플릿 (템플릿 카탈로그, Postgres Seed) | 구현됨 |
| F4 | 스택 자동 설치/배포/이력 (Helm DAG, rollback, WS 로그) | 구현됨 |
| F5 | CI/CD Pipeline 템플릿 (템플릿, Manifest Generator) | 안정화 진행 중 |
| F6 | CI/CD Pipeline 배포/이력 (Manifest Applier, 배포 추적) | 안정화 진행 중 |
| F7 | 모니터링 대시보드 (클러스터·스택·CI/CD 탭) | 구현됨 |
| F8 | 버전 호환성 관리 (매트릭스 조회, 설치 전 대조) | 조회·대조 구현. 매트릭스 등록은 마이그레이션 시드로만 |
| F9 | UI 권한 체계 (OIDC/JWT, 라우트 RBAC) | 구현됨 |
| F10 | 리소스 예상량 계산 | 브라우저에서 계산. 서버 엔드포인트는 있으나 화면이 쓰지 않음 |
| F11 | 기존 사용자 추가 (멀티 조직 멤버십, 이메일 검색) | 구현됨 |
| F12 | 네임스페이스 선택/생성 (K8s API 조회, 스택별 namespace) | 구현됨 |

Phase 1 이후에 더한 것:

| 기능 | 설명 | 상태 |
|------|------|------|
| 파이프라인 보안 게이트 | 시크릿 탐지(gitleaks) · 의존성 취약점(Trivy SCA) · 정적 분석(SonarQube) · 이미지 스캔(Trivy). 스택 정책으로 차단/경고/끄기 | 구현됨 |
| 알림 채널 | Telegram · 이메일(SMTP) · Slack. 비밀은 암호화 저장, 일시적 실패만 3회 재시도 | 구현됨 |
| 스택 경보 전달 | 스택 Prometheus 의 심각 경보를 1분마다 가져와 채널로 보냄 (발생 · 4시간마다 · 해제) | 구현됨 |
| 백업/복구 | 6개 구성요소를 암호화해 외부 오브젝트 스토리지에 뜨고 되돌림 | 백업은 화면 + API. **복구는 API 전용** |
| Gitea + Jenkins 경로 | Jenkins multibranch job 프로비저닝, JCasC 자격증명 | 구현됨 |
| 폐쇄망(에어갭) 번들 | 이미지·차트 반입 스크립트, SBOM·provenance | 구현됨 |
| CLI / MCP | `nullus version` · `stack ls` · `mcp serve`(도구 9종) | 그 셋만 구현. `login` 등은 설계 |
| 알림 규칙 편집 | 규칙은 저장되지만 평가되지 않아 화면을 내려 둠 | 미구현 |

전체 기능 명세: [PRD v1.3](./docs/10_제품기획/nullus_PRD_1.3.md) · 화면별 사용법:
[문서 사이트](https://cloud-nullus.github.io/nullus/features/)

## 프로젝트 구조

```
nullus/
├── cmd/
│   ├── api/                # API 서버 진입점
│   ├── nullus/             # CLI (version · stack ls · mcp serve)
│   ├── nullus-bootstrap/   # 무인 설치용 부트스트랩 자격 발급
│   └── token-source-sync/  # 토큰 소스 동기화
├── configs/                # 설정 파일 (config.yaml)
├── db/migrations/          # DB 마이그레이션 (99개)
├── deploy/helm/nullus/     # Helm 차트
├── airgap/                 # 폐쇄망 번들 스크립트
├── internal/               # 내부 모듈 (Clean Architecture)
│   ├── admin/              # 조직/클러스터/사용자/토큰 관리
│   ├── auth/               # Keycloak OIDC/JWT/RBAC
│   ├── backup/             # 백업/복구
│   ├── cicd/               # CI/CD 파이프라인
│   ├── cli/                # CLI 명령 구현
│   ├── mcp/                # MCP 서버 (도구 9종, stdio)
│   ├── notification/       # 알림 채널 (Telegram·메일·Slack)
│   ├── observability/      # 모니터링, 스택 경보 수집
│   ├── stack/              # DevSecOps 스택 설치
│   └── shared/             # 설정·미들웨어·audit·이벤트 계약
├── pkg/crypto/             # AES-256 암호화
├── scripts/                # 운영 스크립트 (runbook, keycloak, kind)
├── web/                    # React 프론트엔드
│   ├── src/features/       # 기능별 모듈 (admin, auth, cicd, observability, stack)
│   └── e2e/                # Playwright E2E 테스트
├── e2e/                    # Go E2E 테스트
├── website/                # 문서 사이트 (MkDocs Material)
├── CHANGELOG.md            # 변경 이력
├── docs/70_전략/ROADMAP.md # 로드맵
└── Makefile                # 개발 명령어
```

## 아키텍처

### 설계 원칙

**Modular Monolith**: 모놀리스로 시작하되, 모듈 경계를 명확히 하여 향후 마이크로서비스로 전환 가능하도록 설계합니다.

**Clean Architecture**: 의존성은 항상 안쪽(도메인)을 향합니다.

```
[Handler/Controller] → [UseCase/Service] → [Domain/Entity]
        ↓                      ↓
  [Repository Interface]  [Domain Logic]
        ↓
  [Repository Impl (DB/API)]
```

**Domain-Driven Design (DDD)**: 7개 Bounded Context로 구성됩니다.

| Context | 모듈 | 핵심 기능 |
|---------|------|----------|
| Stack Management | `internal/stack/` | DevSecOps 스택 설치/관리 (Helm SDK) |
| CI/CD Pipeline | `internal/cicd/` | 파이프라인 스캐폴딩/배포, 스캔 결과 |
| Observability | `internal/observability/` | 클러스터·스택 지표 수집, 스택 Alertmanager 경보 수집 |
| Organization | `internal/admin/` | 조직/사용자/클러스터/토큰 관리, 감사 로그 |
| Auth | `internal/auth/` | Keycloak OIDC, JWT, RBAC, SSO 프로비저닝 |
| Backup | `internal/backup/` | 백업·복구 (암호화, 외부 오브젝트 스토리지) |
| Notification | `internal/notification/` | 알림 채널, 발송·재시도·이력 |

지표는 Prometheus 질의가 아니라 클러스터 kubeconfig 로 Kubernetes 에 직접 붙어 모읍니다
(`kubectl top` · kubelet `stats/summary`). 스택에 설치된 Prometheus 는 경보를
평가하는 쪽이고, 그 경보를 플랫폼이 1분마다 가져와 알림 채널로 보냅니다.

## 역할 체계

| 역할 | API 접근 | 주요 기능 |
|------|---------|----------|
| Admin | 전체 | 조직 설정, 사용자·토큰 관리, 클러스터 등록/검증, 백업, 알림 채널 |
| DevOps | `/stacks`, `/cicd`, `/observability` | 스택 설치/배포, 파이프라인 관리 |
| Developer | `/cicd`, `/observability` | 파이프라인 생성·배포, 모니터링 조회 |

역할 검사는 **계층이 아니라 명단**입니다 — `/admin` 은 admin 만, `/stacks` 는 admin·devops,
`/cicd` 는 세 역할 모두입니다. `/observability` 는 **역할 검사가 없고** 인증만 합니다.
사이드바에 보이는 항목 수는 admin 19 · devops 11 · developer 4 개입니다. 화면 권한보다
좁은 조회 API 를 부르는 화면이 몇 개 있어 developer 에게는 일부 선택 상자가 비어 보입니다 —
[권한 표](https://cloud-nullus.github.io/nullus/reference/rbac/)에 정리했습니다.

## Helm 차트

```bash
# 린트
helm lint deploy/helm/nullus/

# 템플릿 확인
helm template nullus deploy/helm/nullus/ --values deploy/helm/nullus/values.yaml
```

## 라이선스

Apache License 2.0

## 커뮤니티

- **문서**: [cloud-nullus.github.io/nullus](https://cloud-nullus.github.io/nullus/)
- **라이브 데모**: [cloud-nullus.github.io](https://cloud-nullus.github.io)
- **GitHub**: [cloud-nullus/nullus](https://github.com/cloud-nullus/nullus)
- **Issues**: 기능 요청 및 버그 리포트는 GitHub Issues를 이용
- **Discussions**: 아이디어 및 질문은 GitHub Discussions에서 논의

## 참고 문서

기능별 설명과 사용법은 **[문서 사이트](https://cloud-nullus.github.io/nullus/)** 에
정리돼 있습니다. 아래는 저장소 안의 원본 문서입니다.

- [CHANGELOG.md](./CHANGELOG.md) — 변경 이력
- [ROADMAP.md](./docs/70_전략/ROADMAP.md) — 개발 로드맵
- [CLAUDE.md](./CLAUDE.md) — 아키텍처 원칙 및 개발 규칙
- [kind E2E 테스트 가이드](./docs/guides/kind-e2e-testing-guide.md) — K8s 클러스터 대상 시나리오 테스트
- [테스트 수행 가이드](./docs/60_테스트/Nullus_테스트_수행_가이드.md) — 로컬 E2E·반응형 점검·Playwright 수행법
- [PRD v1.3](./docs/10_제품기획/nullus_PRD_1.3.md) — 제품 요구사항 명세
- [API 설계](./docs/20_아키텍처/Nullus_API_설계.md) — API 상세 설계
- [로컬 개발환경 세팅](./docs/50_운영/Nullus%20로컬%20개발환경%20세팅%20가이드.md) — 개발 환경 설정
- [2차 기술 검증 발표 장표](./docs/80_프로젝트관리/Nullus_2차기술검증_발표장표.html) — OPEN PROJECT SEASON III 발표 자료 (브라우저로 열어 보세요)
