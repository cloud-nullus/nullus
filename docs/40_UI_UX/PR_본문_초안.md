## Summary

배포본 가독성 지적(2026-08 팀 논의 "5) UI 정리 및 디자인 가이드")을 출발점으로 UI/UX 전면 개편에 착수했다. 이 PR 은 **개편의 기반 아키텍처와 가독성 문제 해소**까지를 담는다.

지적된 "흰색 배경에서 스켈레톤 같은 느낌"의 원인을 측정으로 특정했다. 라이트 테마에서 카드 배경과 페이지 배경이 **같은 색(대비 1.00:1)** 이었고, 면을 나누는 유일한 수단인 보더가 `#1f2937`(거의 검정, 14.03:1)이었다. 카드가 배경에 녹아 사라지고 검은 hairline 만 남아 문자 그대로 와이어프레임처럼 보였다. 그 위에 TSX 에 하드코딩된 색 1,517곳(hex 767 + rgba 750)이 전부 다크 기준이라 라이트에서 1.6:1 까지 무너져 있었다.

근본 해결로 **디자인 단일 출처**를 세웠다. `web/DESIGN.md`([google-labs-code/design.md](https://github.com/google-labs-code/design.md) 스펙)를 사람이 고치는 유일한 곳으로 두고, 거기서 MUI 테마 · Tailwind 토큰 · AG Grid 테마를 코드젠으로 파생시킨다. 문서와 구현이 갈라진 것이 이 문제의 발단이었으므로(기존 디자인시스템 문서는 라이트 보더를 `#e2e8f0`으로 적어놨지만 구현은 `#1f2937`이었다) 구조적으로 갈라질 수 없게 만들었다.

컴포넌트 통일은 **어댑터 방식**으로 했다. `components/ui/*` 의 공개 API 를 그대로 두고 내부만 MUI 로 바꿔서, 28개 화면 18,823 LOC 을 **한 줄도 고치지 않고** Button 124곳 · Input 95곳 · Modal 11곳이 한 번에 통일됐다.

기획안: `docs/40_UI_UX/Nullus_UIUX_전면개편_기획안.md`

## Changes

**가독성 (지적 사항 직접 해소)**
- 라이트: 카드 `#ffffff` / 페이지 `#f4f6f8` 로 분리 (1.00:1 → 1.08:1), 보더 `#1f2937` → `#cbd5e1` (14.03:1 → 1.48:1)
- 다크: `--color-text-muted` 3.89:1 → 6.10:1 (AA 미달 해소)
- elevation 3단 신설 — 라이트는 그림자, 다크는 표면 밝기 차. 개편 전 그림자 토큰이 0개였다

**디자인 단일 출처**
- `web/DESIGN.md` 신설 (design.md 스펙, lint 0 errors / 0 warnings)
- `scripts/generate-theme.mjs` → `tokens.generated.ts` / `tokens.generated.css`
- MUI × Tailwind v4 통합: `@layer theme, base, mui, components, utilities` + `StyledEngineProvider enableCssLayer` + `@theme inline`
- 밀도 규칙 명문화 (`size="small"`, 행 40px, 헤더 36px) — MUI 기본 여백을 그대로 쓰면 한 화면의 행 수가 줄고 그건 정보 손실이다

**컴포넌트 (화면 파일 0줄 수정)**
- Button / Input / NativeSelect / Skeleton / Modal / Card → MUI 내부 교체
- Modal: 손으로 만든 포커스 트랩 ~70줄 제거 (MUI Modal 이 동등)
- `Card` 의 `iconBg`/`iconColor` 죽은 prop 복구 — 삼항 양쪽이 같은 값이라 항상 indigo 였다

**발견해서 함께 고친 버그**
- 정의 없이 참조되던 토큰 3개: `--color-primary`(5곳) · `--color-border`(2곳) · `--color-text-tertiary`(1곳). 특히 `--color-primary` 는 "흰 텍스트 + primary 배경" 버튼의 배경이라 라이트 테마에서 사실상 안 보이는 버튼이었다
- 라이트만 `surface-base`/`surface-card` 가 다크와 뒤집혀 있던 의미 불일치
- MUI ThemeProvider 와 zustand 가 `data-theme` 속성을 다투어 라이트 테마가 다크로 렌더되던 회귀 (시각 회귀 스냅샷을 눈으로 보고 발견)
- ko 번역 누락 키 9개 (한국어 사용자에게 영문 폴백이 노출되고 있었다)

**행 확장 → 메인·서브 테이블 분리** (리뷰 제안 반영)
- 확장 패널이 보여주던 6개 필드가 메인 컬럼과 완전히 같아 새 정보가 0 이었다
- 행 확장은 28화면 중 1곳뿐인 일회성 패턴, 분할 상세는 이미 3화면이 쓰는 하우스 패턴
- AG Grid Master/Detail 은 Enterprise 전용 → 메인/서브 분리는 Community 순정 구성

**게이트**
- CI: design.md lint · 테마 생성물 신선도 · 화면 정보 인벤토리 대조
- ESLint: `ag-grid-enterprise`/`@mui/x-data-grid`/`@mui/icons-material` import **error**, TSX 내 색 리터럴 **warn**(1335건 = 청산 대상 목록)
- 대비 감사 45건 (텍스트 3단 + 상태색·액센트 6종 × 두 테마 + primary 버튼 + 골드 금지 규칙)

## Testing

| 검사 | 결과 |
|------|------|
| vitest | **634/634** (개편 전 577 → 신규 57건 추가, **기존 테스트 무수정**) |
| tsc | 통과 |
| vite build | 통과 (vendor-mui 218kB / gzip 68.8kB 분리) |
| eslint | **0 errors**, 1337 warnings(의도) |
| 시각 회귀 | **58/58** (화면 28개 × 2테마) |
| 화면 정보 인벤토리 | **정보 유실 0** (컬럼 34 · i18n 747 · 문자열 1,643) |
| design.md lint | **0 errors / 0 warnings** |
| 대비 감사 | **45/45** — 라이트/다크 모두 AA |

정보 유실 0 은 4중으로 검증했다: ① AST 기반 화면 정보 인벤토리 대조 ② 기존 테스트 무수정 통과(`getByText` 329 / `getByRole` 184 / `getByLabelText` 103) ③ 시각 회귀 58장 ④ i18n 키 정합 테스트.

수정한 기존 테스트는 2건뿐이고 둘 다 구현 디테일 단정이다(Tailwind 클래스명, pointer→mouse 이벤트 방식). 검증하는 동작은 동일하며 커밋 메시지에 사유를 적었다.

## 리뷰가 필요한 결정 사항

- **D6 (신규, 중요)**: `DataTable` → AG Grid 이관을 **착수했다가 되돌렸다.** AG Grid 가 실제 레이아웃 측정에 의존해 jsdom 에서 행을 렌더하지 않아 8개 파일 16개 테스트가 깨졌다(shim 으로도 해결 안 됨). 통과시키려면 목록 화면 7곳의 동기 회귀 검증을 포기해야 해서 혼자 결정하지 않았다. **권장: A안 — 현행 TanStack DataTable 유지** (이미 7화면이 같은 컴포넌트라 통일감 문제가 없고, 필요한 그리드 기능을 이미 갖췄다). 기획안 §Phase 3 에 A/B/C 안과 근거를 적었다. AG Grid 준비(설치·테마·청크 분리)는 끝나 있어 B/C 선택 시 바로 이어갈 수 있다
- **D1**: 골드 CTA 유지 여부. 이 PR 은 **현재 모습을 보존**했다(골드는 11.96:1 로 대비 문제가 없고, 브랜드 결정이라 임의로 바꾸지 않았다)
- **D3**: MUI 밀도. 테마에 `size="small"` 기본값을 넣었으나 화면별 밀도 프로토타입 승인은 남아 있다
- **D5**: 배포 이력에서 '펼치기' 액션이 사라진다(표시 값은 하나도 줄지 않는다)

## 남은 작업

기획안 §6.1 의 커밋 9·10(생 태그 흡수), 12(수제 table 17곳), 14(StatusBadge 통합), 15·16(앱 셸·밀도), 17~19(거대 화면 순수 추출), 20(차트 단일화), 21(하드코딩 색 1335건 청산). 각각 ESLint 경고와 기획안 진행 현황 표에 대상이 명시돼 있다.

Scope: `web/` 프론트엔드 UI 레이어 + `docs/40_UI_UX/` + `.github/workflows/ci.yml`. 백엔드·API·DB 변경 없음. 라우팅·메뉴 체계 변경 없음.
