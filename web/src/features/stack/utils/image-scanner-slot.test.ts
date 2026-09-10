import { describe, expect, it } from 'vitest'

import { toCreateStackBody } from '../api/stack-normalizers'
import { useStackConfigStore } from '../stores/stack-config-store'
import { MATRIX_CATEGORY_BY_SLOT, SECURITY_OPTIONS, SLOT_TOOL_BINDING } from './install-constants'
import { PLANNING_OPTION_DEFS } from './install-planning-utils'

// 이미지 스캐너는 스택 구성의 선택 항목이다. 슬롯을 Go 쪽에만 만들면
// 사용자가 고를 화면이 없어 기능 전체가 도달 불가 상태로 남는다.
describe('security.imageScanner 슬롯', () => {
  it('계획 옵션이 정의돼 있다', () => {
    const defs = PLANNING_OPTION_DEFS['security.imageScanner']
    expect(defs.map((d) => d.key)).toEqual(['scansPerDay', 'concurrentScans'])
  })

  // 슬롯이 draft 의 어느 칸에 붙는지. 빠지면 고른 값이 갈 곳이 없다.
  it('draft 의 security.imageScanner 에 묶인다', () => {
    expect(SLOT_TOOL_BINDING['security.imageScanner']).toEqual({
      section: 'security',
      field: 'imageScanner',
    })
  })

  // 호환성 매트릭스가 검증된 버전을 안내하는 카테고리.
  it('매트릭스 카테고리가 image_scanner 다', () => {
    expect(MATRIX_CATEGORY_BY_SLOT['security.imageScanner']).toBe('image_scanner')
  })

  it('고를 수 있는 도구 목록이 있다', () => {
    expect(SECURITY_OPTIONS.imageScanner.map((o) => o.id)).toContain('trivy')
  })
})

describe('스토어', () => {
  it('setTool 로 스캐너를 고르면 draft 에 남는다', () => {
    useStackConfigStore.getState().resetConfig()
    useStackConfigStore.getState().setTool('security', 'imageScanner', {
      tool: 'trivy',
      version: '0.74.0',
    })

    expect(useStackConfigStore.getState().draft.security.imageScanner.tool).toBe('trivy')
  })

  it('기본값은 고르지 않음이다 — 스캐너는 선택이다', () => {
    useStackConfigStore.getState().resetConfig()
    expect(useStackConfigStore.getState().draft.security.imageScanner.tool).toBe('')
  })
})

describe('배포 요청 본문', () => {
  const baseRequest = {
    templateId: 'gitlab-harbor-v1',
    clusterId: 'cluster-1',
    stackName: 'scan-stack',
    namespace: 'nullus',
    artifacts: {
      packageRegistry: { tool: '', version: '' },
      sourceRepository: { tool: 'gitlab', version: '18.5.1' },
      containerRegistry: { tool: 'harbor', version: '2.15.0' },
      storageBackend: { tool: 'minio', version: 'latest' },
    },
    pipeline: {
      cicdPlatform: { tool: 'gitlab-ci', version: '18.5.1' },
      cdTool: { tool: 'argocd', version: 'v2.8.3' },
    },
    monitoring: {
      collection: { tool: 'prometheus', version: 'v2.54.1' },
      visualization: { tool: 'grafana', version: '11.1.0' },
    },
    logging: {
      collection: { tool: '', version: '' },
      search: { tool: '', version: '' },
      traceLayer: { tool: '', version: '' },
      traceExporter: { tool: '', version: '' },
    },
    resources: {
      developerCount: 10,
      concurrentRunners: 2,
      commitsPerDay: 50,
      buildFrequency: 'medium' as const,
    },
  }

  // 받는 칸이 없으면 마법사가 보낸 선택이 조용히 버려진다.
  // TraceExporter 가 실제로 그랬다.
  it('고른 스캐너를 config.security.image_scanner 로 보낸다', () => {
    const body = toCreateStackBody({
      ...baseRequest,
      security: { imageScanner: { tool: 'trivy', version: '0.74.0' } },
    } as Parameters<typeof toCreateStackBody>[0])

    expect(body.config.security).toEqual({
      image_scanner: { name: 'trivy', version: '0.74.0', enabled: true },
    })
  })

  // 고르지 않았으면 켜지 않는다 — enabled=true 로 보내면 설치 술어가 켜져
  // 아무도 고르지 않은 워크로드가 뜬다.
  it('고르지 않으면 enabled 가 false 다', () => {
    const body = toCreateStackBody(baseRequest as Parameters<typeof toCreateStackBody>[0])

    expect(body.config.security.image_scanner.enabled).toBe(false)
  })
})
