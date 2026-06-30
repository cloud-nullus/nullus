import { describe, expect, it } from 'vitest'
import { toCreateStackBody } from './stack-normalizers'

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
