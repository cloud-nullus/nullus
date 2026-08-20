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
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
})
