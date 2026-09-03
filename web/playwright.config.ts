import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  retries: 0,
  use: {
    // 포트 회피 환경(수행 가이드 §2: web 5174)에서는 이미 떠 있는 서버를 쓴다:
    // E2E_BASE_URL=http://localhost:5174 npx playwright test ...
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
  },
  // E2E_BASE_URL 지정 시 dev 서버를 새로 띄우지 않는다 — 5173 기동이 외부 서버와 무관하게 실패할 수 있다.
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: 'npm run dev',
        port: 5173,
        reuseExistingServer: true,
        timeout: 10000,
      },
  projects: [
    // 기본 프로젝트. 시각 회귀는 뺀다 — 베이스라인이 OS 별로 갈리는데
    // 저장소에 있는 것은 chromium-darwin 뿐이라, 리눅스(CI)에서는 비교 대상이
    // 없어 전부 실패한다. 그 58 건이 잡을 통째로 빨갛게 만들어 다른 회귀를 덮었다.
    { name: 'chromium', use: { browserName: 'chromium' }, testIgnore: /visual\// },

    // 시각 회귀는 별도 프로젝트로 둔다. 로컬에서 `npm run e2e:visual` 로 돌리고,
    // 베이스라인 갱신은 `npm run e2e:visual:update` 다.
    // CI 게이트에 넣으려면 리눅스 베이스라인을 먼저 만들어 커밋해야 한다.
    { name: 'visual', use: { browserName: 'chromium' }, testMatch: /visual\/.*\.spec\.ts/ },
  ],
})
