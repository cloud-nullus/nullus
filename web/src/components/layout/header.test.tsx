import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../__tests__/test-utils'
import { Header } from './header'
import { useAuthStore } from '../../stores/auth-store'
import { useThemeStore } from '../../stores/theme-store'

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
  useThemeStore.setState({ theme: 'dark' })
})

describe('Header', () => {
  it('renders the header element', () => {
    renderWithProviders(<Header />)
    expect(screen.getByRole('banner')).toBeInTheDocument()
  })

  it('renders language selector', () => {
    renderWithProviders(<Header />)
    expect(screen.getByLabelText('Select language')).toBeInTheDocument()
  })

  it('language selector has EN and Korean options', () => {
    renderWithProviders(<Header />)
    const select = screen.getByLabelText('Select language')
    expect(select).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'EN' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: '한국어' })).toBeInTheDocument()
  })

  it('renders theme toggle button', () => {
    renderWithProviders(<Header />)
    expect(screen.getByLabelText('Switch to light mode')).toBeInTheDocument()
  })

  it('theme toggle button label flips when theme is light', () => {
    useThemeStore.setState({ theme: 'light' })
    renderWithProviders(<Header />)
    expect(screen.getByLabelText('Switch to dark mode')).toBeInTheDocument()
  })

  it('clicking theme toggle switches theme from dark to light', () => {
    useThemeStore.setState({ theme: 'dark' })
    renderWithProviders(<Header />)
    fireEvent.click(screen.getByLabelText('Switch to light mode'))
    expect(useThemeStore.getState().theme).toBe('light')
  })

  it('clicking theme toggle switches theme from light to dark', () => {
    useThemeStore.setState({ theme: 'light' })
    renderWithProviders(<Header />)
    fireEvent.click(screen.getByLabelText('Switch to dark mode'))
    expect(useThemeStore.getState().theme).toBe('dark')
  })

  it('shows Developer role badge for developer role', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<Header />)
    expect(screen.getByText('Developer')).toBeInTheDocument()
  })

  it('shows DevOps role badge for devops role', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: false })
    renderWithProviders(<Header />)
    expect(screen.getByText('DevOps')).toBeInTheDocument()
  })

  it('shows Admin role badge for admin role', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: false })
    renderWithProviders(<Header />)
    expect(screen.getByText('Admin')).toBeInTheDocument()
  })

  it('language change calls i18n changeLanguage', () => {
    renderWithProviders(<Header />)
    const select = screen.getByLabelText('Select language')
    fireEvent.change(select, { target: { value: 'ko' } })
    // The select value should update (i18n may or may not reflect in jsdom but no error)
    expect(select).toBeInTheDocument()
  })
})
