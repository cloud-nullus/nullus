# Nullus 개발자 온보딩 가이드

**대상**: Nullus 프로젝트에 새로 합류하는 개발자
**목표**: 합류 첫 날부터 코드를 이해하고, 로컬 환경을 세팅하고, 첫 PR을 올릴 수 있도록

---

## 1. 프로젝트 소개 (5분 읽기)

### 1.1 Nullus란?

새로운 프로젝트를 시작하는 DevOps Engineer가 검증된 CI/CD 베스트 프랙티스 조합(Golden Path)을 선택하고, 즉시 Kubernetes 기반 DevSecOps 파이프라인을 구축할 수 있도록 노코드 UI와 자동 설치 기능을 제공하는 오픈소스 플랫폼이다.

**핵심 가치 3가지:**
- 검증된 Golden Path: 프로덕션 검증된 CI/CD 도구 조합 템플릿을 제공해 6-18개월짜리 플랫폼 구축을 며칠로 단축
- 버전 호환성 보장: 테스트 완료된 도구 버전 조합만 제공해 예측 불가능한 호환성 이슈 제거
- 노코드 설정: 웹 UI에서 체크박스/드롭다운 방식으로 파이프라인을 구성하고 한 클릭으로 Kubernetes 클러스터에 자동 배포

### 1.2 역할 체계

플랫폼은 3개 역할로 접근 권한을 분리한다.

| 역할 | 코드 값 | 접근 가능 기능 |
|------|---------|--------------|
| **Admin** | `admin` | 조직 설정, 사용자 관리, 클러스터 등록 전체 |
| **DevOps Engineer** | `devops` | 스택 템플릿 선택·설치·배포, CI/CD, 모니터링, Admin 기능 전체 |

사용자는 여러 조직에 동시에 소속될 수 있으며 조직별로 다른 역할을 가질 수 있다.
| **Developer** | `developer` | CI/CD 파이프라인 생성·배포, 관측성 대시보드 |

### 1.3 기술 스택

| 영역 | 기술 |
|------|------|
| **Backend** | Go 1.24+ (Echo v4) + PostgreSQL 18+ |
| **Frontend** | React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui 패턴 |
| **상태 관리** | Zustand 5 |
| **API 통신** | TanStack Query 5 + Axios |
| **테스트 (Go)** | `testing`, `testify/mock`, `testcontainers` |
| **테스트 (FE)** | Vitest + Testing Library + Playwright |
| **Infra** | Docker Compose (로컬), Helm, Kubernetes 1.26+ |

### 1.4 아키텍처 원칙

**Modular Monolith**: 5개 모듈(stack, cicd, admin, observability, auth)로 경계를 나눈다. 모듈 간 직접 import는 금지이며 공유 타입은 `internal/shared/`에 둔다.

**Clean Architecture**: 의존성 방향은 항상 안쪽(도메인)을 향한다.

```
HTTP 요청
  → Handler (adapter)
  → UseCase
  → Domain (Entity, 비즈니스 규칙)
  → Repository Interface (port)
  → PostgreSQL/메모리 구현체 (adapter)
```

**DDD**: 모듈 = 바운디드 컨텍스트. PRD/기획 문서의 용어를 코드에 그대로 반영한다(Ubiquitous Language).

**TDD**: RED → GREEN → REFACTOR 사이클. 새 기능 구현 시 테스트를 먼저 작성한다.

---

## 2. 환경 설정 (30분)

### 2.1 사전 요구사항

