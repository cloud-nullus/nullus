import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { backupQueryKeys, requiresQuiesce } from './backup-api'

describe('requiresQuiesce', () => {
  it('should say a backup stops the service only when volumes are included', () => {
    // 정지 창을 만드는 것은 볼륨뿐이다. 화면이 이 규칙으로 경고와 확인 입력을
    // 켜고 끄므로, 서버(handler.requiresQuiesce)와 같은 답을 내야 한다.
    expect(requiresQuiesce(['volume'])).toBe(true)
    expect(requiresQuiesce(['platform_db', 'keycloak_db', 'openbao_kv', 'ns_resources'])).toBe(false)
    expect(requiresQuiesce([])).toBe(false)
  })
})

describe('useBackupRuns 설정', () => {
  it('should not seed the cache with an empty list', () => {
    // 전역 staleTime 이 5분이다. initialData 로 빈 배열을 주면 react-query 가
    // 그것을 **5분간 신선한 값**으로 보고 조회하지 않는다 — 백업이 있어도
    // 화면은 "아직 백업이 없습니다" 로 남는다. 실제로 그렇게 됐다.
    const src = readFileSync(join(__dirname, './backup-api.ts'), 'utf-8')
    // 설명 주석에는 이 낱말이 나온다. **설정 항목**만 본다.
    expect(src).not.toMatch(/^\s*initialData\s*:/m)
  })
})

describe('backupQueryKeys', () => {
  it('should key the run list so a new backup invalidates it', () => {
    expect(backupQueryKeys.runs()).toEqual(['admin', 'backups'])
  })
})
