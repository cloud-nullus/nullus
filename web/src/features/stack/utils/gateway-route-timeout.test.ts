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

describe('buildGatewayManifest listeners', () => {
  // 배포된 앱의 HTTPRoute 는 앱이 서는 네임스페이스(예: default)에 생긴다.
  // 라우트는 같은 네임스페이스의 Service 를 가리켜야 하므로 그 자리를 옮길 수 없다.
  //
  // 리스너가 Same 이면 그 라우트는 게이트웨이에 붙지 못한다. 스택 도구들은 같은
  // 네임스페이스라 잘 열리고 배포된 앱만 안 열린다 — 2026-08-21 운영에서
  // Argo CD 는 Synced/Healthy 인데 sample-frontend.nullus.io 가 열리지 않았다.
  it('accepts routes from other namespaces', () => {
    const manifest = buildGatewayManifest(draft(), [{ toolId: 'harbor' } as never])

    expect(manifest).toContain('from: All')
    expect(manifest).not.toContain('from: Same')
  })

  it('accepts cross-namespace routes on the https listener too', () => {
    const tls = draft()
    tls.accessDomainTls = {
      enabled: true,
      secretName: 'nullus-access-domain-tls',
      secretNamespace: '',
      issuerName: 'nullus-ca-issuer',
    } as never

    const manifest = buildGatewayManifest(tls, [{ toolId: 'harbor' } as never])

    // 두 리스너 모두 열려 있어야 한다. https 만 막히면 앱이 http 로만 열리고
    // 그 사실이 어디에도 드러나지 않는다.
    expect(manifest.match(/from: All/g)?.length).toBe(2)
  })
})
