# Nullus 프론트엔드 상세 설계

**작성일**: 2026-03-14
**버전**: 1.0
**기반 문서**: Nullus_UI_UX_구현계획.md, Nullus_디자인시스템.md, Nullus_메뉴체계.md, nullus_PRD_1.3.md

---

## 목차

1. [프로젝트 초기 셋업](#1-프로젝트-초기-셋업)
2. [디렉토리 구조](#2-디렉토리-구조)
3. [상태 관리 설계](#3-상태-관리-설계)
4. [라우팅 설계](#4-라우팅-설계)
5. [API 통신 레이어](#5-api-통신-레이어)
6. [WebSocket 클라이언트](#6-websocket-클라이언트)
7. [i18n 설계](#7-i18n-설계)
8. [공통 컴포넌트 상세 스펙](#8-공통-컴포넌트-상세-스펙)
9. [테스트 전략](#9-테스트-전략)
10. [빌드/배포 설정](#10-빌드배포-설정)

---

## 1. 프로젝트 초기 셋업

### 1.1 의존성 설치

```bash
# 프로젝트 생성
npm create vite@latest nullus-web -- --template react-ts
cd nullus-web

# 핵심 의존성
npm install react@19 react-dom@19
npm install react-router-dom@7
npm install zustand
npm install @tanstack/react-query @tanstack/react-table
npm install react-hook-form @hookform/resolvers zod
npm install recharts
npm install lucide-react
npm install react-i18next i18next i18next-browser-languagedetector
npm install axios
npm install clsx tailwind-merge class-variance-authority

# shadcn/ui 초기화 (Tailwind CSS 4 기반)
npx shadcn@latest init

# shadcn/ui 컴포넌트 추가
npx shadcn@latest add button input select dialog table badge tabs
npx shadcn@latest add dropdown-menu tooltip alert-dialog separator skeleton
npx shadcn@latest add form label card sheet

# 코드 미리보기 (YAML/스크립트)
npm install @uiw/react-codemirror @codemirror/lang-yaml @codemirror/lang-javascript
npm install @codemirror/theme-one-dark

# 개발 의존성
npm install -D vitest @vitest/ui @testing-library/react @testing-library/jest-dom @testing-library/user-event
npm install -D msw
npm install -D @playwright/test
npm install -D @types/node
```

### 1.2 Tailwind CSS 4 설정

```css
/* src/styles/globals.css */
@import "tailwindcss";

@theme {
  /* 브랜드 컬러 */
  --color-brand-gold: #ffd700;
  --color-brand-gold-end: #f59e0b;

  /* 다크 테마 서피스 */
  --color-surface-base: #0a0a0a;
  --color-surface-card: #0f1419;
  --color-surface-overlay: rgba(0, 0, 0, 0.7);

  /* 보더 */
  --color-border-default: #2d3748;
  --color-border-hover: #4a5568;

  /* 텍스트 */
  --color-text-primary: #f1f5f9;
  --color-text-secondary: #64748b;
  --color-text-muted: #475569;

  /* 레이아웃 토큰 */
  --sidebar-width: 240px;
  --sidebar-collapsed: 64px;
  --header-height: 56px;

  /* 컴포넌트 */
  --card-radius: 12px;
  --card-padding: 18px;
  --icon-size: 38px;
  --icon-radius: 10px;

  /* 트랜지션 */
  --transition-default: 200ms ease;
  --transition-fast: 150ms ease;

  /* z-index */
  --z-sidebar: 40;
  --z-modal: 50;
  --z-toast: 60;
}

/* 다크 테마 기본 변수 */
:root {
  color-scheme: dark;
}

body {
  background: linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 50%, #16213e 100%);
  background-attachment: fixed;
  color: var(--color-text-primary);
  font-family: 'Inter', 'Pretendard', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}

/* 라이트 테마 오버라이드 */
[data-theme="light"] {
  color-scheme: light;
  --color-surface-base: #ffffff;
  --color-surface-card: #f8fafc;
  --color-border-default: #e2e8f0;
  --color-text-primary: #0f172a;
  --color-text-secondary: #475569;
}

/* 코드 폰트 */
code, pre, .font-mono {
  font-family: 'Fira Code', 'Cascadia Code', 'JetBrains Mono', monospace;
}

/* 접근성: 포커스 링 */
*:focus-visible {
  outline: 2px solid #6366f1;
  outline-offset: 2px;
}

/* 모션 감소 */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

### 1.3 TypeScript 설정

```json
// tsconfig.json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    },
    "resolveJsonModule": true,
    "isolatedModules": true,
    "skipLibCheck": true
  },
  "include": ["src", "vite.config.ts"],
  "exclude": ["node_modules"]
}
```

### 1.4 폰트 설정

```html
<!-- index.html -->
<!DOCTYPE html>
<html lang="ko">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Nullus Platform</title>

    <!-- Inter + Fira Code -->
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
      href="https://fonts.googleapis.com/css2?family=Fira+Code:wght@400;500;600;700&family=Inter:wght@300;400;500;600;700;800&display=swap"
      rel="stylesheet"
    />

    <!-- Pretendard (한글) -->
    <link
      href="https://cdn.jsdelivr.net/gh/orioncactus/pretendard/dist/web/static/pretendard.css"
      rel="stylesheet"
    />
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

---

## 2. 디렉토리 구조

```
src/
├── app/
│   ├── layout.tsx              # AppShell: 사이드바 + 헤더 + 메인 영역
│   ├── routes.tsx              # React Router 설정, 역할 기반 가드
│   └── providers.tsx           # QueryClient, i18n, Theme 프로바이더 조합
│
├── components/
│   ├── ui/                     # shadcn/ui 기반 기본 컴포넌트 (자동 생성)
│   │   ├── button.tsx
│   │   ├── input.tsx
│   │   ├── select.tsx
│   │   ├── dialog.tsx
│   │   ├── table.tsx
│   │   ├── badge.tsx
│   │   ├── tabs.tsx
│   │   ├── skeleton.tsx
│   │   └── ...
│   ├── layout/
│   │   ├── Sidebar.tsx         # 사이드바 (240px/64px 접힘, 역할별 메뉴)
│   │   ├── Header.tsx          # 상단 헤더 (56px, 언어/테마/사용자)
│   │   └── PageHeader.tsx      # 페이지별 제목 + 검색 + 액션 버튼
│   └── shared/
│       ├── DataTable.tsx       # TanStack Table 래퍼 (정렬/필터/페이지네이션)
│       ├── Modal.tsx           # 일반(480px) / 와이드(800px) 모달
│       ├── StepWizard.tsx      # 탭 기반 단계별 폼
│       ├── CodePreview.tsx     # YAML/스크립트 미리보기 + 복사
│       ├── StatusBadge.tsx     # Connected/Pending/Error/Inactive 배지
│       ├── ListDetailPanel.tsx # 좌측 리스트 + 우측 상세 패널
│       ├── ConfirmDialog.tsx   # 삭제/배포 확인 다이얼로그
│       ├── RoleSwitcher.tsx    # Admin/DevOps/Developer 3버튼 토글
│       ├── LanguageSwitcher.tsx # en/ko 드롭다운
│       ├── Card.tsx            # 아이콘 + 제목 + 설명 카드
│       └── ErrorBoundary.tsx   # React 에러 바운더리
│
├── features/
│   ├── auth/
│   │   ├── components/
│   │   │   └── LoginForm.tsx
│   │   ├── hooks/
│   │   │   └── useAuth.ts
│   │   ├── pages/
│   │   │   └── LoginPage.tsx
│   │   └── api/
│   │       └── auth.api.ts
│   │
│   ├── home/
│   │   ├── components/
│   │   │   ├── RoleSummaryCard.tsx
│   │   │   └── QuickActionButton.tsx
│   │   └── pages/
│   │       └── HomePage.tsx
│   │
│   ├── stack/
│   │   ├── components/
│   │   │   ├── TemplateCard.tsx
│   │   │   ├── InstallWizard/
│   │   │   │   ├── ArtifactsTab.tsx
│   │   │   │   ├── PipelineTab.tsx
│   │   │   │   ├── MonitoringTab.tsx
│   │   │   │   ├── LoggingTab.tsx
│   │   │   │   ├── ResourcesTab.tsx
│   │   │   │   ├── YamlPreviewTab.tsx
│   │   │   │   ├── ConfigSummaryPanel.tsx
│   │   │   │   ├── ResourceAllocationCard.tsx
│   │   │   │   └── DeployScriptModal.tsx
│   │   │   ├── StackListTable.tsx
│   │   │   ├── StackHistoryTable.tsx
│   │   │   └── CompatibilityMatrix.tsx
│   │   ├── hooks/
│   │   │   ├── useStackInstall.ts
│   │   │   └── useDeployLog.ts
│   │   ├── pages/
│   │   │   ├── StackTemplatePage.tsx
│   │   │   ├── StackInstallPage.tsx
│   │   │   ├── StackListPage.tsx
│   │   │   ├── StackHistoryPage.tsx
│   │   │   └── StackVersionPage.tsx
│   │   └── api/
│   │       └── stack.api.ts
│   │
│   ├── cicd/
│   │   ├── components/
│   │   │   ├── PipelineTemplateCard.tsx
│   │   │   ├── PipelineListTable.tsx
│   │   │   └── AppDeployWizard/
│   │   │       ├── RepoConfigStep.tsx
│   │   │       └── DeploySettingsStep.tsx
│   │   ├── pages/
│   │   │   ├── CicdTemplatePage.tsx
│   │   │   ├── CicdListPage.tsx
│   │   │   ├── CicdHistoryPage.tsx
│   │   │   └── AppDeployPage.tsx
│   │   └── api/
│   │       └── cicd.api.ts
│   │
│   ├── observability/
│   │   ├── components/
│   │   │   ├── MetricCard.tsx
│   │   │   ├── CpuMemoryChart.tsx
│   │   │   ├── PipelineSuccessChart.tsx
│   │   │   ├── AlertRuleTable.tsx
│   │   │   └── AlertHistoryTable.tsx
│   │   ├── pages/
│   │   │   ├── MonitoringDashboardPage.tsx
│   │   │   ├── AlertListPage.tsx
│   │   │   └── AlertHistoryPage.tsx
│   │   └── api/
│   │       └── observability.api.ts
│   │
│   └── admin/
│       ├── components/
│       │   ├── OrganizationForm.tsx
│       │   ├── ClusterAccessScope.tsx
│       │   ├── MemberTable.tsx
│       │   ├── UserRoleSelect.tsx
│       │   └── ClusterRegisterModal.tsx
│       ├── pages/
│       │   ├── OrganizationPage.tsx
│       │   ├── UsersPage.tsx
│       │   └── ClustersPage.tsx
│       └── api/
│           └── admin.api.ts
│
├── stores/
│   ├── auth.ts                 # 역할, 인증 상태
│   ├── theme.ts                # 다크/라이트 테마
│   ├── sidebar.ts              # 사이드바 접기/펼치기
│   └── stack.ts                # 스택 설치 진행 상태 (임시 저장)
│
├── lib/
│   ├── api.ts                  # Axios 인스턴스 + 인터셉터
│   ├── queryClient.ts          # TanStack Query 클라이언트 설정
│   ├── ws.ts                   # WebSocket 클라이언트
│   └── utils.ts                # clsx/twMerge 유틸, 공통 헬퍼
│
├── i18n/
│   ├── index.ts                # i18next 초기화
│   ├── en.json                 # 영문 번역
│   └── ko.json                 # 한글 번역
│
├── types/
│   ├── index.ts                # 공통 타입 re-export
│   ├── auth.types.ts
│   ├── stack.types.ts
│   ├── cicd.types.ts
│   ├── observability.types.ts
│   └── admin.types.ts
│
├── hooks/
│   ├── useDebounce.ts
│   ├── useLocalStorage.ts
│   └── useMediaQuery.ts
│
├── styles/
│   └── globals.css
│
└── main.tsx                    # React 앱 진입점
```

---

## 3. 상태 관리 설계

### 3.1 auth store

```typescript
// src/stores/auth.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type UserRole = 'admin' | 'devops' | 'developer';

export interface User {
  id: string;
  name: string;
  email: string;
  role: UserRole;
  organizationId: string;
}

interface AuthState {
  user: User | null;
  token: string | null;
  activeRole: UserRole | null; // 역할 전환기에서 선택한 현재 역할

  // Actions
  setUser: (user: User, token: string) => void;
  setActiveRole: (role: UserRole) => void;
  logout: () => void;
  isAuthenticated: () => boolean;
  hasRole: (role: UserRole | UserRole[]) => boolean;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      activeRole: null,

      setUser: (user, token) =>
        set({ user, token, activeRole: user.role }),

      setActiveRole: (role) =>
        set({ activeRole: role }),

      logout: () =>
        set({ user: null, token: null, activeRole: null }),

      isAuthenticated: () => !!get().token,

      hasRole: (role) => {
        const activeRole = get().activeRole;
        if (!activeRole) return false;
        if (Array.isArray(role)) return role.includes(activeRole);
        return activeRole === role;
      },
    }),
    {
      name: 'nullus_auth',
      partialize: (state) => ({
        user: state.user,
        token: state.token,
        activeRole: state.activeRole,
      }),
    }
  )
);

/**
 * 역할별 초기 라우트 반환
 */
export function getRoleInitialRoute(role: UserRole): string {
  const routes: Record<UserRole, string> = {
    admin: '/admin/organization',
    devops: '/stack/install',
    developer: '/cicd/templates',
  };
  return routes[role];
}

/**
 * 역할별 접근 가능 메뉴 반환
 * - admin: 관리 메뉴만
 * - devops: 전체 메뉴
 * - developer: CI/CD, 관측성만
 */
export function getRoleMenuVisibility(role: UserRole) {
  return {
    stack: role === 'devops',
    cicd: role === 'devops' || role === 'developer',
    observability: role === 'devops' || role === 'developer',
    admin: role === 'admin' || role === 'devops',
  };
}
```

### 3.2 theme store

```typescript
// src/stores/theme.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

type Theme = 'dark' | 'light';

interface ThemeState {
  theme: Theme;
  toggleTheme: () => void;
  setTheme: (theme: Theme) => void;
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: 'dark',

      toggleTheme: () => {
        const next = get().theme === 'dark' ? 'light' : 'dark';
        set({ theme: next });
        document.documentElement.setAttribute('data-theme', next);
      },

      setTheme: (theme) => {
        set({ theme });
        document.documentElement.setAttribute('data-theme', theme);
      },
    }),
    {
      name: 'nullus_theme',
      onRehydrateStorage: () => (state) => {
        if (state) {
          document.documentElement.setAttribute('data-theme', state.theme);
        }
      },
    }
  )
);
```

### 3.3 sidebar store

```typescript
// src/stores/sidebar.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface SidebarState {
  collapsed: boolean;
  // 각 대메뉴 섹션의 펼침/접힘 상태
  expandedSections: Record<string, boolean>;

  toggleCollapsed: () => void;
  setCollapsed: (collapsed: boolean) => void;
  toggleSection: (section: string) => void;
  expandSection: (section: string) => void;
}

export const useSidebarStore = create<SidebarState>()(
  persist(
    (set, get) => ({
      collapsed: false,
      expandedSections: {
        stack: true,
        cicd: true,
        observability: true,
        admin: true,
      },

      toggleCollapsed: () =>
        set({ collapsed: !get().collapsed }),

      setCollapsed: (collapsed) =>
        set({ collapsed }),

      toggleSection: (section) =>
        set((state) => ({
          expandedSections: {
            ...state.expandedSections,
            [section]: !state.expandedSections[section],
          },
        })),

      expandSection: (section) =>
        set((state) => ({
          expandedSections: {
            ...state.expandedSections,
            [section]: true,
          },
        })),
    }),
    { name: 'nullus_sidebar' }
  )
);
```

### 3.4 stack store (스택 설치 임시 상태)

```typescript
// src/stores/stack.ts
import { create } from 'zustand';

// 5단계 설치 워크플로우의 각 탭 설정값
export interface ArtifactsConfig {
  packageRegistry: { tool: string; version: string };
  sourceRepository: { tool: string; version: string };
  containerRegistry: { tool: string; version: string };
  storageBackend: { tool: string; version: string };
}

export interface PipelineConfig {
  cicdPlatform: { tool: string; version: string };
  cdTool: { tool: string; version: string };
}

export interface MonitoringConfig {
  collection: { tool: string; version: string };
  visualization: { tool: string; version: string };
}

export interface LoggingConfig {
  collection: { tool: string; version: string };
  search: { tool: string; version: string };
}

export interface ResourceConfig {
  developerCount: number;
  concurrentRunners: number;
  commitsPerDay: number;
  buildFrequency: 'low' | 'medium' | 'high';
  currency: 'USD' | 'KRW' | 'CNY';
}

export interface StackInstallDraft {
  selectedTemplateId: string | null;
  clusterId: string | null;
  stackName: string;
  artifacts: ArtifactsConfig;
  pipeline: PipelineConfig;
  monitoring: MonitoringConfig;
  logging: LoggingConfig;
  resources: ResourceConfig;
  activeTab: 'artifacts' | 'pipeline' | 'monitoring' | 'logging' | 'resources' | 'yaml';
}

interface StackState {
  draft: StackInstallDraft;
  isDirty: boolean;

  setTemplate: (templateId: string) => void;
  setCluster: (clusterId: string) => void;
  setStackName: (name: string) => void;
  updateArtifacts: (config: Partial<ArtifactsConfig>) => void;
  updatePipeline: (config: Partial<PipelineConfig>) => void;
  updateMonitoring: (config: Partial<MonitoringConfig>) => void;
  updateLogging: (config: Partial<LoggingConfig>) => void;
  updateResources: (config: Partial<ResourceConfig>) => void;
  setActiveTab: (tab: StackInstallDraft['activeTab']) => void;
  resetDraft: () => void;
}

const DEFAULT_DRAFT: StackInstallDraft = {
  selectedTemplateId: null,
  clusterId: null,
  stackName: '',
  artifacts: {
    packageRegistry: { tool: 'gitlab', version: 'latest' },
    sourceRepository: { tool: 'gitlab', version: 'latest' },
    containerRegistry: { tool: 'gitlab-registry', version: 'latest' },
    storageBackend: { tool: 'minio', version: 'latest' },
  },
  pipeline: {
    cicdPlatform: { tool: 'gitlab-ci', version: 'latest' },
    cdTool: { tool: 'argocd', version: 'latest' },
  },
  monitoring: {
    collection: { tool: 'prometheus', version: 'latest' },
    visualization: { tool: 'grafana', version: 'latest' },
  },
  logging: {
    collection: { tool: 'opentelemetry', version: 'latest' },
    search: { tool: 'opensearch', version: 'latest' },
  },
  resources: {
    developerCount: 10,
    concurrentRunners: 5,
    commitsPerDay: 50,
    buildFrequency: 'medium',
    currency: 'KRW',
  },
  activeTab: 'artifacts',
};

export const useStackStore = create<StackState>()((set) => ({
  draft: DEFAULT_DRAFT,
  isDirty: false,

  setTemplate: (templateId) =>
    set((s) => ({ draft: { ...s.draft, selectedTemplateId: templateId }, isDirty: true })),

  setCluster: (clusterId) =>
    set((s) => ({ draft: { ...s.draft, clusterId }, isDirty: true })),

  setStackName: (name) =>
    set((s) => ({ draft: { ...s.draft, stackName: name }, isDirty: true })),

  updateArtifacts: (config) =>
    set((s) => ({
      draft: { ...s.draft, artifacts: { ...s.draft.artifacts, ...config } },
      isDirty: true,
    })),

  updatePipeline: (config) =>
    set((s) => ({
      draft: { ...s.draft, pipeline: { ...s.draft.pipeline, ...config } },
      isDirty: true,
    })),

  updateMonitoring: (config) =>
    set((s) => ({
      draft: { ...s.draft, monitoring: { ...s.draft.monitoring, ...config } },
      isDirty: true,
    })),

  updateLogging: (config) =>
    set((s) => ({
      draft: { ...s.draft, logging: { ...s.draft.logging, ...config } },
      isDirty: true,
    })),

  updateResources: (config) =>
    set((s) => ({
      draft: { ...s.draft, resources: { ...s.draft.resources, ...config } },
      isDirty: true,
    })),

  setActiveTab: (tab) =>
    set((s) => ({ draft: { ...s.draft, activeTab: tab } })),

  resetDraft: () =>
    set({ draft: DEFAULT_DRAFT, isDirty: false }),
}));
```

---

## 4. 라우팅 설계

### 4.1 라우트 정의

```typescript
// src/app/routes.tsx
import { createBrowserRouter, Navigate } from 'react-router-dom';
import { AppLayout } from './layout';
import { AuthGuard } from '@/components/shared/AuthGuard';
import { RoleGuard } from '@/components/shared/RoleGuard';

// 지연 로딩으로 번들 분할
import { lazy, Suspense } from 'react';
import { PageSkeleton } from '@/components/shared/PageSkeleton';

const LoginPage = lazy(() => import('@/features/auth/pages/LoginPage'));
const HomePage = lazy(() => import('@/features/home/pages/HomePage'));

// Stack
const StackTemplatePage = lazy(() => import('@/features/stack/pages/StackTemplatePage'));
const StackInstallPage = lazy(() => import('@/features/stack/pages/StackInstallPage'));
const StackListPage = lazy(() => import('@/features/stack/pages/StackListPage'));
const StackHistoryPage = lazy(() => import('@/features/stack/pages/StackHistoryPage'));
const StackVersionPage = lazy(() => import('@/features/stack/pages/StackVersionPage'));

// CI/CD
const CicdTemplatePage = lazy(() => import('@/features/cicd/pages/CicdTemplatePage'));
const CicdListPage = lazy(() => import('@/features/cicd/pages/CicdListPage'));
const CicdHistoryPage = lazy(() => import('@/features/cicd/pages/CicdHistoryPage'));
const AppDeployPage = lazy(() => import('@/features/cicd/pages/AppDeployPage'));

// Observability
const MonitoringDashboardPage = lazy(() => import('@/features/observability/pages/MonitoringDashboardPage'));
const AlertListPage = lazy(() => import('@/features/observability/pages/AlertListPage'));
const AlertHistoryPage = lazy(() => import('@/features/observability/pages/AlertHistoryPage'));

// Admin
const OrganizationPage = lazy(() => import('@/features/admin/pages/OrganizationPage'));
const UsersPage = lazy(() => import('@/features/admin/pages/UsersPage'));
const ClustersPage = lazy(() => import('@/features/admin/pages/ClustersPage'));

function withSuspense(Component: React.LazyExoticComponent<() => JSX.Element>) {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <Component />
    </Suspense>
  );
}

export const router = createBrowserRouter([
  {
    path: '/login',
    element: withSuspense(LoginPage),
  },
  {
    path: '/',
    element: (
      <AuthGuard>
        <AppLayout />
      </AuthGuard>
    ),
    children: [
      {
        index: true,
        element: <Navigate to="/home" replace />,
      },
      {
        path: 'home',
        element: withSuspense(HomePage),
      },

      // DevSecOps 스택 (admin 제외, devops만)
      {
        path: 'stack',
        element: <RoleGuard allowed={['devops']} />,
        children: [
          { path: 'templates', element: withSuspense(StackTemplatePage) },
          { path: 'install', element: withSuspense(StackInstallPage) },
          { path: 'install/:stackId', element: withSuspense(StackInstallPage) },
          { path: 'list', element: withSuspense(StackListPage) },
          { path: 'history', element: withSuspense(StackHistoryPage) },
          { path: 'version', element: withSuspense(StackVersionPage) },
        ],
      },

      // CI/CD (devops + developer)
      {
        path: 'cicd',
        element: <RoleGuard allowed={['devops', 'developer']} />,
        children: [
          { path: 'templates', element: withSuspense(CicdTemplatePage) },
          { path: 'list', element: withSuspense(CicdListPage) },
          { path: 'history', element: withSuspense(CicdHistoryPage) },
          {
            path: 'deploy',
            element: (
              <RoleGuard allowed={['developer']}>
                {withSuspense(AppDeployPage)}
              </RoleGuard>
            ),
          },
        ],
      },

      // 관측성 (devops + developer)
      {
        path: 'observability',
        element: <RoleGuard allowed={['devops', 'developer']} />,
        children: [
          { path: 'monitoring', element: withSuspense(MonitoringDashboardPage) },
          { path: 'alerts', element: withSuspense(AlertListPage) },
          { path: 'alert-history', element: withSuspense(AlertHistoryPage) },
        ],
      },

      // 관리 (admin + devops)
      {
        path: 'admin',
        element: <RoleGuard allowed={['admin', 'devops']} />,
        children: [
          { path: 'organization', element: withSuspense(OrganizationPage) },
          { path: 'users', element: withSuspense(UsersPage) },
          { path: 'clusters', element: withSuspense(ClustersPage) },
        ],
      },
    ],
  },
  {
    path: '*',
    element: <Navigate to="/home" replace />,
  },
]);
```

### 4.2 AuthGuard

```typescript
// src/components/shared/AuthGuard.tsx
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/stores/auth';

interface AuthGuardProps {
  children: React.ReactNode;
}

export function AuthGuard({ children }: AuthGuardProps) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated());
  const location = useLocation();

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}
```

### 4.3 RoleGuard

```typescript
// src/components/shared/RoleGuard.tsx
import { Navigate, Outlet } from 'react-router-dom';
import { useAuthStore, UserRole } from '@/stores/auth';

interface RoleGuardProps {
  allowed: UserRole[];
  children?: React.ReactNode;
}

export function RoleGuard({ allowed, children }: RoleGuardProps) {
  const activeRole = useAuthStore((s) => s.activeRole);

  if (!activeRole || !allowed.includes(activeRole)) {
    return <Navigate to="/home" replace />;
  }

  return children ? <>{children}</> : <Outlet />;
}
```

### 4.4 역할별 라우트 매핑 요약

| 경로 | Admin | DevOps | Developer |
|------|-------|--------|-----------|
| `/home` | O | O | O |
| `/stack/*` | X | O | X |
| `/cicd/*` | X | O | O |
| `/cicd/deploy` | X | X | O |
| `/observability/*` | X | O | O |
| `/admin/*` | O | O | X |

---

## 5. API 통신 레이어

### 5.1 Axios 클라이언트

```typescript
// src/lib/api.ts
import axios, { AxiosError, AxiosInstance, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from '@/stores/auth';

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8090/api/v1';

export const apiClient: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 요청 인터셉터: JWT 토큰 주입
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = useAuthStore.getState().token;
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// 응답 인터셉터: 토큰 만료 처리
apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
    }
    return Promise.reject(toApiError(error));
  }
);

// 표준 API 에러 타입
export interface ApiError {
  code: string;       // 예: "CLUSTER_NOT_FOUND"
  message: string;
  status: number;
  details?: Record<string, unknown>;
}

function toApiError(error: AxiosError): ApiError {
  const data = error.response?.data as Record<string, unknown> | undefined;
  return {
    code: (data?.code as string) ?? 'UNKNOWN_ERROR',
    message: (data?.message as string) ?? error.message,
    status: error.response?.status ?? 0,
    details: data?.details as Record<string, unknown> | undefined,
  };
}
```

### 5.2 TanStack Query 클라이언트

```typescript
// src/lib/queryClient.ts
import { QueryClient, QueryCache, MutationCache } from '@tanstack/react-query';
import { toast } from '@/lib/toast';
import type { ApiError } from './api';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5,        // 5분 캐시
      retry: (failureCount, error) => {
        const apiError = error as ApiError;
        // 4xx 에러는 재시도 안 함
        if (apiError.status >= 400 && apiError.status < 500) return false;
        return failureCount < 2;
      },
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: false,
    },
  },
  queryCache: new QueryCache({
    onError: (error) => {
      const apiError = error as ApiError;
      if (apiError.status >= 500) {
        toast.error(`서버 오류가 발생했습니다. (${apiError.code})`);
      }
    },
  }),
  mutationCache: new MutationCache({
    onError: (error) => {
      const apiError = error as ApiError;
      toast.error(apiError.message ?? '요청 처리 중 오류가 발생했습니다.');
    },
  }),
});
```

### 5.3 쿼리 키 컨벤션

```typescript
// src/types/queryKeys.ts
// 계층적 쿼리 키 팩토리 패턴
export const queryKeys = {
  // Stack
  stacks: {
    all: ['stacks'] as const,
    list: (filters?: Record<string, unknown>) =>
      ['stacks', 'list', filters] as const,
    detail: (id: string) => ['stacks', 'detail', id] as const,
    history: (id: string) => ['stacks', 'history', id] as const,
    templates: () => ['stacks', 'templates'] as const,
    compatibility: () => ['stacks', 'compatibility'] as const,
  },
  // CI/CD
  cicd: {
    pipelines: (filters?: Record<string, unknown>) =>
      ['cicd', 'pipelines', filters] as const,
    templates: () => ['cicd', 'templates'] as const,
    history: (id: string) => ['cicd', 'history', id] as const,
  },
  // Observability
  observability: {
    metrics: (clusterId: string) => ['observability', 'metrics', clusterId] as const,
    alerts: (filters?: Record<string, unknown>) =>
      ['observability', 'alerts', filters] as const,
    alertHistory: (filters?: Record<string, unknown>) =>
      ['observability', 'alertHistory', filters] as const,
  },
  // Admin
  admin: {
    organization: () => ['admin', 'organization'] as const,
    users: (orgId: string) => ['admin', 'users', orgId] as const,
    clusters: () => ['admin', 'clusters'] as const,
    cluster: (id: string) => ['admin', 'clusters', id] as const,
  },
};
```

### 5.4 API 모듈 예시 (stack)

```typescript
// src/features/stack/api/stack.api.ts
import { apiClient } from '@/lib/api';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/types/queryKeys';
import type { Stack, StackTemplate, StackInstallRequest, DeployResult } from '@/types/stack.types';

// REST 호출 함수
const stackApi = {
  getTemplates: () =>
    apiClient.get<StackTemplate[]>('/stacks/templates').then((r) => r.data),

  getList: (params?: { status?: string; search?: string; page?: number; size?: number }) =>
    apiClient.get<{ items: Stack[]; total: number }>('/stacks', { params }).then((r) => r.data),

  deploy: (request: StackInstallRequest) =>
    apiClient.post<DeployResult>('/stacks/deploy', request).then((r) => r.data),

  saveDraft: (request: StackInstallRequest) =>
    apiClient.post<{ draftId: string }>('/stacks/draft', request).then((r) => r.data),

  rollback: (stackId: string, targetVersion: string) =>
    apiClient.post(`/stacks/${stackId}/rollback`, { targetVersion }).then((r) => r.data),
};

// React Query 훅
export function useStackTemplates() {
  return useQuery({
    queryKey: queryKeys.stacks.templates(),
    queryFn: stackApi.getTemplates,
  });
}

export function useStackList(filters?: { status?: string; search?: string }) {
  return useQuery({
    queryKey: queryKeys.stacks.list(filters),
    queryFn: () => stackApi.getList(filters),
  });
}

export function useDeployStack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: stackApi.deploy,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.stacks.all });
    },
  });
}
```

---

## 6. WebSocket 클라이언트

### 6.1 WebSocket 클라이언트 구현

```typescript
// src/lib/ws.ts
import { useAuthStore } from '@/stores/auth';

type WsEventType = 'deploy_log' | 'deploy_status' | 'metric_update' | 'alert';

export interface WsMessage<T = unknown> {
  type: WsEventType;
  payload: T;
  timestamp: string;
}

export type WsStatus = 'connecting' | 'connected' | 'disconnected' | 'error';

type MessageHandler<T = unknown> = (message: WsMessage<T>) => void;

class NullusWebSocket {
  private ws: WebSocket | null = null;
  private url: string;
  private handlers = new Map<WsEventType, Set<MessageHandler>>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempts = 0;
  private readonly maxReconnectAttempts = 5;
  private readonly reconnectBaseDelay = 1000;
  private statusListeners = new Set<(status: WsStatus) => void>();

  constructor() {
    this.url = import.meta.env.VITE_WS_BASE_URL ?? 'ws://localhost:8090/ws';
  }

  connect(): void {
    const token = useAuthStore.getState().token;
    if (!token) return;

    this.notifyStatus('connecting');
    this.ws = new WebSocket(`${this.url}?token=${encodeURIComponent(token)}`);

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.notifyStatus('connected');
    };

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const message = JSON.parse(event.data) as WsMessage;
        const handlers = this.handlers.get(message.type);
        handlers?.forEach((h) => h(message));
      } catch {
        // 파싱 실패는 무시
      }
    };

    this.ws.onclose = () => {
      this.notifyStatus('disconnected');
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.notifyStatus('error');
    };
  }

  disconnect(): void {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.ws?.close();
    this.ws = null;
  }

  on<T>(type: WsEventType, handler: MessageHandler<T>): () => void {
    if (!this.handlers.has(type)) {
      this.handlers.set(type, new Set());
    }
    this.handlers.get(type)!.add(handler as MessageHandler);

    // unsubscribe 반환
    return () => {
      this.handlers.get(type)?.delete(handler as MessageHandler);
    };
  }

  onStatusChange(listener: (status: WsStatus) => void): () => void {
    this.statusListeners.add(listener);
    return () => this.statusListeners.delete(listener);
  }

  private notifyStatus(status: WsStatus): void {
    this.statusListeners.forEach((l) => l(status));
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) return;

    const delay = this.reconnectBaseDelay * 2 ** this.reconnectAttempts;
    this.reconnectAttempts += 1;

    this.reconnectTimer = setTimeout(() => {
      this.connect();
    }, delay);
  }
}

// 싱글턴 인스턴스
export const wsClient = new NullusWebSocket();
```

### 6.2 배포 로그 훅

```typescript
// src/features/stack/hooks/useDeployLog.ts
import { useState, useEffect, useRef } from 'react';
import { wsClient } from '@/lib/ws';

export interface DeployLogEntry {
  level: 'info' | 'warn' | 'error' | 'success';
  message: string;
  timestamp: string;
  phase?: string; // 'phase-a' | 'phase-b' | 'phase-c'
}

export type DeployStatus =
  | 'idle'
  | 'running'
  | 'success'
  | 'failed'
  | 'rolling_back';

export function useDeployLog(deployId: string | null) {
  const [logs, setLogs] = useState<DeployLogEntry[]>([]);
  const [status, setStatus] = useState<DeployStatus>('idle');
  const [progress, setProgress] = useState(0); // 0-100
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!deployId) return;

    const unsubLog = wsClient.on<DeployLogEntry>('deploy_log', (msg) => {
      if ((msg.payload as DeployLogEntry & { deployId: string }).deployId !== deployId) return;
      setLogs((prev) => [...prev, msg.payload as DeployLogEntry]);
      // 자동 스크롤
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    });

    const unsubStatus = wsClient.on<{ deployId: string; status: DeployStatus; progress: number }>(
      'deploy_status',
      (msg) => {
        if (msg.payload.deployId !== deployId) return;
        setStatus(msg.payload.status);
        setProgress(msg.payload.progress);
      }
    );

    return () => {
      unsubLog();
      unsubStatus();
    };
  }, [deployId]);

  return { logs, status, progress, bottomRef };
}
```

---

## 7. i18n 설계

### 7.1 i18next 초기화

```typescript
// src/i18n/index.ts
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import en from './en.json';
import ko from './ko.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      ko: { translation: ko },
    },
    fallbackLng: 'ko',
    supportedLngs: ['en', 'ko'],
    detection: {
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: 'nullus_locale',
      caches: ['localStorage'],
    },
    interpolation: {
      escapeValue: false, // React가 XSS 처리
    },
  });

export default i18n;
```

### 7.2 키 네이밍 컨벤션

네임스페이스 없이 단일 파일 사용. 키 구조는 `{도메인}.{컴포넌트}.{요소}` 형식.

```
{도메인}     : nav, common, auth, stack, cicd, observability, admin, home
{컴포넌트}   : sidebar, header, page, table, modal, form, button
{요소}       : title, label, placeholder, tooltip, confirm, success, error
```

### 7.3 en.json 구조

```json
{
  "common": {
    "actions": {
      "save": "Save",
      "saveDraft": "Save Draft",
      "cancel": "Cancel",
      "delete": "Delete",
      "edit": "Edit",
      "register": "Register",
      "deploy": "Deploy",
      "rollback": "Rollback",
      "copy": "Copy",
      "close": "Close",
      "confirm": "Confirm",
      "search": "Search",
      "filter": "Filter",
      "refresh": "Refresh"
    },
    "status": {
      "connected": "Connected",
      "pending": "Pending",
      "error": "Error",
      "inactive": "Inactive"
    },
    "pagination": {
      "prev": "Previous",
      "next": "Next",
      "total": "Total {{count}} items"
    },
    "confirm": {
      "deleteTitle": "Confirm Delete",
      "deleteMessage": "Are you sure you want to delete '{{name}}'? This action cannot be undone.",
      "deployTitle": "Confirm Deploy",
      "deployMessage": "Deploy the stack '{{name}}' to cluster '{{cluster}}'?"
    },
    "loading": "Loading...",
    "noData": "No data found",
    "copySuccess": "Copied to clipboard"
  },
  "nav": {
    "stack": {
      "group": "DevSecOps Stack",
      "templates": "Stack Templates",
      "install": "Stack Install",
      "list": "Stack List",
      "history": "Stack History",
      "version": "Version Management"
    },
    "cicd": {
      "group": "CI/CD",
      "templates": "CI/CD Templates",
      "list": "CI/CD List",
      "history": "CI/CD History",
      "deploy": "App Deploy"
    },
    "observability": {
      "group": "Observability",
      "monitoring": "Monitoring Dashboard",
      "alerts": "Alert Rules",
      "alertHistory": "Alert History"
    },
    "admin": {
      "group": "Management",
      "organization": "Organization",
      "users": "User Management",
      "clusters": "Cluster Management"
    },
    "user": {
      "group": "User",
      "logout": "Logout"
    }
  },
  "auth": {
    "login": {
      "title": "Sign in to Nullus",
      "subtitle": "Kubernetes-based DevSecOps Automation Platform",
      "email": "Email",
      "password": "Password",
      "submit": "Sign In",
      "error": "Invalid email or password"
    },
    "roles": {
      "admin": "Admin",
      "devops": "DevOps Engineer",
      "developer": "Developer"
    }
  },
  "stack": {
    "templates": {
      "title": "Stack Templates",
      "subtitle": "Select a Golden Path template",
      "useTemplate": "Use This Template",
      "tools": "Included Tools",
      "estimatedTime": "Est. Install Time",
      "requiredResources": "Required Resources"
    },
    "install": {
      "title": "Stack Install",
      "subtitle": "Configure your DevSecOps stack",
      "tabs": {
        "artifacts": "Artifacts",
        "pipeline": "Pipeline",
        "monitoring": "Monitoring",
        "logging": "Logging",
        "resources": "Resources",
        "yaml": "YAML Preview"
      },
      "artifacts": {
        "packageRegistry": "Package Registry",
        "sourceRepository": "Source Repository",
        "containerRegistry": "Container Registry",
        "storageBackend": "Storage Backend",
        "version": "Version"
      },
      "pipeline": {
        "cicdPlatform": "CI/CD Platform",
        "cdTool": "CD Tool"
      },
      "resources": {
        "developerCount": "Number of Developers",
        "concurrentRunners": "Concurrent Runners",
        "commitsPerDay": "Commits per Day",
        "buildFrequency": "Build Frequency",
        "estimatedCpu": "Estimated CPU",
        "estimatedMemory": "Estimated Memory",
        "estimatedStorage": "Estimated Storage",
        "estimatedCost": "Estimated Monthly Cost",
        "currency": "Currency"
      },
      "preview": {
        "deployScript": "Preview Deploy Script",
        "k8sObjects": "Preview K8s Objects"
      },
      "deploy": {
        "button": "Deploy",
        "running": "Deploying...",
        "success": "Stack deployed successfully",
        "failed": "Deployment failed"
      }
    },
    "list": {
      "title": "Stack List",
      "columns": {
        "name": "Name",
        "template": "Template",
        "cluster": "Cluster",
        "version": "Version",
        "status": "Status",
        "deployedAt": "Deployed At",
        "actions": "Actions"
      }
    }
  },
  "admin": {
    "organization": {
      "title": "Organization",
      "name": "Organization Name",
      "slug": "Slug",
      "domain": "Domain",
      "clusterScope": "Cluster Access Scope",
      "members": "Members",
      "inviteMember": "Invite Member",
      "inviteEmail": "Email address to invite"
    },
    "users": {
      "title": "User Management",
      "columns": {
        "name": "Name",
        "email": "Email",
        "role": "Role",
        "status": "Status",
        "joinedAt": "Joined At",
        "actions": "Actions"
      },
      "roleChange": "Change Role",
      "deactivate": "Deactivate",
      "activate": "Activate"
    },
    "clusters": {
      "title": "Cluster Management",
      "register": "Register Cluster",
      "name": "Cluster Name",
      "endpoint": "API Endpoint",
      "namespace": "Default Namespace",
      "kubeconfig": "Kubeconfig",
      "uploadKubeconfig": "Upload Kubeconfig",
      "connectionStatus": "Connection Status",
      "testConnection": "Test Connection"
    }
  }
}
```

### 7.4 ko.json 구조 (핵심 부분)

```json
{
  "common": {
    "actions": {
      "save": "저장",
      "saveDraft": "임시 저장",
      "cancel": "취소",
      "delete": "삭제",
      "edit": "수정",
      "register": "등록",
      "deploy": "배포",
      "rollback": "롤백",
      "copy": "복사",
      "close": "닫기",
      "confirm": "확인",
      "search": "검색",
      "filter": "필터",
      "refresh": "새로고침"
    },
    "status": {
      "connected": "연결됨",
      "pending": "대기 중",
      "error": "오류",
      "inactive": "비활성"
    },
    "pagination": {
      "prev": "이전",
      "next": "다음",
      "total": "총 {{count}}개"
    },
    "confirm": {
      "deleteTitle": "삭제 확인",
      "deleteMessage": "'{{name}}'을(를) 삭제하시겠습니까? 이 작업은 되돌릴 수 없습니다.",
      "deployTitle": "배포 확인",
      "deployMessage": "'{{cluster}}' 클러스터에 '{{name}}' 스택을 배포하시겠습니까?"
    },
    "loading": "로딩 중...",
    "noData": "데이터가 없습니다",
    "copySuccess": "클립보드에 복사되었습니다"
  },
  "nav": {
    "stack": {
      "group": "데브섹옵스 스택",
      "templates": "스택 템플릿",
      "install": "스택 설치",
      "list": "스택 목록",
      "history": "스택 이력",
      "version": "스택 버전 관리"
    },
    "cicd": {
      "group": "CI/CD",
      "templates": "CI/CD 템플릿",
      "list": "CI/CD 목록",
      "history": "CI/CD 이력",
      "deploy": "앱 배포"
    },
    "observability": {
      "group": "관측성",
      "monitoring": "모니터링 대시보드",
      "alerts": "알림 규칙",
      "alertHistory": "알림 이력"
    },
    "admin": {
      "group": "관리",
      "organization": "조직",
      "users": "사용자 관리",
      "clusters": "클러스터 관리"
    },
    "user": {
      "group": "사용자",
      "logout": "로그아웃"
    }
  },
  "auth": {
    "login": {
      "title": "Nullus에 로그인",
      "subtitle": "Kubernetes 기반 DevSecOps 자동화 플랫폼",
      "email": "이메일",
      "password": "비밀번호",
      "submit": "로그인",
      "error": "이메일 또는 비밀번호가 올바르지 않습니다"
    },
    "roles": {
      "admin": "관리자",
      "devops": "DevOps 엔지니어",
      "developer": "개발자"
    }
  },
  "stack": {
    "templates": {
      "title": "스택 템플릿",
      "subtitle": "Golden Path 템플릿을 선택하세요",
      "useTemplate": "이 템플릿 사용",
      "tools": "포함 도구",
      "estimatedTime": "예상 설치 시간",
      "requiredResources": "필요 리소스"
    },
    "install": {
      "title": "스택 설치",
      "subtitle": "DevSecOps 스택을 구성하세요",
      "tabs": {
        "artifacts": "아티팩트",
        "pipeline": "파이프라인",
        "monitoring": "모니터링",
        "logging": "로깅",
        "resources": "리소스",
        "yaml": "YAML 미리보기"
      },
      "resources": {
        "developerCount": "개발자 수",
        "concurrentRunners": "동시 러너 수",
        "commitsPerDay": "일일 커밋 수",
        "buildFrequency": "빌드 빈도",
        "estimatedCpu": "예상 CPU",
        "estimatedMemory": "예상 메모리",
        "estimatedStorage": "예상 스토리지",
        "estimatedCost": "예상 월 비용",
        "currency": "통화"
      },
      "deploy": {
        "button": "배포",
        "running": "배포 중...",
        "success": "스택이 성공적으로 배포되었습니다",
        "failed": "배포에 실패했습니다"
      }
    }
  },
  "admin": {
    "organization": {
      "title": "조직",
      "name": "조직명",
      "slug": "슬러그",
      "domain": "도메인",
      "clusterScope": "클러스터 접근 범위",
      "members": "멤버",
      "inviteMember": "멤버 초대",
      "inviteEmail": "초대할 이메일 주소"
    },
    "users": {
      "title": "사용자 관리",
      "columns": {
        "name": "이름",
        "email": "이메일",
        "role": "역할",
        "status": "상태",
        "joinedAt": "가입일",
        "actions": "작업"
      },
      "roleChange": "역할 변경",
      "deactivate": "비활성화",
      "activate": "활성화"
    },
    "clusters": {
      "title": "클러스터 관리",
      "register": "클러스터 등록",
      "name": "클러스터 이름",
      "endpoint": "API 엔드포인트",
      "namespace": "기본 네임스페이스",
      "kubeconfig": "Kubeconfig",
      "uploadKubeconfig": "Kubeconfig 업로드",
      "connectionStatus": "연결 상태",
      "testConnection": "연결 테스트"
    }
  }
}
```

---

## 8. 공통 컴포넌트 상세 스펙

### 8.1 DataTable

```typescript
// src/components/shared/DataTable.tsx
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
} from '@tanstack/react-table';
import { useState } from 'react';
import { ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react';
import { cn } from '@/lib/utils';

interface DataTableProps<TData> {
  /** 컬럼 정의 (TanStack Table ColumnDef) */
  columns: ColumnDef<TData>[];
  /** 테이블 데이터 */
  data: TData[];
  /** 로딩 상태 */
  isLoading?: boolean;
  /** 페이지당 행 수 (기본값: 10) */
  pageSize?: number;
  /** 전역 검색 플레이스홀더 */
  searchPlaceholder?: string;
  /** 행 클릭 핸들러 */
  onRowClick?: (row: TData) => void;
  /** 테이블 높이 고정 (스크롤 영역) */
  maxHeight?: string;
  /** 빈 데이터 메시지 */
  emptyMessage?: string;
}

export function DataTable<TData>({
  columns,
  data,
  isLoading = false,
  pageSize = 10,
  searchPlaceholder = '검색...',
  onRowClick,
  maxHeight,
  emptyMessage = '데이터가 없습니다',
}: DataTableProps<TData>) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [globalFilter, setGlobalFilter] = useState('');

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnFilters, globalFilter },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize } },
  });

  return (
    <div className="space-y-4">
      {/* 검색 입력 */}
      <input
        value={globalFilter}
        onChange={(e) => setGlobalFilter(e.target.value)}
        placeholder={searchPlaceholder}
        className="w-full max-w-sm px-3 py-2 text-sm rounded-lg
          bg-[var(--color-surface-card)] border border-[var(--color-border-default)]
          text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]
          focus:outline-none focus:border-indigo-500"
      />

      {/* 테이블 */}
      <div
        className={cn(
          'rounded-xl border border-[var(--color-border-default)] overflow-hidden',
          maxHeight && 'overflow-y-auto'
        )}
        style={maxHeight ? { maxHeight } : undefined}
      >
        <table className="w-full">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id} className="border-b border-[var(--color-border-default)]">
                {headerGroup.headers.map((header) => (
                  <th
                    key={header.id}
                    className="h-10 px-4 text-left text-xs font-semibold
                      text-[var(--color-text-secondary)] uppercase tracking-wider"
                    style={{ width: header.getSize() }}
                  >
                    {header.isPlaceholder ? null : (
                      <button
                        className={cn(
                          'flex items-center gap-1',
                          header.column.getCanSort() && 'cursor-pointer hover:text-[var(--color-text-primary)]'
                        )}
                        onClick={header.column.getToggleSortingHandler()}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {header.column.getCanSort() && (
                          <span className="text-[var(--color-text-muted)]">
                            {header.column.getIsSorted() === 'asc' ? (
                              <ChevronUp className="h-3 w-3" />
                            ) : header.column.getIsSorted() === 'desc' ? (
                              <ChevronDown className="h-3 w-3" />
                            ) : (
                              <ChevronsUpDown className="h-3 w-3" />
                            )}
                          </span>
                        )}
                      </button>
                    )}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {isLoading ? (
              Array.from({ length: pageSize }).map((_, i) => (
                <tr key={i} className="border-b border-[var(--color-border-default)]">
                  {columns.map((_, j) => (
                    <td key={j} className="h-12 px-4">
                      <div className="h-4 rounded bg-[var(--color-border-default)] animate-pulse" />
                    </td>
                  ))}
                </tr>
              ))
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="h-32 text-center text-[var(--color-text-muted)]"
                >
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className={cn(
                    'h-12 border-b border-[var(--color-border-default)]',
                    'transition-colors duration-150',
                    'hover:bg-indigo-500/5',
                    onRowClick && 'cursor-pointer'
                  )}
                  onClick={() => onRowClick?.(row.original)}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} className="px-4 text-sm text-[var(--color-text-primary)]">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* 페이지네이션 */}
      <div className="flex items-center justify-between text-sm text-[var(--color-text-secondary)]">
        <span>
          총 {table.getFilteredRowModel().rows.length}개
        </span>
        <div className="flex items-center gap-2">
          <button
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
            className="px-3 py-1 rounded border border-[var(--color-border-default)]
              disabled:opacity-40 disabled:cursor-not-allowed hover:border-indigo-500"
          >
            이전
          </button>
          <span>
            {table.getState().pagination.pageIndex + 1} / {table.getPageCount()}
          </span>
          <button
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
            className="px-3 py-1 rounded border border-[var(--color-border-default)]
              disabled:opacity-40 disabled:cursor-not-allowed hover:border-indigo-500"
          >
            다음
          </button>
        </div>
      </div>
    </div>
  );
}
```

### 8.2 Modal

```typescript
// src/components/shared/Modal.tsx
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

type ModalSize = 'default' | 'wide';

interface ModalProps {
  /** 모달 열림 여부 */
  open: boolean;
  /** 닫기 핸들러 */
  onClose: () => void;
  /** 모달 제목 */
  title: string;
  /** 모달 내용 */
  children: React.ReactNode;
  /**
   * 모달 크기
   * - `default`: 480px (등록/수정 폼)
   * - `wide`: 800px (코드 미리보기, K8s YAML)
   */
  size?: ModalSize;
  /** 하단 액션 버튼 영역 */
  footer?: React.ReactNode;
}

export function Modal({
  open,
  onClose,
  title,
  children,
  size = 'default',
  footer,
}: ModalProps) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className={cn(
          'bg-[var(--color-surface-card)] border border-[var(--color-border-default)]',
          'text-[var(--color-text-primary)] p-0',
          size === 'default' ? 'max-w-[480px]' : 'max-w-[800px]'
        )}
        // Radix가 기본 제공하는 닫기 버튼 숨김 (커스텀 사용)
        hideClose
      >
        {/* 헤더 */}
        <DialogHeader className="flex flex-row items-center justify-between px-6 py-4
          border-b border-[var(--color-border-default)]"
        >
          <DialogTitle className="text-base font-semibold">
            {title}
          </DialogTitle>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-white/10 transition-colors"
            aria-label="닫기"
          >
            <X className="h-4 w-4 text-[var(--color-text-secondary)]" />
          </button>
        </DialogHeader>

        {/* 본문 */}
        <div className="px-6 py-4 overflow-y-auto max-h-[70vh]">
          {children}
        </div>

        {/* 푸터 */}
        {footer && (
          <div className="flex items-center justify-end gap-3 px-6 py-4
            border-t border-[var(--color-border-default)]"
          >
            {footer}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
```

### 8.3 StepWizard

```typescript
// src/components/shared/StepWizard.tsx
import { cn } from '@/lib/utils';

export interface WizardStep {
  id: string;
  label: string;
  /** 완료 여부 (선택적 표시) */
  completed?: boolean;
  /** 단계 비활성화 */
  disabled?: boolean;
}

interface StepWizardProps {
  steps: WizardStep[];
  activeStep: string;
  onStepChange: (stepId: string) => void;
  children: React.ReactNode;
}

export function StepWizard({ steps, activeStep, onStepChange, children }: StepWizardProps) {
  return (
    <div className="flex flex-col gap-0">
      {/* 탭 헤더 */}
      <div className="flex border-b border-[var(--color-border-default)]">
        {steps.map((step, index) => {
          const isActive = step.id === activeStep;
          return (
            <button
              key={step.id}
              onClick={() => !step.disabled && onStepChange(step.id)}
              disabled={step.disabled}
              className={cn(
                'relative flex items-center gap-2 px-5 py-3 text-sm font-medium',
                'border-b-2 transition-colors duration-200',
                'disabled:opacity-40 disabled:cursor-not-allowed',
                isActive
                  ? 'border-indigo-500 text-indigo-400'
                  : 'border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
              )}
            >
              {/* 단계 번호 */}
              <span
                className={cn(
                  'flex h-5 w-5 items-center justify-center rounded-full text-xs',
                  step.completed
                    ? 'bg-green-500/20 text-green-400'
                    : isActive
                    ? 'bg-indigo-500/20 text-indigo-400'
                    : 'bg-[var(--color-border-default)] text-[var(--color-text-muted)]'
                )}
              >
                {step.completed ? '✓' : index + 1}
              </span>
              {step.label}
            </button>
          );
        })}
      </div>

      {/* 탭 콘텐츠 */}
      <div className="pt-6">
        {children}
      </div>
    </div>
  );
}
```

### 8.4 CodePreview

```typescript
// src/components/shared/CodePreview.tsx
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { javascript } from '@codemirror/lang-javascript';
import { oneDark } from '@codemirror/theme-one-dark';
import { Copy, Check } from 'lucide-react';
import { useState } from 'react';
import { cn } from '@/lib/utils';

type CodeLang = 'yaml' | 'bash' | 'json';

interface CodePreviewProps {
  /** 표시할 코드 */
  code: string;
  /** 언어 (구문 강조) */
  language?: CodeLang;
  /** 파일명 또는 레이블 */
  filename?: string;
  /** 최대 높이 (스크롤) */
  maxHeight?: string;
  /** 읽기 전용 여부 (기본값: true) */
  readOnly?: boolean;
  /** 코드 변경 핸들러 (readOnly=false 시) */
  onChange?: (value: string) => void;
}

const LANG_EXTENSIONS: Record<CodeLang, ReturnType<typeof yaml | typeof javascript>[]> = {
  yaml: [yaml()],
  bash: [javascript()], // bash 미지원으로 js 하이라이팅 사용
  json: [javascript()],
};

export function CodePreview({
  code,
  language = 'yaml',
  filename,
  maxHeight = '400px',
  readOnly = true,
  onChange,
}: CodePreviewProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="rounded-xl border border-[var(--color-border-default)] overflow-hidden">
      {/* 헤더 바 */}
      <div className="flex items-center justify-between px-4 py-2
        bg-[var(--color-surface-card)] border-b border-[var(--color-border-default)]"
      >
        <div className="flex items-center gap-2">
          {/* 맥OS 스타일 도트 */}
          <span className="h-3 w-3 rounded-full bg-red-500/60" />
          <span className="h-3 w-3 rounded-full bg-amber-500/60" />
          <span className="h-3 w-3 rounded-full bg-green-500/60" />
          {filename && (
            <span className="ml-2 text-xs text-[var(--color-text-muted)] font-mono">
              {filename}
            </span>
          )}
        </div>
        <button
          onClick={handleCopy}
          className={cn(
            'flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium',
            'transition-colors duration-150',
            copied
              ? 'text-green-400 bg-green-500/10'
              : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-white/5'
          )}
          aria-label="클립보드에 복사"
        >
          {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
          {copied ? '복사됨' : '복사'}
        </button>
      </div>

      {/* 코드 에디터 */}
      <div style={{ maxHeight, overflowY: 'auto' }}>
        <CodeMirror
          value={code}
          extensions={LANG_EXTENSIONS[language]}
          theme={oneDark}
          readOnly={readOnly}
          onChange={onChange}
          basicSetup={{
            lineNumbers: true,
            foldGutter: false,
            dropCursor: false,
            allowMultipleSelections: false,
          }}
          style={{ fontSize: '13px' }}
        />
      </div>
    </div>
  );
}
```

### 8.5 StatusBadge

```typescript
// src/components/shared/StatusBadge.tsx
import { CheckCircle, Clock, AlertCircle, MinusCircle } from 'lucide-react';
import { cn } from '@/lib/utils';

export type StatusType = 'connected' | 'pending' | 'error' | 'inactive';

interface StatusBadgeProps {
  status: StatusType;
  /** 커스텀 레이블 (없으면 status 기반 기본값 사용) */
  label?: string;
  size?: 'sm' | 'md';
}

const STATUS_CONFIG: Record<
  StatusType,
  {
    icon: React.ElementType;
    bg: string;
    text: string;
    defaultLabel: string;
  }
> = {
  connected: {
    icon: CheckCircle,
    bg: 'bg-green-500/15',
    text: 'text-green-400',
    defaultLabel: 'Connected',
  },
  pending: {
    icon: Clock,
    bg: 'bg-amber-500/15',
    text: 'text-amber-400',
    defaultLabel: 'Pending',
  },
  error: {
    icon: AlertCircle,
    bg: 'bg-red-500/15',
    text: 'text-red-400',
    defaultLabel: 'Error',
  },
  inactive: {
    icon: MinusCircle,
    bg: 'bg-slate-500/15',
    text: 'text-slate-400',
    defaultLabel: 'Inactive',
  },
};

export function StatusBadge({ status, label, size = 'md' }: StatusBadgeProps) {
  const config = STATUS_CONFIG[status];
  const Icon = config.icon;

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full font-medium',
        config.bg,
        config.text,
        size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-xs'
      )}
    >
      <Icon className={size === 'sm' ? 'h-3 w-3' : 'h-3.5 w-3.5'} />
      {label ?? config.defaultLabel}
    </span>
  );
}
```

### 8.6 ConfirmDialog

```typescript
// src/components/shared/ConfirmDialog.tsx
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog';
import { AlertTriangle } from 'lucide-react';

type ConfirmVariant = 'danger' | 'warning';

interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: string;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: ConfirmVariant;
  /** 확인 중 로딩 상태 */
  isLoading?: boolean;
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel = '확인',
  cancelLabel = '취소',
  variant = 'danger',
  isLoading = false,
}: ConfirmDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(o) => !o && onClose()}>
      <AlertDialogContent className="bg-[var(--color-surface-card)] border border-[var(--color-border-default)]">
        <AlertDialogHeader>
          <div className="flex items-center gap-3 mb-2">
            <span className="flex h-10 w-10 items-center justify-center rounded-full
              bg-red-500/15 text-red-400"
            >
              <AlertTriangle className="h-5 w-5" />
            </span>
            <AlertDialogTitle className="text-[var(--color-text-primary)]">
              {title}
            </AlertDialogTitle>
          </div>
          <AlertDialogDescription className="text-[var(--color-text-secondary)]">
            {description}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            onClick={onClose}
            className="bg-transparent border-[var(--color-border-default)]
              text-[var(--color-text-secondary)] hover:bg-white/5"
          >
            {cancelLabel}
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={isLoading}
            className="bg-red-500/15 text-red-400 border border-red-500/30
              hover:bg-red-500/25 disabled:opacity-40"
          >
            {isLoading ? '처리 중...' : confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
```

### 8.7 ListDetailPanel

```typescript
// src/components/shared/ListDetailPanel.tsx
import { cn } from '@/lib/utils';

interface ListDetailPanelProps<TItem> {
  /** 좌측 리스트 아이템 배열 */
  items: TItem[];
  /** 선택된 아이템 ID */
  selectedId: string | null;
  /** 아이템 ID 추출 함수 */
  getItemId: (item: TItem) => string;
  /** 좌측 아이템 렌더 함수 */
  renderListItem: (item: TItem, isSelected: boolean) => React.ReactNode;
  /** 우측 상세 패널 렌더 함수 */
  renderDetail: (item: TItem) => React.ReactNode;
  /** 아이템 선택 핸들러 */
  onSelect: (item: TItem) => void;
  /** 리스트 너비 (기본값: 320px) */
  listWidth?: string;
  /** 리스트 상단 액션 영역 */
  listHeader?: React.ReactNode;
}

export function ListDetailPanel<TItem>({
  items,
  selectedId,
  getItemId,
  renderListItem,
  renderDetail,
  onSelect,
  listWidth = '320px',
  listHeader,
}: ListDetailPanelProps<TItem>) {
  const selectedItem = items.find((item) => getItemId(item) === selectedId) ?? null;

  return (
    <div className="flex h-full gap-0 overflow-hidden rounded-xl
      border border-[var(--color-border-default)]"
    >
      {/* 좌측 리스트 */}
      <div
        className="flex flex-col border-r border-[var(--color-border-default)] shrink-0"
        style={{ width: listWidth }}
      >
        {listHeader && (
          <div className="px-4 py-3 border-b border-[var(--color-border-default)]">
            {listHeader}
          </div>
        )}
        <div className="flex-1 overflow-y-auto">
          {items.map((item) => {
            const id = getItemId(item);
            const isSelected = id === selectedId;
            return (
              <button
                key={id}
                onClick={() => onSelect(item)}
                className={cn(
                  'w-full text-left px-4 py-3 border-b border-[var(--color-border-default)]',
                  'transition-colors duration-150',
                  isSelected
                    ? 'bg-indigo-500/10 border-l-2 border-l-indigo-500'
                    : 'hover:bg-white/5'
                )}
              >
                {renderListItem(item, isSelected)}
              </button>
            );
          })}
        </div>
      </div>

      {/* 우측 상세 */}
      <div className="flex-1 overflow-y-auto p-6">
        {selectedItem ? (
          renderDetail(selectedItem)
        ) : (
          <div className="flex h-full items-center justify-center
            text-[var(--color-text-muted)] text-sm"
          >
            항목을 선택하세요
          </div>
        )}
      </div>
    </div>
  );
}
```

---

## 9. 테스트 전략

### 9.1 Vitest 설정

```typescript
// vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      thresholds: {
        lines: 70,
        functions: 70,
        branches: 70,
      },
      exclude: [
        'src/components/ui/**',  // shadcn/ui 자동 생성 컴포넌트 제외
        'src/test/**',
        '**/*.d.ts',
      ],
    },
  },
});
```

```typescript
// src/test/setup.ts
import '@testing-library/jest-dom';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeAll, afterAll } from 'vitest';
import { server } from './mocks/server';

// MSW 서버 생명주기
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
});
afterAll(() => server.close());
```

### 9.2 MSW 핸들러 구성

```typescript
// src/test/mocks/handlers/stack.handlers.ts
import { http, HttpResponse } from 'msw';

const BASE_URL = 'http://localhost:8090/api/v1';

export const stackHandlers = [
  http.get(`${BASE_URL}/stacks/templates`, () => {
    return HttpResponse.json([
      {
        id: 'template-1',
        name: 'GitHub + Argo CD + Prometheus',
        tools: ['GitHub', 'GitHub Actions', 'Argo CD', 'Prometheus', 'Grafana'],
        estimatedInstallTime: '45분',
        requiredCpu: 8,
        requiredMemory: 16,
      },
      {
        id: 'template-2',
        name: 'GitLab + Argo CD + Prometheus',
        tools: ['GitLab', 'GitLab CI', 'Argo CD', 'Prometheus', 'Grafana'],
        estimatedInstallTime: '60분',
        requiredCpu: 12,
        requiredMemory: 24,
      },
    ]);
  }),

  http.get(`${BASE_URL}/stacks`, ({ request }) => {
    const url = new URL(request.url);
    const search = url.searchParams.get('search') ?? '';
    return HttpResponse.json({
      items: [
        {
          id: 'stack-1',
          name: search || 'Production Stack',
          status: 'connected',
          cluster: 'prod-cluster',
          version: '1.0.0',
          deployedAt: '2026-03-14T09:00:00Z',
        },
      ],
      total: 1,
    });
  }),

  http.post(`${BASE_URL}/stacks/deploy`, async ({ request }) => {
    const body = await request.json();
    return HttpResponse.json({
      deployId: 'deploy-abc123',
      stackName: (body as Record<string, string>).stackName,
      status: 'running',
    }, { status: 202 });
  }),
];
```

```typescript
// src/test/mocks/server.ts
import { setupServer } from 'msw/node';
import { stackHandlers } from './handlers/stack.handlers';
import { authHandlers } from './handlers/auth.handlers';
import { adminHandlers } from './handlers/admin.handlers';

export const server = setupServer(
  ...authHandlers,
  ...stackHandlers,
  ...adminHandlers,
);
```

### 9.3 컴포넌트 테스트 패턴

```typescript
// src/components/shared/__tests__/StatusBadge.test.tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { StatusBadge } from '../StatusBadge';

describe('StatusBadge', () => {
  it('connected 상태를 올바르게 렌더링한다', () => {
    render(<StatusBadge status="connected" />);
    expect(screen.getByText('Connected')).toBeInTheDocument();
  });

  it('커스텀 레이블을 표시한다', () => {
    render(<StatusBadge status="pending" label="배포 중" />);
    expect(screen.getByText('배포 중')).toBeInTheDocument();
  });

  it('error 상태는 빨간색 텍스트를 사용한다', () => {
    render(<StatusBadge status="error" />);
    const badge = screen.getByText('Error').closest('span');
    expect(badge).toHaveClass('text-red-400');
  });
});
```

```typescript
// src/features/stack/pages/__tests__/StackTemplatePage.test.tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
import { StackTemplatePage } from '../StackTemplatePage';
import { createTestWrapper } from '@/test/utils/testWrapper';

describe('StackTemplatePage', () => {
  it('템플릿 목록을 불러와 카드로 표시한다', async () => {
    render(<StackTemplatePage />, { wrapper: createTestWrapper() });

    // 로딩 스켈레톤 표시 후 데이터 표시
    await waitFor(() => {
      expect(screen.getByText('GitHub + Argo CD + Prometheus')).toBeInTheDocument();
    });
  });

  it('"이 템플릿 사용" 버튼 클릭 시 스택 설치 페이지로 이동한다', async () => {
    const user = userEvent.setup();
    const { mockNavigate } = createTestWrapper();
    render(<StackTemplatePage />, { wrapper: createTestWrapper() });

    await waitFor(() => screen.getByText('이 템플릿 사용'));
    await user.click(screen.getAllByText('이 템플릿 사용')[0]);

    expect(mockNavigate).toHaveBeenCalledWith('/stack/install');
  });
});
```

### 9.4 커스텀 훅 테스트

```typescript
// src/stores/__tests__/auth.store.test.ts
import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useAuthStore } from '../auth';

describe('useAuthStore', () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null, token: null, activeRole: null });
  });

  it('setUser 호출 후 인증 상태가 true가 된다', () => {
    const { result } = renderHook(() => useAuthStore());

    act(() => {
      result.current.setUser(
        { id: '1', name: 'Test', email: 'test@test.com', role: 'devops', organizationId: 'org-1' },
        'token-abc'
      );
    });

    expect(result.current.isAuthenticated()).toBe(true);
    expect(result.current.activeRole).toBe('devops');
  });

  it('logout 후 상태가 초기화된다', () => {
    const { result } = renderHook(() => useAuthStore());

    act(() => {
      result.current.setUser(
        { id: '1', name: 'Test', email: 'test@test.com', role: 'admin', organizationId: 'org-1' },
        'token-abc'
      );
      result.current.logout();
    });

    expect(result.current.isAuthenticated()).toBe(false);
    expect(result.current.user).toBeNull();
  });
});
```

### 9.5 Playwright E2E 테스트

```typescript
// e2e/stack-install.spec.ts
import { test, expect } from '@playwright/test';

test.describe('스택 설치 워크플로우', () => {
  test.beforeEach(async ({ page }) => {
    // DevOps 역할로 로그인
    await page.goto('/login');
    await page.fill('[name=email]', 'devops@test.com');
    await page.fill('[name=password]', 'password');
    await page.click('[type=submit]');
    await page.waitForURL('/stack/install');
  });

  test('5단계 워크플로우 탭이 모두 표시된다', async ({ page }) => {
    await expect(page.getByRole('tab', { name: '아티팩트' })).toBeVisible();
    await expect(page.getByRole('tab', { name: '파이프라인' })).toBeVisible();
    await expect(page.getByRole('tab', { name: '모니터링' })).toBeVisible();
    await expect(page.getByRole('tab', { name: '로깅' })).toBeVisible();
    await expect(page.getByRole('tab', { name: '리소스' })).toBeVisible();
  });

  test('배포 버튼은 클러스터 미선택 시 비활성화된다', async ({ page }) => {
    const deployButton = page.getByRole('button', { name: '배포' });
    await expect(deployButton).toBeDisabled();
  });
});
```

```typescript
// playwright.config.ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { open: 'never' }]],
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
  },
});
```

---

## 10. 빌드/배포 설정

### 10.1 Vite 설정

```typescript
// vite.config.ts
import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');

  return {
    plugins: [
      react({
        // React 19 컴파일러 활성화 (babel 플러그인)
        babel: {
          plugins: [['babel-plugin-react-compiler', {}]],
        },
      }),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    build: {
      target: 'es2022',
      outDir: 'dist',
      sourcemap: mode === 'production' ? false : true,
      // 청크 분할 전략
      rollupOptions: {
        output: {
          manualChunks: {
            // 벤더 라이브러리 분리
            'vendor-react': ['react', 'react-dom', 'react-router-dom'],
            'vendor-query': ['@tanstack/react-query', '@tanstack/react-table'],
            'vendor-ui': ['@radix-ui/react-dialog', '@radix-ui/react-select', 'lucide-react'],
            'vendor-charts': ['recharts'],
            'vendor-editor': ['@uiw/react-codemirror', '@codemirror/lang-yaml'],
            'vendor-i18n': ['i18next', 'react-i18next'],
          },
        },
      },
      // 청크 크기 경고 기준: 500KB
      chunkSizeWarningLimit: 500,
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: env.VITE_API_BASE_URL ?? 'http://localhost:8090',
          changeOrigin: true,
        },
        '/ws': {
          target: env.VITE_WS_BASE_URL ?? 'ws://localhost:8090',
          ws: true,
        },
      },
    },
    preview: {
      port: 4173,
    },
  };
});
```

### 10.2 환경 변수

```bash
# .env.development
VITE_API_BASE_URL=http://localhost:8090/api/v1
VITE_WS_BASE_URL=ws://localhost:8090/ws
VITE_APP_ENV=development

# .env.production
VITE_API_BASE_URL=/api/v1
VITE_WS_BASE_URL=/ws
VITE_APP_ENV=production

# .env.example (커밋 대상, 실제 값 없음)
VITE_API_BASE_URL=
VITE_WS_BASE_URL=
VITE_APP_ENV=
```

```typescript
// src/types/env.d.ts
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string;
  readonly VITE_WS_BASE_URL: string;
  readonly VITE_APP_ENV: 'development' | 'production' | 'test';
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
```

### 10.3 Docker 빌드

```dockerfile
# Dockerfile
# --- Stage 1: 빌드 ---
FROM node:22-alpine AS builder
WORKDIR /app

# 의존성 먼저 복사 (캐시 활용)
COPY package*.json ./
RUN npm ci --frozen-lockfile

# 소스 복사 후 빌드
COPY . .
RUN npm run build

# --- Stage 2: Nginx 서빙 ---
FROM nginx:1.27-alpine AS production

# SPA 라우팅을 위한 Nginx 설정
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 빌드 결과물 복사
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -qO- http://localhost/health || exit 1

CMD ["nginx", "-g", "daemon off;"]
```

```nginx
# nginx.conf
server {
  listen 80;
  server_name _;
  root /usr/share/nginx/html;
  index index.html;

  # gzip 압축
  gzip on;
  gzip_vary on;
  gzip_min_length 1024;
  gzip_types text/plain text/css application/json application/javascript
             text/xml application/xml application/xml+rss text/javascript;

  # 정적 에셋 캐싱 (해시 파일명)
  location ~* \.(js|css|woff2|ico|png|svg)$ {
    expires 1y;
    add_header Cache-Control "public, immutable";
  }

  # API 프록시 (선택적 - Kubernetes Ingress로 대체 가능)
  location /api/ {
    proxy_pass http://nullus-backend:8080/api/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
  }

  # WebSocket 프록시
  location /ws {
    proxy_pass http://nullus-backend:8080/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
  }

  # 헬스체크
  location /health {
    access_log off;
    return 200 "ok";
    add_header Content-Type text/plain;
  }

  # SPA 라우팅: 모든 경로를 index.html로
  location / {
    try_files $uri $uri/ /index.html;
  }
}
```

### 10.4 package.json 스크립트

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest",
    "test:ui": "vitest --ui",
    "test:coverage": "vitest --coverage",
    "test:e2e": "playwright test",
    "test:e2e:ui": "playwright test --ui",
    "lint": "eslint src --ext .ts,.tsx --max-warnings 0",
    "lint:fix": "eslint src --ext .ts,.tsx --fix",
    "type-check": "tsc --noEmit",
    "format": "prettier --write src",
    "docker:build": "docker build -t nullus-web:latest .",
    "docker:run": "docker run -p 8080:80 nullus-web:latest"
  }
}
```

### 10.5 CI/CD 파이프라인 (GitHub Actions)

```yaml
# .github/workflows/frontend.yml
name: Frontend CI

on:
  push:
    branches: [main, develop]
    paths: ['web/**']
  pull_request:
    branches: [main]
    paths: ['web/**']

defaults:
  run:
    working-directory: web

jobs:
  lint-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci

      - name: Type check
        run: npm run type-check

      - name: Lint
        run: npm run lint

      - name: Unit tests with coverage
        run: npm run test:coverage

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          directory: web/coverage

  build:
    runs-on: ubuntu-latest
    needs: lint-and-test
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci

      - name: Build
        run: npm run build

      - name: Upload build artifacts
        uses: actions/upload-artifact@v4
        with:
          name: frontend-dist
          path: web/dist
          retention-days: 7

  e2e:
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci

      - name: Install Playwright browsers
        run: npx playwright install --with-deps chromium

      - name: Run E2E tests
        run: npm run test:e2e
        env:
          CI: true

      - name: Upload Playwright report
        uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: playwright-report
          path: web/playwright-report
          retention-days: 7
```

---

## 부록: lib/utils.ts

```typescript
// src/lib/utils.ts
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * Tailwind 클래스 병합 유틸
 * clsx로 조건부 클래스 처리 후 tailwind-merge로 중복 제거
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

/**
 * 바이트를 사람이 읽기 쉬운 형식으로 변환
 */
export function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'Ti'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / k ** i).toFixed(decimals))} ${sizes[i]}`;
}

/**
 * 날짜를 상대적 시간으로 변환 (예: "3분 전")
 */
export function formatRelativeTime(date: string | Date): string {
  const target = typeof date === 'string' ? new Date(date) : date;
  const diff = Date.now() - target.getTime();
  const minutes = Math.floor(diff / 60_000);
  const hours = Math.floor(diff / 3_600_000);
  const days = Math.floor(diff / 86_400_000);

  if (minutes < 1) return '방금 전';
  if (minutes < 60) return `${minutes}분 전`;
  if (hours < 24) return `${hours}시간 전`;
  return `${days}일 전`;
}
```
