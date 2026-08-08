import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdHistoryPage } from './cicd-history-page'

// 롤백 UI 는 282cc8f 에서 제거됐다. useRollbackDeployment 훅과 그 테스트는
// cicd-api 쪽에 남아 있으므로 여기서는 이력 표시만 검증한다.
vi.mock('../api/cicd-api', () => ({
  useDeployments: () => ({
    data: {
      items: [
        {
          id: 'd-success',
          pipelineId: 'frontend-web',
          pipelineName: 'frontend-web',
          version: 'v1.2.3',
          status: 'success',
          triggeredBy: 'kim.dev',
          startedAt: '2026-03-03T14:22:00Z',
          completedAt: '2026-03-03T14:28:00Z',
        },
        {
          id: 'd-failed',
          pipelineId: 'backend-api',
          pipelineName: 'backend-api',
          version: 'v2.1.0',
          status: 'failed',
          triggeredBy: 'park.dev',
          startedAt: '2026-03-01T16:25:00Z',
          completedAt: '2026-03-01T16:30:00Z',
        },
        {
          id: 'd-running',
          pipelineId: 'batch-runner',
          pipelineName: 'batch-runner',
          version: 'v1.3.1',
          status: 'running',
          triggeredBy: 'choi.devops',
          startedAt: '2026-03-03T10:00:00Z',
          completedAt: null,
        },
      ],
      total: 3,
    },
  }),
}))

describe('CicdHistoryPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<CicdHistoryPage />)

    expect(
      screen.getByRole('heading', { level: 1, name: 'CI/CD History' }),
    ).not.toBeNull()
  })

  it('renders a row per deployment with pipeline, version and deployer', () => {
    renderWithProviders(<CicdHistoryPage />)

    expect(screen.getAllByText('frontend-web').length).toBeGreaterThan(0)
    expect(screen.getAllByText('backend-api').length).toBeGreaterThan(0)
    expect(screen.getAllByText('batch-runner').length).toBeGreaterThan(0)

    expect(screen.getByText('v1.2.3')).not.toBeNull()
    expect(screen.getByText('v2.1.0')).not.toBeNull()
    expect(screen.getByText('kim.dev')).not.toBeNull()
  })

  it('shows the deployment status of each row', () => {
    renderWithProviders(<CicdHistoryPage />)

    expect(screen.getAllByText(/success/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/failed/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/running/i).length).toBeGreaterThan(0)
  })
})
