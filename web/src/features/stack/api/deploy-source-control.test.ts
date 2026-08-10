import { describe, it, expect } from 'vitest'

import { toDeployBody } from './stack-api'
import { toCreateStackBody } from './stack-normalizers'
import type { CreateStackRequest } from '../../../types'

function baseRequest(): CreateStackRequest {
  return {
    templateId: 'github-argocd-v1',
    clusterId: 'cluster-1',
    namespace: 'nullus',
    stackName: 'gh-stack',
    artifacts: {
      sourceRepository: { tool: 'github', version: 'external' },
      containerRegistry: { tool: 'ghcr', version: 'external' },
      packageRegistry: { tool: '', version: '' },
      storageBackend: { tool: 'minio', version: '2024' },
    },
    pipeline: {
      cicdPlatform: { tool: 'github-actions', version: 'external' },
      cdTool: { tool: 'argocd', version: '2.13.2' },
    },
    monitoring: { collection: { tool: 'prometheus', version: '3.1.0' } },
    logging: {},
    resources: {
      developerCount: 5,
      concurrentRunners: 2,
      commitsPerDay: 10,
      buildFrequency: 'medium',
      currency: 'KRW',
      mode: 'auto',
    },
  } as unknown as CreateStackRequest
}

describe('스택 생성 본문', () => {
  it('GitHub organization 을 스택 구성에 담는다', () => {
    const req = baseRequest()
    req.sourceControl = { owner: 'acme', apiBaseUrl: 'https://ghe.acme.test/api/v3' }

    expect(toCreateStackBody(req).config.source_control).toEqual({
      owner: 'acme',
      api_base_url: 'https://ghe.acme.test/api/v3',
    })
  })

  it('organization 이 비면 아예 보내지 않는다', () => {
    const req = baseRequest()
    req.sourceControl = { owner: '   ', apiBaseUrl: '' }

    expect(toCreateStackBody(req).config.source_control).toBeUndefined()
  })

  // 구성은 평문으로 저장되고 조회 API 로 다시 내려온다. 토큰이 여기 실리면
  // 스택을 볼 수 있는 누구에게나 노출된다.
  it('PAT 는 어떤 경우에도 구성에 실리지 않는다', () => {
    const req = baseRequest()
    req.sourceControl = {
      owner: 'acme',
      apiBaseUrl: '',
      // 타입에는 없지만 호출부 실수로 섞여 들어올 수 있다.
      personalAccessToken: 'ghp_secret',
    } as CreateStackRequest['sourceControl']

    expect(JSON.stringify(toCreateStackBody(req))).not.toContain('ghp_secret')
  })
})

describe('배포 요청 본문', () => {
  it('PAT 를 source_control 로 보낸다', () => {
    expect(toDeployBody({ stackId: 's1', sourceControlToken: 'ghp_from_wizard' })).toEqual({
      source_control: { personal_access_token: 'ghp_from_wizard' },
    })
  })

  it('경고 승인과 PAT 를 함께 보낸다', () => {
    expect(
      toDeployBody({ stackId: 's1', acknowledgeWarnings: true, sourceControlToken: 'ghp' }),
    ).toEqual({
      acknowledge_warnings: true,
      source_control: { personal_access_token: 'ghp' },
    })
  })

  it('경고 승인만 있으면 예전과 같은 본문을 보낸다', () => {
    expect(toDeployBody({ stackId: 's1', acknowledgeWarnings: true })).toEqual({
      acknowledge_warnings: true,
    })
  })

  // 예전 클라이언트는 본문 없이 호출한다. 빈 객체를 보내면 서버가 굳이
  // 파싱해야 하고, 승인하지 않은 요청과 형태가 달라진다.
  it('보낼 것이 없으면 본문을 생략한다', () => {
    expect(toDeployBody({ stackId: 's1' })).toBeUndefined()
    expect(toDeployBody({ stackId: 's1', acknowledgeWarnings: false })).toBeUndefined()
  })

  it('공백만 있는 토큰은 보내지 않는다', () => {
    expect(toDeployBody({ stackId: 's1', sourceControlToken: '   ' })).toBeUndefined()
  })
})
