import { describe, expect, it } from 'vitest'

import { buildGatewayManifest } from './install-manifest-builders'
import type { StackConfigDraft } from '../types'

function draft(): StackConfigDraft {
  return {
    stackName: 'nullus-devsecops-stack',
    namespace: 'nullus-devsecops-stack',
    accessDomain: 'nullus.io',
    accessDomainTls: {
      enabled: false,
      secretName: '',
      secretNamespace: '',
      issuerName: '',
    },
  } as unknown as StackConfigDraft
}

describe('buildGatewayManifest', () => {
  // Envoy 의 기본 라우트 타임아웃은 15초다. 이미지 layer push 와 git push 는
  // 그보다 오래 걸리는 단일 요청이라, 타임아웃을 정해 두지 않으면 게이트웨이가
  // 중간에 끊는다 — docker 는 그것을 재시도로 받아 모든 layer 가 끝없이
  // "Retrying in N seconds" 를 반복한다.
  //
  // 앞단 nginx 의 본문 상한을 풀어도(#211) 이 관문이 남는다.
  it('sets a request timeout long enough for image and git pushes', () => {
    const manifest = buildGatewayManifest(draft(), [
      { toolId: 'harbor' } as never,
    ])

    expect(manifest).toContain('timeouts:')
    expect(manifest).toContain('request: 600s')
    // backendRequest 는 request 보다 클 수 없다. 크면 Gateway API 가 라우트를
    // 거부해 도구가 통째로 열리지 않는다.
    expect(manifest).toContain('backendRequest: 600s')
  })
})
