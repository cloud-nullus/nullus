# Nullus 백엔드 API 구현 및 프론트엔드 연동 작업 지시서

## 1. 프로젝트 및 작업 개요
이 프롬프트는 Nullus 플랫폼의 백엔드 API(Go 기반)를 전면 구현하고, 웹 프론트엔드(React/Vite)와 성공적으로 연동하기 위한 에이전트 작업 지시서입니다.

- **프로젝트 명:** Nullus (Kubernetes 기반 DevSecOps 자동화 오픈소스 플랫폼)
- **백엔드 스택:** Go 1.24+, Echo v4, PostgreSQL 18+ (Clean Architecture 패턴 적용)
- **프론트엔드 스택:** React 19, TypeScript, Vite, Tailwind CSS 4, shadcn/ui, TanStack Query, Zustand
- **주요 도메인 (Bounded Contexts):** 
  - `admin/` (Organization, K8s Cluster, User 관리)
  - `auth/` (Role-based UI, OIDC/Keycloak 연동)
  - `stack/` (Golden Path 템플릿, 노코드 설정 저장, 자동 설치 오케스트레이션)
  - `cicd/` (CI/CD 파이프라인 배포 및 이력 관리)
  - `observability/` (모니터링 대시보드 메트릭, Slack 알림 등)

## 2. 에이전트 작업 목표
당신은 백엔드 리드 엔지니어이자 풀스택 통합 전문가로서 다음을 수행해야 합니다.
1. 정의된 PRD(기능명세)와 README의 엔드포인트 목록을 바탕으로 Go (Echo) 백엔드의 Controller(Handler), Usecase, Repository 레이어를 구현하십시오.
2. 프론트엔드(TanStack Query)와의 원활한 연동을 위해 API 명세(OpenAPI 3.0 yaml) 및 페이로드 타입(Typescript 인터페이스)을 정의하십시오.
3. Kubeconfig 업로드, 실시간 설치 로그(WebSocket/SSE) 전송 등 프론트-백 연동의 핵심 병목을 해결할 아키텍처 로직을 작성하십시오.

## 3. 백엔드 API 상세 구현 범위
다음 API들을 `cmd/api/` 진입점을 바탕으로 `internal/` 내 각 도메인 폴더에 클린 아키텍처 구조로 작성하세요.

### 3.1. Admin & Auth Domain (`internal/admin`, `internal/auth`)
- `GET/POST /api/v1/orgs`: Organization 생성 및 조회 (단일 Admin 형태 및 초대 로직 포함)
- `GET/PUT /api/v1/orgs/:id`: Organization 상세 조회 및 수정
- `GET/POST /api/v1/clusters`: K8s 클러스터 Kubeconfig 업로드, 검증(`client-go` 활용), 암호화 저장
- `POST /api/v1/clusters/:id/verify`: 클러스터 헬스체크 및 연결 상태 검증 (kube-system 핑 테스트)

### 3.2. Stack Management Domain (`internal/stack`)
- `GET /api/v1/templates`: 3가지 Golden Path 템플릿(GitHub, GitLab 중심) 목록 및 명세 응답
- `GET/POST /api/v1/stacks`: 노코드 UI(5단계)에서 설정한 도구 버전, 리소스 예상량 등 Configuration JSON 저장 및 조회
- `POST /api/v1/stacks/:id/deploy`: 오케스트레이터를 호출하여 파이프라인 자동 설치 시작 (분리된 워커 고루틴 활용)
- `GET /api/v1/stacks/:id/status`: WebSocket 또는 SSE를 통한 설치 실시간 프로그레스 및 텍스트 로그 스트리밍 엔드포인트 제공

