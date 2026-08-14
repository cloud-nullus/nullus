import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { TOOL_VERSION_CATALOG } from './stack-config-store'

/**
 * 프론트의 버전 표가 백엔드 상수와 갈라지지 않게 붙든다.
 *
 * 설치되는 도구의 버전은 `internal/stack/domain/connection.go` 하나가 소유하고,
 * 백엔드는 그 값이 호환성 매트릭스와 어긋나지 않는지 이미 검사한다
 * (`TestChartVersionsMatchCompatibilityMatrix`). 정작 화면이 보는 표는 그 값을
 * 손으로 베껴 온 것이라 지킴이가 없었고, 실제로 11개 중 9개가 갈라졌다 —
 * 설치는 Argo CD 7.7.16 을 올리는데 화면은 6.8.0 을 말했다.
 *
 * 여기가 사소한 표시 오류로 끝나지 않는 이유는 템플릿 편집기가 기본값을 이
 * 표에서 가져가기 때문이다. 관리자가 버전 칸을 손대지 않으면 존재하지 않는
 * 버전이 그대로 템플릿에 pin 되고, 그 템플릿으로 스택을 만들면 설치 전 검사가
 * 매트릭스에 없는 조합이라고 막는다.
 *
 * Go 파일을 직접 읽는다. 두 언어 사이에 값을 나르는 생성 단계를 만들 수도
 * 있지만, 표가 열 줄 남짓이라 그 배관이 값보다 커진다.
 */

const GO_CONSTANTS = join(__dirname, '../../../../../internal/stack/domain/connection.go')

/** 프론트 도구 id → Go 상수 접두사. 여기 없는 도구는 백엔드에 상수가 없다. */
const BACKEND_OWNED: Record<string, string> = {
  gitlab: 'GitLab',
  'gitlab-registry': 'GitLab',
  'gitlab-ci': 'GitLab',
  argocd: 'ArgoCD',
  minio: 'MinIO',
  prometheus: 'Prometheus',
  grafana: 'Grafana',
  nexus: 'Nexus',
  harbor: 'Harbor',
  tempo: 'Tempo',
  'opentelemetry-collector': 'OTelCollector',
}

function readGoVersions(): Record<string, string> {
  const source = readFileSync(GO_CONSTANTS, 'utf8')
  const found: Record<string, string> = {}
  for (const match of source.matchAll(/^\s*(\w+(?:Chart|App)Version)\s*=\s*"([^"]*)"/gm)) {
    found[match[1]] = match[2]
  }
  return found
}

describe('도구 버전 표', () => {
  it('설치되는 도구의 버전이 백엔드 상수와 같다', () => {
    const go = readGoVersions()
    const drift: string[] = []

    for (const [toolId, prefix] of Object.entries(BACKEND_OWNED)) {
      const entry = TOOL_VERSION_CATALOG[toolId]
      const expectedApp = go[`${prefix}AppVersion`]
      const expectedChart = go[`${prefix}ChartVersion`]

      // 상수 이름이 바뀌면 조용히 통과하지 않고 여기서 걸린다.
      expect(expectedApp, `${prefix}AppVersion 상수를 찾지 못했다`).toBeDefined()
      expect(expectedChart, `${prefix}ChartVersion 상수를 찾지 못했다`).toBeDefined()

      if (entry?.appVersion !== expectedApp) {
        drift.push(`${toolId} app: ${entry?.appVersion} ≠ ${expectedApp}`)
      }
      if (entry?.chartVersion !== expectedChart) {
        drift.push(`${toolId} chart: ${entry?.chartVersion} ≠ ${expectedChart}`)
      }
    }

    expect(drift, `백엔드 상수와 어긋난 값:\n${drift.join('\n')}`).toEqual([])
  })

  // 표에 없는 도구를 고르면 getToolAppVersion 이 '1.0.0' 을 돌려준다. 그 값은
  // 어느 차트에도 없으므로 화면에 뜨는 순간 거짓말이 된다.
  it('설치되는 도구가 표에서 빠지지 않는다', () => {
    const missing = Object.keys(BACKEND_OWNED).filter((id) => !TOOL_VERSION_CATALOG[id])
    expect(missing, `표에 없는 도구:\n${missing.join('\n')}`).toEqual([])
  })
})
