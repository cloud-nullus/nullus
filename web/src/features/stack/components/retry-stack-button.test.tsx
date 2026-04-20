import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { RetryStackButton } from './retry-stack-button'

const toastSuccess = vi.hoisted(() => vi.fn())
const toastError = vi.hoisted(() => vi.fn())
const retryMutate = vi.hoisted(() =>
  vi.fn<(input: unknown, opts: { onSuccess?: (data: unknown) => void; onError?: (err: unknown) => void }) => void>(),
)

vi.mock('sonner', () => ({
  toast: { success: toastSuccess, error: toastError },
}))

vi.mock('../api/stack-api', () => ({
  useRetryStack: () => ({ mutate: retryMutate, isPending: false }),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

describe('RetryStackButton', () => {
  it('fires toast.success when retry succeeds', () => {
    retryMutate.mockImplementation((_input, opts) => {
      opts.onSuccess?.({ stack_id: 's1', status: 'pending' })
    })
    const onRetried = vi.fn()
    renderWithProviders(<RetryStackButton stackId="s1" status="failed" onRetried={onRetried} />)
    fireEvent.click(screen.getByTestId('retry-stack-button'))
    expect(toastSuccess).toHaveBeenCalledTimes(1)
    expect(toastSuccess).toHaveBeenCalledWith(expect.stringMatching(/redeploy started|재배포/i))
    expect(onRetried).toHaveBeenCalledWith('s1')
    expect(toastError).not.toHaveBeenCalled()
  })

  it('fires toast.error including the issue list on DEPLOY_COMPAT_FAIL', () => {
    retryMutate.mockImplementation((_input, opts) => {
      opts.onError?.({
        details: {
          error: {
            code: 'DEPLOY_COMPAT_FAIL',
            verdict: {
              issues: [
                { code: 'K8S_RANGE_MISMATCH', message: 'cluster 1.32 out of range' },
              ],
            },
          },
        },
      })
    })
    renderWithProviders(<RetryStackButton stackId="s1" status="failed" />)
    fireEvent.click(screen.getByTestId('retry-stack-button'))
    expect(toastError).toHaveBeenCalledTimes(1)
    expect(toastError).toHaveBeenCalledWith(expect.stringContaining('K8S_RANGE_MISMATCH'))
    expect(toastSuccess).not.toHaveBeenCalled()
  })

  it('shows the warn-ack modal (no toast) on DEPLOY_COMPAT_WARN_UNACK', async () => {
    retryMutate.mockImplementation((_input, opts) => {
      opts.onError?.({
        details: {
          error: {
            code: 'DEPLOY_COMPAT_WARN_UNACK',
            verdict: {
              issues: [{ code: 'TOOL_ARCH_UNSUPPORTED', message: 'argo lacks arm64' }],
            },
          },
        },
      })
    })
    renderWithProviders(<RetryStackButton stackId="s1" status="rolled_back" />)
    fireEvent.click(screen.getByTestId('retry-stack-button'))
    await waitFor(() => {
      expect(screen.getByTestId('retry-warn-ack')).toBeInTheDocument()
    })
    expect(screen.getByText(/TOOL_ARCH_UNSUPPORTED/)).toBeInTheDocument()
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(toastError).not.toHaveBeenCalled()
  })

  it('fires toast.success after warn-ack → re-submit succeeds', async () => {
    let call = 0
    retryMutate.mockImplementation((_input, opts) => {
      call += 1
      if (call === 1) {
        opts.onError?.({
          details: {
            error: {
              code: 'DEPLOY_COMPAT_WARN_UNACK',
              verdict: { issues: [{ code: 'TOOL_ARCH_UNSUPPORTED', message: 'arm64' }] },
            },
          },
        })
        return
      }
      opts.onSuccess?.({ stack_id: 's1', status: 'pending' })
    })
    renderWithProviders(<RetryStackButton stackId="s1" status="failed" />)
    fireEvent.click(screen.getByTestId('retry-stack-button'))
    await waitFor(() => expect(screen.getByTestId('retry-warn-ack')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('retry-warn-ack'))
    fireEvent.click(screen.getByTestId('retry-warn-confirm'))
    expect(toastSuccess).toHaveBeenCalledTimes(1)
    expect(retryMutate).toHaveBeenCalledTimes(2)
  })
})
