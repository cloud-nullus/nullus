# Nullus 프론트엔드 아키텍처 가이드

**작성일**: 2026-03-22
**기술 스택**: React 19 · TypeScript · Vite · Tailwind CSS 4 · shadcn/ui

---

## 1. 기술 스택

| 영역 | 기술 | 용도 |
|------|------|------|
| UI 프레임워크 | React 19 + TypeScript | 컴포넌트 기반 UI |
| 빌드 | Vite | 개발 서버 + 번들링 |
| 스타일 | Tailwind CSS 4 + CSS Custom Properties | 유틸리티 기반 스타일링 |
| UI 컴포넌트 | shadcn/ui | 기본 UI 컴포넌트 (Button, Card, Input, Modal) |
| 라우팅 | React Router v6 | SPA 라우팅 |
| 서버 상태 | TanStack React Query | API 캐싱, 자동 refetch |
| 클라이언트 상태 | Zustand | 경량 상태 관리 |
| 폼 | React Hook Form + Zod | 타입 안전 폼 검증 |
| HTTP | Axios | API 클라이언트 |
| 차트 | Recharts | 모니터링 대시보드 시각화 |
| 코드 에디터 | @monaco-editor/react | YAML 편집 (양방향 동기화) |
| E2E 테스트 | Playwright | 브라우저 자동화 테스트 |
| 단위 테스트 | Vitest + Testing Library | 컴포넌트 단위 테스트 |
| 다국어 | i18next | 한국어/영어 |

---

## 2. 디렉토리 구조

```
web/src/
├── app/                          # 앱 설정
│   ├── routes.tsx                # 전체 라우트 정의 (React Router v6)
│   └── layout.tsx                # AppLayout (Header + Sidebar + Content)
│
├── features/                     # 도메인별 Feature 모듈
│   ├── admin/                    # 조직/클러스터/사용자 관리
│   │   ├── pages/                # OrganizationPage, ClusterPage, UserManagementPage
│   │   └── api/admin-api.ts      # useOrganization, useCreateCluster, useInviteLinks...
│   ├── auth/                     # 인증
│   │   └── pages/login-page.tsx  # Mock/OIDC 듀얼 로그인
│   ├── stack/                    # DevSecOps 스택 관리
│   │   ├── pages/                # TemplatesPage, InstallPage, ListPage, DeployPage...
│   │   ├── api/stack-api.ts      # useStacks, useDeployStack, useAddTools...
│   │   ├── stores/               # stack-config-store.ts (Zustand)
│   │   ├── hooks/                # use-deploy-log.ts (WebSocket)
│   │   └── components/           # version-diff.tsx
│   ├── cicd/                     # CI/CD 파이프라인
│   │   ├── pages/                # TemplatePage, PipelineSetupPage, HistoryPage...
│   │   └── api/cicd-api.ts       # usePipelines, useRollbackDeployment...
│   ├── observability/            # 모니터링/알림
│   │   ├── pages/                # MonitoringPage, AlertRulesPage, AlertHistoryPage
│   │   └── api/observability-api.ts
│   ├── home/pages/home-page.tsx  # 대시보드 홈
│   └── common/pages/             # NotFoundPage
│
├── components/
│   ├── shared/                   # 범용 컴포넌트 (14개)
│   │   ├── protected-route.tsx   # 역할 기반 라우트 보호
│   │   ├── data-table.tsx        # TanStack Table (정렬/필터/페이지네이션)
│   │   ├── confirm-dialog.tsx    # 확인 다이얼로그 (텍스트 입력 필수)
│   │   ├── step-wizard.tsx       # 다단계 마법사 UI
│   │   ├── yaml-editor.tsx       # Prism.js 기반 YAML 뷰어
│   │   ├── breadcrumb.tsx        # 경로 탐색
│   │   ├── skeleton.tsx          # 로딩 스켈레톤
│   │   ├── status-badge.tsx      # 상태 배지 (success/error/pending)
│   │   ├── error-boundary.tsx    # 에러 경계
│   │   ├── code-preview.tsx      # 코드 미리보기
│   │   ├── list-detail-panel.tsx # 목록-상세 확장 패널
│   │   ├── role-switcher.tsx     # Mock 역할 전환 (개발용)
│   │   └── language-switcher.tsx # 언어 전환 (EN/KO)
│   ├── layout/                   # 레이아웃
│   │   ├── header.tsx            # 상단 헤더
│   │   ├── sidebar.tsx           # 역할별 사이드바 메뉴
│   │   └── page-header.tsx       # 페이지 제목 + 액션 영역
│   └── ui/                       # shadcn/ui 원시 컴포넌트
│       ├── button.tsx, card.tsx, input.tsx, modal.tsx, toast-provider.tsx
│
├── stores/                       # 글로벌 상태
│   ├── auth-store.ts             # 인증 (role, token, login/logout)
│   ├── theme-store.ts            # 테마 (light/dark)
│   └── sidebar-store.ts          # 사이드바 (open/close)
│
├── lib/                          # 유틸리티
│   ├── api.ts                    # Axios 인스턴스 (baseURL, interceptors)
│   ├── oidc-providers.ts         # OIDC Provider 추상화 (Keycloak/Authentik)
│   ├── oidc-config.ts            # OIDC 설정 re-export
│   ├── websocket.ts              # WebSocket 클라이언트
│   ├── query-client.ts           # React Query 클라이언트
│   └── utils.ts                  # cn(), 날짜 포맷 등
│
├── hooks/use-toast.ts            # 토스트 알림 훅
├── types/index.ts                # 전역 타입 (Role, Status 등)
├── i18n/                         # 다국어 (en.json, ko.json)
├── App.tsx                       # 루트 컴포넌트
└── main.tsx                      # 엔트리 포인트 (OIDC AuthProvider 조건부 래핑)
```

