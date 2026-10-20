import { describe, expect, it, vi } from 'vitest'

vi.mock('../../../lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}))

import { api } from '../../../lib/api'
import { cicdApiCalls } from './cicd-api'

// 의존성 스캔 · 시크릿 탐지 결과(Security Dashboard 설계 §8 G1 · G2). 이미지 스캔과 같은 규칙으로
// 옮긴다 — 모르는 숫자는 undefined, 모르는 판정은 error.

const rawDependency = {
  id: 'dep_scan_dep_ci_pip_x_7',
  pipeline_id: 'pip_x',
  deployment_id: 'dep_ci_pip_x_7',
  scanner: 'trivy',
  scanner_version: '0.74.0',
  db_updated_at: '2026-10-19T07:00:00Z',
  db_stale: false,
  counts: { critical: 3, high: 5, medium: 12, low: 0, unknown: 0 },
  targets: 2,
  gate_result: 'block',
  report_uri: 'https://gitlab.example/app/-/jobs/9/artifacts/file/dependency-scan-report.json',
  scanned_at: '2026-10-20T01:00:00Z',
}

const rawSecret = {
  id: 'secret_scan_dep_ci_pip_x_7',
  pipeline_id: 'pip_x',
  deployment_id: 'dep_ci_pip_x_7',
  scanner: 'gitleaks',
  scanner_version: '8.21.0',
  findings_count: 2,
  rules: [
    { rule_id: 'github-pat', count: 1 },
    { rule_id: 'aws-access-key', count: 1 },
  ],
  findings: [
    {
      rule_id: 'github-pat',
      file: 'src/config.ts',
      start_line: 12,
      commit: 'abcdef1234567890',
      fingerprint: 'abcdef1234567890:src/config.ts:github-pat:12',
    },
  ],
  gate_result: 'block',
  report_uri: 'https://gitlab.example/app/-/jobs/8/artifacts/file/gitleaks-report.json',
  scanned_at: '2026-10-20T00:59:00Z',
}

describe('mapPipelineDependencyScan', () => {
  it('should map the response to the screen model', () => {
    expect(cicdApiCalls.mapPipelineDependencyScan(rawDependency)).toEqual({
      id: 'dep_scan_dep_ci_pip_x_7',
      pipelineId: 'pip_x',
      deploymentId: 'dep_ci_pip_x_7',
      scanner: 'trivy',
      scannerVersion: '0.74.0',
      dbUpdatedAt: '2026-10-19T07:00:00Z',
      dbStale: false,
      counts: { critical: 3, high: 5, medium: 12, low: 0, unknown: 0 },
      targets: 2,
      gateResult: 'block',
      reportUri: 'https://gitlab.example/app/-/jobs/9/artifacts/file/dependency-scan-report.json',
      scannedAt: '2026-10-20T01:00:00Z',
    })
  })

  // 건수 · 대상 수를 0 으로 채우면 스캔 오류가 "취약점 0 · 대상 0" 으로 보인다.
  it('should keep missing counts and targets unknown, not zero', () => {
    const got = cicdApiCalls.mapPipelineDependencyScan({
      ...rawDependency,
      gate_result: 'error',
      counts: null,
      targets: undefined,
    })
    expect(got.counts).toBeUndefined()
    expect(got.targets).toBeUndefined()
  })

  it('should treat an unknown gate as error', () => {
    expect(cicdApiCalls.mapPipelineDependencyScan({ ...rawDependency, gate_result: 'maybe' }).gateResult).toBe('error')
  })

  it('should read the list endpoint', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({ data: { items: [rawDependency], total: 1 } })

    const got = await cicdApiCalls.getPipelineDependencyScans('pip_x')

    expect(api.get).toHaveBeenCalledWith('/cicd/pipelines/pip_x/dependency-scans')
    expect(got.total).toBe(1)
    expect(got.items[0].targets).toBe(2)
  })
})

describe('mapPipelineSecretScan', () => {
  it('should map rules and finding locations', () => {
    const got = cicdApiCalls.mapPipelineSecretScan(rawSecret)
    expect(got.findingsCount).toBe(2)
    expect(got.rules).toEqual([
      { ruleId: 'github-pat', count: 1 },
      { ruleId: 'aws-access-key', count: 1 },
    ])
    expect(got.findings).toEqual([
      {
        ruleId: 'github-pat',
        file: 'src/config.ts',
        startLine: 12,
        commit: 'abcdef1234567890',
        fingerprint: 'abcdef1234567890:src/config.ts:github-pat:12',
      },
    ])
    expect(got.gateResult).toBe('block')
  })

  // API 는 비밀값을 보내지 않는다. 그래도 섞여 오면 화면 모델에 들어오지 않아야 한다 —
  // 화면 어디에서도 그릴 길이 없게.
  it('should never carry a secret value into the screen model', () => {
    const leaked = {
      ...rawSecret,
      secret: 'ghp_TOPLEVELLEAK',
      findings: [{ ...rawSecret.findings[0], secret: 'ghp_SUPERSECRETVALUE', match: 'token = "ghp_SUPERSECRETVALUE"' }],
    }
    const serialized = JSON.stringify(cicdApiCalls.mapPipelineSecretScan(leaked))
    expect(serialized).not.toContain('ghp_SUPERSECRETVALUE')
    expect(serialized).not.toContain('ghp_TOPLEVELLEAK')
  })

  it('should keep a missing findings count unknown', () => {
    const got = cicdApiCalls.mapPipelineSecretScan({ ...rawSecret, findings_count: undefined, rules: null, findings: null })
    expect(got.findingsCount).toBeUndefined()
    expect(got.rules).toEqual([])
    expect(got.findings).toEqual([])
  })

  it('should read the list endpoint', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({ data: { items: [rawSecret], total: 1 } })

    const got = await cicdApiCalls.getPipelineSecretScans('pip_x')

    expect(api.get).toHaveBeenCalledWith('/cicd/pipelines/pip_x/secret-scans')
    expect(got.items[0].rules).toHaveLength(2)
  })
})
