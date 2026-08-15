import { describe, expect, it } from 'vitest'
import { normalizeTemplate, toCreateStackBody } from './stack-normalizers'

describe('toCreateStackBody', () => {
  it('includes per-OSS resource override fields', () => {
    const payload = toCreateStackBody({
      templateId: 'gitlab-argocd-v1',
      clusterId: 'cluster-1',
      stackName: 'nullus-devsecops-stack',
      namespace: 'nullus',
      artifacts: {
        packageRegistry: { tool: 'gitlab', version: '18.5.1' },
        sourceRepository: { tool: 'gitlab', version: '18.5.1' },
        containerRegistry: { tool: 'gitlab-registry', version: '18.5.1' },
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
        search: { tool: 'opensearch', version: '2.18.0' },
        traceLayer: { tool: 'tempo', version: '2.7.1' },
        traceExporter: { tool: '', version: '' },
      },
      resources: {
        developerCount: 10,
        concurrentRunners: 5,
        commitsPerDay: 50,
        buildFrequency: 'medium',
        currency: 'KRW',
        mode: 'auto',
        cpuRequest: '4',
        memoryRequest: '8Gi',
        storageRequest: '100Gi',
      },
      appliedResourceOverrides: {
        'artifacts.packageRegistry:gitlab': {
          cpuRequest: 1.5,
          cpuLimit: 2.5,
          memoryRequestGi: 3,
          memoryLimitGi: 4,
          storageRequestGi: 10,
          storageLimitGi: 20,
        },
      },
      rowUnits: {
        'artifacts.packageRegistry:gitlab': { memory: 'Gi', storage: 'Gi' },
      },
      optionOverrides: {
        'artifacts.packageRegistry': { registryCallsPerDay: 3000 },
      },
    })

    expect(payload.config.applied_resource_overrides['artifacts.packageRegistry:gitlab'].cpuRequest).toBe(1.5)
    expect(payload.config.row_units['artifacts.packageRegistry:gitlab'].memory).toBe('Gi')
    expect(payload.config.option_overrides['artifacts.packageRegistry'].registryCallsPerDay).toBe(3000)
  })
})

describe('normalizeTemplate', () => {
  const raw = {
    id: 'gitea-jenkins-argocd-lite-v1',
    name: 'Gitea + Jenkins + Argo CD (Lite)',
    description: '8Gi 노드 하나에 올라가는 최소 구성',
    tools: [{ category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.16', app_version: 'v2.13.3' }],
  }

  it('carries the planning profile so the wizard can size the stack', () => {
    expect(normalizeTemplate({ ...raw, planning_profile: 'local' }).planningProfile).toBe('local')
  })

  it('falls back to standard when the server sends no profile', () => {
    // 프로파일 컬럼이 생기기 전에 만들어진 템플릿이 그대로 남아 있다.
    // 빈 값을 흘려보내면 마법사가 프로파일 없이 계획을 세운다.
    expect(normalizeTemplate(raw).planningProfile).toBe('standard')
  })

  it('falls back to standard when the profile is not one we know', () => {
    expect(normalizeTemplate({ ...raw, planning_profile: 'tiny' }).planningProfile).toBe('standard')
  })
})
