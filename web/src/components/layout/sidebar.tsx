import { type ReactNode, useState } from 'react'
import { NavLink } from 'react-router-dom'
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
} from 'lucide-react'
import { useAuthStore } from '../../stores/auth-store'
import { useSidebarStore } from '../../stores/sidebar-store'
import type { Role } from '../../types'

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
    ],
  },
]

export function Sidebar() {
  const { t } = useTranslation()
  const { role } = useAuthStore()
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
      style={{
        width: collapsed ? 'var(--sidebar-collapsed)' : 'var(--sidebar-width)',
        minHeight: '100vh',
        background: 'var(--color-surface-card)',
        borderRight: '1px solid var(--color-border-default)',
        display: 'flex',
        flexDirection: 'column',
        transition: 'width var(--transition-default)',
        overflow: 'hidden',
        flexShrink: 0,
        zIndex: 'var(--z-sidebar)',
        position: 'relative',
      }}
    >
      {/* Logo + toggle */}
      <div
        style={{
          height: 'var(--header-height)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: collapsed ? 'center' : 'space-between',
          padding: collapsed ? '0' : '0 16px',
          borderBottom: '1px solid var(--color-border-default)',
          flexShrink: 0,
        }}
      >
        {!collapsed && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Box size={20} style={{ color: '#ffd700' }} />
            <span style={{ fontWeight: 700, fontSize: '16px', color: 'var(--color-text-primary)' }}>
              Nullus
            </span>
          </div>
        )}
        <button
          onClick={toggleSidebar}
          aria-label="Toggle sidebar"
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            color: 'var(--color-text-secondary)',
            display: 'flex',
            alignItems: 'center',
            padding: '6px',
            borderRadius: '6px',
          }}
        >
          <Menu size={18} />
        </button>
      </div>

      {/* Nav groups */}
      <nav style={{ flex: 1, overflowY: 'auto', padding: '8px 0' }}>
        {visibleGroups.map((group) => (
          <div key={group.key}>
            <button
              onClick={() => toggleGroup(group.key)}
              style={{
                width: '100%',
                background: 'none',
                border: 'none',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                justifyContent: collapsed ? 'center' : 'space-between',
                padding: collapsed ? '10px 0' : '10px 16px',
                color: '#818cf8',
                fontSize: '11px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}
              aria-label={t(group.label)}
            >
              <span style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
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
                      style={({ isActive }) => ({
                        display: 'flex',
                        alignItems: 'center',
                        gap: '10px',
                        padding: collapsed ? '10px 0' : '8px 16px 8px 32px',
                        justifyContent: collapsed ? 'center' : 'flex-start',
                        color: isActive ? '#a5b4fc' : 'var(--color-text-secondary)',
                        background: isActive ? 'rgba(99,102,241,0.1)' : 'transparent',
                        textDecoration: 'none',
                        fontSize: '14px',
                        borderRight: isActive ? '2px solid #6366f1' : '2px solid transparent',
                        transition: 'all var(--transition-fast)',
                      })}
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
      <div style={{ borderTop: '1px solid var(--color-border-default)', padding: '8px 0' }}>
        <button
          style={{
            width: '100%',
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            padding: collapsed ? '10px 0' : '10px 16px',
            justifyContent: collapsed ? 'center' : 'flex-start',
            color: 'var(--color-text-secondary)',
            fontSize: '14px',
            transition: 'color var(--transition-fast)',
          }}
          aria-label={t('sidebar.logout')}
        >
          <LogOut size={18} />
          {!collapsed && t('sidebar.logout')}
        </button>
      </div>
    </aside>
  )
}
