# Nullus UI/UX 구현 계획

**작성일**: 2026-03-14
**기반 문서**: proto4 화면설계, Nullus PRD v1.2, 기능분해도, 메뉴체계
**대상 독자**: 프론트엔드 엔지니어, 디자이너

---

## 1. 프로젝트 개요

| 항목 | 내용 |
|------|------|
| **제품** | Nullus - Kubernetes 기반 DevSecOps 자동화 플랫폼 |
| **유형** | Admin Dashboard / SaaS Platform |
| **사용자** | Admin, DevOps Engineer, Developer (3 역할) |
| **화면 수** | 15개 페이지 + 공통 레이아웃 + 공통 화면(로그인, 홈) |
| **기반** | proto4 와이어프레임 (HTML/CSS/JS 프로토타입) |
| **소스 경로** | `/기획단계/아키텍처/화면설계/proto4/` |

---

## 2. 기술 스택

| 레이어 | 선택 | 근거 |
|--------|------|------|
| **프레임워크** | React 19 + TypeScript | PRD 아키텍처 문서 명시 (Nullus Web UI: React) |
| **빌드** | Vite | 빠른 HMR, React 생태계 표준 |
| **스타일** | Tailwind CSS 4 | proto4 CSS 변수 체계와 호환, 유틸리티 기반 |
| **컴포넌트** | shadcn/ui | Radix 기반, 커스텀 용이, 다크 모드 네이티브 |
| **상태 관리** | Zustand | 경량, 역할/테마/사이드바 전역 상태 |
| **라우팅** | React Router v7 | SPA 네비게이션, 역할 기반 가드 |
| **폼** | React Hook Form + Zod | 5단계 스택 설치 워크플로우 등 복잡 폼 |
| **테이블** | TanStack Table | 스택 목록, 사용자 관리 등 데이터 테이블 |
| **차트** | Recharts | 모니터링 대시보드 (CPU, 메모리, 파이프라인) |
| **아이콘** | Lucide React | SVG 아이콘 (proto4의 Font Awesome 대체) |
| **i18n** | react-i18next | proto4의 en/ko 다국어 체계 계승 |
| **API 통신** | TanStack Query | REST API 캐싱 + WebSocket 실시간 상태 |

---

## 3. 프로젝트 구조

```
src/
  app/
    layout.tsx            # AppShell (사이드바 + 헤더 + 메인 영역)
    routes.tsx            # React Router 설정, 역할 기반 가드
  components/
    ui/                   # shadcn/ui 기반 기본 컴포넌트 (Button, Input, Select 등)
    layout/               # Sidebar, Header, PageHeader
    shared/               # DataTable, Modal, StepWizard, CodePreview, StatusBadge
  features/
    auth/                 # 로그인, 역할 관리
    stack/                # 템플릿, 설치, 목록, 버전
    cicd/                 # 템플릿, 목록, 이력, 앱 배포
    observability/        # 대시보드, 알림 규칙, 알림 이력
    admin/                # 조직, 사용자, 클러스터
    home/                 # 홈 대시보드
  stores/
    auth.ts               # 역할, 인증 상태 (Admin/DevOps/Developer)
    theme.ts              # 다크/라이트 테마
    sidebar.ts            # 사이드바 접기/펼치기 상태
  i18n/
    en.json               # 영문 번역
    ko.json               # 한글 번역
  lib/
    api.ts                # Axios/fetch API 클라이언트
    ws.ts                 # WebSocket 클라이언트 (설치 로그, 모니터링)
  types/
    index.ts              # 공통 타입 정의
```

---

## 4. 페이지별 구현 계획

### Phase 1: 공통 레이아웃 + 인증 (1~2주차)

| # | 페이지/컴포넌트 | proto4 소스 | 핵심 구현 사항 |
|---|----------------|-------------|----------------|
| 1 | **AppShell** | `index.html` 사이드바 영역 | 사이드바(240px/64px 접기) + 헤더(56px) + 메인 영역 |
| 2 | **역할 전환기** | `role-toggle--three` | Zustand 역할 상태, `applyRole()` 메뉴 가시성 제어 |
| 3 | **테마 토글** | 기존 다크/라이트 토글 | CSS 변수 기반 테마 전환, localStorage 영속화 |
| 4 | **다국어 전환** | `i18n.js` (30KB) | react-i18next, en/ko JSON 분리, `data-i18n` 키 매핑 |
| 5 | **로그인 페이지** | (proto4 미구현) | Keycloak OIDC 연동 준비, 폼 UI |
| 6 | **홈** | `pages/home.js` | 역할별 요약 대시보드, CTA 버튼 (스택 시작/CI/CD) |

