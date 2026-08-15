import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { HomePage } from './home-page'
import { useAuthStore } from '../../../stores/auth-store'
import { roleLandingPath } from '../../auth/role-landing'

// 실제 useClusters 는 배열이 아니라 { items, total } 을 돌려준다. 목을 배열로
// 두면 화면이 clusters.length 를 봐도 테스트는 통과하고, 실제로는 undefined 라
// 버튼이 영원히 비활성으로 남는다 — 목은 실물과 같은 모양이어야 한다.
const mockClusters: Array<{ id: string; name: string; connection_status: string }> = []

vi.mock('../../admin/api/admin-api', () => ({
  useClusters: () => ({ data: { items: mockClusters, total: mockClusters.length }, isLoading: false }),
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

const START_STACK_LABEL = /Start Stack|Stack 시작하기/
const CICD_PIPELINE_LABEL = /CI\/CD Pipeline|CI\/CD 파이프라인/
const CORE_FEATURES_LABEL = /Core Features|핵심 기능/

const QUICK_START_LABEL = /Quick Start|바로 시작/

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
  mockClusters.length = 0
  mockNavigate.mockClear()
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

  it('클러스터가 하나도 없으면 퀵스타트는 비활성이고 이유를 말한다', () => {
    // 클러스터 없이 눌러 봐야 마법사가 설치할 곳을 못 찾는다. 누를 수 있게
    // 두면 사용자는 그 사실을 마법사 끝에 가서야 안다.
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<HomePage />)

    expect(screen.getByRole('button', { name: QUICK_START_LABEL })).toBeDisabled()
    expect(screen.getByText(/Register a cluster first|클러스터를 먼저 등록/)).toBeInTheDocument()
  })

  it('클러스터가 등록돼 있으면 경량 템플릿 설치로 바로 보낸다', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    mockClusters.push({ id: 'cluster-1', name: 'kind-nullus-platform', connection_status: 'connected' })
    renderWithProviders(<HomePage />)

    const button = screen.getByRole('button', { name: QUICK_START_LABEL })
    expect(button).toBeEnabled()
    fireEvent.click(button)
    expect(mockNavigate).toHaveBeenCalledWith('/stack/install?template=gitea-jenkins-argocd-lite-v1')
  })

  it('스택을 설치할 수 없는 역할에게는 퀵스타트도 비활성이다', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    mockClusters.push({ id: 'cluster-1', name: 'kind-nullus-platform', connection_status: 'connected' })
    renderWithProviders(<HomePage />)

    expect(screen.getByRole('button', { name: QUICK_START_LABEL })).toBeDisabled()
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
