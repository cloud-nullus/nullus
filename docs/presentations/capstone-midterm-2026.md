# Nullus — 클릭 몇 번으로 완성하는 DevSecOps 플랫폼

**Cloud Nullus Team | 2026 캡스톤 중간 발표**

---

## 슬라이드 1: 타이틀 (10초)

### Nullus

**클릭 몇 번으로 완성하는 DevSecOps 플랫폼**

Cloud Nullus Team
2026 캡스톤 프로젝트 중간 발표

> github.com/cloud-nullus/draft

---

## 슬라이드 2: 팀 소개 (50초)

### 팀 소개

*"현장에서 DevOps 플랫폼 구축에 6~18개월이 걸리는 현실을 바꾸고 싶었습니다."*

| 역할 | 담당 영역 |
|------|----------|
| **BE Lead** | Go / 아키텍처 설계 / 설치 엔진 |
| **Backend** | API / DB / RBAC / Helm 검증 |
| **FE Lead** | React / Config UI / 실시간 로그 |
| **Frontend** | 대시보드 / 템플릿 / 모니터링 UI |
| **DevOps** | K8s / Helm 차트 / Golden Path 정의 |
| **Full-stack · QA** | 통합 테스트 / E2E / 문서화 / 릴리스 |

---

## 슬라이드 3: 문제 정의 (1분)

### 문제 정의 — DevOps 플랫폼 구축, 왜 이렇게 어려운가?

|  | 현재 현실 | Nullus 목표 |
|--|----------|------------|
| **구축 기간** | 6~18개월 | 수 시간 |
| **필요 인력** | 5~10명 | 1~2명 |
| **도구 선택** | GitLab vs Gitea? Prometheus vs Thanos? → 의사결정 마비 | 검증된 조합 제공 |
| **버전 호환** | ArgoCD 업그레이드 → GitLab 연동 깨짐 → 검증된 조합 부재 | 호환성 매트릭스 보장 |

**Target Persona**

- **미정** (1년차 DevOps) — 어떤 도구를 선택해야 할지 모르는 주니어
- **민수** (5년차 시니어) — 매번 반복되는 수작업에 지친 엔지니어

---

## 슬라이드 4: 우리의 답 — Golden Path (1분)

### 우리의 답 — Golden Path

검증된 OSS 조합을 **선택**하고, 노코드 UI로 **설정**하고, 원클릭으로 K8s에 **배포**

```
  ┌──────────────┐        ┌──────────────┐        ┌──────────────┐
  │  01 SELECT   │  ───▶  │ 02 CONFIGURE │  ───▶  │  03 DEPLOY   │
  │              │        │              │        │              │
  │ Golden Path  │        │ 8단계 위자드  │        │ 3-Phase Helm │
  │ 템플릿 선택   │        │ 노코드 설정   │        │ DAG 자동배포  │
  └──────────────┘        └──────────────┘        └──────────────┘
```

**핵심 메시지**: 여러 OSS를 클릭 몇 번으로 설치하고, 설치 결과가 실시간으로 보이는 도구

---

## 슬라이드 5: 핵심 기능 4축 (2분, 각 30초)

### 핵심 기능 4축

**1. Golden Path 템플릿** — 검증된 도구 조합 3종

| 템플릿 | 구성 |
|--------|------|
| GitLab All-in-One | GitLab + GitLab Runner + Prometheus + Grafana |
| GitLab + ArgoCD | GitLab + ArgoCD + GitLab Runner + Prometheus + Grafana |
| GitHub + ArgoCD | GitHub Actions + ArgoCD + Prometheus + Grafana |

각 템플릿은 호환성 매트릭스(`compatibility_matrices` 테이블)로 버전 간 pass/warn/fail 검증을 수행합니다.

**2. 스택 자동 설치** — 3-Phase Helm DAG 엔진

```
Phase A (Foundation)        Phase B (CI/CD)           Phase C (Observability)
  cert-manager                GitLab                    Prometheus
  metrics-server              ArgoCD                    Grafana
  PostgreSQL                  GitLab Runner             OpenTelemetry
  MinIO                       Webhook 연동 자동화
```

