import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/react'
import type { VulnerabilityListResult } from '../../../types'
import { DEFAULT_VULNERABILITY_FILTER } from '../../../lib/vulnerability-list'
import { PipelineScanVulnerabilities } from './pipeline-scan-vulnerabilities'

const mockUsePipelineScanVulnerabilities = vi.fn()

vi.mock('../api/cicd-api', () => ({
  usePipelineScanVulnerabilities: (...args: unknown[]) => mockUsePipelineScanVulnerabilities(...args),
}))

const list: VulnerabilityListResult = {
  status: 'available',
  reason: '',
  targets: [{ target: 'alpine 3.19.0 (alpine 3.19.0)', class: 'os', total: 1 }],
  items: [
    {
      id: 'CVE-2024-6119',
      pkg: 'libcrypto3',
      installed: '3.1.4-r2',
      fixed: '3.1.7-r0',
      severity: 'critical',
      class: 'os',
      target: 'alpine 3.19.0 (alpine 3.19.0)',
      url: 'https://avd.aquasec.com/nvd/cve-2024-6119',
    },
  ],
  total: 1,
  limit: 50,
  offset: 0,
}

function lastCall() {
  const calls = mockUsePipelineScanVulnerabilities.mock.calls
  return calls[calls.length - 1]
}

describe('PipelineScanVulnerabilities', () => {
  beforeEach(() => {
    mockUsePipelineScanVulnerabilities.mockReset()
    mockUsePipelineScanVulnerabilities.mockReturnValue({
      data: list,
      isLoading: false,
      isError: false,
      isFetching: false,
    })
  })

  // 목록은 CI 리포트를 읽어 만든다. 펼치지 않은 실행마다 CI 를 부르지 않는다.
  it('펼치기 전에는 조회하지 않고 목록을 그리지 않는다', () => {
    render(<PipelineScanVulnerabilities pipelineId="pip_x" scanId="scan_dep_ci_pip_x_2" />)

    expect(lastCall()).toEqual(['pip_x', 'scan_dep_ci_pip_x_2', DEFAULT_VULNERABILITY_FILTER, false])
    const toggle = screen.getByRole('button', { name: 'View vulnerability list' })
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByRole('table')).toBeNull()
  })

  it('펼치면 그 스캔의 목록을 조회해 보여준다', () => {
    render(<PipelineScanVulnerabilities pipelineId="pip_x" scanId="scan_dep_ci_pip_x_2" />)

    fireEvent.click(screen.getByRole('button', { name: 'View vulnerability list' }))

    expect(lastCall()).toEqual(['pip_x', 'scan_dep_ci_pip_x_2', DEFAULT_VULNERABILITY_FILTER, true])
    const toggle = screen.getByRole('button', { name: 'Hide vulnerability list' })
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(within(screen.getByRole('table')).getByText('libcrypto3')).toBeTruthy()
  })

  it('필터를 바꾸면 바뀐 필터로 다시 조회한다', () => {
    render(<PipelineScanVulnerabilities pipelineId="pip_x" scanId="scan_dep_ci_pip_x_2" />)
    fireEvent.click(screen.getByRole('button', { name: 'View vulnerability list' }))

    fireEvent.click(within(screen.getByRole('group', { name: 'Severity' })).getByRole('button', { name: 'Critical' }))

    expect(lastCall()).toEqual([
      'pip_x',
      'scan_dep_ci_pip_x_2',
      { ...DEFAULT_VULNERABILITY_FILTER, severities: ['critical'] },
      true,
    ])
  })
})
