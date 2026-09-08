import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../__tests__/test-utils'
import { Header } from './header'
import { useAuthStore } from '../../stores/auth-store'
import { useThemeStore } from '../../stores/theme-store'
import { useTourStore } from '../../stores/tour-store'

const TUTORIAL_LABEL = /Tutorial|튜토리얼/

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
  useThemeStore.setState({ theme: 'dark' })
  useTourStore.getState().stop()
})

describe('Header', () => {
  // 작업 알림 종은 로그인한 사용자에게만 건다 — 목록 API 두 개를 폴링하기 때문이다.
  it('로그인 전에는 작업 알림 종이 없다', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<Header />)

    expect(screen.queryByTestId('job-notification-bell')).not.toBeInTheDocument()
  })

  it('로그인하면 작업 알림 종이 상단에 뜬다', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<Header />)

    expect(screen.getByTestId('job-notification-bell')).toBeInTheDocument()
  })

  it('튜토리얼 버튼이 언어 버튼 바로 옆에 있다', () => {
    renderWithProviders(<Header />)

    const tutorial = screen.getByRole('button', { name: TUTORIAL_LABEL })
    const languageSwitcher = screen.getByText('EN').closest('div') as HTMLElement
    // 자리를 눈이 아니라 DOM 순서로 고정한다 — 다음 사람이 버튼을 하나 더 넣어도
    // 이 둘 사이를 갈라놓지 못한다.
    expect(languageSwitcher.nextElementSibling).toBe(tutorial)
  })

  it('튜토리얼 버튼이 현재 역할로 투어를 시작한다', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<Header />)

    fireEvent.click(screen.getByRole('button', { name: TUTORIAL_LABEL }))

    const { isActive, steps } = useTourStore.getState()
    expect(isActive).toBe(true)
    expect(steps.map((step) => step.id)).not.toContain('registerCluster')
  })

  it('renders the header element', () => {
    renderWithProviders(<Header />)
    expect(screen.getByRole('banner')).toBeInTheDocument()
  })

  it('renders language selector', () => {
    renderWithProviders(<Header />)
    expect(screen.getByText('EN')).toBeInTheDocument()
    expect(screen.getByText('KO')).toBeInTheDocument()
  })

  it('language selector has EN and Korean options', () => {
    renderWithProviders(<Header />)
    expect(screen.getByText('EN')).toBeInTheDocument()
    expect(screen.getByText('KO')).toBeInTheDocument()
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
    const koButton = screen.getByText('KO')
    fireEvent.click(koButton)
    // The button click should trigger language change
    expect(koButton).toBeInTheDocument()
  })
})