- Helm SDK를 Go에서 직접 호출 (helm.sh/helm v3.16+)
- 각 단계별 WebSocket 실시간 로그 스트리밍 (`gorilla/websocket`)
- 배포 로그 DB 영구 저장 (`deployment_logs` 테이블, step/level별)
- 실패 시 Phase 단위 롤백 지원

**3. CI/CD 파이프라인** — 템플릿 기반 앱 배포

- 6종 앱 타입: web / backend / batch + web-backend / web-frontend / batch-job
- 6단계 위자드: 앱 이름 → Git URL → 클러스터/NS → 리소스 → 환경변수 → 매니페스트 확인
- 풀 빌드 파이프라인: Git Clone → Docker Build → Kind Load → K8s Deploy 6단계 자동화
- Monaco YAML 에디터 양방향 동기화 (350ms debounce)
- Developer 역할이 스스로 배포 가능 (셀프서비스)

**4. 모니터링 & 알림** — Prometheus 연동 + AlertRule CRUD

- Prometheus 클라이언트 API (`/api/v1/observability/dashboard`)
- AlertRule 생성·수정·삭제·활성화/비활성화
- 알림 이력 조회 + Slack webhook 연동
- 감사 로그로 모든 관리 작업 추적

---

## 슬라이드 6: 아키텍처 (1분)

### 아키텍처

```
┌─────────────────────────────────────────────────────────────────┐
│                    Frontend (React 19)                           │
│       TypeScript 5.9 + Vite 8 + Tailwind CSS 4 + shadcn/ui     │
│       TanStack Query v5 + React Hook Form + Zod + Zustand       │
└──────────────────────────┬──────────────────────────────────────┘
                           │ REST API / WebSocket
┌──────────────────────────▼──────────────────────────────────────┐
│                  API Server (Go 1.26 + Echo v4)                  │
│                                                                  │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌──────────────────┐  │
│  │  Admin   │  │  Stack  │  │  CI/CD  │  │  Observability   │  │
│  │ Context  │  │ Context │  │ Context │  │    Context       │  │
│  └─────────┘  └─────────┘  └─────────┘  └──────────────────┘  │
│       │            │             │              │                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │            Auth Context (Keycloak / Authentik OIDC)      │   │
│  │           + DualAuth Middleware + RBAC 3역할              │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Helm SDK Orchestrator │ K8s client-go │ WebSocket Streamer     │
└──────┬──────────┬──────────┬──────────────────────┬─────────────┘
       │          │          │                      │
  PostgreSQL   Redis      MinIO              K8s Cluster(s)
    18+       (캐시)    (오브젝트 스토리지)      1.26+
```

**설계 원칙**

| 원칙 | 적용 |
|------|------|
| Clean Architecture | Handler → UseCase → Domain → Infrastructure 4계층 |
| Modular Monolith | 5 Bounded Context + shared, 모듈별 테이블 소유, 마이크로서비스 전환 가능 |
| DDD | Aggregate Root 단위 Repository, 도메인 이벤트, Ubiquitous Language |
| 암호화 기본 | kubeconfig AES-256-GCM 암호화 저장 |

---

## 슬라이드 7: 구현 현황 (1분)

### 구현 현황 — v0.2.0-alpha (2026.03.30 릴리스)

**Backend (Go)** — `internal/` 모듈별 구현 완료

| 모듈 | 패키지 | 핵심 구현 |
|------|--------|----------|
| Admin | `internal/admin/` | Org CRUD, User 관리, Cluster 등록, Known Issues, 감사 로그 |
| Stack | `internal/stack/` | Config 8단계, 3-Phase Helm 배포, 버전 이력, 롤백 |
| CI/CD | `internal/cicd/` | 파이프라인 템플릿, 풀 빌드 파이프라인, 배포, 이력 관리 |
| Observability | `internal/observability/` | AlertRule CRUD, Prometheus 클라이언트, 대시보드 |
| Auth | `internal/auth/` | Keycloak/Authentik OIDC 추상화, JWT, DualAuth 미들웨어 |
| Template | `internal/template/` | Golden Path 3종, CI/CD 템플릿 3종 |
| Compatibility | `internal/compatibility/` | 버전 호환성 매트릭스 (pass/warn/fail) |

