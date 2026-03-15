# UI/UX 100% 완성도 달성 구현 계획

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 감사에서 발견된 모든 기능적 Gap 해소 + 기술 스택(Tailwind CSS 4, React Hook Form + Zod, TanStack Table, Recharts) 정렬을 통해 UI/UX 구현계획 100% 달성

**Architecture:** 4개 워크스트림(Foundation → Components → Features → Migration)으로 진행. Foundation은 순차, 나머지는 병렬 가능. Tailwind 마이그레이션은 최후단에서 일괄 수행하여 기능 작업과 충돌 방지.

**Tech Stack:** React 19, TypeScript, Tailwind CSS 4, shadcn/ui, Zustand, React Router v7, React Hook Form + Zod, TanStack Table, TanStack Query, Recharts, Lucide React, react-i18next

**의존 관계:**
```
Stream 1 (Foundation) ──→ 모든 후속 작업의 전제
Stream 2 (Components) ──→ Stream 3 일부에서 사용
Stream 3 (Features)   ──→ 독립 실행 가능 (컴포넌트 완료 후)
Stream 4 (Migration)  ──→ 최종 단계, 모든 기능 완료 후 일괄 수행
```

---

## Stream 1: Foundation (순차 실행)

### Task 1: 누락 패키지 설치

**Files:**
- Modify: `web/package.json`

**Step 1: 패키지 설치**

```bash
cd web
npm install recharts react-hook-form zod @hookform/resolvers @tanstack/react-table
npm install -D @types/prismjs prismjs
```

**Step 2: Vite 설정 확인**

`vite.config.ts`에 Recharts 청크 분리 추가:

```typescript
if (id.includes('recharts')) {
  return 'vendor-charts'
}
if (id.includes('react-hook-form') || id.includes('zod') || id.includes('@hookform')) {
  return 'vendor-form'
}
if (id.includes('@tanstack/react-table')) {
  return 'vendor-table'
}
```

**Step 3: 빌드 확인**

```bash
npm run build
```

Expected: 빌드 성공, 새 청크 생성

---

### Task 2: 도메인 타입 완성

**Files:**
- Modify: `web/src/types/index.ts`

현재 5개 타입만 정의됨. 전체 도메인 모델 추가:

**추가할 타입:**
- `Organization`, `Member`, `InviteMember`
- `Cluster`, `ClusterType`
- `Stack`, `StackConfig`, `StackStatus`
- `Template`, `ToolSelection`
- `Pipeline`, `PipelineStatus`, `Deployment`
- `CICDTemplate`, `AppTemplate`
- `DashboardMetrics`, `ToolHealth`
- `AlertRule`, `AlertSeverity`, `AlertHistory`
- `CompatibilityMatrix`, `CompatibilityStatus`
- `ResourceEstimate`

각 feature의 api 파일(`stack-api.ts`, `cicd-api.ts`, `admin-api.ts`, `observability-api.ts`)에 이미 로컬 타입이 정의되어 있으므로, 이를 `types/index.ts`로 통합하고 각 api 파일에서 import하도록 변경.

---

### Task 3: Auth Store 완성 + API 클라이언트 토큰 관리

**Files:**
- Modify: `web/src/stores/auth-store.ts`
- Modify: `web/src/lib/api.ts`

**auth-store.ts 보강:**
- `token` 상태 추가 (sessionStorage 영속화)
- `login()` 시 토큰 저장
- `logout()` 시 토큰 제거 + 라우트 리다이렉트
- `isAuthenticated` computed getter

**api.ts 보강:**
- Axios request interceptor에 `Authorization: Bearer ${token}` 주입
- 401 응답 시 `logout()` 호출
- Request/response 에러 핸들링 표준화

---

### Task 4: Route Guard 구현

**Files:**
- Create: `web/src/components/shared/protected-route.tsx`
- Modify: `web/src/app/routes.tsx`

**ProtectedRoute 컴포넌트:**
- `requiredRoles: Role[]` prop
- auth-store에서 현재 역할 확인
- 미인증 시 `/login`으로 리다이렉트
- 권한 불일치 시 `/` (홈)으로 리다이렉트
- `<Outlet />` 렌더링

**routes.tsx 적용:**
- Admin 전용 라우트: Organization, User Management
- DevOps 전용 라우트: Stack Install, Stack Deploy
- Developer 전용 라우트: Developer Deploy
- 공통 라우트: Home, Stack List, Monitoring 등

---

### Task 5: Error Boundary 구현

**Files:**
- Create: `web/src/components/shared/error-boundary.tsx`
- Modify: `web/src/app/layout.tsx`

