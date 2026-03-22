import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdHistoryPage } from './cicd-history-page'

const mockMutate = vi.fn()

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
          id: 'd-prev',
          pipelineId: 'frontend-web',
          pipelineName: 'frontend-web',
          version: 'v1.2.2',
          status: 'success',
          triggeredBy: 'lee.devops',
          startedAt: '2026-03-02T11:00:00Z',
          completedAt: '2026-03-02T11:20:00Z',
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
        {
          id: 'd-pending',
          pipelineId: 'batch-runner',
          pipelineName: 'batch-runner',
          version: 'v1.3.0',
          status: 'pending',
          triggeredBy: 'choi.devops',
          startedAt: '2026-03-03T09:00:00Z',
          completedAt: null,
        },
      ],
      total: 5,
    },
  }),
  useRollbackDeployment: () => ({
    mutate: mockMutate,
    isPending: false,
  }),
}))

describe('CicdHistoryPage rollback flow', () => {
  beforeEach(() => {
    mockMutate.mockReset()
  })

  it('shows rollback action only for success and failed deployments', () => {
    renderWithProviders(<CicdHistoryPage />)

    const rollbackButtons = screen.getAllByTestId('rollback-btn')
    expect(rollbackButtons).toHaveLength(3)
  })

  it('requires typing ROLLBACK before confirm and calls rollback mutation', async () => {
    renderWithProviders(<CicdHistoryPage />)

    fireEvent.click(screen.getAllByTestId('rollback-btn')[0])

    expect(screen.queryByText('배포 롤백 확인')).not.toBeNull()
    const confirmButton = screen.getByTestId('rollback-confirm')
    expect(confirmButton.hasAttribute('disabled')).toBe(true)

    fireEvent.change(screen.getByPlaceholderText('ROLLBACK'), { target: { value: 'ROLLBACK' } })
    await waitFor(() => expect(confirmButton.hasAttribute('disabled')).toBe(false))

    fireEvent.click(confirmButton)
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        { pipelineId: 'frontend-web', deploymentId: 'd-success', preservePVC: true },
        expect.any(Object)
      )
    })
  })
})