**Frontend (React 19)** — 주요 페이지 구현 완료

| 영역 | 구현 내용 |
|------|----------|
| Home | 역할별 대시보드 |
| Admin | Org 관리, User 관리, Cluster 관리, Known Issues, 감사 로그 |
| Stack | 8단계 Config 위자드, Golden Path 선택, WebSocket 실시간 로그, 이력 |
| CI/CD | 6단계 Deploy 위자드, 템플릿 선택, 매니페스트 편집, 배포 모니터링, 상세 4탭 (Info/Monitoring/History/Actions) |
| Observability | Prometheus 대시보드, Alert 규칙 CRUD, Alert 이력 |

**Database** — 39개 마이그레이션 완료 (UUID v7, JSONB, 소프트 삭제)

**테스트** — Go 테스트 395개 함수 + Playwright E2E 107개 시나리오

**Infrastructure** — Helm 차트, Docker Compose (dev/auth), kind 클러스터 E2E

---

## 슬라이드 8: 12개 기능 체크리스트 (30초)

### v0.2.0-alpha 기능 구현 현황

| # | 기능 | 상태 |
|---|------|------|
| F0 | Org 관리 (CRUD, 멤버 초대, 역할 할당) | ✅ 완료 |
| F1 | 클러스터 등록 (kubeconfig 암호화, 연결 검증) | ✅ 완료 |
| F2 | 노코드 설정 UI (8단계 위자드) | ✅ 완료 |
| F3 | Golden Path 3종 (GitLab AiO / GitLab+Argo / GitHub+Argo) | ✅ 완료 |
| F4 | 3-Phase 설치 엔진 (Helm DAG, WebSocket 로그) | ✅ 완료 |
| F5 | CI/CD 파이프라인 템플릿 (6종 앱 타입) | ✅ 완료 |
| F6 | CI/CD 배포 + 이력 관리 + 풀 빌드 파이프라인 | ✅ 완료 |
| F7 | 모니터링 대시보드 + AlertRule CRUD | ✅ 완료 |
| F8 | 호환성 매트릭스 (pass/warn/fail 검증) | ✅ 완료 |
| F9 | RBAC 3역할 (Admin/DevOps/Developer) + OIDC | ✅ 완료 |
| F10 | 리소스 계산기 (CPU/메모리 추정) | ✅ 완료 |
| F11 | Known Issues 레지스트리 (자동 진단) | ✅ 완료 |
| F12 | 감사 로그 (전체 관리 작업 추적) | ✅ 완료 |
| F13 | Go 테스트 395건 + Playwright E2E 107건 | ✅ 완료 |

---

## 슬라이드 9: 데모 시나리오 (7분)

### 라이브 데모 — 7분 하이브리드 (라이브 + 녹화 2배속)

**구간 A — 클러스터 등록 (1분 30초, 라이브)**

```
Admin 로그인 → Admin > Clusters 진입
→ "+ 클러스터 추가" 클릭
→ kubeconfig 붙여넣기 (사전 준비)
→ 클러스터 이름 입력, 타입 선택 (pipeline)
→ "연결 테스트" 클릭 → 🟢 연결 성공 + 네임스페이스 자동 로드
```

> "kubeconfig는 AES-256-GCM으로 암호화 저장되고, 연결 검증까지 자동으로 수행됩니다."

**구간 B — 스택 배포 (3분, 라이브 → 녹화 전환)**

```
라이브:
→ Stack > Templates → "GitLab All-in-One" Golden Path 선택
→ 8단계 위자드 진행 (Artifacts → CI/CD → Observability → Resources → Storage → YAML View → Deploy Script → Dry Run)
→ "Deploy" 클릭 → WebSocket 로그 스트리밍 시작

녹화 영상 (2배속):
→ Phase A: cert-manager, metrics-server, PostgreSQL, MinIO 설치
→ Phase B: GitLab, ArgoCD, Runner 설치
→ Phase C: Prometheus, Grafana, OpenTelemetry 설치
→ ✅ 배포 완료
```

