import { describe, expect, it } from 'vitest'
import YAML from 'yaml'

import { toCreateStackBody } from '../api/stack-normalizers'
import { TOOL_VERSION_CATALOG, useStackConfigStore } from '../stores/stack-config-store'
import type { StackTemplate } from '../types'
import { MATRIX_CATEGORY_BY_SLOT, SECURITY_OPTIONS, SLOT_TOOL_BINDING, TOOL_HELM_META } from './install-constants'
import {
  GATEWAY_BACKENDS,
  buildGatewayManifest,
  buildHelmStepResourceOverride,
  getInstallType,
} from './install-manifest-builders'
import { PLANNING_OPTION_DEFS } from './install-planning-utils'
import { TOOL_CATEGORY_LOOKUP } from './template-config'
import { buildInstallOverridesFromTemplate } from './template-overrides'
import { sastStorageConflict } from './security-tools'

// 소스 정적 분석(SAST)은 이미지 스캐너와 같은 선택 슬롯이다. 백엔드는 슬롯과 설치
// 단계를 이미 갖고 있어, 화면에 고를 자리가 없으면 템플릿 밖에서는 도달할 수 없다.
describe('security.sast 슬롯', () => {
  // Go 쪽(internal/stack/domain/planning.go PlanningOptionDefs)과 같은 값이어야 한다 —
  // 다르면 화면 설치와 API 설치가 서로 다른 크기로 깔린다.
  it('계획 옵션이 백엔드와 같다', () => {
    expect(PLANNING_OPTION_DEFS['security.sast']).toEqual([
      expect.objectContaining({ key: 'scansPerDay', baseline: 40, min: 1, max: 2000, weight: 0.6, impact: { cpu: 1, memory: 0.5, storage: 0.2 } }),
      expect.objectContaining({ key: 'projectCount', baseline: 20, min: 1, max: 1000, weight: 0.4, impact: { cpu: 0.3, memory: 0.8, storage: 0.6 } }),
    ])
  })

  it('draft 의 security.sast 에 묶인다', () => {
    expect(SLOT_TOOL_BINDING['security.sast']).toEqual({ section: 'security', field: 'sast' })
  })

  it('매트릭스 카테고리가 sast 다', () => {
    expect(MATRIX_CATEGORY_BY_SLOT['security.sast']).toBe('sast')
  })

  it('고를 수 있는 도구가 SonarQube 다', () => {
    expect(SECURITY_OPTIONS.sast.map((o) => o.id)).toEqual(['sonarqube'])
  })
})

describe('스토어', () => {
  it('setTool 로 SonarQube 를 고르면 draft 에 남는다', () => {
    useStackConfigStore.getState().resetConfig()
    useStackConfigStore.getState().setTool('security', 'sast', { tool: 'sonarqube', version: '26.9.0.129388' })
    expect(useStackConfigStore.getState().draft.security.sast.tool).toBe('sonarqube')
  })

  it('기본값은 고르지 않음이다 — 정적 분석은 선택이다', () => {
    useStackConfigStore.getState().resetConfig()
    expect(useStackConfigStore.getState().draft.security.sast.tool).toBe('')
  })
})