**ErrorBoundary 컴포넌트:**
- React `ErrorBoundary` class component
- fallback UI: 에러 메시지 + 재시도 버튼 + 홈 이동 버튼
- 에러 로깅 (console.error)

**layout.tsx 적용:**
- `<ErrorBoundary>` 로 메인 콘텐츠 영역 래핑

---

## Stream 2: 공통 컴포넌트 (병렬 실행 가능)

### Task 6: PageHeader 완성

**Files:**
- Modify: `web/src/components/layout/page-header.tsx`

현재 35 LOC 스텁 → 완전한 컴포넌트로 확장:

**Props:**
- `title: string`
- `subtitle?: string`
- `searchPlaceholder?: string`
- `onSearch?: (query: string) => void`
- `actions?: ReactNode` (우측 액션 버튼 슬롯)

**UI:**
- 좌측: 제목 + 부제
- 중앙/우측: 검색 입력 (돋보기 아이콘)
- 최우측: 액션 버튼 슬롯

---

### Task 7: StepWizard 컴포넌트 추출

**Files:**
- Create: `web/src/components/shared/step-wizard.tsx`

현재 `stack-install-page.tsx`와 `developer-deploy-page.tsx`에 인라인으로 탭/스텝 UI가 구현되어 있음. 공통 컴포넌트로 추출:

**Props:**
- `steps: { id: string; label: string; icon?: ReactNode }[]`
- `activeStep: string`
- `onStepChange: (stepId: string) => void`
- `children: ReactNode`
- `completedSteps?: string[]`

**UI:**
- 상단 스텝 인디케이터 (번호 + 라벨)
- 완료 스텝은 체크마크
- 현재 스텝 하이라이트
- 스텝 간 진행선

추출 후 `stack-install-page.tsx`와 `developer-deploy-page.tsx`에서 이 컴포넌트를 사용하도록 리팩토링.

---

### Task 8: RoleSwitcher 컴포넌트

**Files:**
- Create: `web/src/components/shared/role-switcher.tsx`

**Props:**
- `currentRole: Role`
- `onRoleChange: (role: Role) => void`

**UI:**
- 3버튼 토글 (Admin / DevOps Engineer / Developer)
- 현재 역할 하이라이트 (골드 보더)
- 각 역할에 아이콘 (Shield, Wrench, Code2)
- 컴팩트 모드 (사이드바 접힘 시 아이콘만)

**사용처:**
- `sidebar.tsx` 상단 영역에 배치

---

### Task 9: LanguageSwitcher 컴포넌트

**Files:**
- Create: `web/src/components/shared/language-switcher.tsx`

현재 `header.tsx`에 인라인 구현. 독립 컴포넌트로 추출:

**Props:**
- `currentLanguage: string`
- `onLanguageChange: (lang: string) => void`
- `variant?: 'dropdown' | 'toggle'`

**UI:**
- 드롭다운: 국기 아이콘 + 언어명 (English / 한국어)
- 토글: EN / KO 토글 버튼
- localStorage `nullus_locale` 영속화

---

### Task 10: ListDetailPanel 컴포넌트

**Files:**
- Create: `web/src/components/shared/list-detail-panel.tsx`

현재 `cluster-page.tsx`에 인라인 구현 (280px 리스트 + flex 상세). 추출:

**Props:**
- `listWidth?: number` (default 280)
- `listContent: ReactNode`
- `detailContent: ReactNode`
- `emptyDetailMessage?: string`
- `showDetail: boolean`

**UI:**
- 좌측 리스트 패널 (고정 너비, 스크롤)
- 우측 상세 패널 (flex-1)
- 상세 미선택 시 빈 상태 메시지

추출 후 `cluster-page.tsx` 리팩토링.

---

### Task 11: 구문 강조 (CodePreview + YamlEditor)

**Files:**
- Modify: `web/src/components/shared/code-preview.tsx`
- Modify: `web/src/components/shared/yaml-editor.tsx`

**Prism.js 통합:**
- `prismjs` + 필요 언어 (yaml, json, bash, typescript)
- 라인 넘버링은 기존 구현 유지
- 다크 테마에 맞는 Prism 테마 적용 (One Dark 계열)
- 하이라이팅된 HTML을 `dangerouslySetInnerHTML`로 렌더링
- language prop에 따라 동적 언어 로드

---

### Task 12: 로딩 스켈레톤

**Files:**
- Create: `web/src/components/shared/skeleton.tsx`

**컴포넌트:**
- `Skeleton` 기본 (width, height, borderRadius)
- `SkeletonCard` (카드 형태)
- `SkeletonTable` (테이블 행 반복)
- `SkeletonText` (텍스트 라인)
- 펄스 애니메이션 (CSS @keyframes)

