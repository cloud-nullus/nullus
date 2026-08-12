import { type ReactNode, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Box,
  Boxes,
  BookOpen,
  List,
  History,
  Shield,
  GitBranch,
  BarChart3,
  Bell,
  BellOff,
  Settings,
  Users,
  Network,
  LogOut,
  Menu,
  ChevronDown,
  ChevronRight,
  AlertTriangle,
  Database,
  KeyRound,
  Route,
} from 'lucide-react'
import { useAuthStore } from '../../stores/auth-store'
import { useSidebarStore } from '../../stores/sidebar-store'
import { isOidcMode, getProviderConfig } from '../../lib/oidc-providers'
import type { Role } from '../../types'
import { cn } from '../../lib/utils'
import { IconButton } from '../ui/icon-button'

interface NavItem {
  key: string
  label: string
  path: string
  icon: ReactNode
  roles: Role[]
}

interface NavGroup {
  key: string
  label: string
  icon: ReactNode
  items: NavItem[]
  roles: Role[]
}

const navGroups: NavGroup[] = [
  {
    key: 'devsecops',
    label: 'sidebar.devsecopsStack',
    icon: <Boxes size={16} />,
    roles: ['admin', 'devops'],
    items: [
      { key: 'stackTemplate', label: 'sidebar.stackTemplate', path: '/stack/templates', icon: <BookOpen size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackList', label: 'sidebar.stackList', path: '/stack/list', icon: <List size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackHistory', label: 'sidebar.stackHistory', path: '/stack/history', icon: <History size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackVersion', label: 'sidebar.stackVersion', path: '/stack/version', icon: <Shield size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackVersionsAdmin', label: 'sidebar.stackVersionsAdmin', path: '/admin/stack-versions', icon: <Shield size={16} />, roles: ['admin'] },
      { key: 'stackOssResourceDefault', label: 'sidebar.stackOssResourceDefault', path: '/stack/oss-resource-default', icon: <Database size={16} />, roles: ['admin', 'devops'] },
    ],
  },
  {
    key: 'cicd',
    label: 'sidebar.cicd',
    icon: <GitBranch size={16} />,
    roles: ['admin', 'devops', 'developer'],
    items: [
      { key: 'cicdTemplate', label: 'sidebar.cicdTemplate', path: '/cicd/templates', icon: <BookOpen size={16} />, roles: ['admin', 'devops'] },
      { key: 'cicdGoldenPath', label: 'sidebar.cicdGoldenPath', path: '/cicd/golden-paths', icon: <Route size={16} />, roles: ['admin', 'devops'] },
      { key: 'cicdList', label: 'sidebar.cicdList', path: '/cicd/list', icon: <List size={16} />, roles: ['admin', 'devops', 'developer'] },
      { key: 'cicdHistory', label: 'sidebar.cicdHistory', path: '/cicd/history', icon: <History size={16} />, roles: ['admin', 'devops', 'developer'] },
    ],
  },
  {
    key: 'observability',
    label: 'sidebar.observability',
    icon: <BarChart3 size={16} />,
    roles: ['admin', 'devops', 'developer'],
    items: [
      { key: 'monitoringDashboard', label: 'sidebar.monitoringDashboard', path: '/observability/monitoring', icon: <BarChart3 size={16} />, roles: ['admin', 'devops', 'developer'] },
      { key: 'alertRules', label: 'sidebar.alertRules', path: '/observability/alerts', icon: <Bell size={16} />, roles: ['admin', 'devops'] },
      { key: 'alertHistory', label: 'sidebar.alertHistory', path: '/observability/alert-history', icon: <BellOff size={16} />, roles: ['admin', 'devops', 'developer'] },
    ],
  },
  {
    key: 'admin',
    label: 'sidebar.admin',
    icon: <Settings size={16} />,
    roles: ['admin'],
    items: [
      { key: 'organization', label: 'sidebar.organization', path: '/admin/organization', icon: <Settings size={16} />, roles: ['admin'] },
      { key: 'userManagement', label: 'sidebar.userManagement', path: '/admin/users', icon: <Users size={16} />, roles: ['admin'] },
      { key: 'clusterManagement', label: 'sidebar.clusterManagement', path: '/admin/clusters', icon: <Network size={16} />, roles: ['admin'] },
      { key: 'knownIssues', label: 'sidebar.knownIssues', path: '/admin/known-issues', icon: <AlertTriangle size={16} />, roles: ['admin'] },
      { key: 'tokenManagement', label: 'sidebar.tokenManagement', path: '/admin/token-management', icon: <KeyRound size={16} />, roles: ['admin'] },
    ],
  },
]

