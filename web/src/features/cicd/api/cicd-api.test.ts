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
  usePipelineImageScans,
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

describe('파이프라인 이미지 스캔', () => {
  const rawScan = {
    id: 'scan_dep_ci_pip_x_2',
    pipeline_id: 'pip_x',
    deployment_id: 'dep_ci_pip_x_2',
    image_repository: 'harbor.example/nullus/app',
    image_tag: '09b48b6e',
    image_digest: 'sha256:0c92',
    scan_source: 'central',
    scanner: 'trivy',
    scanner_version: '0.74.0',
    db_updated_at: '2026-09-13T07:13:14Z',
    counts: { critical: 0, high: 6, medium: 23, low: 19, unknown: 0 },
    gate_result: 'warn',
    report_uri: 'https://gitlab.example/nullus/app/-/jobs/11/artifacts/file/trivy-report.json',
    scanned_at: '2026-09-13T13:29:51Z',
    db_stale: false,
  }

  beforeEach(() => {
    mockUseQuery.mockReset()
    mockUseQuery.mockReturnValue({})
    vi.mocked(mockApi.get).mockReset()
  })

  it('응답을 화면 모델로 옮긴다', () => {
    expect(cicdApiCalls.mapPipelineImageScan(rawScan)).toEqual({
      id: 'scan_dep_ci_pip_x_2',
      pipelineId: 'pip_x',
      deploymentId: 'dep_ci_pip_x_2',
      imageRepository: 'harbor.example/nullus/app',
      imageTag: '09b48b6e',
      imageDigest: 'sha256:0c92',
      scanSource: 'central',
      scanner: 'trivy',
      scannerVersion: '0.74.0',
      dbUpdatedAt: '2026-09-13T07:13:14Z',
      counts: { critical: 0, high: 6, medium: 23, low: 19, unknown: 0 },
      gateResult: 'warn',
      reportUri: 'https://gitlab.example/nullus/app/-/jobs/11/artifacts/file/trivy-report.json',
      scannedAt: '2026-09-13T13:29:51Z',
      dbStale: false,
    })
  })

  it('camelCase 표기도 받는다', () => {
    const scan = cicdApiCalls.mapPipelineImageScan({
      id: 's1',
      pipelineId: 'pip_x',
      deploymentId: 'dep_1',
      gateResult: 'block',
      dbStale: true,
      reportUri: 'https://x/report.json',
    })

    expect(scan.deploymentId).toBe('dep_1')
    expect(scan.gateResult).toBe('block')
    expect(scan.dbStale).toBe(true)
    expect(scan.reportUri).toBe('https://x/report.json')
  })

  // counts 가 없다는 건 건수를 모른다는 뜻이다. 0 으로 채우면 스캔 오류가 "취약점 0" 으로 보인다.
  it('counts 가 없으면 undefined 로 남긴다', () => {
    const withoutCounts: Record<string, unknown> = { ...rawScan, gate_result: 'error' }
    delete withoutCounts.counts
    const scan = cicdApiCalls.mapPipelineImageScan(withoutCounts)

    expect(scan.counts).toBeUndefined()
    expect(scan.gateResult).toBe('error')
  })

  it('빈 선택 필드는 없음으로, db_stale 은 불리언으로 옮긴다', () => {
    const scan = cicdApiCalls.mapPipelineImageScan({
      id: 's1',
      pipeline_id: 'pip_x',
      report_uri: '',
      deployment_id: '',
      db_stale: true,
      gate_result: 'pass',
    })

    expect(scan.reportUri).toBeUndefined()
    expect(scan.deploymentId).toBeUndefined()
    expect(scan.dbStale).toBe(true)
  })

  // 모르는 판정 값을 pass 로 떨어뜨리면 초록 통과로 보인다. 판정 불가(error)로 둔다.
  it('모르는 gate_result 는 error 로 둔다', () => {
    const scan = cicdApiCalls.mapPipelineImageScan({ id: 's1', gate_result: 'maybe' })

    expect(scan.gateResult).toBe('error')
    expect(scan.dbStale).toBe(false)
  })

  it('파이프라인의 스캔 목록 경로를 부른다', async () => {
    vi.mocked(mockApi.get).mockResolvedValueOnce({ data: { items: [rawScan], total: 1 } } as never)

    const result = await cicdApiCalls.getPipelineImageScans('pip_x')

    expect(vi.mocked(mockApi.get)).toHaveBeenCalledWith('/cicd/pipelines/pip_x/image-scans')
    expect(result.total).toBe(1)
    expect(result.items[0].gateResult).toBe('warn')
  })

  // 실행 목록 조회가 서버에서 스캔 기록을 만든다. 실행 목록이 새로 오면 스캔도
  // 다시 읽도록 그 갱신 시각을 키에 넣는다.
  it('실행 목록 갱신 시각을 키에 넣고, 실행 목록을 받기 전에는 조회하지 않는다', () => {
    usePipelineImageScans('pip_x', 1234)
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.objectContaining({
        queryKey: ['cicd', 'pipelineImageScans', 'pip_x', 1234],
        enabled: true,
        retry: false,
      }),
    )

    usePipelineImageScans('pip_x', 0)
    expect(mockUseQuery).toHaveBeenLastCalledWith(expect.objectContaining({ enabled: false }))

    usePipelineImageScans('', 1234)
    expect(mockUseQuery).toHaveBeenLastCalledWith(expect.objectContaining({ enabled: false }))
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