describe('배포 요청 본문', () => {
  const baseRequest = {
    templateId: 'gitlab-argocd-sonarqube-v1',
    clusterId: 'cluster-1',
    stackName: 'sast-stack',
    namespace: 'nullus',
    artifacts: {
      packageRegistry: { tool: '', version: '' },
      sourceRepository: { tool: 'gitlab', version: 'v17.7.0' },
      containerRegistry: { tool: 'gitlab-registry', version: 'v17.7.0' },
      storageBackend: { tool: 'minio', version: 'latest' },
    },
    pipeline: {
      cicdPlatform: { tool: 'gitlab-ci', version: 'v17.7.0' },
      cdTool: { tool: 'argocd', version: 'v2.13.3' },
    },
    monitoring: {
      collection: { tool: 'prometheus', version: 'v3.1.0' },
      visualization: { tool: 'grafana', version: '11.5.1' },
    },
    logging: {
      collection: { tool: '', version: '' },
      search: { tool: '', version: '' },
      traceLayer: { tool: '', version: '' },
      traceExporter: { tool: '', version: '' },
    },
    resources: { developerCount: 10, concurrentRunners: 2, commitsPerDay: 50, buildFrequency: 'medium' as const },
  }

  it('고른 SonarQube 를 config.security.sast 로 보낸다', () => {
    const body = toCreateStackBody({
      ...baseRequest,
      security: {
        imageScanner: { tool: 'trivy', version: '0.74.0' },
        sast: { tool: 'sonarqube', version: '26.9.0.129388' },
      },
    } as Parameters<typeof toCreateStackBody>[0])

    expect(body.config.security).toEqual({
      image_scanner: { name: 'trivy', version: '0.74.0', enabled: true },
      sast: { name: 'sonarqube', version: '26.9.0.129388', enabled: true },
    })
  })

  // enabled=true 로 보내면 설치 술어가 켜져 아무도 고르지 않은 SonarQube 가 뜬다.
  it('고르지 않으면 enabled 가 false 다', () => {
    const body = toCreateStackBody(baseRequest as Parameters<typeof toCreateStackBody>[0])
    expect(body.config.security.sast.enabled).toBe(false)
  })
})

describe('템플릿', () => {
  // gitlab-argocd-sonarqube-v1 처럼 SonarQube 를 담은 템플릿을 고르면 마법사가 그대로 채운다.
  it('템플릿의 sast 도구가 security.sast 로 들어온다', () => {
    const overrides = buildInstallOverridesFromTemplate({
      id: 'gitlab-argocd-sonarqube-v1',
      name: 'GitLab + Argo CD + SonarQube',
      toolDetails: [
        { category: 'source_repository', name: 'GitLab CE', app_version: 'v17.7.0' },
        { category: 'sast', name: 'SonarQube', app_version: '26.9.0.129388' },
      ],
    } as unknown as StackTemplate)

    expect(overrides.security?.sast).toEqual({ tool: 'sonarqube', version: '26.9.0.129388' })
  })

  it('템플릿 편집기에서 SAST 칸을 고를 수 있다', () => {
    expect(TOOL_CATEGORY_LOOKUP.get('sast')?.options).toEqual(['SonarQube'])
  })
})

describe('도구 정보', () => {
  // 템플릿 편집기의 기본 버전과 화면 표시가 이 표를 본다. 없으면 '1.0.0' 으로 나온다.
  it('버전 표에 SonarQube 가 있다', () => {
    expect(TOOL_VERSION_CATALOG.sonarqube).toEqual({ appVersion: '26.9.0.129388', chartVersion: '2026.5.1002' })
  })

  // 매니페스트 미리보기가 라우트 백엔드를 이 표에서 찾는다. 없으면 sonarqube-svc 를
  // 가리키는 라우트를 보여 준다(서버가 바로잡지만 미리보기가 실제와 다르다).
  it('게이트웨이 백엔드가 서버와 같다', () => {
    expect(GATEWAY_BACKENDS.sonarqube).toEqual({ serviceName: 'sonarqube', port: 9000 })
  })

  it('차트 정보가 설치 경로와 같다', () => {
    expect(TOOL_HELM_META.sonarqube).toEqual({
      repoUrl: 'https://SonarSource.github.io/helm-chart-sonarqube',
      chartName: 'sonarqube/sonarqube',
    })
  })
})

