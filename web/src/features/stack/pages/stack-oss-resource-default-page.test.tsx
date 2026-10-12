import { describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'

import { renderWithProviders } from '../../../__tests__/test-utils'
import type { StackResourceDefault } from '../../../types'
import { StackOssResourceDefaultPage } from './stack-oss-resource-default-page'

const row = (tool_key: string, display_name: string): StackResourceDefault => ({
  tool_key,
  display_name,
  cpu_request: 0.5,
  cpu_limit: 2,
  memory_request_gi: 3,
  memory_limit_gi: 4,
  storage_request_gi: 10,
  storage_limit_gi: 20,
  is_default: true,
  updated_at: '2026-10-10T00:00:00Z',
})

vi.mock('../api/stack-api', () => ({
  useResourceDefaults: () => ({
    data: { items: [row('trivy', 'Trivy'), row('sonarqube', 'SonarQube'), row('harbor', 'Harbor')] },
    isLoading: false,
  }),
  useUpsertResourceDefault: () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false }),
}))

// 보안 도구의 자원 기본값도 시드돼 있다. 분류표에 없으면 기본값인 Artifacts 로 보여
// 저장소 도구처럼 읽힌다.
describe('StackOssResourceDefaultPage', () => {
  it('보안 도구를 Security 로 분류한다', () => {
    renderWithProviders(<StackOssResourceDefaultPage />)

    for (const name of ['Trivy', 'SonarQube']) {
      const tr = screen.getByDisplayValue(name).closest('tr') as HTMLElement
      expect(within(tr).getByText('Security')).toBeInTheDocument()
    }
    const harbor = screen.getByDisplayValue('Harbor').closest('tr') as HTMLElement
    expect(within(harbor).getByText('Artifacts')).toBeInTheDocument()
  })
})
