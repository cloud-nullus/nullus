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
    poolOptions: {
      forks: {
        execArgv: ['--max-old-space-size=4096'],
      },
    },
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'lcov'],
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.test.{ts,tsx}', 'src/test-setup.ts', 'src/main.tsx', 'e2e/**'],
    },
  },
})