describe('설치 계획', () => {
  const vector = { cpuRequest: 0.5, cpuLimit: 2, memoryRequestGi: 3, memoryLimitGi: 4, storageRequestGi: 10, storageLimitGi: 20 }

  // 계획한 자원이 installing_sonarqube 단계의 values 로 간다. 키가 틀리면 계획이 버려진다.
  it('SonarQube 는 helm 으로 깔리고 자원 계획이 installing_sonarqube 로 간다', () => {
    expect(getInstallType('sonarqube')).toBe('helm')
    const override = buildHelmStepResourceOverride('sonarqube', vector)
    expect(override?.key).toBe('installing_sonarqube')
    expect(override?.values).toHaveProperty('resources')
  })

  // yaml 도구로 보면 화면이 만든 Deployment 매니페스트가 그대로 Trivy 차트의 values 로 들어간다.
  it('Trivy 도 helm 으로 깔린다', () => {
    expect(getInstallType('trivy')).toBe('helm')
    expect(buildHelmStepResourceOverride('trivy', vector)?.key).toBe('installing_trivy')
  })

  // 화면이 만든 게이트웨이가 서버 기본값을 대신한다. 여기 라우트가 없으면 sonarqube.<도메인> 이
  // 열리지 않고, 그 주소가 ACS 라 Keycloak 로그인도 실패한다.
  it('게이트웨이에 SonarQube 라우트를 만들고 Trivy 는 열지 않는다', () => {
    useStackConfigStore.getState().resetConfig()
    const draft = { ...useStackConfigStore.getState().draft, stackName: 'sast-stack', accessDomain: 'nullus.local' }
    const entry = (toolId: string) => ({
      toolId,
      toolLabel: toolId,
      installType: 'helm' as const,
      toolVersion: '1',
      hasVersionConflict: false,
      roles: [],
      sourceToolIds: [toolId],
      sourceVersions: ['1'],
    })
    const manifest = buildGatewayManifest(draft, [entry('sonarqube'), entry('trivy')])

    const docs = YAML.parseAllDocuments(manifest).map((d) => d.toJSON())
    const route = docs.find((doc) => doc?.kind === 'HTTPRoute' && JSON.stringify(doc).includes('sonarqube.nullus.local'))
    expect(route).toBeTruthy()
    expect(JSON.stringify(route)).toContain('"name":"sonarqube","port":9000')
    expect(manifest).not.toContain('trivy.nullus.local')
  })
})

describe('외부 DB', () => {
  const draftWith = (planMode: string, dbMode: string, sast: string) => {
    useStackConfigStore.getState().resetConfig()
    const draft = useStackConfigStore.getState().draft
    return {
      ...draft,
      storage: { ...draft.storage, planMode, database: { ...draft.storage.database, mode: dbMode } },
      security: { ...draft.security, sast: { tool: sast, version: sast ? '26.9.0.129388' : '' } },
    } as typeof draft
  }

  // 백엔드는 installing_sonarqube 에서야 이 조합을 거부해, 앞 단계를 다 깐 뒤에 실패한다.
  it('DB 를 외부 연결로 고르면 SonarQube 를 막는다', () => {
    expect(sastStorageConflict(draftWith('existing-all', 'existing', 'sonarqube'))).toMatch(/PostgreSQL/)
  })

  it('DB 를 새로 만들거나 SonarQube 를 고르지 않으면 막지 않는다', () => {
    expect(sastStorageConflict(draftWith('integrated-create', 'create', 'sonarqube'))).toBeNull()
    expect(sastStorageConflict(draftWith('existing-all', 'existing', ''))).toBeNull()
    // 저장소 계획을 쓰지 않으면 스택이 PostgreSQL 을 함께 깐다.
    expect(sastStorageConflict(draftWith('none', 'existing', 'sonarqube'))).toBeNull()
  })
})

describe('도구 설명', () => {
  // 설명은 i18n 키에서 찾고, 없으면 옵션의 한국어 설명으로 떨어진다 — 영어 화면에도 한국어가 나온다.
  it('en·ko 에 Trivy·SonarQube 설명이 있다', async () => {
    const en = (await import('../../../i18n/en.json')).default as Record<string, any>
    const ko = (await import('../../../i18n/ko.json')).default as Record<string, any>
    for (const dict of [en, ko]) {
      for (const tool of ['trivy', 'sonarqube']) {
        expect(dict.stackAddTools.tools[tool]?.description).toBeTruthy()
      }
    }
  })
})