> "3-Phase DAG로 Foundation → CI/CD → Observability 순서로 자동 설치됩니다."

**구간 C — CI/CD 파이프라인 (1분 30초, 라이브)**

```
→ CI/CD > Templates → Backend 템플릿 선택
→ 6단계 위자드: 앱 이름 → Git URL → 클러스터/NS → 리소스 → 환경변수 → 매니페스트 확인
→ "Deploy" 클릭 → 6단계 실시간 진행: Git Clone → Docker Build → Image Load → Namespace → Deployment → Service
→ 배포 완료 → 생성된 K8s 리소스 목록 확인
```

> "Git clone부터 Docker build, K8s 배포까지 풀 빌드 파이프라인이 자동으로 실행됩니다."

**구간 D — 모니터링 & 관리 (1분, 라이브)**

```
→ Observability > Monitoring: Prometheus 대시보드
→ Observability > Alerts: 알림 규칙 CRUD
→ Admin > Clusters: 클러스터 상태 확인
→ Admin > Users: 멤버 관리 (Admin/DevOps/Developer 역할)
```

> "RBAC으로 Admin · DevOps · Developer 3역할이 분리되어 있습니다."

---

## 슬라이드 10: 기술적 도전과 해결 (1분)

### 기술적 도전과 해결

**1. Helm 설치 순서 의존성 → 3-Phase DAG 엔진**

도전: 20개 이상의 OSS 도구는 설치 순서가 있고, 일부는 선행 도구에 의존합니다.
해결: Foundation → CI/CD → Observability 3단계 DAG를 구현하여 의존성을 보장합니다. Helm SDK를 Go에서 직접 호출하여 단일 프로세스에서 제어합니다.

**2. Kubeconfig 보안 → AES-256-GCM 암호화**

도전: kubeconfig에는 클러스터 접근 크리덴셜이 포함되어 평문 저장이 불가합니다.
해결: `ENCRYPTION_KEY` 환경변수 기반 AES-256-GCM으로 암호화 후 DB에 저장합니다. 복호화는 런타임에서만 수행합니다.

**3. 실시간 배포 상태 전달 → WebSocket 스트리밍**

도전: Helm 배포는 10~15분 소요되며, 사용자는 진행 상황을 알 수 없습니다.
해결: `gorilla/websocket`으로 Helm 설치 로그를 실시간 스트리밍합니다. 동시에 `deployment_logs` 테이블에 영구 저장하여 이후 조회도 가능합니다.

**4. OIDC 프로바이더 다양성 → 추상화 인터페이스**

도전: Keycloak과 Authentik은 역할 클레임 구조가 다릅니다 (realm_access vs groups).
해결: `OIDCProvider` 인터페이스를 정의하고, 각 프로바이더별 구현체를 분리했습니다. DualAuth 미들웨어로 개발 환경과 프로덕션 환경을 전환합니다.

**5. 기존 파이프라인과 빌드 파이프라인 공존 → 옵셔널 DI**

도전: 기존 매니페스트 배포 파이프라인에 Docker 빌드를 추가하면서 기존 동작을 깨뜨리면 안 됩니다.
해결: `WithImagePreparer` 옵셔널 DI로 빌드 단계를 주입합니다. `dockerfile_path`가 없으면 기존 흐름, 있으면 Git Clone → Docker Build → Kind Load → K8s Deploy 6단계를 실행합니다.

---

## 슬라이드 11: 현재 고민 & 향후 계획 (2분)

### 현재 고민

| 고민 | 현황 | 목표 |
|------|------|------|
| 설치 성공률 | 측정 중 | Alpha ≥70% → Beta ≥85% → GA ≥90% |
| 테스트 커버리지 | Go 395건 + E2E 107건 | Alpha ≥30% → Beta ≥50% → GA ≥70% |
| OIDC 프로덕션 검증 | 로컬 Keycloak 테스트 완료 | Beta까지 실환경 SSO 검증 |
| 프로덕션 배포 가이드 | Day 0 체크리스트 작성 완료 | GA까지 3개 이상 조직 검증 |

