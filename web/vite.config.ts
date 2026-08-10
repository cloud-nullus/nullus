import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  optimizeDeps: {
    include: ['monaco-editor', 'monaco-yaml'],
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8090',
        ws: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: (id: string) => {
          if (id.includes('react-dom') || id.includes('react-router-dom') || (id.includes('node_modules/react/') && !id.includes('react-'))) {
            return 'vendor-react'
          }
          if (id.includes('@tanstack/react-query')) {
            return 'vendor-query'
          }
          if (id.includes('zustand') || id.includes('lucide-react')) {
            return 'vendor-ui'
          }
          // MUI + emotion 은 앱 전역에서 쓰이므로 별도 벤더 청크로 뺀다.
          // 메인 청크에 섞이면 라우트 코드와 함께 무효화돼 캐시 효율이 떨어진다.
          if (id.includes('@mui/') || id.includes('@emotion/')) {
            return 'vendor-mui'
          }
          if (id.includes('ag-grid-')) {
            return 'vendor-grid'
          }
          if (id.includes('react-i18next') || id.includes('i18next')) {
            return 'vendor-i18n'
          }
          if (id.includes('recharts')) {
            return 'vendor-charts'
          }
          if (id.includes('react-hook-form') || id.includes('zod') || id.includes('@hookform')) {
            return 'vendor-form'
          }
          if (id.includes('@tanstack/react-table')) {
            return 'vendor-table'
          }
        },
      },
    },
    chunkSizeWarningLimit: 500,
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    exclude: ['e2e/**', 'node_modules/**'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'text-summary', 'lcov'],
      include: ['src/features/**', 'src/components/**', 'src/stores/**'],
      exclude: ['**/*.test.*', '**/__tests__/**', '**/node_modules/**'],
      thresholds: {
        statements: 60,
        branches: 50,
        functions: 55,
        lines: 60,
      },
    },
  },
})