| 도구 | 버전 | 설치 방법 |
|------|------|----------|
| Git | 2.40+ | 시스템 패키지 관리자 |
| Docker Desktop + Docker Compose | 최신 | [docker.com](https://www.docker.com) |
| Go | 1.24+ | `brew install go` |
| Node.js | 22+ | `nvm install 22` |
| golang-migrate | 최신 | `make dev` 실행 시 자동 설치 |
| golangci-lint | 최신 | `brew install golangci-lint` |
| Playwright Chromium | 최신 | `npx playwright install chromium` |

> macOS 기준. Linux(WSL2)에서는 `brew` 대신 각 공식 설치 방법을 따른다.

### 2.2 리포지토리 클론

```bash
git clone https://github.com/cloud-nullus/draft.git
cd draft
```

### 2.3 환경변수 설정

```bash
cp .env.example .env.dev
```

`.env.dev` 파일은 `make run` 실행 시 자동으로 로드된다. 기본값으로 로컬 개발이 가능하며 별도 수정이 필요 없다.

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `ENCRYPTION_KEY` | `nullus-dev-key-32bytes-padding!!` | kubeconfig 암호화 키 (32바이트 필수) |
| `NULLUS_DB_HOST` | `localhost` | PostgreSQL 호스트 |
| `NULLUS_DB_PORT` | `5433` | PostgreSQL 포트 |

> `ENCRYPTION_KEY`를 변경하면 기존 DB에 저장된 kubeconfig가 복호화되지 않는다. 클러스터를 다시 등록해야 한다.

### 2.4 인프라 기동

```bash
make dev
```

`make dev`는 다음을 순서대로 실행한다:

1. Docker Compose로 PostgreSQL, MinIO, Redis 컨테이너 기동
2. `golang-migrate`로 DB 마이그레이션 실행

기동 완료 후 접근 가능한 서비스:

| 서비스 | 주소 | 비고 |
|--------|------|------|
| PostgreSQL 17 | `localhost:5433` | 기본 포트(5432)와 다름 — 로컬 충돌 방지 |
| MinIO API | `localhost:9000` | |
| MinIO 콘솔 | `localhost:9001` | |
| Redis 7 | `localhost:6380` | 기본 포트(6379)와 다름 — 로컬 충돌 방지 |

### 2.5 백엔드 빌드 + 실행

```bash
make run
```

`make run`은 `.env.dev`에서 환경변수를 자동 로드하고 API 서버를 빌드 후 실행한다. 빌드 없이 빠르게 실행:

```bash
make run-dev
```

서버 기동 확인:

```bash
curl http://localhost:8090/health
# {"status":"healthy","db":"connected","version":"0.1.0-alpha"}
```

### 2.6 프론트엔드 실행

```bash
cd web && npm install && cd ..
make web-dev
```

- URL: `http://localhost:5173`
- HMR(Hot Module Replacement) 지원
- 백엔드 미연결 상태에서는 mock 데이터로 동작

### 2.7 테스트 실행

```bash
make test              # Go 단위 + 통합 테스트 전체
make web-test          # React vitest (npx vitest run)
cd web && npm run e2e  # Playwright E2E (dev 서버 자동 기동)
```

로컬 로그인 테스트 계정은 `docs/50_운영/Nullus_로컬_테스트_가이드.md`의 계정 표를 사용한다.

---

## 3. 프로젝트 구조 이해 (20분)

### 3.1 디렉토리 구조 (전체)

```
draft/
├── cmd/api/main.go          # API 서버 진입점 — DI 조립, 라우트 등록
├── internal/                # Go 백엔드 모듈
│   ├── stack/               # DevSecOps 스택 모듈
│   │   ├── domain/          # Entity, Value Object, 상태 머신 (DeploymentState)
│   │   ├── usecase/         # 비즈니스 로직 (install, create, list, ...)
│   │   ├── port/            # 인터페이스 (Repository, Installer, LogStreamer)
│   │   └── adapter/
│   │       ├── handler/     # Echo HTTP 핸들러
│   │       ├── repository/  # 메모리 + PostgreSQL 구현체
│   │       ├── helm/        # Helm 설치 구현체
│   │       └── log/         # WebSocket 로그 스트리머
│   ├── cicd/                # CI/CD 파이프라인 모듈
│   ├── admin/               # 조직/사용자/클러스터 모듈
│   ├── observability/       # 모니터링/알림 모듈
│   ├── auth/                # 인증 미들웨어 모듈
│   └── shared/              # 공통 (에러, 이벤트, 설정, 미들웨어)
├── web/                     # React 프론트엔드
│   ├── src/
│   │   ├── app/             # 레이아웃(layout.tsx), 라우팅(routes.tsx)
│   │   ├── components/      # 재사용 UI 컴포넌트
│   │   │   ├── ui/          # 기본 UI 요소 (Button, Card, Modal, Input)
│   │   │   ├── layout/      # 레이아웃 컴포넌트 (Header, Sidebar)
│   │   │   └── shared/      # 공통 컴포넌트 (DataTable, StatusBadge, YamlEditor)
│   │   ├── features/        # 기능별 모듈 (stack, cicd, admin, observability, auth)
│   │   ├── stores/          # Zustand 전역 상태 (auth, theme, sidebar)
│   │   ├── i18n/            # 다국어 (en.json, ko.json)
│   │   └── lib/             # API 클라이언트(api.ts), WebSocket(websocket.ts)
│   └── e2e/                 # Playwright E2E 테스트
├── e2e/                     # Go E2E + UAT 테스트
├── db/migrations/           # PostgreSQL 마이그레이션 파일
├── configs/config.yaml      # 서버/DB 설정
├── docs/                    # 프로젝트 문서
├── Makefile                 # 개발 워크플로우 명령
└── docker-compose.dev.yaml  # 로컬 개발 인프라
```

### 3.2 백엔드 모듈 설명

| 모듈 | 경로 | 핵심 Aggregate | 담당 |
|------|------|--------------|------|
| **stack** | `internal/stack/` | Stack, Template, Compatibility | Golden Path 템플릿 관리, 스택 설치·배포·이력, 호환성 매트릭스 |
| **cicd** | `internal/cicd/` | Pipeline, Deployment | CI/CD 파이프라인 생성·배포·이력 관리 |
| **admin** | `internal/admin/` | Organization, User, Cluster | 조직 설정, 사용자 관리, Kubernetes 클러스터 등록 |
| **observability** | `internal/observability/` | Dashboard, Alert | 모니터링 대시보드 조회, 알림 규칙 생성·관리 |
| **auth** | `internal/auth/` | Session, Token | 인증 미들웨어 (Echo middleware 형태) |

### 3.3 프론트엔드 기능 모듈 설명

`web/src/features/` 하위 각 모듈이 담당하는 페이지:

| 모듈 | 담당 페이지 | 주요 URL |
|------|-----------|---------|
| **stack** | 스택 템플릿 선택, 설치 설정(6탭), 배포 현황, 이력, 버전 호환성 | `/stack/*` |
| **cicd** | CI/CD 템플릿 선택, 파이프라인 목록·이력, Developer 5단계 배포 위자드 | `/cicd/*` |
| **admin** | 조직 설정, 사용자 관리, 클러스터 등록·검증 | `/admin/*` |
| **observability** | 모니터링 KPI 대시보드, 알림 규칙 관리, 알림 이력 | `/observability/*` |
| **auth** | 로그인 페이지 | `/login` |
| **home** | 역할별 CTA 버튼이 있는 홈 화면 | `/` |

각 기능 모듈 내부 구조:
```
features/{module}/
  pages/    # 라우트에 연결되는 페이지 컴포넌트
  api/      # TanStack Query hook + Axios API 호출
  hooks/    # 커스텀 훅
  stores/   # 모듈 전용 Zustand store (필요 시)
```

### 3.4 Clean Architecture 레이어 흐름

```
HTTP 요청
  └─ adapter/handler/        (Echo 핸들러 — 요청 파싱, 응답 직렬화)
       └─ usecase/           (비즈니스 로직 — 트랜잭션, 규칙 조합)
            └─ domain/       (Entity, Value Object, 도메인 규칙)
            └─ port/         (Repository 인터페이스 — 구현체를 모름)
                 └─ adapter/repository/  (실제 DB/메모리 구현체)
```

**의존성 규칙 요약:**

| 레이어 | 의존 가능 | 의존 불가 |
|--------|---------|---------|
| `domain/` | 순수 Go stdlib만 | 프레임워크, DB, 외부 패키지 |
| `usecase/` | `domain/`, `port/` 인터페이스 | `adapter/`, Echo, pgx |
| `port/` | `domain/` | 구현체 |
| `adapter/` | `usecase/`, `port/`, `domain/` | 다른 모듈의 내부 패키지 |

---

## 4. 코드 읽기 가이드 (30분)

### 4.1 백엔드: 첫 번째로 읽을 파일들

가장 단순한 모듈(admin)부터 레이어 순서대로 읽는다.

1. `internal/admin/domain/user.go` — Role enum(`admin`, `devops`, `developer`), `CanAccess` 메서드
2. `internal/admin/domain/organization.go` — Organization Entity, `ValidateSlug` Value Object 패턴
3. `internal/admin/port/repository.go` — Repository 인터페이스 정의
4. `internal/admin/usecase/org_usecase.go` — UseCase 패턴, Repository 인터페이스 주입
5. `internal/admin/adapter/handler/org_handler.go` — Echo HTTP 핸들러, 라우트 등록
6. `cmd/api/main.go` — 전체 DI 조립 + 라우트 등록 흐름

### 4.2 백엔드: 핵심 도메인 읽기

1. `internal/stack/domain/stack.go` — `DeploymentState` 상태 머신 (`pending` → `validating` → `installing` → `configuring` → `health_check` → `completed`), `TransitionTo` 메서드
2. `internal/stack/usecase/install_stack.go` — Install Engine (상태 전이 + 로그 스트리밍)
3. `internal/stack/adapter/log/memory_streamer.go` — WebSocket 로그 스트리밍 구현

### 4.3 프론트엔드: 첫 번째로 읽을 파일들

1. `web/src/stores/auth-store.ts` — Zustand store 패턴, 역할(role) 상태 관리
2. `web/src/components/layout/sidebar.tsx` — 역할별 메뉴 필터링 로직
3. `web/src/app/routes.tsx` — React Router 7 라우팅 구조, lazy loading 패턴
4. `web/src/features/stack/pages/stack-template-page.tsx` — 페이지 컴포넌트 패턴

### 4.4 테스트 코드 읽기

1. `internal/admin/domain/user_test.go` — 가장 단순한 Go 단위 테스트
2. `internal/admin/usecase/org_usecase_test.go` — mock 기반 UseCase 테스트 패턴
3. `web/src/stores/auth-store.test.ts` — Zustand store 테스트
4. `e2e/uat_test.go` — Go E2E + UAT 시나리오 전체 흐름

---

## 5. 첫 번째 작업 (1-2시간)

### 5.1 Good First Issues

새 개발자가 시작하기 좋은 작업 유형:

- 도메인 Entity에 필드 추가 + 단위 테스트
- 기존 API에 입력 검증 로직 추가
- 프론트엔드 페이지 UI 개선
- 테스트 커버리지 향상
- 번역 키 추가 (`web/src/i18n/en.json`, `ko.json`)
- 문서 오타 수정

### 5.2 작업 워크플로우 예시: "Organization에 description 필드 추가"

#### Step 1: 브랜치 생성

```bash
git checkout -b feat/admin/add-org-description
```

#### Step 2: 테스트 먼저 작성 (TDD — RED)

```go
// internal/admin/domain/organization_test.go
func TestOrganization_Description(t *testing.T) {
    org := &Organization{Description: "My team description"}
    assert.Equal(t, "My team description", org.Description)
}
```

```bash
go test ./internal/admin/domain/... -v
# FAIL — 컴파일 에러. 예상된 결과.
```

#### Step 3: 도메인 수정 (GREEN)

```go
// internal/admin/domain/organization.go
type Organization struct {
    // ...기존 필드
    Description string `json:"description"`
}
```

#### Step 4: DB 마이그레이션 파일 추가

```sql
-- db/migrations/000007_org_description.up.sql
ALTER TABLE organizations ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- db/migrations/000007_org_description.down.sql
ALTER TABLE organizations DROP COLUMN description;
```

#### Step 5: UseCase + Handler 업데이트

UseCase의 Create/Update 입력 구조체에 `Description` 필드를 추가하고, Handler의 요청 파싱 로직을 업데이트한다.

#### Step 6: 프론트엔드 업데이트 (필요 시)

`web/src/features/admin/pages/organization-page.tsx`에서 폼 필드와 `web/src/features/admin/api/admin-api.ts`의 타입을 업데이트한다.

#### Step 7: 테스트 실행

```bash
make test && make web-test
```

#### Step 8: PR 생성

```bash
git add internal/admin/domain/organization.go \
        internal/admin/domain/organization_test.go \
        db/migrations/000007_org_description.up.sql \
        db/migrations/000007_org_description.down.sql
git commit -m "feat(admin): Organization에 description 필드 추가"
git push -u origin feat/admin/add-org-description
```

커밋 메시지는 한국어 설명 + Semantic prefix를 사용한다.

```text
feat(admin): 조직 기본 정보 수정 API 검증 로직 추가
fix(stack): 배포 상태 전이 조건 오류 수정
test(cicd): 파이프라인 배포 실패 케이스 회귀 테스트 추가
docs: 온보딩 가이드 환경변수 섹션 갱신
```

### 5.3 PR 전 자기 검증 체크리스트

- [ ] `make test` 통과
- [ ] `make web-test` 통과
- [ ] Clean Architecture 레이어 위반 없음 (`domain/`에 외부 import 없음)
- [ ] 모듈 간 직접 import 없음
- [ ] 테스트가 PR에 함께 포함됨
- [ ] 커밋 메시지가 `<type>(<module>): <description>` 형식을 따름
- [ ] 도메인 용어가 PRD/기획 문서와 일치함

---

## 6. 주요 문서 목록

| 문서 | 경로 | 설명 |
|------|------|------|
| 프로젝트 지침 | `CLAUDE.md` | 아키텍처 원칙, TDD 규칙, 코딩 규칙 |
| 기여 가이드 | `CONTRIBUTING.md` | 브랜치 전략, 커밋 메시지, 리뷰 기준 |
| PRD v1.3 | `docs/10_제품기획/nullus_PRD_1.3.md` | 제품 요구사항 전체 |
| 마스터 플랜 | `docs/10_제품기획/Nullus 개발 마스터 플랜.md` | 12주 개발 계획, 팀 구성, 역할 분담 |
| API 설계 | `docs/20_아키텍처/Nullus_API_설계.md` | REST API 엔드포인트 상세 |
| DB 스키마 | `docs/20_아키텍처/Nullus_DB_스키마.md` | 테이블 설계, ERD |
| 백엔드 설계 | `docs/20_아키텍처/Nullus_백엔드_상세설계.md` | Go 모듈 상세 설계 |
| 프론트엔드 설계 | `docs/40_UI_UX/Nullus_프론트엔드_상세설계.md` | React 컴포넌트 상세 |
| 디자인 시스템 | `docs/40_UI_UX/Nullus_디자인시스템.md` | 컬러, 타이포그래피, 컴포넌트 규칙 |
| 로컬 테스트 | `docs/50_운영/Nullus_로컬_테스트_가이드.md` | 테스트 실행 방법 전체 |

---

## 7. 팀 소통

### 7.1 커뮤니케이션 채널

- **GitHub Issues**: 버그 리포트, 기능 요청
- **GitHub Discussions**: Q&A, 아이디어 논의
- **Discord**: 실시간 소통

### 7.2 주간 루틴

- 데일리 스탠드업: 매일 10시 (15분)
- 스프린트 플래닝: 격주 월요일
- 코드 리뷰: PR 생성 후 24시간 이내 응답

### 7.3 팀 구성 참고

| 역할 | 주요 책임 |
|------|---------|
| BE-1 (백엔드 리드) | API 설계, 자동 설치 엔진 코어, 이력 관리 |
| BE-2 (백엔드) | 클러스터 연결, 배포 워커, 헬스체크, 권한 체계 |
| FE-1 (프론트엔드 리드) | 설정 UI 워크플로우, 실시간 로그, YAML 에디터 |
| FE-2 (프론트엔드) | 대시보드, 템플릿 UI, 모니터링 뷰, 권한 UI |
| DevOps | Helm 차트 관리, CI/CD 파이프라인, 인프라 자동화 |
| 풀스택/QA | 통합 테스트, E2E 자동화, 문서화, 릴리스 자동화 |

---

## 8. FAQ

**Q: `domain/`에서 외부 패키지를 import해도 되나요?**

A: 아니요. `domain/`은 순수 Go stdlib만 사용합니다. 외부 의존은 `adapter/`에서 처리합니다. `domain/`에 Echo, pgx, 기타 라이브러리 import가 있으면 레이어 위반입니다.

**Q: 새 모듈을 추가하려면?**

A: `internal/` 하위에 `domain/`, `usecase/`, `port/`, `adapter/` 구조로 디렉토리를 만들고 `cmd/api/main.go`에 DI를 조립합니다. 기존 `internal/admin/` 구조를 참고하세요.

**Q: 프론트엔드에서 API 호출은 어떻게 하나요?**

A: TanStack Query + Axios를 사용합니다. `web/src/features/{module}/api/` 디렉토리에 hook을 작성하고 `web/src/lib/api.ts`의 Axios 인스턴스를 사용합니다.

**Q: 테스트 없이 PR을 올려도 되나요?**

A: 아니요. TDD가 원칙입니다. 최소한 `domain/` 레이어의 단위 테스트는 필수입니다. 테스트 없는 PR은 리뷰에서 반려됩니다.

**Q: 로컬에 PostgreSQL이 이미 실행 중인데 포트 충돌이 나지 않나요?**

A: `docker-compose.dev.yaml`에서 PostgreSQL은 `5433`, Redis는 `6380`으로 호스트 포트를 매핑해 기본 포트(5432, 6379)와 충돌을 피했습니다. 별도 조치 없이 `make dev`를 실행하면 됩니다.

**Q: `make run`이 DB에 연결되지 않습니다.**

A: `make dev`로 인프라가 먼저 기동되어 있어야 합니다. DB 연결에 실패한 경우 `make dev-clean && make dev`로 볼륨을 초기화한 뒤 다시 시도하세요.
