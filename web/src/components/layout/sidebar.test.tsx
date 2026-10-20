import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../__tests__/test-utils'
import { Sidebar } from './sidebar'
import { useAuthStore } from '../../stores/auth-store'
import { useSidebarStore } from '../../stores/sidebar-store'

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
  useSidebarStore.setState({ collapsed: false })
})

describe('Sidebar', () => {
  it('renders the Nullus logo when expanded', () => {
    renderWithProviders(<Sidebar />)
    expect(screen.getByText('Nullus')).toBeInTheDocument()
  })

  it('renders toggle sidebar button', () => {
    renderWithProviders(<Sidebar />)
    expect(screen.getByLabelText('Toggle sidebar')).toBeInTheDocument()
  })

  it('renders logout button', () => {
    renderWithProviders(<Sidebar />)
    expect(screen.getByLabelText('Logout')).toBeInTheDocument()
  })

  describe('role-based menu filtering', () => {
    it('developer sees CI/CD and Observability but not DevSecOps Stack', () => {
      useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      expect(screen.getByText('CI/CD')).toBeInTheDocument()
      expect(screen.getByText('Observability')).toBeInTheDocument()
      expect(screen.queryByText('DevSecOps Stack')).not.toBeInTheDocument()
      expect(screen.queryByText('Admin')).not.toBeInTheDocument()
    })

    it('devops sees DevSecOps Stack, CI/CD and Observability but not Admin', () => {
      useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      expect(screen.getByText('DevSecOps Stack')).toBeInTheDocument()
      expect(screen.getByText('CI/CD')).toBeInTheDocument()
      expect(screen.getByText('Observability')).toBeInTheDocument()
      expect(screen.queryByText('Admin')).not.toBeInTheDocument()
    })

    it('admin sees only Admin group', () => {
      useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      expect(screen.getByText('Admin')).toBeInTheDocument()
    })

    it('developer can see CI/CD List nav link', () => {
      useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      expect(screen.getByText('CI/CD List')).toBeInTheDocument()
    })

    it('developer cannot see CI/CD Template nav link', () => {
      useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      expect(screen.queryByText('CI/CD Template')).not.toBeInTheDocument()
    })

    it('devops can see Stack Template nav link', () => {
      useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      expect(screen.getByText('Stack Template')).toBeInTheDocument()
    })

    it('admin can see Organization, User Management, Cluster Management nav links', () => {
      useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      expect(screen.getByText('Organization')).toBeInTheDocument()
      expect(screen.getByText('User Management')).toBeInTheDocument()
      expect(screen.getByText('Cluster Management')).toBeInTheDocument()
    })
  })

  describe('collapse/expand toggle', () => {
    it('toggling sidebar calls toggleSidebar', () => {
      renderWithProviders(<Sidebar />)
      const toggleBtn = screen.getByLabelText('Toggle sidebar')
      fireEvent.click(toggleBtn)
      expect(useSidebarStore.getState().collapsed).toBe(true)
    })

    it('toggling twice restores expanded state', () => {
      renderWithProviders(<Sidebar />)
      const toggleBtn = screen.getByLabelText('Toggle sidebar')
      fireEvent.click(toggleBtn)
      fireEvent.click(toggleBtn)
      expect(useSidebarStore.getState().collapsed).toBe(false)
    })

    it('hides logo text when collapsed', () => {
      useSidebarStore.setState({ collapsed: true })
      renderWithProviders(<Sidebar />)
      expect(screen.queryByText('Nullus')).not.toBeInTheDocument()
    })
  })

  describe('group collapse toggle', () => {
    it('clicking a group header collapses the group items', () => {
      useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: false })
      renderWithProviders(<Sidebar />)
      // DevSecOps Stack group is expanded by default — Stack Template link is visible
      expect(screen.getByText('Stack Template')).toBeInTheDocument()
      // Click the group header to collapse
      fireEvent.click(screen.getByLabelText('DevSecOps Stack'))
      expect(screen.queryByText('Stack Template')).not.toBeInTheDocument()
    })
  })
})

describe('연결되지 않았던 화면의 메뉴 노출', () => {
  // 백엔드에 토큰 소스 API 가 9개 있는데 화면이 메뉴에 없어 회전 실패나 승인 대기를
  // UI 로 확인할 방법이 없었다.
  it('admin 은 토큰 관리 메뉴를 본다', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.getByText('Token Management')).toBeInTheDocument()
  })

  it('devops 는 토큰 관리 메뉴를 보지 못한다', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.queryByText('Token Management')).not.toBeInTheDocument()
  })

  it('admin·devops 는 골든패스 메뉴를 본다', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.getByText('Golden Paths')).toBeInTheDocument()
  })
})

// 메뉴 3단(그룹 › 하위 그룹 › 항목). 보안은 관측성 아래 하위 그룹이고 첫 항목이 보안 대시보드다.
describe('하위 그룹(3단 메뉴)', () => {
  it.each(['admin', 'devops', 'developer'] as const)(
    'should show the Security sub-group and its dashboard link to %s',
    (role) => {
      useAuthStore.setState({ role, user: null, isAuthenticated: true })
      renderWithProviders(<Sidebar />)
      const subGroup = screen.getByRole('button', { name: 'Security' })
      expect(subGroup.getAttribute('aria-expanded')).toBe('true')
      expect(screen.getByRole('link', { name: 'Security Dashboard' }).getAttribute('href')).toBe(
        '/observability/security/dashboard',
      )
    },
  )

  it('should collapse and expand the sub-group without closing its group', () => {
    renderWithProviders(<Sidebar />)
    fireEvent.click(screen.getByRole('button', { name: 'Security' }))
    expect(screen.queryByRole('link', { name: 'Security Dashboard' })).toBeNull()
    // 그룹(관측성)의 다른 항목은 그대로다.
    expect(screen.getByText('Alert History')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Security' }))
    expect(screen.getByRole('link', { name: 'Security Dashboard' })).toBeInTheDocument()
  })

  it('should indent sub-group items one level deeper than group items', () => {
    renderWithProviders(<Sidebar />)
    expect(screen.getByRole('link', { name: 'Security Dashboard' }).className).toContain('pl-10')
    expect(screen.getByText('Alert History').closest('a')?.className).toContain('pl-6')
  })

  it('should keep the sub-group open on its child route', () => {
    renderWithProviders(<Sidebar />, { route: '/observability/security/dashboard' })
    expect(screen.getByRole('button', { name: 'Security' }).getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('link', { name: 'Security Dashboard' })).toBeInTheDocument()
  })

  // 접힌 아이콘 레일에는 펼침 단추를 둘 자리가 없다 — 자식을 평평하게 늘어놓는다.
  it('should flatten sub-group children in the collapsed icon rail', () => {
    useSidebarStore.setState({ collapsed: true })
    const { container } = renderWithProviders(<Sidebar />)
    expect(screen.queryByRole('button', { name: 'Security' })).toBeNull()
    expect(container.querySelector('a[href="/observability/security/dashboard"]')).not.toBeNull()
  })
})
