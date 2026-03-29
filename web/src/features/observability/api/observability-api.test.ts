import { describe, it, expect, beforeEach, vi } from 'vitest'

const useQueryMock = vi.hoisted(() => vi.fn())
const useMutationMock = vi.hoisted(() => vi.fn())
const invalidateQueriesMock = vi.hoisted(() => vi.fn())
const useQueryClientMock = vi.hoisted(() => vi.fn())

const apiGetMock = vi.hoisted(() => vi.fn())
const apiPostMock = vi.hoisted(() => vi.fn())
const apiPatchMock = vi.hoisted(() => vi.fn())
const apiDeleteMock = vi.hoisted(() => vi.fn())

vi.mock('@tanstack/react-query', () => ({
  useQuery: useQueryMock,
  useMutation: useMutationMock,
  useQueryClient: useQueryClientMock,
}))

vi.mock('../../../lib/api', () => ({
  api: {
    get: apiGetMock,
    post: apiPostMock,
    patch: apiPatchMock,
    delete: apiDeleteMock,
  },
}))

import {
  useDashboard,
  useAlertRules,
  useCreateAlertRule,
  useUpdateAlertRule,
  useDeleteAlertRule,
  useAlertHistory,
} from './observability-api'

describe('observability-api hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useQueryMock.mockReturnValue({ data: undefined })
    useMutationMock.mockImplementation((options: unknown) => options)
    useQueryClientMock.mockReturnValue({ invalidateQueries: invalidateQueriesMock })
    apiGetMock.mockResolvedValue({ data: { items: [], total: 0 } })
    apiPostMock.mockResolvedValue({ data: { id: 'rule-1' } })
    apiPatchMock.mockResolvedValue({ data: { id: 'rule-1' } })
    apiDeleteMock.mockResolvedValue({ data: {} })
  })

  it('defines useDashboard query with key and refetch interval', () => {
    useDashboard(9000)

    expect(useQueryMock).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['observability', 'dashboard'],
        refetchInterval: 9000,
      })
    )
  })

  it('defines useAlertRules query key', () => {
    useAlertRules()

    expect(useQueryMock).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['observability', 'alert-rules'],
      })
    )
  })

  it('defines useAlertHistory query key with filters', () => {
    useAlertHistory({ severity: 'critical' })

    expect(useQueryMock).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['observability', 'alert-history', { severity: 'critical' }],
      })
    )
  })

  it('useCreateAlertRule invalidates rules and history after success', () => {
    useCreateAlertRule()
    const calls = useMutationMock.mock.calls
    const options = calls[calls.length - 1]?.[0] as { onSuccess?: () => void }

    expect(useMutationMock).toHaveBeenCalled()
    options.onSuccess?.()

    expect(invalidateQueriesMock).toHaveBeenCalledWith({ queryKey: ['observability', 'alert-rules'] })
    expect(invalidateQueriesMock).toHaveBeenCalledWith({ queryKey: ['observability', 'alert-history'] })
  })

  it('useUpdateAlertRule mutation function patches alert rule by id', async () => {
    useUpdateAlertRule()
    const calls = useMutationMock.mock.calls
    const options = calls[calls.length - 1]?.[0] as {
      mutationFn: (variables: { id: string; data: Record<string, unknown> }) => Promise<unknown>
    }

    await options.mutationFn({ id: 'rule-99', data: { enabled: false } })

    expect(apiPatchMock).toHaveBeenCalledWith('/observability/alert-rules/rule-99', { enabled: false })
  })

  it('useCreateAlertRule mutation function posts warning and critical thresholds', async () => {
    useCreateAlertRule()
    const calls = useMutationMock.mock.calls
    const options = calls[calls.length - 1]?.[0] as {
      mutationFn: (variables: {
        name: string
        metric_name: string
        warning_threshold: number
        critical_threshold: number
        channel: string
      }) => Promise<unknown>
    }

    await options.mutationFn({
      name: 'High CPU',
      metric_name: 'cpu_usage',
      warning_threshold: 70,
      critical_threshold: 85,
      channel: 'slack',
    })

    expect(apiPostMock).toHaveBeenCalledWith('/observability/alert-rules', {
      name: 'High CPU',
      metric_name: 'cpu_usage',
      warning_threshold: 70,
      critical_threshold: 85,
      channel: 'slack',
    })
  })

  it('useDeleteAlertRule mutation function deletes alert rule by id', async () => {
    useDeleteAlertRule()
    const calls = useMutationMock.mock.calls
    const options = calls[calls.length - 1]?.[0] as {
      mutationFn: (id: string) => Promise<unknown>
    }

    await options.mutationFn('rule-21')

    expect(apiDeleteMock).toHaveBeenCalledWith('/observability/alert-rules/rule-21')
  })
})
