// 사이드바 메뉴 구조. 사이드바만의 것이 아니다.
//
// PageHeader 도 이 구조를 읽어 브레드크럼 맨 앞에 상위 메뉴를 붙인다 — 화면마다
// 손으로 적으면 "Stack List" 처럼 상위가 빠지거나 "Admin" 만 영어로 남는 식으로
// 갈린다(실제로 25화면 중 절반이 상위 없이 잎만 적고 있었다). 메뉴 트리는 한 곳에서만
// 정의하고, 두 화면 요소가 같은 트리를 읽는다.

import { type ReactNode } from 'react'
import { BookOpen, Boxes, Bug, Building2, ChartColumn, Database, DatabaseBackup, FileCode2, GitBranch, History, KeyRound, LayoutDashboard, List, Network, Route, Send, Settings, Shield, ShieldCheck, Tag, Users, Workflow } from 'lucide-react'
import { iconProps } from '../ui/icon'
import type { Role } from '../../types'

export interface NavItem {
  key: string
  label: string
  /**
   * 화면 경로. 자식이 있는 항목(하위 그룹)은 화면이 아니라 펼침 단추다 — 그때 path 는
   * 첫 자식의 경로이고, 브레드크럼·접힌 레일이 "이 묶음의 대표 화면" 으로 쓴다.
   */
  path: string
  icon: ReactNode
  roles: Role[]
  /**
   * 하위 그룹(3단 메뉴). 메뉴는 그룹 › 하위 그룹 › 항목까지만 간다.
   *
   * 보안 화면이 관측성 아래 한 묶음으로 들어오면서 생겼다 — 대시보드 하나로 시작하지만
   * 스캔 정책 화면 같은 후속이 같은 묶음에 붙는다(Security Dashboard 설계 §7).
   */
  children?: NavItem[]
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
      { key: 'alertHistory', label: 'sidebar.alertHistory', path: '/observability/alert-history', icon: <History {...iconProps('sm')} />, roles: ['admin', 'devops', 'developer'] },
      {
        key: 'security',
        label: 'sidebar.security',
        path: '/observability/security/dashboard',
        icon: <Shield {...iconProps('sm')} />,
        roles: ['admin', 'devops', 'developer'],
        // 개발자도 본다 — 다만 화면 안에서 설치 OSS 탭은 admin · devops 만이다(설계 §4.4).
        children: [
          { key: 'securityDashboard', label: 'sidebar.securityDashboard', path: '/observability/security/dashboard', icon: <ShieldCheck {...iconProps('sm')} />, roles: ['admin', 'devops', 'developer'] },
        ],
      },
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
      { key: 'notifications', label: 'sidebar.notifications', path: '/admin/notifications', icon: <Send {...iconProps('sm')} />, roles: ['admin'] },
    ],
  },
]

/**
 * 경로가 속한 상위 메뉴들의 i18n 키 — 그룹부터 하위 그룹까지 바깥에서 안쪽 순서다.
 * 못 찾으면 빈 배열.
 *
 * 메뉴에 없는 하위 경로(/stack/install, /stack/history/:id 같은 것)가 많아서
 * 두 단계로 찾는다: 먼저 메뉴 항목 경로의 접두사로, 안 되면 첫 세그먼트로.
 * /admin/stack-versions 처럼 URL 세그먼트와 소속 그룹이 다른 항목이 있어
 * 항목 매칭이 먼저다.
 *
 * 하위 그룹은 화면이 아니라 묶음이라 잎(자식 없는 항목)만 맞춰 본다. 잎이 하위
 * 그룹 안에 있으면 그 하위 그룹의 라벨도 상위로 붙는다 — 관측성 › 보안 › 보안 대시보드.
 */
export function resolveNavTrail(pathname: string): string[] {
  let best: { length: number; trail: string[] } | null = null
  const visit = (items: NavItem[], ancestors: string[]) => {
    for (const item of items) {
      if (item.children && item.children.length > 0) {
        visit(item.children, [...ancestors, item.label])
        continue
      }
      if (pathname === item.path || pathname.startsWith(`${item.path}/`)) {
        if (!best || item.path.length > best.length) {
          best = { length: item.path.length, trail: ancestors }
        }
      }
    }
  }
  for (const group of navGroups) {
    visit(group.items, [group.label])
  }
  const found = best as { length: number; trail: string[] } | null
  if (found) {
    return found.trail
  }

  const segment = pathname.split('/').filter(Boolean)[0]
  const fallback = segment ? SEGMENT_GROUP[segment] : undefined
  return fallback ? [fallback] : []
}

/** 경로가 속한 그룹(맨 바깥)의 i18n 키. 못 찾으면 null. */
export function resolveNavGroupLabel(pathname: string): string | null {
  return resolveNavTrail(pathname)[0] ?? null
}

const SEGMENT_GROUP: Record<string, string> = {
  stack: 'sidebar.devsecopsStack',
  cicd: 'sidebar.cicd',
  observability: 'sidebar.observability',
  admin: 'sidebar.admin',
}
