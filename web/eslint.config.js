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
    // 잔여 청산이 끝나 'error' 로 올렸다(기획안 커밋 21). 이제 새 hex 하나가
    // CI 를 세운다 — 규칙이 경고로 남아 있는 동안은 개수가 다시 늘 뿐이다.
    // .ts 도 포함한다. 상태 배지 팔레트가 utils/*.ts 에 있었고, .tsx 만 보던 규칙이
    // 화면 전반의 상태색을 통째로 놓쳤다.
    files: ['src/**/*.{ts,tsx}'],
    // 외부 도구(GitLab · Kubernetes …)의 브랜드 색은 우리 팔레트가 아니라
    // 그 프로젝트가 정한 고정값이다. 한 파일에 모아 두고 거기만 면제한다.
    //
    // 로고 마크도 같은 이유로 면제한다. 마크의 3색은 테마를 따라가면 안 되는
    // 정체성 값이고(라이트에서 파랑, 다크에서 회색인 로고는 로고가 아니다),
    // 그 사이를 잇는 48단 그라데이션은 docs/40_UI_UX/logo/emit.mjs 가 굽는
    // 생성물이다 — 토큰 48개로 옮길 대상이 아니다. 마스크의 #fff/#000 도
    // 색이 아니라 "남긴다/끊는다" 를 뜻하는 명도값이다.
    ignores: [
      'src/**/*.test.{ts,tsx}',
      'src/theme/**',
      'src/lib/tool-brand-colors.ts',
      'src/components/brand/**',
    ],
    rules: {
      // 여기에 no-restricted-imports 를 다시 선언하지 않는다.
      //
      // 이 블록의 files 가 src/** 를 다시 잡으므로, 같은 키를 여기서 정의하면
      // flat config 규칙상 위 블록의 설정을 통째로 덮어쓴다. 실제로 Phase 5 이관
      // 전까지 chart.js 경고만 남기려고 여기에 재선언해 뒀는데, 그 바람에
      // ag-grid · @mui/x-data-grid · @mui/icons-material 금지가 src/** 전체에서
      // 조용히 꺼져 있었다. chart.js 금지는 이제 위 블록에 error 로 있다.
      'no-restricted-syntax': [
        'error',
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
        // hex/rgba 만 막으면 Tailwind 기본 팔레트로 우회할 수 있다. 실제로
        // text-emerald-400 · bg-amber-500/15 같은 클래스가 17곳 남아 있었고,
        // 그 초록은 --color-success 와 다른 값이라 같은 "정상" 이 화면마다
        // 달라 보였다. 클래스명은 문자열이라 hex 규칙에 걸리지 않는다.
        {
          selector:
            'Literal[value=/(^|[\\s:\\[])(bg|text|border|ring|fill|stroke|from|via|to)-(slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\\d{2,3}\\b/]',
          message:
            'Tailwind 기본 팔레트를 쓰지 않는다(예: text-emerald-400). 같은 뜻의 색이 화면마다 달라진다 — var(--color-*) 토큰을 쓰고, 투명도는 color-mix(in srgb, var(--color-*) N%, transparent) 로 만든다.',
        },
        // 아이콘 크기를 숫자로 직접 쓰면 선 굵기가 따라오지 않는다. lucide 는
        // 24 격자에 stroke 2 라 size 만 줄이면 렌더 굵기가 함께 줄어든다 —
        // 개편 전 크기가 12가지로 흩어지고 strokeWidth 지정이 0곳이던 이유다.
        // MUI 는 size 에 문자열('small')을 받으므로 숫자만 막으면 겹치지 않는다.
        {
          // 숫자 리터럴을 정규식으로 거르려 하면 esquery 가 문자열만 비교해 걸리지 않는다.
          // size={x} 처럼 변수를 넘기면 Identifier 라 애초에 매치되지 않으므로,
          // Literal 이 오는 경우만 잡으면 숫자 하드코딩이 정확히 걸린다.
          //
          // NullusMark 는 뺀다. 로고는 아이콘이 아니라서 이 스케일을 따르지 않는다 —
          // 로그인 카드 52px, 홈 히어로 80px 처럼 자리마다 크기가 따로 정해진다.
          selector:
            "JSXOpeningElement:not([name.name='NullusMark']) > JSXAttribute[name.name='size'] > JSXExpressionContainer > Literal",
          message:
            "아이콘 크기를 숫자로 쓰지 않는다. iconProps('xs'|'sm'|'md'|'lg') 로 크기와 굵기를 함께 받는다 — 단계는 web/DESIGN.md §아이콘 이 단일 출처다.",
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