**역할별 초기 화면:**
- Admin → 조직(Organization) 페이지
- DevOps Engineer → 스택 설치(Install) 페이지
- Developer → CI/CD 템플릿 페이지

**역할별 메뉴 가시성:**

| 역할 | 표시 메뉴 |
|------|-----------|
| Admin | 관리(조직, 사용자 관리, 클러스터 관리), 사용자(로그아웃) |
| DevOps Engineer | 데브섹옵스 스택, CI/CD, 관측성, 관리, 사용자 (전체) |
| Developer | CI/CD, 관측성, 사용자 (Admin/DevSecOps 스택 숨김) |

### Phase 2: DevSecOps 스택 (3~5주차) — 핵심 기능

| # | 페이지 | proto4 소스 | 핵심 구현 사항 |
|---|--------|-------------|----------------|
| 7 | **스택 템플릿** | `pages/stack-template.js` | Golden Path 카드 목록, 상세 모달, "Use This Template" CTA |
| 8 | **스택 설치** | `pages/stack-install.js` | **5단계 워크플로우**: Artifacts → Pipeline → Monitoring → Logging → Resources |
| 9 | **스택 목록** | `pages/stack-list.js` | DataTable + 필터/검색/정렬, 상태 배지 |
| 10 | **스택 버전 관리** | `pages/stack-version.js` | OSS 호환성 매트릭스 테이블 |

**스택 설치 상세 (핵심 페이지):**

```
┌─────────────────────────────────────────────────────────────┐
│  Resource Allocation Compare (상단)                          │
│  ┌──────────────┬──────────────┬──────────────┐             │
│  │ Cluster      │ Config       │ Resource     │             │
│  │ Config       │ Summary      │ Allocation   │             │
│  └──────────────┴──────────────┴──────────────┘             │
├─────────────────────────────────────────────────────────────┤
│  탭: [Artifacts] [Pipeline] [Monitoring] [Logging]          │
│       [Resources] [YAML Preview]                            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  각 탭별 노코드 설정 UI                               │    │
│  │  - 도구 선택 카드                                      │    │
│  │  - 버전 드롭다운                                       │    │
│  │  - 인스턴스 수량 조절                                   │    │
│  └─────────────────────────────────────────────────────┘    │
│  [Save Draft] [Preview Deploy Script] [Deploy]              │
└─────────────────────────────────────────────────────────────┘
```

- Deploy Script Preview 모달: 생성된 Helm/kubectl 스크립트 표시 + 복사
- K8s Object Preview 모달: Namespace/Deployments/Services/Ingress YAML

### Phase 3: CI/CD (6~7주차)

| # | 페이지 | proto4 소스 | 핵심 구현 사항 |
|---|--------|-------------|----------------|
| 11 | **CI/CD 템플릿** | `pages/cicd-template.js` | Web/API/Batch 파이프라인 템플릿 카드 |
| 12 | **CI/CD 목록** | `pages/cicd.js` | 파이프라인 목록/관리, 상태 표시 |
| 13 | **앱 배포 (Developer)** | `pages/cicd-developer.js` | Developer Self-Service 배포 위자드 |

### Phase 4: 관측성 (8주차)

| # | 페이지 | proto4 소스 | 핵심 구현 사항 |
|---|--------|-------------|----------------|
| 14 | **모니터링 대시보드** | `pages/obs-dashboard.js` | Recharts: CPU/메모리/파이프라인 차트, KPI 카드, WebSocket 실시간 갱신 |
| 15 | **알림 규칙** | `pages/obs-alert-list.js` | 알림 규칙 CRUD, 심각도 배지 |
| 16 | **알림 이력** | `pages/obs-alert-history.js` | 알림 히스토리 테이블, 필터/검색 |

### Phase 5: 관리 (9~10주차)

| # | 페이지 | proto4 소스 | 핵심 구현 사항 |
|---|--------|-------------|----------------|
| 17 | **조직 관리** | `pages/org-organization.js` | 조직 정보 폼, 클러스터 접근 범위(체크박스), 멤버 초대/관리 테이블 |
| 18 | **사용자 관리** | `pages/org-users.js` | 사용자 목록, 역할 부여/변경 Select, 비활성화 |
| 19 | **클러스터 관리** | `pages/org-clusters.js` | 리스트+상세 패널(좌우 분할), 등록/수정 모달, 연결 상태 카드 |