### 설계 대비 미구현 항목 (알파 단계 의도적 스코프 조정)

| 항목 | 설계 | 현재 | 계획 |
|------|------|------|------|
| `/api/v1/auth` REST 엔드포인트 | 전체 세트 | 최소 구현 | Beta에서 보완 |
| 세션 기반 인증 | gorilla/sessions | X-User-* 헤더 | 프로덕션에서 JWT 전환 |
| OIDC 프론트엔드 콜백 | 전체 흐름 | 플레이스홀더 | Beta에서 완성 |
| retry/timeout 상태 | 전체 상태 머신 | 부분 구현 | GA에서 완성 |

### 로드맵

```
v0.2-alpha ✅          v0.2-beta              v1.0 GA
2026.03.30             2026.04.27             2026.05.25
    │                      │                      │
    ├─ 13개 핵심 기능       ├─ 테스트 ≥50%         ├─ 테스트 ≥70%
    ├─ Golden Path 3종     ├─ E2E CI 자동화       ├─ 3개 조직 검증
    ├─ 풀 빌드 파이프라인   ├─ SSO 실환경 검증     ├─ 멀티 클러스터
    └─ 기본 RBAC           └─ 성공률 ≥85%        └─ 성공률 ≥90%
```

### 장기 비전

| Phase | 시기 | 핵심 기능 |
|-------|------|----------|
| **Phase 2: DevSecOps** | 2026 Q3-Q4 | SAST/DAST 통합, 보안 스캐닝, nullus-cli |
| **Phase 3: InfraOps** | 2027+ | 클러스터 프로비저닝, 멀티 클러스터, IaC 통합 |
| **CNCF Sandbox** | 2027 Q2 | 5,000+ Stars, 500+ Monthly Installs |

---

## 슬라이드 12: 클로징 (30초)

### DevOps 플랫폼 구축의 진입장벽을 낮추는 오픈소스

# Nullus

github.com/cloud-nullus/draft

**감사합니다 — Q&A**

---

## Q&A 대비 예상 질문

### 기본 질문 (발표 직후 빈출)

| 예상 질문 | 답변 포인트 |
|-----------|------------|
| 기존 도구(Jenkins X, Backstage)와 차이점? | Nullus는 **설치 자동화**에 집중. Backstage는 개발자 포털 → 향후 Backstage 플러그인 전환 가능. Jenkins X는 CI/CD에 특화, Nullus는 전체 DevOps 스택 프로비저닝 |
| 대규모 클러스터에서 성능은? | Alpha 단계, 단일 클러스터 검증 완료. v1.0에서 멀티 클러스터 지원 예정. Helm SDK 직접 호출로 중간 레이어 없이 최적 성능 |
| 왜 Go를 선택? | client-go 네이티브, 단일 바이너리 배포, Helm SDK 직접 호출, goroutine 기반 동시 설치 |
| 보안은 어떻게? | kubeconfig AES-256-GCM 암호화, Keycloak/Authentik OIDC SSO, RBAC 3역할, TLS 1.3, 감사 로그 전수 기록 |
| 오픈소스 라이선스? | Apache 2.0 예정, CNCF Sandbox 제출 목표 |
| Golden Path가 안 맞으면? | 커스텀 구성 가능 — 도구 개별 추가/제거, Monaco 에디터로 YAML 직접 편집 |
| 테스트는 어떻게 하고 있나? | Go: testify + testcontainers, FE: Vitest + testing-library, E2E: Playwright 107 시나리오, 통합: kind 클러스터 |
| Clean Architecture를 선택한 이유? | 모듈 간 독립성 보장, Repository 인터페이스로 DB 교체 용이, 테스트 시 외부 의존 모킹 가능 |
| DDD Bounded Context 분리 기준? | PRD 기능 영역 기반: Admin(조직/유저/클러스터), Stack(설정/배포), CI/CD(파이프라인), Observability(모니터링/알림), Auth(인증/인가). 각 Context가 자신의 테이블만 소유 |
| 설치 실패 시 어떻게 대응? | Phase 단위 롤백 + Known Issues 레지스트리 자동 진단 + WebSocket 로그로 실패 지점 즉시 파악 |