export function Sidebar() {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const role = useAuthStore((state) => state.role)
  const logout = useAuthStore((state) => state.logout)
  const { collapsed, toggleSidebar } = useSidebarStore()
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({
    devsecops: true,
    cicd: true,
    observability: true,
    admin: true,
  })

  const toggleGroup = (key: string) => {
    setOpenGroups((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  const visibleGroups = navGroups.filter((g) => g.roles.includes(role))

  return (
    <aside
      className={cn(
        // h-full: 껍데기(AppLayout)가 뷰포트 높이를 못박으므로 거기에 맞춘다.
        // min-h-screen 이던 시절엔 문서가 길어지면 사이드바도 같이 늘어나
        // 화면 밖으로 밀렸다.
        'relative z-[var(--z-sidebar)] flex h-full shrink-0 flex-col overflow-hidden border-r border-[var(--color-border-default)] bg-[var(--color-surface-card)] transition-all duration-200 ease-in-out',
        'border-r-[var(--color-sidebar-border)]',
        collapsed ? 'w-[var(--sidebar-collapsed)]' : 'w-[var(--sidebar-width)]'
      )}
    >
      {/* Logo + toggle */}
      <div
        className={cn(
          'flex h-[var(--header-height)] shrink-0 items-center border-b border-[var(--color-sidebar-border)]',
          collapsed ? 'justify-center px-0' : 'justify-between px-3'
        )}
      >
        {!collapsed && (
          <button
            type="button"
            onClick={() => navigate('/')}
            className="flex cursor-pointer items-center gap-2 border-none bg-transparent p-0"
            aria-label="Go to home"
          >
            <Box size={18} className="text-[var(--color-brand-gold)]" />
            <span className="text-base font-bold text-[var(--color-text-primary)]">
              Nullus
            </span>
          </button>
        )}
        <IconButton onClick={toggleSidebar} aria-label="Toggle sidebar">
          <Menu size={16} />
        </IconButton>
      </div>

      {/* Nav groups — 로고 줄과 로그아웃 줄은 고정, 남는 높이만 이 목록이 갖는다.
          min-h-0 이 없으면 flex 자식이 내용보다 작아지지 않아 overflow 가 안 걸린다. */}
      <nav className="min-h-0 flex-1 overflow-y-auto py-1">
        {visibleGroups.map((group) => (
          <div key={group.key}>
            <button
              type="button"
              onClick={() => toggleGroup(group.key)}
              className={cn(
                'flex w-full cursor-pointer items-center border-none bg-none text-[11px] font-semibold tracking-[0.08em] text-[var(--color-sidebar-group-text)] uppercase',
                collapsed ? 'justify-center px-0 py-2' : 'justify-between px-3 py-1.5'
              )}
              aria-label={t(group.label)}
            >
              <span className="flex items-center gap-2">
                {group.icon}
                {!collapsed && t(group.label)}
              </span>
              {!collapsed && (
                openGroups[group.key]
                  ? <ChevronDown size={14} />
                  : <ChevronRight size={14} />
              )}
            </button>

            {(openGroups[group.key] || collapsed) && (
              <div>
                {group.items
                  .filter((item) => item.roles.includes(role))
                  .map((item) => (
                    <NavLink
                      key={item.key}
                      to={item.path}
                      className={({ isActive }) =>
                        cn(
                          'flex h-8 items-center gap-2 border-l-2 text-[13px] no-underline transition-colors duration-150 ease-in-out',
                          collapsed ? 'justify-center px-0' : 'justify-start pl-6 pr-3',
                          isActive
                            ? 'border-l-[var(--color-sidebar-item-active-border)] bg-[var(--color-sidebar-item-active-bg)] font-medium text-[var(--color-sidebar-item-active-text)]'
                            : 'border-l-transparent bg-transparent text-[var(--color-sidebar-item-text)] hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_5%,_transparent)]'
                        )
                      }
                    >
                      {item.icon}
                      {!collapsed && t(item.label)}
                    </NavLink>
                  ))}
              </div>
            )}
          </div>
        ))}
      </nav>

      {/* 로그아웃 줄은 항상 사이드바 맨 아래에 붙어 있다 — 목록이 길어도
          shrink-0 이라 눌리지 않는다. */}
      <div className="shrink-0 border-t border-[var(--color-sidebar-border)] py-1">
        <button
          type="button"
          onClick={async () => {
            if (isOidcMode) {
              const config = getProviderConfig()
              if (config.getLogoutUrl) {
                // react-oidc-context(oidc-client-ts) 가 저장한 user 를 비워야
                // 로그아웃 후 복귀 시 stale 세션으로 재로그인되지 않는다.
                let idToken = ''
                try {
                  for (const k of Object.keys(sessionStorage)) {
                    if (k.startsWith('oidc.')) {
                      try {
                        const u = JSON.parse(sessionStorage.getItem(k) || '{}')
                        if (u && u.id_token) idToken = u.id_token
                      } catch { /* ignore */ }
                      sessionStorage.removeItem(k)
                    }
                  }
                  for (const k of Object.keys(localStorage)) {
                    if (k.startsWith('oidc.')) localStorage.removeItem(k)
                  }
                } catch { /* ignore */ }
                const redirectUri = window.location.origin + '/'
                logout()
                window.location.href = config.getLogoutUrl(idToken, redirectUri)
                return
              }
            }
            logout()
            navigate('/login')
          }}
          className={cn(
            'flex w-full cursor-pointer items-center gap-2.5 border-none bg-none text-sm text-[var(--color-text-secondary)] transition-all duration-150 ease-in-out',
            collapsed ? 'justify-center px-0 py-2' : 'justify-start px-3 py-2'
          )}
          aria-label={t('sidebar.logout')}
        >
          <LogOut size={16} />
          {!collapsed && t('sidebar.logout')}
        </button>
      </div>
    </aside>
  )
}
