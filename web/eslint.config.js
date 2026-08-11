import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist', 'coverage', 'playwright-report']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      'react-refresh/only-export-components': 'off',
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/purity': 'off',
      'react-hooks/incompatible-library': 'off',
      'no-case-declarations': 'off',

      // ag-grid-enterprise 는 상용 라이선스다. Nullus 는 오픈소스 플랫폼이므로
      // 실수로 import 되면 배포할 수 없는 산출물이 된다.
      // Community(MIT) 로 안 되는 기능은 도입 전에 반드시 논의한다.
      // 그리드/차트/아이콘 라이브러리 중복 도입도 함께 막는다 — DESIGN.md §Components.
      'no-restricted-imports': [
        'error',
        {
          paths: [
            // 표는 components/shared/data-table.tsx (TanStack Table) 하나로 통일한다.
            // AG Grid 이관은 시도했다가 되돌렸다 — 실제 레이아웃 측정에 의존해 jsdom 에서
            // 행을 렌더하지 않아 목록 화면 7곳의 회귀 테스트 16건이 깨졌다(기획안 D6/A안).
            // 다시 검토하려면 그 대가부터 다시 본다.
            {
              name: 'ag-grid-community',
              message:
                '표는 DataTable(TanStack) 하나로 통일한다. AG Grid 는 jsdom 에서 행을 렌더하지 않아 목록 화면의 회귀 검증을 잃는다 — 기획안 D6 참조.',
            },
            {
              name: 'ag-grid-enterprise',
              message: 'ag-grid-enterprise 는 상용 라이선스다. Nullus 는 오픈소스 플랫폼이다.',
            },
            {
              name: '@mui/x-data-grid',
              message: '표는 DataTable 하나로 통일한다. 그리드 라이브러리를 2개 두지 않는다.',
            },
            {
              name: '@mui/icons-material',
              message: '아이콘은 lucide-react 하나만 쓴다. 세트가 2개면 같은 뜻의 아이콘이 화면마다 달라진다.',
            },
            // 차트는 recharts(SVG) 하나다. chart.js 는 <canvas> 에 그리므로
            // CSS 변수를 해석하지 못한다 — 토큰을 var() 로 넘기면 색을 못 읽어
            // 차트가 통째로 검게 렌더된다. 실제로 그 회귀가 났고, getComputedStyle
            // 로 var() 를 푸는 다리(theme/resolve-token.ts)를 두고 색 58곳을
            // 감싸야 했다. 캔버스를 버리면서 그 다리도 지웠다.
            {
              name: 'chart.js',
              message:
                '차트는 recharts 하나로 통일한다. chart.js 는 <canvas> 라 --color-* 토큰을 해석하지 못한다 — 기획안 Phase 5 참조.',
            },
            {
              name: 'react-chartjs-2',
              message: '차트는 recharts 하나로 통일한다 — 기획안 Phase 5 참조.',
            },
          ],
          patterns: [
            {
              group: ['ag-grid-*'],
              message: '표는 DataTable(TanStack) 하나로 통일한다 — 기획안 D6 참조.',
            },
            {
              group: ['chart.js/*'],
              message: '차트는 recharts 하나로 통일한다 — 기획안 Phase 5 참조.',
            },
          ],
        },
      ],
    },
  },
  {
    // 디자인 토큰 강제 — 색은 web/DESIGN.md 가 단일 출처이고 코드는 --color-* 를 참조한다.
    // 개편 전 TSX 에 hex 767곳 / rgba 750곳이 박혀 있었고, 그 값들이 전부 다크 기준이라
    // 라이트 테마에서 대비가 1.6:1 까지 무너졌다. 이 규칙이 그 재발을 막는다.
    //
    // 아직 잔여 하드코딩이 남아 있어 'warn' 으로 시작한다. Phase 6 청산이 끝나면
    // 'error' 로 올린다 (기획안 커밋 21).
    // .ts 도 포함한다. 상태 배지 팔레트가 utils/*.ts 에 있었고, .tsx 만 보던 규칙이
    // 화면 전반의 상태색을 통째로 놓쳤다.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/*.test.{ts,tsx}', 'src/theme/**'],
    rules: {
      // 여기에 no-restricted-imports 를 다시 선언하지 않는다.
      //
      // 이 블록의 files 가 src/** 를 다시 잡으므로, 같은 키를 여기서 정의하면
      // flat config 규칙상 위 블록의 설정을 통째로 덮어쓴다. 실제로 Phase 5 이관
      // 전까지 chart.js 경고만 남기려고 여기에 재선언해 뒀는데, 그 바람에
      // ag-grid · @mui/x-data-grid · @mui/icons-material 금지가 src/** 전체에서
      // 조용히 꺼져 있었다. chart.js 금지는 이제 위 블록에 error 로 있다.
      'no-restricted-syntax': [
        'warn',
        {
          selector: 'Literal[value=/#[0-9a-fA-F]{3,8}\\b/]',
          message:
            '색 리터럴(hex)을 TSX 에 쓰지 않는다. web/DESIGN.md 에 토큰을 정의하고 var(--color-*) 를 참조한다.',
        },
        {
          selector: 'Literal[value=/rgba?\\(/]',
          message:
            '색 리터럴(rgb/rgba)을 TSX 에 쓰지 않는다. 토큰이 없으면 web/DESIGN.md 에 추가한다. 투명도가 필요하면 color-mix(in srgb, var(--color-*) N%, transparent) 를 쓴다.',
        },
      ],
    },
  },
  {
    files: ['e2e/**/*.ts', 'e2e/**/*.tsx'],
    rules: {
      '@typescript-eslint/no-unused-vars': 'off',
    },
  },
])