### 클라우드 엔지니어 — 초급

| 예상 질문 | 답변 포인트 |
|-----------|------------|
| Golden Path가 뭔가요? 왜 필요한가요? | Netflix/Spotify에서 시작된 개념. 검증된 도구 조합의 "고속도로"를 제공해 의사결정 피로를 줄이고, 조직 표준을 만들어 온보딩을 단축. 원하면 커스텀도 가능 |
| Helm이 뭔지, 왜 Helm을 사용하나요? | K8s 패키지 매니저. apt/brew처럼 복잡한 K8s 리소스를 하나의 차트로 묶어 설치/업그레이드/롤백. OSS 도구 대부분이 Helm 차트를 공식 제공하므로 사실상 표준 |
| Docker Compose 대신 K8s를 선택한 이유? | Docker Compose는 단일 노드용. 프로덕션 DevOps 플랫폼은 HA, 자동 복구, 오토스케일이 필요 → K8s가 표준. Nullus는 K8s 위에서 동작하는 플랫폼 |
| kind 클러스터는 프로덕션에서도 쓰나요? | 아니요. kind는 로컬 개발/E2E 테스트용. 프로덕션은 EKS/GKE/AKS 같은 매니지드 K8s를 사용. Nullus는 kubeconfig만 등록하면 어떤 클러스터든 지원 |
| RBAC이 뭐고, 왜 3개 역할로 나눴나요? | Role-Based Access Control. Admin(플랫폼 관리), DevOps(스택 설치/관리), Developer(앱 배포 셀프서비스) — 최소 권한 원칙으로 실수와 보안 사고를 방지 |

### 클라우드 엔지니어 — 중급

| 예상 질문 | 답변 포인트 |
|-----------|------------|
| 멀티 클러스터 지원 계획은? | 현재 Alpha에서 단일 클러스터 배포 검증 완료. v1.0 GA에서 멀티 클러스터 지원 예정. 이미 `clusters` 테이블과 `ClusterTargetProvider` 포트가 복수 클러스터를 고려한 설계 |
| GitOps(pull 방식) vs 현재 push 방식의 차이점은? | Nullus는 현재 push 방식(kubectl apply). ArgoCD가 Golden Path에 포함되어 있어 ArgoCD 설치 후에는 GitOps 전환 가능. W5 로드맵에 Webhook 트리거(push→merge→자동배포) 예정 |
| Helm 설치 중 실패하면 어떻게 되나요? | Phase 단위 롤백. `RollbackManager`가 설치된 릴리스를 LIFO 순서로 `helm uninstall`. WebSocket으로 실패 지점 즉시 확인. `deployment_logs` 테이블에 영구 보존되어 사후 분석 가능 |
| kubeconfig 저장 방식의 보안 수준은? | AES-256-GCM으로 암호화 후 DB 저장. 복호화 키는 `ENCRYPTION_KEY` 환경변수로 런타임에만 제공. DB 유출 시에도 kubeconfig 평문 노출 불가. 추가로 TLS 1.3으로 전송 암호화 |
| Prometheus가 이미 설치되어 있는 기존 클러스터에도 적용 가능한가요? | 가능. Golden Path의 Observability Phase는 옵셔널. 기존 Prometheus가 있으면 해당 Phase를 스킵하고, Nullus 대시보드에서 기존 Prometheus를 쿼리할 수 있도록 엔드포인트만 연동 |

### 클라우드 엔지니어 — 고급