---

## Stream 3: 기능 페이지 Gap 해소 (병렬 실행 가능)

### Task 13: Stack — Deploy Script Preview + K8s Object Preview + Template Detail 모달

**Files:**
- Modify: `web/src/features/stack/pages/stack-install-page.tsx`
- Modify: `web/src/features/stack/pages/stack-template-page.tsx`

**Deploy Script Preview 모달:**
- "Preview Deploy Script" 버튼 추가 (Deploy 버튼 좌측)
- 모달 (wide): 생성된 Helm/kubectl 명령어 표시
- `CodePreview` 컴포넌트 사용 (language: bash)
- 클립보드 복사 버튼

**K8s Object Preview 모달:**
- "Preview K8s Objects" 버튼 추가
- 모달 (wide): Namespace, Deployment, Service, Ingress YAML 탭
- `YamlEditor` 컴포넌트 사용 (readOnly)
- 각 오브젝트 타입별 탭

**Template Detail 모달:**
- 카드 클릭 시 상세 모달 열기
- 포함 도구 목록, 추정 시간, 리소스 요구사항, "Use This Template" CTA

---

### Task 14: CI/CD — 버튼 핸들러 연결

**Files:**
- Modify: `web/src/features/cicd/pages/cicd-template-page.tsx`
- Modify: `web/src/features/cicd/pages/cicd-list-page.tsx`

**cicd-template-page.tsx:**
- "Use Template" 버튼 → `/cicd/pipelines/new?template={id}` 네비게이션

**cicd-list-page.tsx:**
- "View" 버튼 → 파이프라인 상세 패널 또는 모달 표시

---

### Task 15: Observability — Recharts 차트 + Alert CRUD 완성

**Files:**
- Modify: `web/src/features/observability/pages/monitoring-page.tsx`
- Modify: `web/src/features/observability/pages/alert-rules-page.tsx`
- Modify: `web/src/features/observability/pages/alert-history-page.tsx`
- Modify: `web/src/features/observability/api/observability-api.ts`

**monitoring-page.tsx:**
- KPI 단순 바 → `Recharts` `AreaChart` / `BarChart` 교체
- CPU 사용률: `AreaChart` (시간별 추세)
- 메모리 사용률: `AreaChart` (시간별 추세)
- 파이프라인 성공률: `BarChart` (일별)
- 파드 상태: `PieChart` (Running/Pending/Failed)
- 시간 범위 선택기 (1h / 6h / 24h / 7d)
- WebSocket 연결로 실시간 갱신 (lib/websocket.ts 활용)

**alert-rules-page.tsx:**
- 심각도 레벨 추가 (Critical / Warning / Info)
- Edit 모달 구현 (기존 Create 모달 재활용)
- Delete 버튼 + ConfirmDialog
- Edit 핸들러 연결

**alert-history-page.tsx:**
- 검색 기능 추가 (규칙명 검색)
- 날짜 범위 필터

---

### Task 16: Admin — User 핸들러 + 확인 다이얼로그

**Files:**
- Modify: `web/src/features/admin/pages/user-management-page.tsx`
- Modify: `web/src/features/admin/pages/organization-page.tsx`
- Modify: `web/src/features/admin/pages/cluster-page.tsx`

**user-management-page.tsx:**
- 역할 Select: `defaultValue` → controlled `value` + `onChange` 핸들러
- `useUpdateUserRole` mutation hook 연결
- 비활성화 버튼: `ConfirmDialog` 연결 + `useDeactivateUser` mutation
- 검색 기능 추가

**organization-page.tsx:**
- 멤버 제거 시 `ConfirmDialog` 사용

**cluster-page.tsx:**
- 클러스터 수정 모달 추가
- 클러스터 삭제 버튼 + `ConfirmDialog`
- 연결 검증 버튼 (Verify Connection)

---

### Task 17: Home + Sidebar — 핸들러 연결

**Files:**
- Modify: `web/src/features/home/pages/home-page.tsx`
- Modify: `web/src/components/layout/sidebar.tsx`

**home-page.tsx:**
- CTA 버튼에 `useNavigate()` 연결
  - Admin → `/admin/organizations`
  - DevOps → `/stack/install`
  - Developer → `/cicd/templates`
- 역할별 요약 카드 추가 (간단한 통계)

**sidebar.tsx:**
- 로그아웃 버튼 `onClick` → auth-store `logout()` + navigate `/login`

---

## Stream 4: 기술 스택 정렬 (기능 완료 후)

