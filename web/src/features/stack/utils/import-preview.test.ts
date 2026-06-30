import { describe, expect, it } from 'vitest'
import { summarizeImportPreview } from './import-preview'

describe('summarizeImportPreview', () => {
  it('formats create mode clearly', () => {
    expect(summarizeImportPreview({ mode: 'create', name: 'demo' })).toEqual([
      '현재 같은 이름의 스택이 없어 새 스택으로 생성됩니다.',
    ])
  })

  it('formats per-OSS resource changes clearly', () => {
    const lines = summarizeImportPreview({
      mode: 'update',
      name: 'demo',
      existing_state: 'completed',
      changes: {
        added: {},
        removed: {},
        changed: {
          'config.applied_resource_overrides.artifacts.packageRegistry:gitlab.cpuRequest': [1.5, 2],
          'config.option_overrides.pipeline.cdTool.deploymentsPerDay': [40, 60],
        },
      },
    })

    expect(lines[0]).toBe('기존 스택이 업데이트됩니다 (현재 상태: 완료).')
    expect(lines[1]).toBe('GitLab Package Registry CPU 요청: 1.5 -> 2')
    expect(lines[2]).toBe('Argo CD 일일 배포 횟수: 40 -> 60')
  })
})