| 예상 질문 | 답변 포인트 |
|-----------|------------|
| Helm DAG 엔진에서 동일 Phase 내의 병렬 설치는 지원하나요? | 현재 Phase 내부는 순차 설치. Phase 간 의존성만 DAG로 보장. Phase 내 병렬화는 goroutine + errgroup으로 구현 가능하며 Beta 로드맵에 포함 |
| etcd 부하나 API server throttling 이슈는 고려하고 있나요? | Alpha에서는 단일 클러스터 기준 설치 속도에 문제 없음. 대규모 클러스터에서는 Helm SDK의 `--wait` 타임아웃 조정과 `client-go` rate limiter 설정으로 대응 예정 |
| CRD 의존성은 어떻게 처리하나요? | Phase A에서 cert-manager 등 CRD 제공자를 먼저 설치. CRD가 Ready 상태인지 확인 후 Phase B 진행. BackendTLSPolicy CRD가 없는 클러스터에서는 해당 매니페스트를 자동 스킵하는 로직 구현 완료 |
| Terraform/Pulumi 같은 IaC 도구와의 차별점은? | Terraform은 인프라 프로비저닝(VM, VPC, EKS 생성), Nullus는 인프라 위의 DevOps 도구 설치 자동화. 상호 보완적. Phase 3 로드맵에서 Terraform과 통합하여 클러스터 프로비저닝 → Nullus 자동 설치 파이프라인 계획 |
| Operator 패턴 대신 Helm을 선택한 이유는? | Operator는 도구별 커스텀 컨트롤러 개발이 필요해 유지보수 비용이 큼. Helm은 대부분의 CNCF 프로젝트가 공식 차트를 제공하므로 신규 도구 추가가 `values.yaml` 수준에서 가능. 운영 복잡도를 최소화하면서 20+ 도구를 지원하는 데 Helm이 적합 |
| 서비스 메시(Istio/Linkerd) 통합 계획은? | Phase 3 InfraOps 로드맵에 포함. 현재 Gateway API + HTTPRoute 기반 인그레스를 구현해 둬서, Istio Gateway 전환이 비교적 용이. Service mesh는 설치 복잡도가 높아 별도 Golden Path 템플릿으로 제공 예정 |

### 개발자 — 초급

| 예상 질문 | 답변 포인트 |
|-----------|------------|
| 개발자 역할로 로그인하면 뭘 할 수 있나요? | CI/CD 템플릿으로 앱 배포 셀프서비스. 6단계 위자드로 앱 이름 → Git URL → 클러스터 → 리소스 → 환경변수 → 매니페스트 확인 후 원클릭 배포. 클러스터 관리나 스택 설치는 Admin/DevOps만 가능 |
| React 19에서 새로 나온 기능을 사용하셨나요? | React 19의 use() 훅과 개선된 Suspense를 라우트 레벨 코드 스플리팅에 활용. TanStack Query v5와 조합해 서버 상태 관리. Zustand로 클라이언트 상태 분리 |
| 프론트엔드 테스트는 어떻게 하나요? | Vitest + React Testing Library로 컴포넌트 단위 테스트. Playwright로 E2E 107개 시나리오. MSW(Mock Service Worker)로 API 모킹. CI에서 매 PR마다 자동 실행 |
| WebSocket 실시간 로그는 어떻게 구현했나요? | 백엔드: gorilla/websocket + StepTracker(pub/sub 패턴)가 Helm 설치 진행 상태를 구독자에게 실시간 push. 프론트엔드: `useCicdDeployLog` 커스텀 훅이 WebSocket 연결, 로그/진행률/상태를 React 상태로 관리 |
| Clean Architecture가 초보자한테 너무 복잡하지 않나요? | 초기 학습 곡선은 있지만, 모듈별 독립성 덕분에 "내가 담당하는 모듈만" 이해하면 됨. Repository 인터페이스로 DB를 몰라도 UseCase 테스트가 가능하고, 이것이 395개 Go 테스트의 기반 |

### 개발자 — 중급

