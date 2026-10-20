import { describe, expect, it } from 'vitest'
import {
  buildScopeOptions,
  daysSince,
  groupAppsByStack,
  hiddenAppCount,
  isGroupOpenByDefault,
  isProblemRank,
  normalizeSecurityAppsReport,
  normalizeSecurityCell,
  normalizeStackSecurityReport,
  runHistoryHref,
  safeExternalHref,
  securityAppsParams,
  shortImageRef,
  stackImageScansHref,
} from './security-posture'

const rawApp = (pipelineId: string, stackId: string, rank: number) => ({
  pipeline_id: pipelineId,
  name: pipelineId,
  stack_id: stackId,
  rank,
  latest_run: null,
  cells: {
    secret_scan: { state: 'pass' },
    dependency_scan: { state: 'pass' },
    sast: { state: 'pass' },
    image_scan: { state: 'pass' },
  },
})

describe('normalizeSecurityCell', () => {
  it('should keep missing numbers unknown instead of zero', () => {
    const cell = normalizeSecurityCell({ state: 'block', findings: null })
    expect(cell.findings).toBeUndefined()
    expect(cell.counts).toBeUndefined()
    expect(cell.targets).toBeUndefined()
    expect(cell.vulnerabilities).toBeUndefined()
  })

  it('should keep a real zero as zero', () => {
    const cell = normalizeSecurityCell({ state: 'pass', targets: 0, findings: 0 })
    expect(cell.targets).toBe(0)
    expect(cell.findings).toBe(0)
  })

  // 모르는 상태를 pass 로 떨어뜨리면 초록 통과로 보인다.
  it('should treat an unknown or missing state as error', () => {
    expect(normalizeSecurityCell({ state: 'maybe' }).state).toBe('error')
    expect(normalizeSecurityCell(undefined).state).toBe('error')
  })

  it('should map every field of the contract', () => {
    expect(
      normalizeSecurityCell({
        state: 'warn',
        deployment_id: 'dep_41',
        version: '#41',
        at: '2026-10-20T07:00:00Z',
        stale_run: true,
        vulnerabilities: 4,
        security_hotspots: 2,
        failed_conditions: ['new_violations'],
        link: 'https://sonar.example/dashboard?id=x',
      }),
    ).toMatchObject({
      state: 'warn',
      deploymentId: 'dep_41',
      version: '#41',
      staleRun: true,
      vulnerabilities: 4,
      securityHotspots: 2,
      failedConditions: ['new_violations'],
      link: 'https://sonar.example/dashboard?id=x',
    })
  })
})

describe('normalizeSecurityAppsReport', () => {
  it('should fill in all four cells even when the server omits one', () => {
    const report = normalizeSecurityAppsReport({
      summary: { apps: 1 },
      apps: [{ pipeline_id: 'p1', name: 'a', stack_id: '', rank: 3, latest_run: null, cells: { sast: { state: 'pass' } } }],
    })
    expect(Object.keys(report.apps[0].cells).sort()).toEqual(['dependency_scan', 'image_scan', 'sast', 'secret_scan'])
    expect(report.apps[0].cells.image_scan.state).toBe('error')
    expect(report.summary.blocked).toBeUndefined()
    expect(report.truncated).toBe(false)
  })

  it('should read the truncated flag and coverage', () => {
    const report = normalizeSecurityAppsReport({
      truncated: true,
      coverage: { secret_scan: 20, sast: 12 },
      groups: [],
      apps: [],
    })
    expect(report.truncated).toBe(true)
    expect(report.coverage).toEqual({ secret_scan: 20, sast: 12 })
  })
})

describe('normalizeStackSecurityReport', () => {
  it('should keep image counts of an unscanned stack unknown', () => {
    const report = normalizeStackSecurityReport({
      summary: { stacks: 1, not_scanned_stacks: 1 },
      stacks: [{ stack_id: 's1', stack_name: 'dev', state: 'not_scanned', reason: 'scanner_not_installed', worst_image: null }],
    })
    expect(report.stacks[0]).toMatchObject({ state: 'not_scanned', reason: 'scanner_not_installed', worstImage: null })
    expect(report.stacks[0].images).toBeUndefined()
    expect(report.stacks[0].gradeAImages).toBeUndefined()
    expect(report.summary.actionImages).toBeUndefined()
  })
})

