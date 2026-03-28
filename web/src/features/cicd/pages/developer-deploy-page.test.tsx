import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { DeveloperDeployPage } from './developer-deploy-page'

const mockUseAppTemplates = vi.fn()
const mockUseDeployApp = vi.fn()
const mockUseClusters = vi.fn()
const mockDeployMutate = vi.fn()

vi.mock('../api/cicd-api', () => ({
  useAppTemplates: () => mockUseAppTemplates(),
  useDeployApp: () => mockUseDeployApp(),
}))

vi.mock('../../admin/api/admin-api', () => ({
  useClusters: () => mockUseClusters(),
}))

vi.mock('../../../components/shared/code-preview', () => ({
  CodePreview: ({ title }: { title: string }) => <div>{title}</div>,
}))

const appTemplates = [
  {
    id: 'react-spa',
    name: 'React SPA',
    description: 'React 기반 단일 페이지 앱',
    runtime: 'Node.js',
    language: 'TypeScript',
  },
]

const clusters = {
  items: [{ id: 'c1', name: 'prod-k8s' }],
  total: 1,
}

describe('DeveloperDeployPage', () => {
  beforeEach(() => {
    mockUseAppTemplates.mockReset()
    mockUseDeployApp.mockReset()
    mockUseClusters.mockReset()
    mockDeployMutate.mockReset()

    mockUseAppTemplates.mockReturnValue({ data: appTemplates, isLoading: false })
    mockUseClusters.mockReturnValue({ data: clusters, isLoading: false })
    mockUseDeployApp.mockReturnValue({ mutate: mockDeployMutate, isPending: false })
  })

  it('renders loading state safely', () => {
    mockUseAppTemplates.mockReturnValue({ data: undefined, isLoading: true })
    mockUseClusters.mockReturnValue({ data: undefined, isLoading: true })

    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.getByText('CI/CD Pipeline Setup & Developer Deploy')).not.toBeNull()
    expect(screen.getByText('앱 템플릿')).not.toBeNull()
  })

  it('renders app template data', () => {
    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.getByRole('button', { name: /React SPA/i })).not.toBeNull()
    expect(screen.getByText('TypeScript')).not.toBeNull()
  })

  it('renders empty-state shell when app templates are empty', () => {
    mockUseAppTemplates.mockReturnValue({ data: [], isLoading: false })

    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.queryByText('React SPA')).toBeNull()
    expect(screen.getByText('앱 템플릿')).not.toBeNull()
  })

  it('progresses to next step when step 1 becomes valid', async () => {
    renderWithProviders(<DeveloperDeployPage />)

    fireEvent.change(screen.getByPlaceholderText('my-awesome-app'), {
      target: { value: 'demo-app' },
    })

    fireEvent.click(screen.getByRole('button', { name: '다음' }))

    expect(await screen.findByText('Git Repository URL')).not.toBeNull()
  })
})