---

## 3. 라우팅 구조

`web/src/app/routes.tsx`에서 전체 라우트를 관리한다.

| 경로 | 페이지 | 접근 역할 |
|------|--------|----------|
| `/login` | 로그인 | 모든 사용자 |
| `/` | 홈 대시보드 | admin, devops, developer |
| `/stack/templates` | Golden Path 템플릿 | admin, devops |
| `/stack/install` | 스택 설치 Wizard (5단계) | admin, devops |
| `/stack/list` | 설치된 스택 목록 | admin, devops |
| `/stack/:id/add-tools` | 기존 스택 도구 추가 | admin, devops |
| `/stack/deploy` | 스택 배포 | admin, devops |
| `/stack/deploy/:id/logs` | 배포 로그 스트리밍 | admin, devops |
| `/stack/history` | 스택 배포 이력 | admin, devops |
| `/stack/versions` | 버전 호환성 관리 | admin, devops |
| `/cicd/templates` | CI/CD 파이프라인 템플릿 | admin, devops |
| `/cicd/pipelines` | 파이프라인 목록 | admin, devops, developer |
| `/cicd/setup` | 파이프라인 설정 | admin, devops |
| `/cicd/history` | CI/CD 배포 이력 | admin, devops, developer |
| `/cicd/deploy` | Developer Self-Service 배포 | developer |
| `/observability/monitoring` | 모니터링 대시보드 | admin, devops, developer |
| `/observability/alert-rules` | 알림 규칙 관리 | admin, devops |
| `/observability/alert-history` | 알림 이력 | admin, devops, developer |
| `/admin/organization` | 조직 설정 | admin |
| `/admin/clusters` | 클러스터 관리 | admin |
| `/admin/users` | 사용자 관리 | admin |

역할 접근 제어는 `ProtectedRoute` 컴포넌트로 처리:

```tsx
<Route element={<ProtectedRoute allowedRoles={['admin', 'devops']} />}>
  <Route path="/stack/*" element={...} />
</Route>
```

---

## 4. 상태 관리 패턴

### Zustand (클라이언트 상태)

```typescript
// stores/auth-store.ts
import { create } from 'zustand'

interface AuthState {
  user: User | null
  role: Role
  login: (email: string, password: string) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  role: 'developer',
  login: (email, password) => { /* ... */ },
  logout: () => set({ user: null, role: 'developer' }),
}))
```

