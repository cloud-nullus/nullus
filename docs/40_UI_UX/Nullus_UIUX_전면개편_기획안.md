# Nullus UI/UX 전면 개편 기획안

**작성일**: 2026-08-11
**대상 독자**: 프론트엔드 엔지니어, 디자이너, 팀 리뷰어
**배경**: 2026-08 팀 논의 "5) UI 정리 및 디자인 가이드" (박준희, 이기하)
**참고**: [google-labs-code/design.md](https://github.com/google-labs-code/design.md) 스펙,
[getdesign.md](https://getdesign.md/coinbase/design-md), [MUI Material UI v9](https://mui.com/material-ui/)
**관련 문서**: `Nullus_디자인시스템.md`(현행 토큰), `Nullus_UI_UX_구현계획.md`(초기 계획),
`Nullus_프론트엔드_상세설계.md`

---

## 0. 요약 (TL;DR)

| 항목 | 내용 |
|------|------|
| **문제** | 라이트 테마에서 화면이 "와이어프레임/스켈레톤"처럼 보이고 가독성이 낮다. 컴포넌트 룩이 화면마다 다르다. |
| **진짜 원인** | 디자인 토큰이 단일 출처가 아니고, 색상 1,517곳이 TSX에 하드코딩돼 있으며(hex 767 + rgba 750) 그 값들이 **전부 다크 기준**으로 작성돼 라이트에서 무너진다. 라이트 테마의 카드 배경과 페이지 배경이 **동일 색(대비 1.00:1)** 이고, 보더는 거의 검정(14.03:1)이라 "흰 종이에 검은 선" = 와이어프레임처럼 보인다. |
| **해결 축 1** | `DESIGN.md`(google design.md 스펙)를 **디자인 단일 출처**로 두고 → MUI 테마 → Tailwind `@theme inline` 로 **한 갈래로 파생**시킨다. CI에서 `design.md lint`로 대비를 게이트한다. |
| **해결 축 2** | 손으로 만든 프리미티브(Button/Input/Select/Modal/Card)를 **MUI v9로 교체**한다. 단, 공개 API를 유지하는 **어댑터 방식**이라 28개 화면 코드는 Phase 2에서 거의 손대지 않는다. |
| **해결 축 3** | 표를 **`DataTable`(TanStack) 하나로** 통일한다. 이미 7화면이 이 컴포넌트를 쓰고, 손으로 만든 `<table>` 17곳을 여기로 흡수한다. AG Grid 이관은 시도 후 되돌렸다(D6/A안). |
| **정보 유실 0 보장** | 기존 vitest 577개(그중 `getByText` 329 / `getByRole` 184 / `getByLabelText` 103 = 이미 "보이는 텍스트" 계약)를 **수정 금지 회귀 계약**으로 고정 + Phase 0에서 화면 정보 인벤토리 스냅샷 + Playwright 시각 회귀 베이스라인(28화면 × 2테마). |
| **범위** | 28개 화면, 18,823 LOC. 7 Phase. **단일 브랜치에서 커밋 24개로 진행하고 최종 1 PR** (§6.1). |

---

## 1. 목표와 비목표

### 1.1 목표

1. **가독성**: 라이트/다크 양쪽에서 본문·보조텍스트·상태색 전부 WCAG AA(4.5:1) 이상.
2. **통일감**: 같은 의미의 UI는 같은 컴포넌트로. 버튼·입력·표·모달·배지·차트가 화면마다 다르지 않게.
3. **단일 출처**: 색/타입/간격/그림자 값이 사람이 읽는 문서와 코드에서 **갈라지지 않게**.
4. **AI 협업 가능**: 클로드/코덱스가 새 화면을 만들 때 `DESIGN.md` 하나만 읽고 기존 룩과 맞게 짜도록.
5. **정보 유실 0**: 현재 화면이 보여주는 필드·컬럼·액션·상태·빈상태·에러 메시지가 **하나도 사라지지 않게**.

### 1.2 비목표 (이번 개편에서 하지 않는 것)

- 정보 구조(IA)·메뉴 체계 변경 → `Nullus_메뉴체계.md` 유지
- 신규 기능 추가, API 변경
- 모바일 최적화 (관리 도구 특성상 1024px 이상 유지)
- 브랜드 로고·워드마크 재디자인

---

## 2. 현황 진단 (측정 결과)

### 2.1 "스켈레톤처럼 보인다"의 정확한 원인

`web/src/index.css`의 라이트 테마 오버라이드를 실제 값으로 대비 계산한 결과다.

| 측정 대상 | 현재 값 | 대비 | 판정 |
|-----------|---------|------|------|
| 라이트: 카드 `#f8fafc` vs 페이지 배경 `#f8fafc` | 동일 색 | **1.00:1** | 카드가 배경과 구분 불가 |
| 라이트: 보더 `#1f2937` on 카드 | 거의 검정 | **14.03:1** | 선이 과하게 튄다 → 와이어프레임 룩 |
| 라이트: `#a5b4fc`(Secondary 버튼 텍스트) | 다크용 값 | **1.91:1** | ❌ AA 미달 |
| 라이트: `#34d399`(성공 상태) | 다크용 값 | **1.84:1** | ❌ AA 미달 |
| 라이트: `#fbbf24`(경고 상태) | 다크용 값 | **1.60:1** | ❌ AA 미달 |
| 라이트: `#818cf8`(기능 아이콘) | 다크용 값 | **2.85:1** | ❌ AA 미달 |
| 다크: `--color-text-muted` `#64748b` on 카드 | — | **3.89:1** | ❌ AA 미달 |

**정리**: 카드와 배경이 같은 색이라 면(surface)으로 구분이 안 되고, 그림자 토큰이 아예 없어 입체감도 없다.
남은 유일한 구분선인 보더가 거의 검정이다. 이게 "흰 배경에 검은 박스만 있는 스켈레톤" 인상의 직접 원인이다.
그 위에 다크 기준으로 하드코딩된 상태색이 라이트에서 전부 흐려진다.

> **문서-구현 불일치**: `Nullus_디자인시스템.md` §2.4는 라이트 보더를 `#e2e8f0`으로 적어놨지만
> 실제 `index.css`는 `#1f2937`이다. 문서가 이미 구현과 갈라져 있다 — 단일 출처가 필요한 이유.

### 2.2 토큰이 지켜지지 않는다

| 지표 | 수치 |
|------|------|
| CSS 커스텀 프로퍼티 토큰 | 46개 (`index.css`) |
| TSX에 하드코딩된 hex | **767곳** |
| TSX에 하드코딩된 rgba | **750곳** |
| `[var(--...)]` 임의값 클래스 | **1,582곳** |
| 인라인 `style={{}}` | 43곳 |
| 시맨틱 surface 토큰 | 5개뿐 (base/card/overlay + border 2) |
| 그림자·elevation 토큰 | **0개** |

토큰이 46개인데 하드코딩이 1,500곳 이상이면 토큰은 사실상 장식이다.
그리고 `[var(--color-text-secondary)]` 같은 임의값 클래스 1,582곳은 Tailwind의 이점(짧은 시맨틱 유틸리티)을
포기한 형태다 — `text-secondary` 로 줄일 수 있다.

### 2.3 컴포넌트 파편화

| 영역 | 현황 |
|------|------|
| **버튼** | `<Button>` 124곳 vs 생 `<button>` **95곳** |
| **입력** | `<Input>` 95곳 vs 생 `<input>` **45곳** |
| **표** | `DataTable`(TanStack) 7화면 vs 손으로 만든 `<table>` **17곳** |
| **차트** | recharts 3파일 + chart.js 1파일 → **차트 라이브러리 2개 병존** |
| **`StatusBadge`** | 존재하지만 실제 사용 **1곳** — 나머지는 각자 배지를 다시 만듦 |
| **`Card`** | `iconBg`/`iconColor` prop이 **동작하지 않는다**(삼항 양쪽이 같은 값) → 디자인시스템 §2.3 "기능별 색상 매핑"이 코드에 미구현 |
| **`components/ui`** | 초기 계획은 shadcn/ui였으나 실제로는 Radix 의존 없는 **손수 구현**. → MUI 교체 시 버릴 코드가 적다(호재) |

### 2.4 자산 (개편의 안전망이 될 것들)

| 자산 | 수치 | 의미 |
|------|------|------|
| vitest 테스트 | **577개** (80파일) | 회귀 감지의 핵심 |
| `getByText` | 329회 | **보이는 텍스트를 이미 계약으로 검증 중** |
| `getByRole` | 184회 | 접근성 구조 계약 |
| `getByLabelText` | 103회 | 폼 라벨 계약 |
| `getByTestId` | 43회만 | 깨지기 쉬운 결합이 적다 |
| Playwright 스펙 | 17파일 (`web/e2e/`) | E2E 흐름 존재 |
| i18n 키 | ko 1,159 / en 1,170 | 문자열이 코드 밖에 있어 텍스트 유실 추적 가능 |

**이게 이번 개편의 최대 강점이다.** 테스트 대부분이 CSS 클래스가 아니라 "화면에 이 텍스트/역할이 있는가"를
검증하므로, 리팩터링으로 정보가 사라지면 **테스트가 먼저 깨진다**.

---

## 3. 개편 원칙

1. **문서가 코드를 만든다, 그 역은 없다.** 색·타입·간격·그림자는 `DESIGN.md`에만 정의하고 코드는 파생물이다.
2. **정보는 옮기되 버리지 않는다.** 밀도·배치·컴포넌트는 바꿔도 되고, 필드·값·액션·상태·에러 문구는 못 버린다.
3. **테스트를 고쳐서 통과시키지 않는다.** `getByText`/`getByRole`/`getByLabelText` 실패는 정보 유실 신호로 간주한다.
   테스트 수정이 허용되는 유일한 경우는 그 테스트가 구현 디테일(클래스명, testid)을 검증할 때이며,
   해당 커밋 메시지 본문에 사유를 적는다.
4. **어댑터로 바꾼다.** 프리미티브 내부만 MUI로 교체하고 공개 prop은 유지한다 → 화면 파일은 안 건드린다.
5. **한 종류에 라이브러리 하나.** 표=`DataTable`(TanStack), 차트=하나, 아이콘=lucide 하나. 중복 도입 금지.
6. **밀도를 지킨다.** 이건 데이터 밀집 운영 도구다. MUI 기본값은 여백이 넉넉해서 그대로 쓰면 한 화면에
   들어가던 정보가 줄어든다 → 이는 정보 유실과 같다. 테마에서 density를 조여서 시작한다.

---

## 4. 타깃 아키텍처 — 토큰 단일 파이프라인

```
                    web/DESIGN.md                     ← 사람이 고치는 유일한 곳
         (YAML front matter = 토큰 / 본문 = 근거·적용법)
                          │
        ┌─────────────────┼──────────────────┬────────────────────┐
        ▼                 ▼                  ▼                    ▼
  src/theme/           index.css                        CI 게이트
  mui-theme.ts       @theme inline                    design.md lint
  (createTheme        (생성된 --color-*                (대비 · 깨진 토큰
   cssVariables)       → Tailwind 토큰)                 참조 검출)
        │                 │
        ▼                 ▼
   MUI 컴포넌트      Tailwind 유틸리티
        └─────────────────┘
                          ▼
                   28개 화면 (일관된 룩)
```

### 4.1 왜 `DESIGN.md`인가

google-labs-code의 DESIGN.md 스펙은 **YAML front matter(기계가 읽는 토큰) + 마크다운 본문(사람/AI가 읽는 근거)**
2계층 구조다. 섹션 순서가 규정돼 있고(Overview → Colors → Typography → Layout → Elevation & Depth → Shapes →
Components → Do's and Don'ts), `{colors.primary}` 형태의 토큰 참조와 `npx @google/design.md lint`(WCAG 대비 검사,
깨진 참조 검출, JSON 출력) / `diff`(토큰 회귀 감지) CLI를 제공한다.

우리에게 맞는 이유:
- **팀 논의의 요구와 정확히 일치**: "레퍼런스를 정해 디자인 기준 문서를 만들어 전체를 관리한다"
- **CI 게이트 가능**: lint가 JSON을 뱉으므로 대비 미달을 PR에서 막을 수 있다 → §2.1 문제의 재발 방지
- **AI 에이전트 친화**: 클로드가 새 화면을 짤 때 읽을 단일 문서

### 4.2 getdesign.md / Coinbase 레퍼런스를 어떻게 쓸 것인가

솔직히 적어둔다 — `getdesign.md/coinbase/design-md`는 **Coinbase 공식 산출물이 아니라 제3자의 패턴 분석**이고,
공개 페이지에 실제 토큰 값(hex, 타입 스케일)은 노출돼 있지 않다. 페이지가 주는 건 방향 서술
("clean blue identity, trust-focused, institutional feel")과 DESIGN.md 파일 배포 방식(`npx getdesign@latest add coinbase`)이다.

따라서 이렇게 쓴다:
- **형식**: google design.md 스펙을 따른다 (규범)
- **톤 방향**: "신뢰감 있는 institutional" 방향은 채택한다 — 채도 낮은 중립 면 + 액센트 1개 집중.
  현재의 골드 그라데이션 CTA는 이 방향과 충돌한다(§5.3 결정 필요 항목)
- **동종 OSS 레퍼런스 리서치**(팀 논의 항목): ArgoCD / Grafana / Backstage / Rancher / Harbor 의
  대비·밀도·상태색 처리를 Phase 1에서 조사해 근거로 삼는다. 이들은 우리와 같은 "다크 우선 데이터 밀집 운영 도구"다.
- **`npx getdesign add coinbase` 로 받은 파일을 그대로 넣지 않는다.** 금융 브랜드 팔레트를 DevSecOps
  콘솔에 이식하면 상태색 의미 체계(성공/경고/위험)가 어긋난다. 구조만 참고한다.

---

## 5. 라이브러리 결정

### 5.1 도입

| 라이브러리 | 버전 | 라이선스 | 역할 |
|-----------|------|---------|------|
| `@mui/material` | v9 | MIT | 프리미티브·폼·모달·메뉴·탭 등 컴포넌트 층 |
| `@emotion/react`, `@emotion/styled` | v11 | MIT | MUI 스타일 엔진 (필수 peer) |
| ~~`ag-grid-react`, `ag-grid-community`~~ | — | — | **도입 취소** — D6/A안. 표는 `DataTable`(TanStack) 유지 |
| `@google/design.md` (devDep) | — | — | DESIGN.md lint/diff CI |

**라이선스 주의**: Nullus는 오픈소스 플랫폼이다. `ag-grid-enterprise`(상용)를 **실수로 import하면 안 된다.**
Phase 3에서 ESLint `no-restricted-imports` 로 `ag-grid-enterprise` 를 금지 규칙에 넣는다.

### 5.2 제거 / 미도입

| 대상 | 처리 | 이유 |
|------|------|------|
| `@tanstack/react-table` | Phase 3 완료 후 제거 | AG Grid로 대체 |
| `@mui/x-data-grid` | **도입 안 함** | AG Grid와 그리드 2개 병존 방지 |
| `@mui/icons-material` | **도입 안 함** | lucide-react 유지 — 아이콘 세트 2개 병존 방지 |
| `chart.js` + `react-chartjs-2` **또는** `recharts` | 하나만 남긴다 (Phase 5) | 현재 2개 병존. **recharts 존치 권장** (사용처 3파일 > 1파일, React 선언형이 테마 토큰 주입에 유리) |
| 손수 구현 `components/ui/*` | 내부만 MUI로 교체, 파일·export 유지 | 화면 코드 무변경 (§6.2) |

### 5.3 팀 결정이 필요한 항목

| # | 결정 항목 | 권장안 | 근거 |
|---|-----------|--------|------|
| D1 | **골드 그라데이션 Primary CTA 유지 여부** | 골드는 **로고/브랜드 전용**으로 축소, Primary 액션은 Indigo 단색 | 골드 위 텍스트 자체는 11.96:1로 문제없지만, 라이트 배경에서 골드는 텍스트/보더로 못 쓴다(1.40:1). 화면마다 CTA가 골드+인디고 2개로 갈리는 현재 상태가 통일감 저해의 큰 축. |
| D2 | **기본 테마** | 다크 기본 유지, 라이트를 **1급 지원**으로 승격 | 배포본 가독성 지적이 라이트에서 나왔다. 다크만 진짜인 현 상태가 문제. |
| D3 | **MUI Material 룩 수용 범위** | Material 컴포넌트 **동작·접근성**은 그대로, **표면 스타일(모양·밀도)은 테마로 재정의** | 기본 Material 룩은 여백이 커서 데이터 밀도가 떨어진다. `size="small"` 기본값 + spacing 축소로 시작. |
| D4 | **개편 후 다크 팔레트 미세 조정 허용 여부** | 허용 (`#0f1419` → `#141b24` 등) | 다크도 `text-muted` 3.89:1로 AA 미달이라 손대야 한다. |
| D5 | **`cicd-history` 행 확장 제거 + 메인/서브 테이블 분리** | 채택 (Phase 3 참조) | 현재 확장 패널은 메인 컬럼과 동일한 6필드를 반복할 뿐 새 정보가 없다. 행 확장은 28화면 중 1곳뿐인 일회성 패턴이고, 좌우/상하 분할 상세는 이미 3화면에서 쓰는 하우스 패턴이다. 표시 값 손실은 0이지만 '펼치기' 액션이 사라진다. |

---

## 6. 실행 계획

### Phase 0 — 기준 고정 (정보 유실 0의 전제)

**어떤 스타일도 건드리지 않는다.** 개편 전 현재 상태를 계약으로 굳히는 단계다.

| 작업 | 산출물 |
|------|--------|
| 28개 화면 정보 인벤토리 스냅샷 — 화면별 (표시 필드 / 표 컬럼 / 액션 버튼 / 상태값 / 빈 상태 문구 / 에러 문구 / 모달 / 탭) | `docs/40_UI_UX/화면_정보_인벤토리.md` |
| Playwright 시각 회귀 베이스라인: 28화면 × {dark, light} = 56 스냅샷. 현재 `toHaveScreenshot` 사용 0회 → 신규 도입 | `web/e2e/visual/*.spec.ts` + `__screenshots__/` |
| axe-core 접근성 + 대비 자동 검사 스펙 추가 (개편 전 실패 목록도 기록 = 개선 증거) | `web/e2e/a11y.spec.ts` |
| i18n 키 사용 현황 덤프 (개편 후 "쓰이지 않게 된 키" = 사라진 문자열 후보) | `docs/40_UI_UX/i18n_키_사용현황.md` |
| ko/en 키 수 불일치 11개 정리 | 커밋 2 (§6.1) |

**게이트**: 인벤토리를 팀이 리뷰·승인한 뒤에 Phase 1로 간다.

---

### Phase 1 — `DESIGN.md` 작성 + 토큰 파이프라인

| 작업 | 상세 |
|------|------|
| 동종 OSS 리서치 | ArgoCD / Grafana / Backstage / Rancher / Harbor 의 대비·밀도·상태색 조사 → DESIGN.md 근거 문단에 인용 |
| `web/DESIGN.md` 작성 | google design.md 스펙 준수 (§7 초안) |
| `src/theme/mui-theme.ts` | `createTheme({ cssVariables: { colorSchemeSelector: '[data-theme=%s]' }, colorSchemes: { light, dark } })` — 기존 `[data-theme]` 토글 방식과 그대로 맞물린다 |
| `index.css` 브릿지 | MUI 공식 Tailwind v4 통합 방식: `@layer theme, base, mui, components, utilities;` 선언 + `@theme inline`으로 `--color-*` → `var(--mui-palette-*)` 매핑 |
| `main.tsx` | `<StyledEngineProvider enableCssLayer>` + `<GlobalStyles styles="@layer theme, base, mui, components, utilities;" />` |
| `src/theme/ag-grid-theme.ts` | `themeQuartz.withPart(iconSetMaterial).withParams({...},'light').withParams({...},'dark')` — 값은 DESIGN.md 토큰 참조 |
| CI 게이트 | `npx @google/design.md lint web/DESIGN.md` 를 `lint-review.yml`에 추가. errors>0 이면 실패 |

**이 Phase의 핵심 이득**: `--color-surface-card`, `--color-border-default` 등 기존 토큰 이름을 유지하면서
값의 출처만 MUI 팔레트로 바꾼다 → **1,582곳의 `[var(--...)]` 클래스가 코드 수정 없이 새 팔레트를 따라간다.**
§2.1의 라이트 테마 붕괴가 이 시점에 대부분 해소된다.

**게이트**: 시각 회귀 스냅샷 diff를 팀이 육안 승인. 여기서는 "달라짐"이 정상이고, 검사 대상은 "정보가 사라졌는가"다.

---

### Phase 2 — 프리미티브를 MUI로 (어댑터 방식)

`components/ui/*`의 **파일 경로·export 이름·prop 시그니처를 그대로 두고 내부만 MUI로 바꾼다.**

| 파일 | 현재 | 교체 후 |
|------|------|---------|
| `ui/button.tsx` | 손수 구현, variant 5종 하드코딩 색 | MUI `Button` 래핑. `variant` 매핑: primary→`contained`, secondary→`contained color=secondary`, outline→`outlined`, danger→`contained color=error`, ghost→`text`. `loading` prop → MUI `loading` |
| `ui/input.tsx` | 생 input 래핑 | MUI `TextField size="small"` |
| `ui/native-select.tsx` | 생 select (+`index.css`의 select 강제 스타일 해킹 제거 가능) | MUI `Select`/`TextField select` |
| `ui/modal.tsx` | 손수 구현 | MUI `Dialog` (포커스 트랩·Esc·스크롤 락 공짜로 획득) |
| `ui/card.tsx` | **`iconBg`/`iconColor` 죽은 prop** | MUI `Paper` + elevation. **죽은 prop을 살려서** 디자인시스템 §2.3 기능별 색상 매핑 실제 구현 |
| `ui/skeleton.tsx` | 손수 구현 | MUI `Skeleton` |
| `ui/toast-provider.tsx` | sonner | **sonner 유지** (테마 토큰만 주입) — 교체 이득 없음 |
| `shared/confirm-dialog.tsx` | 손수 구현 | MUI `Dialog` 기반으로 재작성, prop 유지 |
| `shared/step-wizard.tsx` | 손수 구현 (3화면 사용) | MUI `Stepper` — **prop 유지 필수** |

**정보 유실 위험: 낮음.** 화면 파일 0줄 수정. 기존 컴포넌트 단위 테스트(`button.test.tsx`, `modal.test.tsx`,
`confirm-dialog.test.tsx`, `step-wizard.test.tsx`)가 그대로 계약이 된다.

이어서 **생 태그 흡수**: 생 `<button>` 95곳 → `<Button>`, 생 `<input>` 45곳 → `<Input>`, 생 `<select>` 4곳 → `<NativeSelect>`.
커밋 9~10으로 쪼갠다(§6.1).

---

### Phase 3 — 표를 `DataTable`(TanStack) 하나로 통일 ✅ **D6 결정: A안**

> **2026-08-11 실측 결과 — 이 Phase 는 착수했다가 되돌렸다.**
>
> `DataTable` 내부를 AG Grid 로 교체하고(ColumnDef → ColDef 런타임 번역으로 화면 7곳은
> 무수정) 테스트를 돌린 결과 **8개 파일 16개 테스트가 깨졌다.** 원인은 AG Grid 가 실제
> 레이아웃 측정(폭/높이 · ResizeObserver)에 의존해 행을 렌더하는데 jsdom 에는 그게
> 없어서, 표 안의 셀 내용이 **하나도 렌더되지 않는다**는 것이다.
> `ResizeObserver` + `offsetWidth/clientWidth` shim 을 넣어도 해결되지 않았다 —
> 렌더가 비동기라 `waitFor` 없이는 잡히지 않는다.
>
> 깨진 16건은 전부 `getByText` 로 표의 값을 확인하는 테스트다. §3 원칙 3 은
> `getByText` 실패를 **정보 유실 신호**로 규정하고 테스트를 고쳐 통과시키는 것을 금지한다.
> 이걸 통과시키려면 목록 화면 7곳(스택 목록·이력, CI/CD 목록·이력, 사용자 관리,
> 알림 규칙·이력)의 회귀 그물을 영구적으로 약화시켜야 한다. 그건 이 개편의 최상위
> 제약("정보 유실 0")과 맞바꾸는 거래라 혼자 결정하지 않았다.
>
> **결정 완료 (D6) — A안 채택**
>
> `ag-grid-community` / `ag-grid-react` 의존성과 `src/theme/ag-grid-theme.ts`,
> `vendor-grid` 청크를 제거했다. ESLint 가 `ag-grid-*` 전체를 error 로 막는다.
> 표는 `components/shared/data-table.tsx` 하나이며, 남은 통일 작업은 손으로 만든
> `<table>` 17곳을 이 컴포넌트로 흡수하는 것이다(커밋 12).
> 되돌리려면 아래 B/C 의 대가를 다시 검토한다.
>
> | 안 | 내용 | 대가 |
> |----|------|------|
> | **A ✅ 채택** | 현행 TanStack `DataTable` 유지. 이미 7화면이 같은 컴포넌트를 쓰므로 통일감 문제가 없다. AG Grid 는 정말 그리드가 필요한 신규 화면에만 쓴다 | 추가 비용 0. AG Grid 도입 효과는 포기 |
> | B | AG Grid 로 이관하고 16개 테스트를 `waitFor` 기반으로 재작성 | 목록 7화면의 동기 회귀 검증을 잃는다 |
> | C | AG Grid 이관 + Playwright E2E 로 목록 화면 검증을 보강해 잃은 그물을 대체 | E2E 작성·유지 비용, 실행 시간 증가 |
>
> 근거: 팀 논의에서 "그리드가 **필요할 경우에는** ag-grid 커뮤니티" 였고, 현행 DataTable
> 이 이미 정렬·필터·페이지네이션을 갖춘 그리드다. 새 라이브러리가 주는 것은 가상 스크롤·
> 열 고정 등인데 현재 데이터 규모에서 필요하지 않다.
>
> 아래 원래 계획은 훗날 B/C 를 재검토할 때의 실행안으로 남겨 둔다.

| 대상 | 작업 |
|------|------|
| `shared/data-table.tsx` (TanStack) | 내부를 `AgGridReact`로 교체, **`DataTableProps` 시그니처 유지** (`columns`/`data`/`getRowKey`/`onSort`/`onRowClick`/`emptyMessage`/`pageSize`/`toolbar`) |
| 사용 화면 7개 | user-management, alert-rules, alert-history, cicd-list, cicd-history, stack-list, stack-history — 컬럼 정의만 `ColumnDef` → `ColDef` 변환 |
| 손수 만든 `<table>` 17곳 | 데이터성 표 → `DataTable`. 레이아웃성/정적 표(`code-preview`, `shortcut-help-modal`, 호환성 매트릭스) → MUI `Table` 유지 |

**`cicd-history-page`의 행 확장 → 메인/서브 테이블 분리 (팀 제안 채택)**

AG Grid의 Master/Detail 은 Enterprise 전용이지만, **행 확장 자체를 없애고 메인 테이블 + 서브 테이블로
분리**하면 Enterprise도 Full Width Rows 같은 우회도 필요 없다. `AgGridReact` 2개를 나란히 두고
메인 그리드의 선택 행이 서브 그리드의 데이터 소스가 되는, Community 순정 구성이다.

조사 결과 이 방향이 단순한 대체가 아니라 **개선**이다:

| 근거 | 내용 |
|------|------|
| 현재 확장 패널에 **새 정보가 0** | `renderExpanded`가 보여주는 6필드(Pipeline/Version/Triggered By/Status/Started At/Completed At)가 **메인 테이블 컬럼 6개와 완전히 동일**하다. 같은 행을 세로로 다시 그린 것이다. |
| 진짜 상세 데이터는 **이미 API에 있으나 이 화면이 안 쓴다** | `cicd-api.ts:288` `getDeployment(id)` 가 `steps[]`(단계명·상태·메시지·로그)를 반환한다. `Deployment` 타입(`types/index.ts:377`)에는 8필드뿐 — 하위 엔티티는 목록 응답이 아니라 **상세 조회에만** 있다. 즉 deployment 1 : N steps 라는 진짜 master/detail 관계가 이미 존재한다. |
| 행 확장은 **28화면 중 1곳뿐인 일회성 패턴** | 반면 좌우 분할 상세는 `shared/list-detail-panel.tsx` 로 컴포넌트화돼 cluster / organization / stack-versions **3화면**에서 쓰인다. 통일감 관점에서 행 확장을 없애는 게 맞다. |
| 확장을 검증하는 **테스트가 없다** | `cicd-history-page.test.tsx` 의 3개 테스트는 heading·행·상태만 본다 → 회귀 위험 낮음 |

**구현 (2단계로 분리)**

```
┌─ 메인: 배포 이력 (AgGridReact #1) ─────────────────────┐
│ Pipeline │ Version │ Status │ Deployer │ Started │ Done │  ← 행 선택
└────────────────────────────────────────────────────────┘
             ▼ rowSelection: 'singleRow' → selectedId
┌─ 서브: 선택 배포의 단계 (AgGridReact #2) ──────────────┐
│ Step │ Status │ Message │ (로그 보기)                   │
└────────────────────────────────────────────────────────┘
```

- **3-a (개편 범위, 필수)**: 행 확장 제거 + 메인/서브 2그리드 골격. 서브 그리드는 선택 행의 6필드를 표시해
  **정보 동등을 먼저 보장**한다. 실제로 그 6필드는 이미 메인 컬럼에 다 있으므로 이 시점에 정보 유실은 0이다.
- **3-b (권장, 별도 티켓)**: 서브 그리드를 `useDeploymentStatus(selectedId)` 의 `steps[]` 로 교체.
  이건 **기능 추가**(§1.2 비목표)이므로 이 개편 브랜치에 넣지 않고 별 티켓으로 뺀다. 다만 원래 이 확장
  패널이 의도했던 것이 이쪽으로 보인다.

> **팀 승인 필요 (D5)**: 3-a에서 '펼치기' **액션**이 사라진다. 표시되는 값은 하나도 줄지 않지만
> §3 원칙 2("액션은 못 버린다")에 걸리므로 인벤토리 대조 시 명시적 승인을 받는다.

**정보 유실 위험: 중.** 표는 컬럼이 정보다. **커밋 11~13 메시지 본문에 Phase 0 인벤토리의 컬럼 목록과
개편 후 컬럼을 나란히 적고, 최종 PR 본문에 취합한다.**

---

### Phase 4 — 레이아웃·내비게이션·밀도

| 대상 | 작업 |
|------|------|
| `layout/sidebar.tsx` | MUI `List`/`ListItemButton`/`Collapse`. 240px↔64px 접기, 역할별 필터링, active 상태 유지 |
| `layout/header.tsx` | MUI `AppBar` + `Toolbar` (56px 유지). 언어/테마/사용자 메뉴 → `Menu` |
| `layout/page-header.tsx` | 제목 + 검색 + 액션 슬롯 규격 통일 → 28화면 상단 룩 일치 |
| `shared/breadcrumb.tsx` | MUI `Breadcrumbs` |
| `shared/list-detail-panel.tsx` | 좌우 분할 규격 통일 (cluster, organization, stack-versions 3화면) |
| `shared/status-badge.tsx` | MUI `Chip` 기반으로 재작성 + **각 화면이 자체 제작한 배지를 전부 여기로 흡수** (현재 사용 1곳) |
| 밀도 토큰 | `--page-padding: 48px` → 32px 등 재검토. **밀도를 높이되 정보를 줄이지 않는다** |
| 거대 화면 분해 | `stack-install-page.tsx` **3,662줄**, `cicd-list-page.tsx` 2,153줄, `developer-deploy-page.tsx` 1,421줄 → 섹션 컴포넌트 분리. **동작 변경 금지, 순수 추출만** |

**정보 유실 위험: 높음** (특히 stack-install 3,662줄). **화면 하나 = 커밋 하나**(커밋 17~19)로 쪼개고,
인벤토리 대조를 필수로 한다.

---

### Phase 5 — 차트 단일화

| 작업 | 상세 |
|------|------|
| chart.js 사용처 1파일(`stack-monitoring-overview.tsx`)을 recharts로 이관 | 차트 종류·계열·축·툴팁 라벨 **동일 유지** |
| `chart.js` / `react-chartjs-2` 의존성 제거 | |
| 차트 팔레트를 DESIGN.md 토큰에서 주입 | 계열색은 색만으로 구분하지 않도록 패턴/라벨 병행 (§8 접근성) |
| 대상 | `monitoring-cicd-view`, `monitoring-cluster-view`, `stack-monitoring-overview`, `cicd-list-page` |

---

### Phase 6 — 하드코딩 청산 + 접근성 검증

| 작업 | 상세 |
|------|------|
| 잔여 hex 767 / rgba 750 제거 | 화면군별로 나눠 토큰 치환. **커밋마다 잔여 개수를 숫자로 기록** |
| ESLint 규칙 추가 | TSX 내 hex 리터럴 금지 (`no-restricted-syntax`), `ag-grid-enterprise` import 금지 |
| `@theme inline` 시맨틱 유틸리티로 축약 | `[var(--color-text-secondary)]` → `text-secondary` (1,582곳 점진 정리, 필수 아님) |
| 최종 접근성 감사 | axe-core 0 violation, 대비 AA 100%, 키보드 전 경로 도달, 포커스 링 가시성 |
| 번들 예산 확인 | MUI+emotion+ag-grid 추가 vs tanstack-table+chart.js 제거. **실측 후 판단** — 초기 로드 회귀 시 코드 분할로 대응 |
| 문서 갱신 | `Nullus_디자인시스템.md`를 "DESIGN.md 참조" 문서로 축소 (값 중복 제거) |

---

### 6.1 브랜치 · 커밋 전략 — 단일 브랜치, 최종 1 PR

**단일 장기 브랜치에서 끝까지 작업하고, 완성본을 PR 하나로 올린다.** 커밋은 아래처럼 쪼개
리뷰어가 커밋 단위로 따라 읽을 수 있게 한다.

```
main
 └── refactor/ui/design-system-overhaul     ← 이 브랜치 하나로 Phase 0~6 전부
```

브랜치 타입은 `docs/20_개발가이드/Nullus_PR_커밋_컨벤션.md`(v3)의 3종(`feat|fix|chore`) 제약을 받는다.
`refactor` 는 커밋 타입 전용이므로 **브랜치명은 `chore/ui/design-system-overhaul`** 을 쓴다.

**진행 현황 (2026-08-11 기준, 브랜치 `chore/ui/design-system-overhaul`)**

| 상태 | 커밋 | 비고 |
|------|------|------|
| ✅ | 1 인벤토리 + 시각 회귀 베이스라인 | 화면 28개 / 스냅샷 58장 |
| ✅ | 2 i18n 키 정합 | 누락 9개 + 정합 테스트 4건 |
| ✅ | 3 라이트 테마 대비 교정 | **가독성 문제 해소** — 부록 A1·A2·A4 |
| ✅ | 4 DESIGN.md 신설 | lint 0 errors / 0 warnings |
| ✅ | 5 CI 게이트 | design.md lint + 인벤토리 대조 + 테마 신선도 |
| ✅ | 6 토큰 파이프라인 | DESIGN.md → MUI · Tailwind · AG Grid. 깨진 토큰 3개 복구 |
| ✅ | 7 프리미티브 6종 MUI 이관 | 화면 파일 0줄 수정 (커밋 8 포함) |
| ✅ | 11 D6 결정 = A안 | AG Grid 의존성·테마·청크 제거, ESLint 로 재도입 차단 |
| ✅ | 13 행 확장 → 메인·서브 테이블 | 순서를 11 앞으로 옮겼다 |
| ✅ | 22 ESLint 규칙 | hex 1335건 warn, 상용 라이선스 error |
| ✅ | 24 문서 갱신 | 디자인시스템 문서를 DESIGN.md 참조로 |
| ✅ | 15·16 앱 셸 · 밀도 | `PageHeader` 채택 0 → 24화면, 레이아웃 토큰을 DESIGN.md 로 |
| ⬜ | 9·10 생 태그 흡수 | button 95 / input·select 49 |
| ⬜ | 12 수제 table 17곳 → DataTable 흡수 | D6=A 로 대상 확정 |
| ⬜ | 14 StatusBadge 통합 | 현재 사용 1곳, 화면별 자체 배지 흡수 |
| ⬜ | 17~19 거대 화면 순수 추출 | stack-install 3,662줄 등 |
| ⬜ | 20 차트 단일화 | ESLint 경고로 표시해 뒀다 |
| ⬜ | 21 하드코딩 색 청산 | ESLint 경고 1335건이 대상 목록 |
| ➖ | 23 tanstack-table 제거 | D6=A 라 해당 없음 (계속 사용) |

측정값: vitest **640/640**, tsc 통과, vite build 통과,
시각 회귀 **58/58**, eslint **0 errors / 72 warnings**(전부 잔여 hex — 커밋 21 대상),
design.md lint **0 errors**, 대비 감사 **45/45**.

> ⚠️ **인벤토리 `--check` 가 3건 빨강이다.** `stack-monitoring-overview.tsx` 의
> jsxText `(Req)` · `(Lim)` · `0` 이 커밋 21bb2d1(KPI 라벨 겹침 교정)에서 사라졌는데
> 스냅샷 승인을 안 받았다. 같은 정보는 `CPU Req/Limit` · `Mem Req/Limit` 라벨이
> 그대로 들고 있으므로 의도된 제거로 보이지만, §8.1 절차상 **승인 후 스냅샷 갱신**이
> 필요하다. 커밋 15·16 은 이 3건 외에 새 유실을 만들지 않았다(대조 완료).

**커밋 15·16 에서 실제로 한 일**

| 무엇 | 상세 |
|------|------|
| `PageHeader` 규격 신설 + 채택 | 파일은 있는데 **쓰는 화면이 0곳**이었다. 28화면이 각자 `mb-6/mb-7` + 아이콘 + `text-[22px] font-extrabold` 를 손으로 다시 만들고 있었고, `items-start`/`items-center`·`justify-between` 유무가 제각각이라 상단 룩이 미묘하게 어긋났다 — "흐트러진 느낌"의 절반이 여기였다. 24화면을 흡수했다(로그인·404·홈은 셸 밖 화면이라 제외) |
| 레이아웃 토큰을 DESIGN.md 로 | `--sidebar-width` 등이 `generate-theme.mjs` 에 하드코딩돼 "단일 출처" 계약에 구멍이 있었다. front matter 의 `layout` 블록으로 끌어올렸다 |
| 밀도 상향 | 표 행 40→32, 헤더 36→28, 카드 여백 16→12, 페이지 여백 32→24, 헤더 56→44, 사이드바 접힘 64→48. 1080p 목록 기준 한 화면 6줄 증가 |
| 라운드 축소 | `rounded` 6/10/12 → 4/6/8 |
| 아래로 펼치던 상세 → 좌우 분할 | `alert-history`, `stack-history`. `ListDetailPanel` 에 `detailWidth` 를 더해 "넓은 표 + 고정폭 사이드 레일" 배치를 같은 컴포넌트로 처리한다. `DataTable` 에 `flush` 를 더해 액자 겹침을 없앴다 |
| 잔재 정리 | 다크 body 의 남보라 그라데이션(`#0a0a0a→#1a1a2e→#16213e`)과 홈 히어로 그라데이션 제거. `typography.h1` 이 2rem 인데 쓰는 화면이 0곳이라 실제 값(1.375rem)으로 정정 |
| 고친 버그 | `ListDetailPanel` 이 `listWidth` 를 240/280 만 클래스로 매핑하고 나머지를 조용히 280 으로 접고 있었다(Tailwind 임의값은 런타임 값으로 안 만들어진다) → 인라인 스타일로. MUI Button 에 `gap` 이 없어 아이콘이 JSX 줄바꿈 여부에 따라 글자에 붙었다 떨어졌다 했다 |

---

| # | 커밋 | Phase |
|---|------|-------|
| 1 | `test(ui): 개편 전 화면 정보 인벤토리와 시각/a11y 베이스라인 고정` | 0 |
| 2 | `fix(ui): ko/en i18n 키 불일치 11개 정리` | 0 |
| 3 | `fix(ui): 라이트 테마 카드·보더·다크 muted 대비 즉시 교정` | 부록 A (A1·A2·A4) |
| 4 | `docs(ui): DESIGN.md 디자인 단일 출처 신설` | 1 |
| 5 | `ci(ui): design.md lint 대비 게이트 추가` | 1 |
| 6 | `feat(ui): MUI 테마 + Tailwind @theme 브릿지 + AG Grid 테마 파이프라인` | 1 |
| 7 | `refactor(ui): 프리미티브 9종 내부를 MUI로 교체 (공개 API 유지)` | 2 |
| 8 | `fix(ui): Card iconBg/iconColor 죽은 prop 복구 및 기능별 색상 매핑 적용` | 2 |
| 9 | `refactor(ui): 생 button 95곳을 Button 컴포넌트로 흡수` | 2 |
| 10 | `refactor(ui): 생 input·select 49곳을 Input/NativeSelect로 흡수` | 2 |
| 11 | `refactor(ui): DataTable 내부를 AG Grid Community로 교체` | 3 |
| 12 | `refactor(ui): 손수 만든 table 17곳을 DataTable·MUI Table로 이관` | 3 |
| 13 | `refactor(cicd): 배포 이력 행 확장을 메인·서브 테이블 분리로 대체` | 3 |
| 14 | `refactor(ui): StatusBadge를 Chip 기반으로 통합하고 화면별 자체 배지 흡수` | 4 |
| 15 | `refactor(ui): 앱 셸(Sidebar·Header·PageHeader·Breadcrumb) MUI 이관` | 4 |
| 16 | `refactor(ui): 밀도 토큰 재조정` | 4 |
| 17 | `refactor(stack): stack-install-page 섹션 컴포넌트 순수 추출` | 4 |
| 18 | `refactor(cicd): cicd-list-page 섹션 컴포넌트 순수 추출` | 4 |
| 19 | `refactor(cicd): developer-deploy-page 섹션 컴포넌트 순수 추출` | 4 |
| 20 | `refactor(o11y): 차트를 recharts로 단일화하고 chart.js 제거` | 5 |
| 21 | `refactor(ui): 잔여 하드코딩 색 전면 토큰 치환` | 6 |
| 22 | `ci(ui): hex 리터럴·ag-grid-enterprise import 금지 ESLint 규칙` | 6 |
| 23 | `chore(ui): tanstack-table 의존성 제거` | 6 |
| 24 | `docs(ui): 디자인시스템 문서를 DESIGN.md 참조로 갱신` | 6 |

**커밋 규율** (긴 브랜치에서 리뷰 가능성을 유지하는 장치)

- 커밋 하나는 되돌릴 수 있는 단위로 유지한다. 커밋마다 `npm run build` + vitest 577개가 통과해야 한다.
- **커밋 17~19(거대 화면 분해)는 순수 추출만.** 동작·마크업 변경을 같은 커밋에 섞지 않는다.
- 커밋 13은 유일하게 사용자 인터랙션이 바뀌는 지점이다(§5.3 D5) → 단독 커밋으로 격리한다.
- 커밋 21은 화면군별로 더 쪼개도 좋다. 메시지 본문에 `잔여 hex N → M` 을 적는다.
- **`main` 을 주 1회 이상 브랜치로 머지(또는 리베이스)** 한다 → R8 대응.

**최종 PR** 제목: `refactor(ui): 디자인 시스템 단일 출처화와 MUI·AG Grid 전면 이관`
본문은 `.github/pull_request_template.md` 구조(`## Summary` / `## Changes` / `## Testing` + `Scope:`)를
따르고, §8.1 UI 체크리스트와 화면별 before/after를 함께 싣는다.

> 최종 PR이 커진다는 점은 인정한다(28화면 규모). 그래서 **리뷰 단위를 PR이 아니라 커밋과
> Phase 게이트로 옮긴다** — Phase 0·1·2 종료 시점에 브랜치 상태로 팀에 공유해 중간 리뷰를 받고,
> 마지막 PR은 이미 합의된 내용의 취합이 되게 한다. PR 컨벤션 검사는
> `pull_request: [opened, reopened]` 에서만 돌므로 제목·본문을 고쳤다면
> `gh pr close <n> && gh pr reopen <n>` 로 재검사한다.

### 6.2 어댑터 방식이 왜 핵심인가

18,823 LOC의 화면 28개를 MUI로 "다시 쓰면" 정보 유실이 필연이다.
대신 프리미티브 9개 파일의 **내부만** 바꾸면 124곳의 `<Button>`, 95곳의 `<Input>`, 7화면의 `<DataTable>`이
한 번에 통일된다. 화면 파일은 Phase 4에서 **레이아웃 이유로만** 손댄다.

---

## 7. `DESIGN.md` 초안 (Phase 1 산출물 스켈레톤)

아래 토큰 값은 **대비를 실제로 계산해 AA 통과를 확인한 값**이다. 팀 리뷰에서 조정 가능.

```md
---
version: alpha
name: Nullus Platform
description: Kubernetes DevSecOps 자동화 콘솔. 다크 우선, 데이터 밀집 운영 도구.
colors:
  # ── Light scheme ──
  light-bg:            "#f4f6f8"   # 페이지 배경
  light-surface:       "#ffffff"   # 카드/패널 (배경과 반드시 다른 값)
  light-divider:       "#dfe4ea"   # 은은한 경계선 (검정 hairline 금지)
  light-text:          "#0f172a"   # 17.85:1
  light-text-secondary:"#44546a"   #  7.71:1
  light-text-muted:    "#5f6f85"   #  5.12:1
  light-primary:       "#4338ca"   #  7.90:1
  light-success:       "#047857"   #  5.48:1
  light-warning:       "#a15c07"   #  5.19:1
  light-error:         "#c81e1e"   #  5.74:1
  light-info:          "#1d4ed8"   #  6.70:1
  # ── Dark scheme ──
  dark-bg:             "#0b0f14"
  dark-surface:        "#141b24"
  dark-divider:        "#26303d"
  dark-text:           "#e9eff7"   # 14.98:1
  dark-text-secondary: "#a9b8ca"   #  8.58:1
  dark-text-muted:     "#8496a9"   #  5.71:1  (현행 #64748b 3.89:1 → 개선)
  dark-primary:        "#8f9bff"   #  6.85:1
  dark-success:        "#3ddc97"   #  9.80:1
  dark-warning:        "#f5b544"   #  9.55:1
  dark-error:          "#ff8080"   #  7.14:1
  dark-info:           "#6aa8fb"   #  7.10:1
  # ── Brand (면/로고 전용, 텍스트·보더로 사용 금지) ──
  brand-gold:          "#ffd700"
typography:
  h1:        { fontFamily: Inter, fontSize: 2rem,     fontWeight: 700, lineHeight: 1.25 }
  h2:        { fontFamily: Inter, fontSize: 1.125rem, fontWeight: 700, lineHeight: 1.3 }
  h3:        { fontFamily: Inter, fontSize: 0.875rem, fontWeight: 700, lineHeight: 1.4 }
  body-md:   { fontFamily: Inter, fontSize: 0.875rem, fontWeight: 400, lineHeight: 1.6 }
  label-sm:  { fontFamily: Inter, fontSize: 0.75rem,  fontWeight: 600, lineHeight: 1.5 }
  code:      { fontFamily: "Fira Code", fontSize: 0.8125rem, fontWeight: 400, lineHeight: 1.5 }
spacing:  { xs: 4px, sm: 8px, md: 12px, lg: 16px, xl: 24px, "2xl": 32px }
rounded:  { sm: 6px, md: 10px, lg: 12px, full: 9999px }
components:
  button-primary:   { backgroundColor: "{colors.light-primary}", textColor: "#ffffff",
                      rounded: "{rounded.md}", padding: 10px }
  card:             { backgroundColor: "{colors.light-surface}", rounded: "{rounded.lg}", padding: 16px }
  badge-success:    { textColor: "{colors.light-success}", rounded: "{rounded.full}", padding: 4px }
---

## Overview
## Colors
## Typography
## Layout
## Elevation & Depth     ← 현행 토큰에 완전히 없는 섹션. 라이트 테마 "면 분리"의 핵심
## Shapes
## Components
## Do's and Don'ts
```

### 7.1 `Elevation & Depth` — 가장 중요한 신규 섹션

라이트 테마가 스켈레톤처럼 보이는 근본 이유는 **면을 나누는 수단이 보더밖에 없었다**는 것이다.
DESIGN.md에 elevation 3단계를 정의하고 라이트/다크가 **다른 수단**으로 깊이를 표현한다.

| 단계 | 용도 | 라이트 | 다크 |
|------|------|--------|------|
| `flat` | 페이지 배경 | `light-bg` | `dark-bg` |
| `raised` | 카드, 패널 | `light-surface` + `0 1px 2px rgba(15,23,42,.06)` + `light-divider` 1px | `dark-surface` + 보더 `dark-divider` (그림자 없음) |
| `overlay` | 모달, 드롭다운, 팝오버 | `light-surface` + `0 8px 24px rgba(15,23,42,.12)` | `dark-surface` 밝게 + 오버레이 `rgba(0,0,0,.7)` |

**라이트는 그림자로, 다크는 밝기 차로 깊이를 만든다.** 이게 §2.1 "대비 1.00:1" 문제의 정면 해결이다.

### 7.2 `Do's and Don'ts` (초안)

- ✅ 색은 항상 토큰으로. TSX에 hex를 쓰지 않는다.
- ✅ 상태는 색 + 아이콘 + 텍스트를 함께 쓴다 (색맹 사용자 / 흑백 인쇄).
- ✅ 표는 `DataTable`. 정적/레이아웃 표만 `Table`.
- ✅ 밀도를 높이려면 여백을 줄인다. **정보를 줄이지 않는다.**
- ❌ 골드를 텍스트·보더·아이콘 색으로 쓰지 않는다 (흰 배경에서 1.40:1).
- ❌ 라이트 테마에서 `400`대 톤(`#34d399`, `#fbbf24`, `#818cf8`)을 텍스트로 쓰지 않는다.
- ❌ 라이트 테마 보더에 `gray-800` 이상 어두운 값을 쓰지 않는다 (와이어프레임 룩).
- ❌ 새 차트/그리드/아이콘 라이브러리를 추가하지 않는다.

---

## 8. 정보 유실 0 — 5중 안전망

| # | 장치 | 잡아내는 것 |
|---|------|-------------|
| 1 | **화면 정보 인벤토리** (Phase 0) | 필드/컬럼/액션/상태/빈상태/에러 문구 목록. 커밋마다 대조 |
| 2 | **vitest 577개 무수정 원칙** | `getByText` 329 / `getByRole` 184 / `getByLabelText` 103 이 보이는 텍스트·역할·라벨을 계약으로 검증. 실패 = 정보 유실 신호 |
| 3 | **Playwright 시각 회귀** 56 스냅샷 | 레이아웃 붕괴, 잘림, 사라진 섹션 |
| 4 | **i18n 키 사용 현황 diff** | 개편 후 참조가 끊긴 키 = 사라진 문자열 |
| 5 | **Playwright E2E 17스펙** | 다단계 흐름(스택 설치 5단계, 배포, 재시도, 역할별 UAT) 동작 보존 |

### 8.1 체크리스트

단일 브랜치이므로 검증을 **커밋 단위(작업 중)** 와 **최종 PR(취합)** 두 층으로 나눈다.

**커밋마다 (self-check, 커밋 메시지 본문에 기록)**

```
- [ ] 인벤토리 대조: 이 커밋이 건드린 화면의 필드/컬럼/액션이 개편 전과 동일
- [ ] vitest 577개 무수정 통과 (수정했다면 사유 명시)
- [ ] npm run build 통과
- [ ] 잔여 하드코딩 색: 이전 N → 현재 M
```

**Phase 게이트마다 (팀 중간 리뷰 — 브랜치 상태로 공유)**

| 게이트 | 공유물 |
|--------|--------|
| Phase 0 종료 | 화면 정보 인벤토리 → **팀 승인 필수** |
| Phase 1 종료 | `DESIGN.md` + 토큰 적용 후 시각 회귀 diff 56장 |
| Phase 2 종료 | 밀도 프로토타입(stack-list) → **D3 승인** |
| Phase 3 중 | 커밋 13 메인/서브 테이블 → **D5 승인** |

**최종 PR 본문** (`.github/pull_request_template.md` 구조 + 아래 UI 섹션)

```
## UI 개편 체크
- [ ] 화면 28개 전부 인벤토리 대조 완료 (화면별 표)
- [ ] vitest 577개 무수정 통과 / E2E 17스펙 통과
- [ ] 화면별 before/after 스크린샷 (dark, light)
- [ ] axe-core violation 0, 대비 AA 100%
- [ ] TSX 내 hex 리터럴 0 (ESLint 강제)
- [ ] design.md lint errors 0
- [ ] 번들 크기 before/after
Scope: web/ 프론트엔드 전체 UI 레이어. 백엔드·API 변경 없음.
```

---

## 9. 리스크와 대응

| # | 리스크 | 영향 | 대응 |
|---|--------|------|------|
| R1 | **MUI × Tailwind 스타일 충돌** | 높음 | MUI 공식 Tailwind v4 통합 경로 사용: `@layer theme, base, mui, components, utilities` 순서 고정 + `StyledEngineProvider enableCssLayer`. Phase 1 첫 작업으로 검증하고, 실패 시 Phase 2 이후 전체를 재검토 |
| R2 | **Material 기본 룩이 데이터 밀도를 떨어뜨림** | 높음 | 테마에서 `size="small"` 기본화 + spacing 축소. Phase 2 진입 전 stack-list 1화면으로 밀도 프로토타입을 만들어 팀 승인 |
| R3 | **`stack-install-page.tsx` 3,662줄 분해 중 로직 유실** | 높음 | 순수 추출만(동작 변경 금지). 단독 커밋(17). 5단계 E2E 스펙 필수 통과 |
| R4 | **AG Grid Master/Detail Enterprise 제약** | **해소** | 행 확장을 없애고 메인/서브 테이블 분리로 대체 → Enterprise 모듈도 Full Width Rows 우회도 불필요 (§Phase 3, D5) |
| R5 | **번들 크기 증가** | 중 | tanstack-table + chart.js 제거로 일부 상쇄. Phase 6에서 실측하고 회귀 시 라우트별 코드 분할 |
| R6 | **`ag-grid-enterprise` 오도입 (라이선스)** | 중 | ESLint `no-restricted-imports` 금지 규칙 |
| R7 | **Phase 중간 상태에서 룩이 반쪽(구/신 혼재)** | 중 | 어댑터 방식이라 Phase 2 종료 시점에 대부분 통일됨. Phase 2를 최우선 완주 |
| R8 | **장기 단일 브랜치가 `main` 과 벌어진다** | **높음** (단일 PR 전략의 최대 비용) | 개편이 28화면·18,823 LOC을 건드리므로 같은 기간 기능 작업과 충돌 지점이 넓다. ① `main` 을 **주 1회 이상** 브랜치로 머지 ② Phase 4의 대형 화면 커밋(17~19)은 해당 화면 기능 작업이 없는 시점에 배치 ③ 개편 기간에 `web/src/components/ui/*` 와 `index.css` 는 이 브랜치가 단독 소유임을 팀에 공지 ④ 기간이 길어지면 Phase 2 종료 시점을 중간 머지 지점으로 쓸 수 있게 커밋 1~10을 언제든 단독으로 떼어낼 수 있는 상태로 유지 |
| R9 | **최종 PR이 커서 리뷰가 형식적으로 흐른다** | 중 | 리뷰 단위를 PR이 아니라 커밋 + Phase 게이트로 옮긴다(§8.1). Phase 0·1·2 종료 시 브랜치 상태로 중간 리뷰를 받아, 최종 PR은 합의된 내용의 취합이 되게 한다 |

---

## 10. 완료 기준 (DoD)

| 항목 | 기준 |
|------|------|
| 대비 | 라이트/다크 **모두** 본문·보조·상태색 AA 4.5:1 이상, axe-core violation 0 |
| 면 분리 | 라이트 카드 vs 페이지 배경이 색 또는 그림자로 구분됨 (현행 1.00:1 → 해소) |
| 토큰 준수 | TSX 내 hex 리터럴 0, ESLint 규칙으로 강제 |
| 컴포넌트 통일 | 생 `<button>`/`<input>`/`<select>` 0, 데이터성 표 전부 `DataTable` |
| 라이브러리 | 차트 1개, 그리드 1개, 아이콘 1개 |
| 단일 출처 | `web/DESIGN.md` 하나만 고치면 MUI·Tailwind·AG Grid가 함께 바뀜 |
| CI | `design.md lint` errors 0 게이트 통과 |
| 정보 유실 | vitest 577개 무수정 통과, E2E 17스펙 통과, 인벤토리 대조 전 화면 완료 |
| 문서 | `Nullus_디자인시스템.md`가 DESIGN.md를 참조하도록 갱신 (값 중복 제거) |

---

## 부록 A. 즉시 수정 가능한 결함 (Phase와 무관하게 지금 고칠 수 있음)

| # | 위치 | 내용 |
|---|------|------|
| A1 | `web/src/index.css:78` | 라이트 `--color-border-default: #1f2937` → 문서값 `#e2e8f0` 계열로. **한 줄로 스켈레톤 룩 대부분 완화** |
| A2 | `web/src/index.css:77` + `:93` | 라이트 `--color-surface-card`(`#f8fafc`)와 `body` 배경(`#f8fafc`)이 동일 → 카드 `#ffffff`, 배경 `#f4f6f8`로 분리 |
| A3 | `web/src/components/ui/card.tsx:29-30` | `iconBg`/`iconColor` prop이 삼항 양쪽 같은 값이라 무조건 indigo. 죽은 코드 |
| A4 | `web/src/index.css:23` | 다크 `--color-text-muted: #64748b` → 3.89:1, AA 미달 |
| A5 | `web/src/i18n/` | en 1,170 / ko 1,159 키 수 불일치 11개 |
| A6 | `web/src/components/shared/status-badge.tsx` | 만들었지만 1곳만 사용 — 화면별 자체 배지가 룩 파편화의 원인 |

> A1·A2만 먼저 반영해도 배포본 가독성 체감은 즉시 개선된다. Phase 0 인벤토리와 병행 가능.

## 부록 B. 화면 목록과 개편 등급

**등급** — A: 토큰만 적용(코드 무변경) / B: 프리미티브·표 교체 / C: 레이아웃 재설계 필요

| 화면 | LOC | 등급 | 비고 |
|------|-----|------|------|
| `stack-install-page` | 3,662 | **C** | 5단계 워크플로우. 최고 위험 |
| `cicd-list-page` | 2,153 | **C** | DataTable + 차트 |
| `developer-deploy-page` | 1,421 | **C** | 배포 위자드 |
| `stack-template-page` | 1,241 | B | 카드 그리드 + 모달 |
| `user-management-page` | 939 | B | DataTable + 생 table 혼재 |
| `cluster-page` | 872 | B | ListDetailPanel |
| `stack-list-page` | 865 | B | DataTable + 차트 |
| `cicd-template-page` | 690 | B | |
| `organization-page` | 626 | B | ListDetailPanel + 생 table |
| `cicd-pipeline-setup-page` | 609 | B | |
| `stack-deployment-logs-page` | 593 | B | 생 table |
| `stack-add-tools-page` | 564 | B | 생 table |
| `stack-deploy-page` | 544 | B | 실시간 로그 |
| `stack-history-page` | 503 | B | DataTable |
| `stack-versions-page`(admin) | 466 | B | ListDetailPanel + 생 table |
| `alert-rules-page` | 458 | B | DataTable |
| `cicd-golden-path-page` | 388 | B | |
| `stack-oss-resource-default-page` | 371 | B | 생 table |
| `cicd-pipeline-logs-page` | 302 | B | |
| `home-page` | 243 | B | 히어로 + 카드 |
| `alert-history-page` | 241 | B | DataTable |
| `monitoring-page` | 206 | B | 차트 |
| `cicd-history-page` | 195 | **B+** | **행 확장 제거 → 메인/서브 테이블 분리** (D5 승인 필요) |
| `login-page` | 187 | A | |
| `stack-version-page` | 175 | B | 호환성 매트릭스 |
| `token-management-page` | 165 | B | 생 table |
| `known-issues-page` | 116 | B | 생 table |
| `not-found-page` | 28 | A | |

합계 **28화면 / 18,823 LOC** — A 2, B 23, C 3.
