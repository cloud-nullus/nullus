import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderWithProviders } from '../../__tests__/test-utils'
import { PageHeader } from './page-header'
import { resolveNavGroupLabel, resolveNavTrail } from './nav-model'

// 브레드크럼 맨 앞은 사이드바의 상위 메뉴다. 메뉴가 3단이 되면서 하위 그룹도 붙는다.
describe('resolveNavTrail', () => {
  it('should include the sub-group for a screen inside it', () => {
    expect(resolveNavTrail('/observability/security/dashboard')).toEqual([
      'sidebar.observability',
      'sidebar.security',
    ])
  })

  it('should return only the group for a two-level screen', () => {
    expect(resolveNavTrail('/observability/alert-history')).toEqual(['sidebar.observability'])
    // URL 세그먼트와 소속 그룹이 다른 항목은 항목 매칭이 먼저다.
    expect(resolveNavTrail('/admin/stack-versions')).toEqual(['sidebar.devsecopsStack'])
  })

  it('should fall back to the first segment for paths not in the menu', () => {
    expect(resolveNavTrail('/stack/install')).toEqual(['sidebar.devsecopsStack'])
    expect(resolveNavTrail('/nowhere')).toEqual([])
  })

  it('should keep resolveNavGroupLabel returning the outermost group', () => {
    expect(resolveNavGroupLabel('/observability/security/dashboard')).toBe('sidebar.observability')
    expect(resolveNavGroupLabel('/nowhere')).toBeNull()
  })
})

describe('PageHeader breadcrumb', () => {
  it('should render Observability › Security › Security Dashboard', () => {
    renderWithProviders(
      <PageHeader title="Security Dashboard" breadcrumb={[{ label: 'Security Dashboard' }]} />,
      { route: '/observability/security/dashboard' },
    )
    const crumbs = within(screen.getByRole('navigation', { name: 'Breadcrumb' }))
    const labels = crumbs.getAllByText(/./).map((node) => node.textContent)
    expect(labels).toEqual(['Observability', 'Security', 'Security Dashboard'])
  })

  it('should not repeat an ancestor the page already wrote', () => {
    renderWithProviders(
      <PageHeader title="Alert History" breadcrumb={[{ label: 'Observability' }, { label: 'Alert History' }]} />,
      { route: '/observability/alert-history' },
    )
    const crumbs = within(screen.getByRole('navigation', { name: 'Breadcrumb' }))
    expect(crumbs.getAllByText('Observability')).toHaveLength(1)
  })
})