| 예상 질문 | 답변 포인트 |
|-----------|------------|
| Go에서 Helm SDK를 직접 호출하는 것과 CLI를 exec하는 것의 차이는? | SDK 직접 호출: 타입 안전성, 에러 핸들링, 메모리 내 values 전달. CLI exec: 프로세스 생성 오버헤드, stdout 파싱 필요, 보안 위험(인자 노출). Helm SDK가 성능·보안·제어 모두 우월 |
| 5개 모듈 간 통신은 어떻게 하나요? | 포트 인터페이스를 통한 의존성 주입. 예: CI/CD 모듈이 Stack 정보가 필요할 때 `StackReader` 인터페이스를 import하고, 런타임에 Stack 모듈이 구현체를 제공. 직접 import는 금지 |
| DB 마이그레이션 전략은? | golang-migrate 사용. `db/migrations/` 에 순번 파일 관리 (현재 39번까지). Up/Down 양방향 지원. `runbook_local.sh up` 시 자동 실행. 프로덕션은 Helm pre-install Job으로 처리 |
| API 에러 핸들링 패턴은? | Domain 레이어에서 커스텀 에러 타입 정의(`errors.go`). UseCase에서 비즈니스 에러를 반환. Handler에서 HTTP 상태 코드로 변환. 일관된 JSON 에러 응답 포맷 (`{"error": "...", "code": "..."}`) |
| 프론트엔드 상태 관리 전략은? | 서버 상태는 TanStack Query (캐싱, 재시도, 리페치). 클라이언트 상태는 Zustand (인증, UI 상태). 폼 상태는 React Hook Form + Zod 스키마 검증. WebSocket 상태는 커스텀 훅으로 분리 |

### 개발자 — 고급

| 예상 질문 | 답변 포인트 |
|-----------|------------|
| Modular Monolith에서 마이크로서비스 전환은 구체적으로 어떻게? | 각 Bounded Context가 독립된 Repository와 테이블을 소유. 모듈 간 통신이 이미 인터페이스 기반. 전환 시: 인터페이스 구현체를 gRPC/HTTP 클라이언트로 교체, 모듈을 별도 바이너리로 빌드, 각 모듈에 자체 DB 할당 |
| 이벤트 기반 아키텍처를 도입할 계획이 있나요? | 현재는 동기 호출 기반. 도메인 이벤트(`StackDeployed`, `PipelineCreated`)를 설계에는 정의해 두었으나 아직 비동기 발행은 미구현. Beta에서 Redis Pub/Sub 또는 NATS JetStream으로 모듈 간 이벤트 전파 예정 |
| API 버전 관리 전략은? | 현재 `/api/v1/` 단일 버전. Breaking change 시 `/api/v2/` 를 추가하고 v1은 deprecation period 유지. Go Echo의 라우트 그룹으로 버전별 핸들러 분리. OpenAPI 스펙 자동 생성은 GA 목표 |
| testcontainers-go로 통합 테스트를 어떻게 작성하나요? | PostgreSQL testcontainer를 테스트 시작 시 자동 생성 → 마이그레이션 적용 → 테스트 실행 → 컨테이너 자동 정리. 실제 DB 쿼리를 검증하므로 mock보다 신뢰도 높음. 현재 E2E 17개 테스트에서 사용 중 |
| DDD Aggregate 설계에서 어려웠던 점은? | Stack Aggregate가 Config(8단계 설정) + Deployment(배포 상태) + History(이력)를 포함하면서 비대해짐. Config를 Value Object로 분리하고, DeploymentLog를 별도 Aggregate로 추출(`deployment_logs` 테이블). 일관성 경계 설정이 핵심 과제였음 |
| 동시에 여러 사용자가 같은 클러스터에 배포하면? | 현재: 네임스페이스 수준 격리로 충돌 방지. 동일 네임스페이스 동시 배포 시에는 K8s의 optimistic concurrency(resourceVersion)에 의존. 향후: 배포 큐 + 락 메커니즘 도입 예정 (Redis 기반 distributed lock) |
| CNCF Sandbox 제출 기준을 충족하려면? | Sandbox 최소 요건: 오픈소스 라이선스(Apache 2.0), 2+ 커밋터, 보안 정책(SECURITY.md), CLA. 경쟁력 확보에는 5,000+ GitHub Stars, 활발한 커뮤니티, 프로덕션 사용 사례 3건 이상이 필요. Phase 3 목표 |
