import { describe, expect, it } from 'vitest'

import { toCreateStackBody } from '../api/stack-normalizers'
import { useStackConfigStore } from '../stores/stack-config-store'
import type { StackTemplate } from '../types'
import { SECURITY_OPTIONS } from './install-constants'
import { TOOL_CATEGORY_LOOKUP } from './template-config'
import { buildInstallOverridesFromTemplate } from './template-overrides'

// 서드파티 의존성 취약점(SCA, nullus-plan#79). 선택지는 미선택 · Trivy 둘이고, 따로 설치하는 것이
// 없다 — 파이프라인이 스택 Trivy 서버에 붙는다. 백엔드는 이것만 골라도 Trivy 서버를 세운다.
describe('security.sca 슬롯', () => {
  it('고를 수 있는 도구가 Trivy 다', () => {
    expect(SECURITY_OPTIONS.sca.map((o) => o.id)).toEqual(['trivy'])
  })

  it('기본값은 고르지 않음이다', () => {
    useStackConfigStore.getState().resetConfig()
    expect(useStackConfigStore.getState().draft.security.sca.tool).toBe('')
  })

  it('setTool 로 Trivy 를 고르면 draft 에 남는다', () => {
    useStackConfigStore.getState().resetConfig()
    useStackConfigStore.getState().setTool('security', 'sca', { tool: 'trivy', version: '0.69.3' })
    expect(useStackConfigStore.getState().draft.security.sca).toEqual({ tool: 'trivy', version: '0.69.3' })
  })
})

describe('배포 요청 본문', () => {
  const base = {
    templateId: 'gitlab-argocd-v1',
    clusterId: 'cluster-1',
    stackName: 'sca-stack',
    namespace: 'nullus',
    artifacts: {
      packageRegistry: { tool: '', version: '' },
      sourceRepository: { tool: 'gitlab', version: 'v17.7.0' },
      containerRegistry: { tool: 'gitlab-registry', version: 'v17.7.0' },
      storageBackend: { tool: 'minio', version: 'latest' },
    },
    pipeline: { cicdPlatform: { tool: 'gitlab-ci', version: 'v17.7.0' }, cdTool: { tool: 'argocd', version: 'v2.13.3' } },
    monitoring: { collection: { tool: '', version: '' }, visualization: { tool: '', version: '' } },
    logging: {
      collection: { tool: '', version: '' },
      search: { tool: '', version: '' },
      traceLayer: { tool: '', version: '' },
      traceExporter: { tool: '', version: '' },
    },
    resources: { developerCount: 10, concurrentRunners: 2, commitsPerDay: 50, buildFrequency: 'medium' as const },
  }

  it('고른 Trivy 를 config.security.sca 로 보낸다 — 이미지 스캐너와 별개다', () => {
    const body = toCreateStackBody({
      ...base,
      security: {
        imageScanner: { tool: '', version: '' },
        sast: { tool: '', version: '' },
        sca: { tool: 'trivy', version: '0.69.3' },
      },
    } as Parameters<typeof toCreateStackBody>[0])
    expect(body.config.security.sca).toEqual({ name: 'trivy', version: '0.69.3', enabled: true })
    expect(body.config.security.image_scanner.enabled).toBe(false)
  })

  // enabled=true 로 보내면 아무도 고르지 않은 Trivy 서버가 선다.
  it('고르지 않으면 enabled 가 false 다', () => {
    const body = toCreateStackBody(base as Parameters<typeof toCreateStackBody>[0])
    expect(body.config.security.sca.enabled).toBe(false)
  })
})

describe('템플릿', () => {
  it('템플릿의 sca 도구가 security.sca 로 들어온다', () => {
    const overrides = buildInstallOverridesFromTemplate({
      id: 'custom',
      name: 'Custom',
      toolDetails: [{ category: 'sca', name: 'Trivy', app_version: '0.69.3' }],
    } as unknown as StackTemplate)
    expect(overrides.security?.sca).toEqual({ tool: 'trivy', version: '0.69.3' })
  })

  it('템플릿 편집기에서 SCA 칸을 고를 수 있다', () => {
    expect(TOOL_CATEGORY_LOOKUP.get('sca')?.options).toEqual(['Trivy'])
  })
})
