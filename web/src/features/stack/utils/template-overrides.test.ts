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

  // toolDetails 없이 tools 문자열 배열만 있는 템플릿(Lite 등)도 도구명으로 카테고리를 찾아
  // 마법사에 온전히 반영한다 (#170). 없으면 카테고리를 몰라 모든 슬롯이 비어 0개가 된다.
  it('should resolve tool categories from legacy tool name array', () => {
    const overrides = buildInstallOverridesFromTemplate({
      id: 'gitea-jenkins-argocd-lite-v1',
      name: 'Lite Template',
      tools: ['Gitea', 'Jenkins', 'Harbor', 'Argo CD'],
    } as unknown as StackTemplate)

    expect(overrides.artifacts?.sourceRepository.tool).toBe('gitea')
    expect(overrides.pipeline?.cicdPlatform.tool).toBe('jenkins')
    expect(overrides.artifacts?.containerRegistry.tool).toBe('harbor')
    expect(overrides.pipeline?.cdTool.tool).toBe('argocd')
  })

  // toolDetails 에 category 가 비어있는 경우에도 도구명으로 역추론하여 복구한다.
  it('should fallback to resolving category when toolDetails category is empty', () => {
    const overrides = buildInstallOverridesFromTemplate(
      templateWith([
        { category: '', name: 'Gitea', helm_version: '12.7.0', app_version: '1.27.0' },
        { category: '', name: 'Jenkins', helm_version: '5.9.54', app_version: '2.568.2' },
      ])
    )

    expect(overrides.artifacts?.sourceRepository.tool).toBe('gitea')
    expect(overrides.pipeline?.cicdPlatform.tool).toBe('jenkins')
  })
})
