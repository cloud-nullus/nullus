import { describe, expect, it } from 'vitest'
import { toCreateStackBody } from './stack-normalizers'
import type { CreateStackRequest } from '../../../types'

// 설치 마법사가 고른 리소스 계획이 생성 요청에 실려야 한다.
//
// 실제 DB 의 스택 8개 전부 config.applied_resource_overrides 가 비어 있었다.
// Go 바인딩은 정상이고(같은 JSON 을 언마샬해 확인), 배선도 이제 붙었으므로
// 남는 의심 지점이 이 변환 함수였다.
function request(overrides: Partial<CreateStackRequest> = {}): CreateStackRequest {
  return {
    stackName: 'demo',
    clusterId: 'c1',
    namespace: 'nullus-demo',
    templateId: 't1',
    artifacts: {
      sourceRepository: { tool: 'gitlab', version: '9.5.1' },
      containerRegistry: { tool: 'gitlab-registry', version: '9.5.1' },
      packageRegistry: { tool: 'gitlab-package-registry', version: '9.5.1' },
      storageBackend: { tool: 'minio', version: '5.2.0' },
    },
    pipeline: {
      cicdPlatform: { tool: 'gitlab-ci', version: '9.5.1' },
      cdTool: { tool: 'argocd', version: '6.8.0' },
    },
    monitoring: {
      collection: { tool: 'prometheus', version: '67.0.0' },
      visualization: { tool: 'grafana', version: '8.5.0' },
    },
    logging: {},
    resources: {
      developerCount: 10,
      concurrentRunners: 2,
      commitsPerDay: 50,
      buildFrequency: 'daily',
    },
    ...overrides,
  } as CreateStackRequest
}

describe('toCreateStackBody — 리소스 계획', () => {
  it('계획값을 config.applied_resource_overrides 로 싣는다', () => {
    const body = toCreateStackBody(
      request({
        appliedResourceOverrides: {
          'pipeline.cdTool:argocd': {
            cpuRequest: 2,
            cpuLimit: 4,
            memoryRequestGi: 3,
            memoryLimitGi: 6,
            storageRequestGi: 0,
            storageLimitGi: 0,
          },
        },
      }),
    ) as { config: Record<string, unknown> }

    expect(body.config.applied_resource_overrides).toEqual({
      'pipeline.cdTool:argocd': {
        cpuRequest: 2,
        cpuLimit: 4,
        memoryRequestGi: 3,
        memoryLimitGi: 6,
        storageRequestGi: 0,
        storageLimitGi: 0,
      },
    })
  })

  // 계획 행이 하나도 만들어지지 않았을 때 무엇이 실리는지 못박는다. Go 쪽
  // omitempty 가 빈 맵을 지우므로, 이 경우 DB 에는 키 자체가 남지 않는다.
  it('계획이 비면 빈 객체가 실린다', () => {
    const body = toCreateStackBody(request({ appliedResourceOverrides: {} })) as {
      config: Record<string, unknown>
    }

    expect(body.config.applied_resource_overrides).toEqual({})
  })
})
