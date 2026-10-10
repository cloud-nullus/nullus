import { describe, expect, it } from 'vitest'

import { goldenPathToStackOverrides } from './cicd-golden-path-page'

// "Use this Golden Path" 로 설치 화면에 들어가면 Golden Path 의 도구가 그대로 채워져야 한다.
// 보안 도구를 무시하면 스캐너·SonarQube 가 조용히 빠진 채 설치된다.
describe('goldenPathToStackOverrides', () => {
  it('보안 도구를 security 로 옮긴다', () => {
    const overrides = goldenPathToStackOverrides([
      { name: 'GitLab CE', category: 'source_repository', helm_version: '8.7.2' },
      { name: 'Trivy', category: 'image_scanner', helm_version: '0.26.0' },
      { name: 'SonarQube', category: 'sast', helm_version: '2026.5.1002' },
    ] as Parameters<typeof goldenPathToStackOverrides>[0])

    expect(overrides.security.imageScanner.tool).toBe('trivy')
    expect(overrides.security.sast.tool).toBe('sonarqube')
  })

  it('보안 도구가 없으면 고르지 않음이다', () => {
    const overrides = goldenPathToStackOverrides([
      { name: 'GitLab CE', category: 'source_repository', helm_version: '8.7.2' },
    ] as Parameters<typeof goldenPathToStackOverrides>[0])

    expect(overrides.security.imageScanner.tool).toBe('')
    expect(overrides.security.sast.tool).toBe('')
  })
})
