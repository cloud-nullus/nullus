import { type ReactNode, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Box,
  Boxes,
  BookOpen,
  Download,
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
} from 'lucide-react'
import { useAuthStore } from '../../stores/auth-store'
import { useSidebarStore } from '../../stores/sidebar-store'
import type { Role } from '../../types'
import { cn } from '../../lib/utils'

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
    icon: <Boxes size={18} />,
    roles: ['admin', 'devops'],
    items: [
      { key: 'stackTemplate', label: 'sidebar.stackTemplate', path: '/stack/templates', icon: <BookOpen size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackInstall', label: 'sidebar.stackInstall', path: '/stack/install', icon: <Download size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackList', label: 'sidebar.stackList', path: '/stack/list', icon: <List size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackHistory', label: 'sidebar.stackHistory', path: '/stack/history', icon: <History size={16} />, roles: ['admin', 'devops'] },
      { key: 'stackVersion', label: 'sidebar.stackVersion', path: '/stack/version', icon: <Shield size={16} />, roles: ['admin', 'devops'] },
    ],
  },
  {
    key: 'cicd',
    label: 'sidebar.cicd',
    icon: <GitBranch size={18} />,
    roles: ['admin', 'devops', 'developer'],
    items: [
      { key: 'cicdTemplate', label: 'sidebar.cicdTemplate', path: '/cicd/templates', icon: <BookOpen size={16} />, roles: ['admin', 'devops'] },
      { key: 'cicdList', label: 'sidebar.cicdList', path: '/cicd/list', icon: <List size={16} />, roles: ['admin', 'devops', 'developer'] },
      { key: 'cicdHistory', label: 'sidebar.cicdHistory', path: '/cicd/history', icon: <History size={16} />, roles: ['admin', 'devops', 'developer'] },
    ],
  },
  {
    key: 'observability',
    label: 'sidebar.observability',
    icon: <BarChart3 size={18} />,
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
    icon: <Settings size={18} />,
    roles: ['admin'],
    items: [
      { key: 'organization', label: 'sidebar.organization', path: '/admin/organization', icon: <Settings size={16} />, roles: ['admin'] },
      { key: 'userManagement', label: 'sidebar.userManagement', path: '/admin/users', icon: <Users size={16} />, roles: ['admin'] },
      { key: 'clusterManagement', label: 'sidebar.clusterManagement', path: '/admin/clusters', icon: <Network size={16} />, roles: ['admin'] },
      { key: 'knownIssues', label: 'sidebar.knownIssues', path: '/admin/known-issues', icon: <AlertTriangle size={16} />, roles: ['admin'] },
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
        'relative z-[var(--z-sidebar)] flex min-h-screen shrink-0 flex-col overflow-hidden border-r border-[var(--color-border-default)] bg-[var(--color-surface-card)] transition-all duration-200 ease-in-out',
        collapsed ? 'w-[var(--sidebar-collapsed)]' : 'w-[var(--sidebar-width)]'
      )}
    >
      {/* Logo + toggle */}
      <div
        className={cn(
          'flex h-[var(--header-height)] shrink-0 items-center border-b border-[var(--color-border-default)]',
          collapsed ? 'justify-center px-0' : 'justify-between px-4'
        )}
      >
        {!collapsed && (
          <div className="flex items-center gap-2">
            <Box size={20} className="text-[#ffd700]" />
            <span className="text-base font-bold text-[var(--color-text-primary)]">
              Nullus
            </span>
          </div>
        )}
        <button
          type="button"
          onClick={toggleSidebar}
          aria-label="Toggle sidebar"
          className="flex cursor-pointer items-center rounded-md border-none bg-none p-1.5 text-[var(--color-text-secondary)]"
        >
          <Menu size={18} />
        </button>
      </div>

      {/* Nav groups */}
      <nav className="flex-1 overflow-y-auto py-2">
        {visibleGroups.map((group) => (
          <div key={group.key}>
            <button
              type="button"
              onClick={() => toggleGroup(group.key)}
              className={cn(
                'flex w-full cursor-pointer items-center border-none bg-none text-[11px] font-semibold tracking-[0.08em] text-[#818cf8] uppercase',
                collapsed ? 'justify-center px-0 py-2.5' : 'justify-between px-4 py-2.5'
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
                          'flex items-center gap-2.5 border-r-2 text-sm no-underline transition-all duration-150 ease-in-out',
                          collapsed ? 'justify-center px-0 py-2.5' : 'justify-start px-4 py-2 pl-8',
                          isActive
                            ? 'border-r-[#6366f1] bg-[rgba(99,102,241,0.1)] text-[#a5b4fc]'
                            : 'border-r-transparent bg-transparent text-[var(--color-text-secondary)]'
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

      {/* Logout */}
      <div className="border-t border-[var(--color-border-default)] py-2">
        <button
          type="button"
          onClick={() => {
            logout()
            navigate('/login')
          }}
          className={cn(
            'flex w-full cursor-pointer items-center gap-2.5 border-none bg-none text-sm text-[var(--color-text-secondary)] transition-all duration-150 ease-in-out',
            collapsed ? 'justify-center px-0 py-2.5' : 'justify-start px-4 py-2.5'
          )}
          aria-label={t('sidebar.logout')}
        >
          <LogOut size={18} />
          {!collapsed && t('sidebar.logout')}
        </button>
      </div>
    </aside>
  )
}
