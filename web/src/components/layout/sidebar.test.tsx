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
