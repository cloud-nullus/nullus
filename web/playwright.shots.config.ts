import { defineConfig } from '@playwright/test'

// 문서용 화면 캡처 전용 설정. 검증이 아니라 `docs/assets/backup/*.png` 를 만든다.
//
// 기본 설정과 분리한 이유가 둘 있다.
//   - 기본 설정은 5173 을 reuseExistingServer 로 재사용한다. 그 포트에 다른
//     프로젝트가 떠 있으면 엉뚱한 화면을 찍는다 — 실제로 그렇게 됐다.
//   - 로컬 개발 설정이 OIDC 모드면 Keycloak 으로 튕겨 찍을 화면이 없다.
//     세션 모드로 띄워 ID/PW 폼을 거친다.
export default defineConfig({
  testDir: './e2e/shots',
  use: { baseURL: 'http://localhost:5173', headless: true },
  webServer: {
    command: 'VITE_AUTH_MODE=session VITE_OIDC_PROVIDER= npm run dev -- --port 5173 --strictPort',
    port: 5173,
    reuseExistingServer: false,
    timeout: 60000,
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
})