describe('groupAppsByStack', () => {
  it('should keep the server order of groups and of apps inside each group', () => {
    const report = normalizeSecurityAppsReport({
      groups: [
        { stack_id: 'stk_b', stack_name: 'b', problems: 1 },
        { stack_id: '', stack_name: '', problems: 0 },
        { stack_id: 'stk_a', stack_name: 'a', problems: 2 },
      ],
      apps: [rawApp('a1', 'stk_a', 1), rawApp('b1', 'stk_b', 2), rawApp('a2', 'stk_a', 4), rawApp('n1', '', 6)],
    })
    const sections = groupAppsByStack(report)
    expect(sections.map((s) => s.group.stackId)).toEqual(['stk_b', '', 'stk_a'])
    expect(sections[2].apps.map((a) => a.pipelineId)).toEqual(['a1', 'a2'])
    expect(sections[1].apps.map((a) => a.pipelineId)).toEqual(['n1'])
  })

  // 묶음 머리가 빠졌다고 앱이 화면에서 사라지면 안 된다.
  it('should not drop apps whose stack has no group', () => {
    const report = normalizeSecurityAppsReport({ groups: [], apps: [rawApp('x', 'stk_x', 2)] })
    const sections = groupAppsByStack(report)
    expect(sections).toHaveLength(1)
    expect(sections[0].apps[0].pipelineId).toBe('x')
  })
})

describe('group defaults and hidden count', () => {
  it('should open groups with problems or unknown problems and collapse the rest', () => {
    const base = { stackId: 's', stackName: 's', clusterId: '', clusterName: '' }
    expect(isGroupOpenByDefault({ ...base, problems: 3 })).toBe(true)
    expect(isGroupOpenByDefault({ ...base, problems: undefined })).toBe(true)
    expect(isGroupOpenByDefault({ ...base, problems: 0 })).toBe(false)
  })

  it('should count apps hidden by problems only', () => {
    const report = { summary: { apps: 24 }, apps: new Array(8).fill(null) } as never
    expect(hiddenAppCount(report, true)).toBe(16)
    expect(hiddenAppCount(report, false)).toBe(0)
    expect(hiddenAppCount({ summary: {}, apps: [] } as never, true)).toBeUndefined()
  })

  it('should treat rank 1 to 4 as problems', () => {
    expect([1, 2, 3, 4, 5, 6].map(isProblemRank)).toEqual([true, true, true, true, false, false])
  })
})

describe('buildScopeOptions', () => {
  it('should build cluster and stack options from groups and stacks without duplicates', () => {
    const options = buildScopeOptions(
      [
        { stackId: 'stk_p', stackName: 'stack-prod', clusterId: 'c1', clusterName: 'prod-a' },
        { stackId: '', stackName: '', clusterId: '', clusterName: '' },
      ],
      normalizeStackSecurityReport({
        stacks: [
          { stack_id: 'stk_p', stack_name: 'stack-prod', cluster_id: 'c1', cluster_name: 'prod-a' },
          { stack_id: 'stk_d', stack_name: 'stack-dev', cluster_id: 'c2', cluster_name: 'dev-a' },
        ],
      }).stacks,
    )
    expect(options.clusters.map((c) => c.name)).toEqual(['dev-a', 'prod-a'])
    expect(options.stacks.map((s) => [s.id, s.clusterId])).toEqual([
      ['stk_d', 'c2'],
      ['stk_p', 'c1'],
    ])
    expect(options.hasNoStack).toBe(true)
  })
})

describe('links and params', () => {
  it('should build the CI/CD run history deep link', () => {
    expect(runHistoryHref('pip_1', 'dep_42')).toBe('/cicd/list?pipeline=pip_1&tab=history&run=dep_42')
    expect(runHistoryHref('pip_1')).toBe('/cicd/list?pipeline=pip_1&tab=history')
  })

  it('should build the stack image scans deep link', () => {
    expect(stackImageScansHref('stk_1')).toBe('/stack/list?stack=stk_1&tab=image-scans')
  })

  it('should only send non-empty scope params', () => {
    expect(securityAppsParams({ problemOnly: true })).toEqual({ problem_only: 'true' })
    expect(securityAppsParams({ clusterId: 'c1', stackId: 'none', q: '  order ', problemOnly: false })).toEqual({
      cluster_id: 'c1',
      stack_id: 'none',
      q: 'order',
    })
  })

  it('should refuse non-http links', () => {
    expect(safeExternalHref('javascript:alert(1)')).toBeUndefined()
    expect(safeExternalHref('https://ci.example/report.json')).toBe('https://ci.example/report.json')
  })
})

describe('formatting helpers', () => {
  it('should count whole days since a date', () => {
    const now = new Date('2026-10-20T00:00:00Z').getTime()
    expect(daysSince('2026-09-16T00:00:00Z', now)).toBe(34)
    expect(daysSince(undefined, now)).toBeUndefined()
    expect(daysSince('not-a-date', now)).toBeUndefined()
  })

  it('should shorten an image reference to its last path segment', () => {
    expect(shortImageRef('registry.example/team/order-api@sha256:abcdef0123456789abcdef')).toBe(
      'order-api@sha256:abcdef012345…',
    )
    expect(shortImageRef('registry.gitlab.com/gitlab-org/build/cng/gitlab-toolbox-ee:v17.7.0')).toBe(
      'gitlab-toolbox-ee:v17.7.0',
    )
  })
})