### React Query (서버 상태)

```typescript
// features/stack/api/stack-api.ts
export function useStacks(orgId: string) {
  return useQuery({
    queryKey: ['stacks', 'list', orgId],
    queryFn: () => api.get(`/api/v1/stacks?orgId=${orgId}`).then(r => r.data),
  })
}

export function useDeployStack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (stackId: string) => api.post(`/api/v1/stacks/${stackId}/deploy`),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['stacks'] }),
  })
}
```

---

## 5. API 통합 패턴

### Axios 인스턴스

```typescript
// lib/api.ts
export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api/v1',
  headers: { 'Content-Type': 'application/json' },
})
```

### API 에러 처리

API 실패 시 빈 배열 또는 `null`을 반환하여 에러 상태를 명시적으로 처리한다.
MOCK 데이터 fallback은 오류를 은폐하므로 사용하지 않는다.

```typescript
const { data: stacks, isLoading, error } = useStacks(orgId)
const displayStacks = stacks?.items ?? []  // 빈 배열 fallback
```

---

## 6. 폼 처리 패턴

```typescript
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const schema = z.object({
  name: z.string().min(1, '이름을 입력하세요'),
  email: z.string().email('유효한 이메일을 입력하세요'),
})

type FormData = z.infer<typeof schema>

function MyForm() {
  const { register, handleSubmit, formState: { errors } } = useForm<FormData>({
    resolver: zodResolver(schema),
  })
  const onSubmit = (data: FormData) => { /* API 호출 */ }

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <input {...register('name')} />
      {errors.name && <span>{errors.name.message}</span>}
    </form>
  )
}
```

---

## 7. 새 Feature 모듈 추가 방법

예시: `notification` 모듈 추가

### Step 1: 디렉토리 생성

```
web/src/features/notification/
  ├── pages/
  │   └── notification-page.tsx
  └── api/
      └── notification-api.ts
```

### Step 2: API 훅 작성

```typescript
// notification-api.ts
export function useNotifications() {
  return useQuery({
    queryKey: ['notifications'],
    queryFn: () => api.get('/api/v1/notifications').then(r => r.data),
  })
}
```

### Step 3: 페이지 작성

```tsx
export default function NotificationPage() {
  const { data, isLoading } = useNotifications()
  if (isLoading) return <Skeleton />
  return <DataTable columns={columns} data={data} />
}
```

### Step 4: 라우트 추가 (`routes.tsx`)

```tsx
const NotificationPage = lazy(() => import('../features/notification/pages/notification-page'))
// Route 정의 내부:
<Route path="/notifications" element={<NotificationPage />} />
```

### Step 5: 사이드바 메뉴 추가 (`sidebar.tsx`)

```typescript
{ label: 'Notifications', icon: Bell, href: '/notifications', roles: ['admin', 'devops'] }
```

---

## 8. 테스트

### 단위 테스트 (Vitest)

```bash
cd web && npx vitest run                    # 전체
cd web && npx vitest run stack-add-tools    # 특정 파일
```

```typescript
// stack-add-tools-page.test.tsx
describe('StackAddToolsPage', () => {
  it('should display 3-step wizard', () => {
    render(<StackAddToolsPage />)
    expect(screen.getByText('Category Selection')).toBeInTheDocument()
  })
})
```

### E2E 테스트 (Playwright)

```bash
cd web && npx playwright test               # 전체
cd web && npx playwright test --ui          # UI 모드
```

테스트 파일 규칙: `web/e2e/*.spec.ts`

---

## 9. 참고 자료

| 자료 | 경로 |
|------|------|
| 라우트 정의 | `web/src/app/routes.tsx` |
| API 클라이언트 | `web/src/lib/api.ts` |
| OIDC 설정 | `web/src/lib/oidc-providers.ts` |
| 디자인 시스템 | `docs/40_UI_UX/Nullus_디자인시스템.md` |
| 프론트엔드 상세설계 | `docs/40_UI_UX/Nullus_프론트엔드_상세설계.md` |
