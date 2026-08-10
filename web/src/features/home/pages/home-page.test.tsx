import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { HomePage } from './home-page'
import { useAuthStore } from '../../../stores/auth-store'
import { roleLandingPath } from '../../auth/role-landing'

const START_STACK_LABEL = /Start Stack|Stack 시작하기/
const CICD_PIPELINE_LABEL = /CI\/CD Pipeline|CI\/CD 파이프라인/
const CORE_FEATURES_LABEL = /Core Features|핵심 기능/

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
})

describe('HomePage', () => {
  it('renders the welcome heading', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByText('Nullus Platform')).toBeInTheDocument()
  })

  it('renders Stack Start button for all roles', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('renders CI/CD Pipeline button for all roles', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByRole('button', { name: CICD_PIPELINE_LABEL })).toBeInTheDocument()
  })

  it('shows core features section for developer role', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText(CORE_FEATURES_LABEL)).toBeInTheDocument()
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('shows core features section for devops role', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText(CORE_FEATURES_LABEL)).toBeInTheDocument()
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('shows core features section for admin role', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText(CORE_FEATURES_LABEL)).toBeInTheDocument()
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('does not show View Stacks button for developer role', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.queryByText('View Stacks')).not.toBeInTheDocument()
  })

  it('admin sees all three CTA buttons enabled', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<HomePage />)

    expect(screen.getByRole('button', { name: 'Register Cluster' })).toBeEnabled()
    expect(screen.getByRole('button', { name: START_STACK_LABEL })).toBeEnabled()
    expect(screen.getByRole('button', { name: CICD_PIPELINE_LABEL })).toBeEnabled()
  })

  it('devops sees Register Cluster disabled and other two enabled', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<HomePage />)

    expect(screen.getByRole('button', { name: 'Register Cluster' })).toBeDisabled()
    expect(screen.getByRole('button', { name: START_STACK_LABEL })).toBeEnabled()
    expect(screen.getByRole('button', { name: CICD_PIPELINE_LABEL })).toBeEnabled()
  })

  it('developer sees only CI/CD Pipeline enabled', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<HomePage />)

    expect(screen.getByRole('button', { name: 'Register Cluster' })).toBeDisabled()
    expect(screen.getByRole('button', { name: START_STACK_LABEL })).toBeDisabled()
    expect(screen.getByRole('button', { name: CICD_PIPELINE_LABEL })).toBeEnabled()
  })
})

describe('역할별 시작 경로', () => {
  // 로그인은 developer 를 /cicd/developer-deploy 로 보내는데 홈은 /cicd/templates 로
  // 보냈다. 그런데 사이드바는 CI/CD 템플릿을 developer 에게 숨기므로, 홈에서 한 번
  // 들어가면 메뉴로 다시 찾아갈 수 없었다.
  it('developer 의 시작 경로는 로그인 리다이렉트와 같아야 한다', () => {
    expect(roleLandingPath('developer')).toBe('/cicd/developer-deploy')
  })

  it('devops 는 스택 템플릿에서 시작한다', () => {
    expect(roleLandingPath('devops')).toBe('/stack/templates')
  })

  it('admin 은 조직 화면에서 시작한다', () => {
    expect(roleLandingPath('admin')).toBe('/admin/organization')
  })
})
