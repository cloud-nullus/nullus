import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { RetryStackButton } from './retry-stack-button'

const toastSuccess = vi.hoisted(() => vi.fn())
const toastError = vi.hoisted(() => vi.fn())
const toastLoading = vi.hoisted(() => vi.fn().mockReturnValue('toast-id-xyz'))
const toastDismiss = vi.hoisted(() => vi.fn())
const navigateMock = vi.hoisted(() => vi.fn())
const retryMutate = vi.hoisted(() =>
  vi.fn<(input: unknown, opts: { onSuccess?: (data: unknown) => void; onError?: (err: unknown) => void }) => void>(),
)

vi.mock('sonner', () => ({
  toast: { success: toastSuccess, error: toastError, loading: toastLoading, dismiss: toastDismiss },
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => navigateMock }
})

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
    // Message is the first arg; options (id from toast.loading) is the second.
    const [message] = toastSuccess.mock.calls[0]
    expect(message).toMatch(/redeploy started|재배포/i)
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
    // F8-UIUX-RetryFeedback now routes the error through the loading toast's id,
    // so the message is the first arg with options in the second.
    const [message] = toastError.mock.calls[0]
    expect(message).toContain('K8S_RANGE_MISMATCH')
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
    // F8-UIUX-WarnAckI18n: dedicated confirmWarn labels, not cross-feature admin keys.
    expect(screen.getByRole('button', { name: /^Cancel$/ })).toBeInTheDocument()
    expect(screen.getByTestId('retry-warn-confirm')).toHaveTextContent(/Acknowledge and retry/)
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

describe('RetryStackButton progressive feedback (F8-UIUX-RetryFeedback)', () => {
  it('emits a loading toast immediately on click', () => {
    retryMutate.mockImplementation(() => {
      /* never resolve — stay in loading */
    })
    renderWithProviders(<RetryStackButton stackId="s1" status="failed" />)
    fireEvent.click(screen.getByTestId('retry-stack-button'))
    expect(toastLoading).toHaveBeenCalledTimes(1)
  })

  it('success replaces the loading toast via the same id', () => {
    retryMutate.mockImplementation((_input, opts) => {
      opts.onSuccess?.({ stack_id: 's1', status: 'pending' })
    })
    renderWithProviders(<RetryStackButton stackId="s1" status="failed" />)
    fireEvent.click(screen.getByTestId('retry-stack-button'))
    expect(toastLoading).toHaveBeenCalledTimes(1)
    expect(toastSuccess).toHaveBeenCalledTimes(1)
    const successOpts = toastSuccess.mock.calls[0][1]
    expect(successOpts).toEqual(expect.objectContaining({ id: 'toast-id-xyz' }))
  })

  it('fires a toast.error with fix-combination action on DEPLOY_COMPAT_FAIL', () => {
    retryMutate.mockImplementation((_input, opts) => {
      opts.onError?.({
        details: {
          error: {
            code: 'DEPLOY_COMPAT_FAIL',
            verdict: { issues: [{ code: 'K8S_RANGE_MISMATCH', message: 'out of range' }] },
          },
        },
      })
    })
    renderWithProviders(<RetryStackButton stackId="s1" status="failed" />)
    fireEvent.click(screen.getByTestId('retry-stack-button'))
    expect(toastError).toHaveBeenCalledTimes(1)
    const [, opts] = toastError.mock.calls[0]
    expect(opts.id).toBe('toast-id-xyz')
    expect(opts.action).toBeDefined()
    expect(typeof opts.action.label).toBe('string')
    expect(typeof opts.action.onClick).toBe('function')

    // Clicking the action navigates to the install wizard for the stack id.
    opts.action.onClick()
    expect(navigateMock).toHaveBeenCalledWith('/stack/install?stackId=s1')
  })
})
