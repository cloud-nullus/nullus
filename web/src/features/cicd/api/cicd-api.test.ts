import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockUseQuery = vi.fn()
const mockUseMutation = vi.fn()
const mockUseQueryClient = vi.fn()
const mockInvalidateQueries = vi.fn()

vi.mock('@tanstack/react-query', () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
  useQueryClient: () => mockUseQueryClient(),
}))

vi.mock('../../../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

import {
  useAppTemplates,
  useCicdTemplates,
  useCreateCicdTemplate,
  useCreatePipeline,
  useDeleteCicdTemplate,
  useDeployApp,
  useDeployments,
  useDeployPipeline,
  usePipelines,
  useRollbackDeployment,
  useUpdateCicdTemplate,
} from './cicd-api'
import { api as mockApi } from '../../../lib/api'

describe('cicd-api hooks and exports', () => {
  const latestMutationConfig = () => {
    const calls = mockUseMutation.mock.calls
    return calls[calls.length - 1]?.[0]
  }

  beforeEach(() => {
    mockUseQuery.mockReset()
    mockUseMutation.mockReset()
    mockUseQueryClient.mockReset()
    mockInvalidateQueries.mockReset()
    vi.mocked(mockApi.get).mockReset()
    vi.mocked(mockApi.post).mockReset()
    vi.mocked(mockApi.put).mockReset()
    vi.mocked(mockApi.delete).mockReset()

    mockUseQuery.mockReturnValue({})
    mockUseMutation.mockReturnValue({})
    mockUseQueryClient.mockReturnValue({ invalidateQueries: mockInvalidateQueries })
    vi.mocked(mockApi.post).mockResolvedValue({ data: {} })
  })

  it('exports all expected hooks as functions', () => {
    expect(typeof useCicdTemplates).toBe('function')
    expect(typeof useCreateCicdTemplate).toBe('function')
    expect(typeof useUpdateCicdTemplate).toBe('function')
    expect(typeof useDeleteCicdTemplate).toBe('function')
    expect(typeof usePipelines).toBe('function')
    expect(typeof useCreatePipeline).toBe('function')
    expect(typeof useDeployPipeline).toBe('function')
    expect(typeof useDeployments).toBe('function')
    expect(typeof useAppTemplates).toBe('function')
    expect(typeof useDeployApp).toBe('function')
    expect(typeof useRollbackDeployment).toBe('function')
  })

  it('defines query hooks with expected query keys', () => {
    useCicdTemplates()
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ['cicd', 'templates'] })
    )

    usePipelines({ status: 'success', search: 'frontend' })
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['cicd', 'pipelines', { status: 'success', search: 'frontend' }],
      })
    )

    useDeployments({ pipelineId: 'p1', status: 'failed' })
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['cicd', 'deployments', { pipelineId: 'p1', status: 'failed' }],
      })
    )
  })

  it('configures template mutation hooks to invalidate template queries', () => {
    useCreateCicdTemplate()
    const createConfig = latestMutationConfig()
    createConfig.onSuccess()

    useUpdateCicdTemplate()
    const updateConfig = latestMutationConfig()
    updateConfig.onSuccess()

    useDeleteCicdTemplate()
    const deleteConfig = latestMutationConfig()
    deleteConfig.onSuccess()

    expect(mockInvalidateQueries).toHaveBeenCalledWith({ queryKey: ['cicd', 'templates'] })
  })

  it('configures deploy-related mutation invalidation', () => {
    useDeployPipeline()
    const deployPipelineConfig = latestMutationConfig()
    deployPipelineConfig.onSuccess()

    useDeployApp()
    const deployAppConfig = latestMutationConfig()
    deployAppConfig.onSuccess()

    expect(mockInvalidateQueries).toHaveBeenCalledWith({ queryKey: ['cicd', 'pipelines'] })
    expect(mockInvalidateQueries).toHaveBeenCalledWith({ queryKey: ['cicd', 'deployments'] })
  })

  it('defines rollback mutation function endpoint', async () => {
    useRollbackDeployment()
    const rollbackConfig = latestMutationConfig()

    await rollbackConfig.mutationFn({
      pipelineId: 'pipeline-1',
      deploymentId: 'deploy-1',
      preservePVC: true,
    })

    expect(vi.mocked(mockApi.post)).toHaveBeenCalledWith(
      '/api/v1/cicd/pipelines/pipeline-1/rollback/deploy-1',
      { preservePVC: true }
    )
  })
})
