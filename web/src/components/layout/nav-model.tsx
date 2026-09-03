// 사이드바 메뉴 구조. 사이드바만의 것이 아니다.
//
// PageHeader 도 이 구조를 읽어 브레드크럼 맨 앞에 상위 메뉴를 붙인다 — 화면마다
// 손으로 적으면 "Stack List" 처럼 상위가 빠지거나 "Admin" 만 영어로 남는 식으로
// 갈린다(실제로 25화면 중 절반이 상위 없이 잎만 적고 있었다). 메뉴 트리는 한 곳에서만
// 정의하고, 두 화면 요소가 같은 트리를 읽는다.

import { type ReactNode } from 'react'
import { Bell, BookOpen, Boxes, Bug, Building2, ChartColumn, Database, DatabaseBackup, FileCode2, GitBranch, History, KeyRound, LayoutDashboard, List, Network, Route, Settings, ShieldCheck, Tag, Users, Workflow } from 'lucide-react'
import { iconProps } from '../ui/icon'
import type { Role } from '../../types'

export interface NavItem {
  key: string
  label: string
  path: string
  icon: ReactNode
  roles: Role[]
}

export interface NavGroup {
  key: string
  label: string
  icon: ReactNode
  items: NavItem[]
  roles: Role[]
}

export const navGroups: NavGroup[] = [
  {
    key: 'devsecops',
    label: 'sidebar.devsecopsStack',
    icon: <Boxes {...iconProps('sm')} />,
    roles: ['admin', 'devops'],
    items: [
      { key: 'stackTemplate', label: 'sidebar.stackTemplate', path: '/stack/templates', icon: <BookOpen {...iconProps('sm')} />, roles: ['admin', 'devops'] },
      { key: 'stackList', label: 'sidebar.stackList', path: '/stack/list', icon: <List {...iconProps('sm')} />, roles: ['admin', 'devops'] },
      { key: 'stackHistory', label: 'sidebar.stackHistory', path: '/stack/history', icon: <History {...iconProps('sm')} />, roles: ['admin', 'devops'] },
      { key: 'stackVersion', label: 'sidebar.stackVersion', path: '/stack/version', icon: <Tag {...iconProps('sm')} />, roles: ['admin', 'devops'] },
      { key: 'stackVersionsAdmin', label: 'sidebar.stackVersionsAdmin', path: '/admin/stack-versions', icon: <ShieldCheck {...iconProps('sm')} />, roles: ['admin'] },
      { key: 'stackOssResourceDefault', label: 'sidebar.stackOssResourceDefault', path: '/stack/oss-resource-default', icon: <Database {...iconProps('sm')} />, roles: ['admin', 'devops'] },
    ],
  },
  {
    key: 'cicd',
    label: 'sidebar.cicd',
    icon: <GitBranch {...iconProps('sm')} />,
    roles: ['admin', 'devops', 'developer'],
    items: [
      { key: 'cicdTemplate', label: 'sidebar.cicdTemplate', path: '/cicd/templates', icon: <FileCode2 {...iconProps('sm')} />, roles: ['admin', 'devops'] },
      { key: 'cicdGoldenPath', label: 'sidebar.cicdGoldenPath', path: '/cicd/golden-paths', icon: <Route {...iconProps('sm')} />, roles: ['admin', 'devops'] },
      { key: 'cicdList', label: 'sidebar.cicdList', path: '/cicd/list', icon: <Workflow {...iconProps('sm')} />, roles: ['admin', 'devops', 'developer'] },
      { key: 'cicdHistory', label: 'sidebar.cicdHistory', path: '/cicd/history', icon: <History {...iconProps('sm')} />, roles: ['admin', 'devops', 'developer'] },
    ],
  },
  {
    key: 'observability',
    label: 'sidebar.observability',
    icon: <ChartColumn {...iconProps('sm')} />,
    roles: ['admin', 'devops', 'developer'],
    items: [
      { key: 'monitoringDashboard', label: 'sidebar.monitoringDashboard', path: '/observability/monitoring', icon: <LayoutDashboard {...iconProps('sm')} />, roles: ['admin', 'devops', 'developer'] },
      { key: 'alertRules', label: 'sidebar.alertRules', path: '/observability/alerts', icon: <Bell {...iconProps('sm')} />, roles: ['admin', 'devops'] },
      { key: 'alertHistory', label: 'sidebar.alertHistory', path: '/observability/alert-history', icon: <History {...iconProps('sm')} />, roles: ['admin', 'devops', 'developer'] },
    ],
  },
  {
    key: 'admin',
    label: 'sidebar.admin',
    icon: <Settings {...iconProps('sm')} />,
    roles: ['admin'],
    items: [
      { key: 'organization', label: 'sidebar.organization', path: '/admin/organization', icon: <Building2 {...iconProps('sm')} />, roles: ['admin'] },
      { key: 'userManagement', label: 'sidebar.userManagement', path: '/admin/users', icon: <Users {...iconProps('sm')} />, roles: ['admin'] },
      { key: 'clusterManagement', label: 'sidebar.clusterManagement', path: '/admin/clusters', icon: <Network {...iconProps('sm')} />, roles: ['admin'] },
      { key: 'knownIssues', label: 'sidebar.knownIssues', path: '/admin/known-issues', icon: <Bug {...iconProps('sm')} />, roles: ['admin'] },
      { key: 'tokenManagement', label: 'sidebar.tokenManagement', path: '/admin/token-management', icon: <KeyRound {...iconProps('sm')} />, roles: ['admin'] },
      { key: 'backup', label: 'sidebar.backup', path: '/admin/backup', icon: <DatabaseBackup {...iconProps('sm')} />, roles: ['admin'] },
    ],
  },
]

/**
 * 경로가 속한 상위 메뉴의 i18n 키. 못 찾으면 null.
 *
 * 메뉴에 없는 하위 경로(/stack/install, /stack/history/:id 같은 것)가 많아서
 * 두 단계로 찾는다: 먼저 메뉴 항목 경로의 접두사로, 안 되면 첫 세그먼트로.
 * /admin/stack-versions 처럼 URL 세그먼트와 소속 그룹이 다른 항목이 있어
 * 항목 매칭이 먼저다.
 */
export function resolveNavGroupLabel(pathname: string): string | null {
  let best: { length: number; label: string } | null = null
  for (const group of navGroups) {
    for (const item of group.items) {
      if (pathname === item.path || pathname.startsWith(`${item.path}/`)) {
        if (!best || item.path.length > best.length) {
          best = { length: item.path.length, label: group.label }
        }
      }
    }
  }
  if (best) {
    return best.label
  }

  const segment = pathname.split('/').filter(Boolean)[0]
  return segment ? (SEGMENT_GROUP[segment] ?? null) : null
}

const SEGMENT_GROUP: Record<string, string> = {
  stack: 'sidebar.devsecopsStack',
  cicd: 'sidebar.cicd',
  observability: 'sidebar.observability',
  admin: 'sidebar.admin',
}