### Task 18: Tailwind CSS 마이그레이션

**Files:**
- 모든 `.tsx` 파일의 `style={{...}}` → Tailwind 클래스 변환

**전략:**
- CSS 변수 (`var(--color-*)`)는 Tailwind `theme.extend`로 매핑
  - `index.css`의 CSS 변수를 Tailwind 커스텀 유틸리티로 참조
  - 예: `style={{ color: 'var(--color-text-primary)' }}` → `className="text-[var(--color-text-primary)]"`
- 고정 값은 Tailwind 유틸리티로 직접 변환
  - `padding: '16px'` → `p-4`
  - `display: 'flex'` → `flex`
  - `gap: '12px'` → `gap-3`
- 조건부 스타일: `cn()` 유틸리티 함수 사용 (clsx 패턴)

**파일별 변환 순서 (의존성 순):**
1. UI 컴포넌트: `button.tsx`, `card.tsx`, `input.tsx`, `modal.tsx`
2. Shared 컴포넌트: `data-table.tsx`, `status-badge.tsx`, `confirm-dialog.tsx`, `code-preview.tsx`, `yaml-editor.tsx`, `skeleton.tsx`
3. Layout 컴포넌트: `sidebar.tsx`, `header.tsx`, `page-header.tsx`, `layout.tsx`
4. Feature 페이지: 각 features/ 하위 페이지 전체

**cn() 유틸리티 생성:**
- Create: `web/src/lib/utils.ts`
```typescript
import { type ClassValue, clsx } from 'clsx'
export function cn(...inputs: ClassValue[]) {
  return clsx(inputs)
}
```
- `npm install clsx`

---

### Task 19: React Hook Form + Zod 폼 검증

**Files:**
- 폼이 포함된 모든 페이지

**대상 폼:**
1. `login-page.tsx` — 로그인 폼 (email, password)
2. `stack-install-page.tsx` — 스택 설정 폼 (stackName, resources)
3. `developer-deploy-page.tsx` — 5단계 배포 위자드 (appName, gitUrl, resources, envVars)
4. `cicd-list-page.tsx` — 파이프라인 생성 모달
5. `alert-rules-page.tsx` — 알림 규칙 생성/편집 모달
6. `organization-page.tsx` — 조직 정보 + 멤버 초대 모달
7. `user-management-page.tsx` — 사용자 초대 모달
8. `cluster-page.tsx` — 클러스터 등록/수정 모달

**각 폼 적용 패턴:**
```typescript
const schema = z.object({ ... })
type FormData = z.infer<typeof schema>
const { register, handleSubmit, formState: { errors } } = useForm<FormData>({
  resolver: zodResolver(schema)
})
```

---

### Task 20: TanStack Table 통합

**Files:**
- Modify: `web/src/components/shared/data-table.tsx`
- 모든 테이블 사용 페이지

**DataTable 리팩토링:**
- 현재 커스텀 구현 → `@tanstack/react-table` 기반으로 교체
- `useReactTable` 훅 사용
- `getCoreRowModel`, `getSortedRowModel`, `getFilteredRowModel`, `getPaginationRowModel`
- 컬럼 정의는 `ColumnDef<T>[]` 타입
- 기존 API (`columns`, `data`, `getRowKey`) 최대한 호환 유지

**테이블 사용 페이지 (7개):**
1. `stack-list-page.tsx`
2. `stack-history-page.tsx`
3. `cicd-list-page.tsx`
4. `cicd-history-page.tsx`
5. `alert-rules-page.tsx`
6. `alert-history-page.tsx`
7. `user-management-page.tsx`

---

## 검증 및 완료

### Task 21: 전체 빌드 + 테스트 + 린트

```bash
cd web
npm run build          # 프로덕션 빌드
npx vitest run         # 단위 테스트
npm run lint           # ESLint
npm run e2e            # E2E 테스트 (Playwright)
```

모든 명령이 통과해야 완료.

---

## 실행 순서 요약

```
[Task 1] 패키지 설치
    ↓
[Task 2] 타입 완성
[Task 3] Auth + API       ──→  [Task 4] Route Guard
[Task 5] Error Boundary        [Task 17] Home/Sidebar 핸들러
    ↓
[Tasks 6-12] 공통 컴포넌트 (병렬)
    ↓
[Tasks 13-16] 기능 페이지 Gap (병렬)
    ↓
[Task 18] Tailwind 마이그레이션
[Task 19] RHF + Zod (병렬)
[Task 20] TanStack Table (병렬)
    ↓
[Task 21] 전체 검증
```

**예상 작업량:** 21개 태스크, 약 45개 파일 수정/생성
