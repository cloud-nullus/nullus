import { describe, expect, it } from 'vitest'
import type { StackTemplate } from '../api/stack-api'
import type { TemplateToolDetail } from '../../../types'
import { buildInstallOverridesFromTemplate } from './template-overrides'

const templateWith = (toolDetails: TemplateToolDetail[]) =>
  ({ id: 'custom-harbor-trivy', name: 'GitLab + Harbor + Trivy', tools: [], toolDetails }) as unknown as StackTemplate

describe('buildInstallOverridesFromTemplate', () => {
  // 템플릿에 스캐너를 넣어도 설치 마법사가 그 카테고리를 몰라 버렸다. 템플릿으로
  // 시작한 설치에는 스캐너가 빠지고, 만든 사람은 그것을 알 길이 없었다.
  it('should select the image scanner when the template includes Trivy', () => {
    const overrides = buildInstallOverridesFromTemplate(
      templateWith([
        { category: 'container_registry', name: 'Harbor', helm_version: '1.15.0', app_version: '2.11.0' },
        { category: 'image_scanner', name: 'Trivy', helm_version: '0.26.0', app_version: '0.74.0' },
      ])
    )

    expect(overrides.security?.imageScanner).toEqual({ tool: 'trivy', version: '0.74.0' })
  })

  // 다른 섹션처럼 템플릿이 고르지 않은 스캐너는 비운다. 앞서 고른 값이 남으면
  // 템플릿에 없는 도구가 설치된다.
  it('should clear the image scanner when the template has none', () => {
    const overrides = buildInstallOverridesFromTemplate(
      templateWith([{ category: 'container_registry', name: 'Harbor', helm_version: '1.15.0', app_version: '2.11.0' }])
    )

    expect(overrides.security?.imageScanner).toEqual({ tool: '', version: '' })
  })
})
