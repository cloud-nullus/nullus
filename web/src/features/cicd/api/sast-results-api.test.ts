import { describe, expect, it, vi } from 'vitest'

vi.mock('../../../lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}))

import { api } from '../../../lib/api'
import { cicdApiCalls } from './cicd-api'

const rawResult = {
  id: 'sast_dep_ci_pip_x_7',
  pipeline_id: 'pip_x',
  deployment_id: 'dep_ci_pip_x_7',
  project_key: 'app',
  analysis_id: 'an-1',
  quality_gate_status: 'ERROR',
  gate_result: 'block',
  conditions: [
    { metric: 'new_violations', comparator: 'GT', threshold: '0', actual: '1', status: 'ERROR' },
  ],
  metrics: { bugs: 0, vulnerabilities: 3, coverage: 12.5 },
  dashboard_url: 'https://sonarqube.example.com/dashboard?id=app',
  analyzed_at: '2026-10-11T02:00:00Z',
}

describe('mapPipelineSASTResult', () => {
  it('응답을 화면 모델로 옮긴다', () => {
    expect(cicdApiCalls.mapPipelineSASTResult(rawResult)).toEqual({
      id: 'sast_dep_ci_pip_x_7',
      pipelineId: 'pip_x',
      deploymentId: 'dep_ci_pip_x_7',
      projectKey: 'app',
      qualityGateStatus: 'ERROR',
      gateResult: 'block',
      conditions: [
        { metric: 'new_violations', comparator: 'GT', threshold: '0', actual: '1', status: 'ERROR' },
      ],
      metrics: { bugs: 0, vulnerabilities: 3, coverage: 12.5 },
      dashboardUrl: 'https://sonarqube.example.com/dashboard?id=app',
      analyzedAt: '2026-10-11T02:00:00Z',
    })
  })

  // 분석하지 못한 실행은 지표가 없다. 0 으로 채우면 "문제 0건" 으로 보인다.
  it('지표가 없으면 undefined 로, 0 은 0 으로 남긴다', () => {
    const failed = cicdApiCalls.mapPipelineSASTResult({ ...rawResult, gate_result: 'error', metrics: undefined })
    expect(failed.metrics).toBeUndefined()

    const partial = cicdApiCalls.mapPipelineSASTResult({ ...rawResult, metrics: { bugs: 0, coverage: 'x' } })
    expect(partial.metrics).toEqual({ bugs: 0 })
  })

  // 모르는 판정을 pass 로 떨어뜨리면 초록 통과로 보인다.
  it('모르는 판정은 분석 실패(error)로 둔다', () => {
    expect(cicdApiCalls.mapPipelineSASTResult({ ...rawResult, gate_result: 'maybe' }).gateResult).toBe('error')
  })

  it('빈 선택 필드는 없음으로 둔다', () => {
    const r = cicdApiCalls.mapPipelineSASTResult({
      id: 's1', pipeline_id: 'pip_x', gate_result: 'pass', dashboard_url: '', quality_gate_status: '', conditions: null,
    })
    expect(r.dashboardUrl).toBeUndefined()
    expect(r.qualityGateStatus).toBeUndefined()
    expect(r.conditions).toEqual([])
  })

  it('목록 응답을 옮긴다', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({ data: { items: [rawResult], total: 1 } })

    const got = await cicdApiCalls.getPipelineSASTResults('pip_x')

    expect(api.get).toHaveBeenCalledWith('/cicd/pipelines/pip_x/sast-results')
    expect(got.total).toBe(1)
    expect(got.items[0].gateResult).toBe('block')
  })
})