---

## 5. 공통 컴포넌트 라이브러리

proto4에서 추출한 반복 UI 패턴:

| 컴포넌트 | 사용처 | 구현 상세 |
|----------|--------|-----------|
| `Sidebar` | 전체 | 접기/펼치기(240px↔64px), 역할별 메뉴 필터링, nav-section 클래스 |
| `PageHeader` | 전체 | 제목 + 검색 입력 + 액션 버튼 (등록/필터 등) |
| `DataTable` | 목록 페이지 7개 | TanStack Table, 정렬/필터/페이지네이션, 행 호버 강조 |
| `Modal` | 등록/수정/상세 | 일반(480px) / 와이드(800px, 코드 미리보기용) |
| `Card` | 템플릿, 대시보드 | 아이콘(38x38) + 제목 + 설명 패턴, 호버 보더 효과 |
| `StepWizard` | 스택 설치, 앱 배포 | 탭 기반 단계별 폼, 진행률 표시 |
| `CodePreview` | YAML, K8s, 스크립트 | Fira Code, 구문 강조, "Copy to Clipboard" 버튼 |
| `StatusBadge` | 목록 전체 | Connected(녹)/Pending(황)/Error(적)/Inactive(회) |
| `ListDetailPanel` | 클러스터, 멤버 관리 | 좌측 리스트 패널 + 우측 상세 패널 (선택 시 표시) |
| `ConfirmDialog` | 삭제/배포 | 위험 액션 확인, 빨간 CTA 버튼 |
| `RoleSwitcher` | 사이드바 헤더 | 3버튼 토글 (Admin/DevOps/Developer) |
| `LanguageSwitcher` | 헤더 | en/ko 드롭다운, localStorage `nullus_locale` 영속화 |

---

## 6. 기능분해도 매핑 (proto4 검증 결과)

proto4에서 반영된 기능 ID 약 60개, 미반영 약 10개 (대부분 백엔드 전용).

### 미반영 항목 (추후 구현 대상)

| 기능 ID | 단위 프로세스 | 미반영 사유 | 구현 시점 |
|---------|--------------|------------|-----------|
| NULLUS_USR_020_010 | 로그인 페이지 | 프로토타입 범위 외 | Phase 1 |
| NULLUS_USR_010_010 | 역할별 요약 조회 (홈 대시보드) | proto4 미구현 | Phase 1 |
| NULLUS_ORG_030_020 | 초대 링크 수락 | /invite 전용 페이지 없음 | Phase 5 |
| NULLUS_DSS_010_010 | 템플릿 등록 (Admin/DevOps) | 추후 | Phase 2 확장 |
| NULLUS_ORG_010_040 | Organization 삭제 | 의도적 미포함 | 정책 결정 후 |

---

## 7. UX 가이드라인

| 원칙 | 적용 사항 |
|------|-----------|
| **역할 기반 UI** | 로그인 역할에 따라 메뉴/기능 자동 필터링, 불필요한 메뉴 숨김 |
| **점진적 노출** | 5단계 스택 설치를 탭별로 복잡도 분리, 한 번에 하나씩 |
| **즉각적 피드백** | 배포 상태 WebSocket 실시간 업데이트, 로딩 스피너/스켈레톤 |
| **비파괴적 액션** | 삭제/배포 시 ConfirmDialog 필수, 롤백 지원 (스택 이력) |
| **코드 투명성** | YAML/K8s 매니페스트/Deploy Script 미리보기로 신뢰 확보 |
| **다국어** | en/ko 전환, localStorage 영속화, 메뉴/버튼/라벨 전체 적용 |
| **키보드 접근성** | Tab 순서 = 시각 순서, 모달 포커스 트랩, Esc 닫기 |
| **반응형** | 최소 1024px (관리 도구 특성상 데스크톱 우선) |

---

## 8. 구현 일정 요약

```
Phase 1 (1~2주)   ▸ 공통 레이아웃 + 인증 + 홈
Phase 2 (3~5주)   ▸ DevSecOps 스택 (핵심 가치) ← 가장 복잡
Phase 3 (6~7주)   ▸ CI/CD 파이프라인
Phase 4 (8주)     ▸ 관측성 (모니터링/알림)
Phase 5 (9~10주)  ▸ 관리 (조직/사용자/클러스터)
```

**의존 관계:** Phase 1 완료 후 Phase 2~5 순차 진행. Phase 2가 가장 복잡 (5단계 워크플로우 + 코드 미리보기 + 리소스 계산).
