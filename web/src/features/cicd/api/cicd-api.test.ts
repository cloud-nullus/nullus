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
  cicdApiCalls,
  useAppTemplates,
  useCicdTemplates,
  useCreateCicdTemplate,
  useCreatePipeline,
  useDeletePipeline,
  useDeleteCicdTemplate,
  useDeployApp,
  useDeploymentStatus,
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
    expect(typeof useDeletePipeline).toBe('function')
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

    useDeletePipeline()
    const deletePipelineConfig = latestMutationConfig()
    deletePipelineConfig.onSuccess()

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

  it('normalizes deployment step fields for pipeline logs output', async () => {
    vi.mocked(mockApi.get).mockResolvedValueOnce({
      data: {
        ID: 'dep-1',
        Status: 'success',
        Steps: [
          {
            Name: 'Deploy',
            Status: 'success',
            Kind: 'Deployment',
            Message: 'deployment completed',
            Logs: ['line-1', 'line-2'],
          },
        ],
      },
    } as any)

    useDeploymentStatus('dep-1')
    const config = mockUseQuery.mock.calls[mockUseQuery.mock.calls.length - 1]?.[0]
    const result = await config.queryFn()

    expect(result.steps).toEqual([
      expect.objectContaining({
        name: 'Deploy',
        status: 'success',
        kind: 'Deployment',
        message: 'deployment completed',
        logs: ['line-1', 'line-2'],
      }),
    ])
  })
})

describe('getDeployments 응답 표기 대응', () => {
  // 회귀 배경: API 는 camelCase 로 응답하는데 매퍼가 snake_case 만 읽어서
  // CI/CD 이력 화면의 파이프라인·배포자·시작·완료 컬럼이 항상 비어 보였다.
  // 실데이터로 앱을 돌려 보고 발견했다.
  const rows = [
    {
      shape: 'camelCase (실제 API 응답)',
      row: {
        id: 'dep_1',
        pipelineId: 'pip_1',
        pipelineName: 'sample-backend',
        version: 'v1.0.0',
        status: 'success',
        triggeredBy: 'kim.dev',
        startedAt: '2026-08-11T10:07:45+09:00',
        completedAt: '2026-08-11T10:07:46+09:00',
      },
    },
    {
      shape: 'snake_case (구 표기)',
      row: {
        id: 'dep_1',
        pipeline_id: 'pip_1',
        version: 'v1.0.0',
        status: 'success',
        deployed_by: 'kim.dev',
        started_at: '2026-08-11T10:07:45+09:00',
        completed_at: '2026-08-11T10:07:46+09:00',
      },
    },
  ]

  it.each(rows)('$shape 를 빠짐없이 매핑한다', async ({ row }) => {
    vi.mocked(mockApi.get).mockImplementation((url: string) => {
      if (url === '/cicd/deployments') return Promise.resolve({ data: { items: [row], total: 1 } })
      if (url === '/cicd/pipelines')
        return Promise.resolve({ data: { items: [{ id: 'pip_1', name: 'sample-backend' }] } })
      return Promise.resolve({ data: { items: [] } })
    })

    const result = await cicdApiCalls.getDeployments()
    const item = result.items[0]

    expect(item.pipelineId).toBe('pip_1')
    expect(item.pipelineName).toBe('sample-backend')
    expect(item.triggeredBy).toBe('kim.dev')
    expect(item.startedAt).toBe('2026-08-11T10:07:45+09:00')
    expect(item.completedAt).toBe('2026-08-11T10:07:46+09:00')
  })
})