### 3.3. CI/CD & Observability Domain (`internal/cicd`, `internal/observability`)
- `GET/POST /api/v1/pipelines`: 파이프라인 템플릿 기반 생성 및 설정(Config) 저장
- `POST /api/v1/pipelines/:id/deploy`: 앱 파이프라인 K8s 매니페스트 배포 트리거 (Deployment, Svc, Ingress Object 생성)
- `GET /api/v1/pipelines/:id/deployments`: 배포 이력(버전 지정 롤백 및 상태 기록) 조회
- `GET /api/v1/monitoring/dashboard`: 홈 메뉴 및 모니터링 메뉴용 Prometheus 집계 메트릭 전달용 백엔드 집계 API

## 4. 프론트엔드-백엔드 연동 개발 계획 및 지침
프론트엔드 에이전트와 백엔드의 병렬 진행을 위해, 다음 단계적 연동 지침을 엄수하십시오.

### **Phase 1: API 스펙 확정 및 프론트엔드 통합 모킹 (1일차)**
- **행동 지시:** 프로젝트 루트에 `api/openapi.yaml` (OpenAPI 3.0 포맷) 스펙 문서 초안을 작성하십시오. 
- 프론트엔드가 페칭 로직을 선반영할 수 있도록, Mock 서버 세팅(선택)을 위한 Response JSON 예시를 주석 혹은 문서를 통해 제공하십시오.
- 프론트엔드 측 연동을 위해 `web/src/lib/api.ts` 에 Axios 호출 래퍼 함수들과 그에 매핑되는 응답 모델(`web/src/types/api.ts`) 틀을 구성하십시오.

### **Phase 2: 코어 비즈니스 로직 및 DB (PostgreSQL) 연동 (2~3일차)**
- **행동 지시:** Go Echo 핸들러, UseCase, DB 쿼리를 GORM (또는 sqlc)을 이용한 Repository 구현체로 구성하십시오.
- Postgres Migration 파일(`/db/migrations/`)을 생성하여 Organization, Cluster, Stack 스키마를 초기화하십시오.
- Kubeconfig에 대한 AES-256-GCM 양방향 암호화/복호화 로직을 `internal/shared` 혹은 `pkg/` 층에 작성하십시오.

### **Phase 3: 실시간 연동 (스트리밍) 적용 및 상태 동기화 (4일차)**
- **행동 지시:** 긴 처리 시간이 소요되는 스택 배포(`Deploy` 버튼 클릭)를 위해, WebSocket을 이용한 스트리밍 로그 및 배포율(Progress Bar) 연동을 구현하십시오.
- 프론트엔드는 React의 `useEffect` 훅 내에서 WebSocket 인스턴스를 관리하며, 수신된 실시간 로그를 배열로 누적하여 `CodePreview` 컴포넌트(터미널 UI)에 자동 Append/Scroll 하는 로직을 작성하십시오.

## 5. 제약 사항 및 품질 기준
- **오류 처리 규격:** 모든 실패는 통일된 에러 스펙 (`{ "error": "...", "code": ... }`) 포맷으로 반환해야 하며, 클라이언트는 HTTP Status Code(400, 401, 403, 404, 500)에 따라 Zustand나 Toast 알림으로 처리합니다.
- **RESTful 가이드:** 리소스 지향적 URL 및 명확한 메소드 (GET, POST, PUT, DELETE)를 엄수하세요. RPC 형태의 호출은 상황에 맞춰 한정적으로 사용합니다.
- **클린 아키텍처:** Controller(Handler)에서 비즈니스 로직(DB 질의, 복잡한 검증 등)을 직접 처리하지 말고 UseCase로 책임 위임(DI 적용)하여 단일 책임 원칙을 보장하십시오.

---
**[실행 트리거: 해당 프롬프트를 받은 상대 에이전트는 아래 지시부터 즉시 수행합니다]**
"위의 요구사항을 숙지했다면, 가장 우선적으로 전체 API 레이아웃을 정의하는 `api/openapi.yaml` 스펙 초안과 K8s 클러스터 Kubeconfig 등록/검증을 지원하는 `internal/admin/` 도메인의 핸들러 및 유스케이스 골격을 작성하는 것으로 첫 번째 모듈 작업을 시작해 주세요."
