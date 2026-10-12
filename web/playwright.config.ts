import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'npm run dev',
    port: 5173,
    reuseExistingServer: true,
    timeout: 10000,
  },
  projects: [
    // 기본 프로젝트. 시각 회귀는 뺀다 — 베이스라인이 OS 별로 갈리는데
    // 저장소에 있는 것은 chromium-darwin 뿐이라, 리눅스(CI)에서는 비교 대상이
    // 없어 전부 실패한다. 그 58 건이 잡을 통째로 빨갛게 만들어 다른 회귀를 덮었다.
    // shots/ 는 문서용 화면 캡처다. 검증이 아니라 산출물 생성이므로 게이트에서 뺀다
    // (`npm run e2e:shots` 로 따로 돌린다).
    { name: 'chromium', use: { browserName: 'chromium' }, testIgnore: [/visual\//, /shots\//] },

    // 시각 회귀는 별도 프로젝트로 둔다. 로컬에서 `npm run e2e:visual` 로 돌리고,
    // 베이스라인 갱신은 `npm run e2e:visual:update` 다.
    // CI 게이트에 넣으려면 리눅스 베이스라인을 먼저 만들어 커밋해야 한다.
    { name: 'visual', use: { browserName: 'chromium' }, testMatch: /visual\/.*\.spec\.ts/ },
  ],
})
