---
version: alpha
name: Nullus Platform
description: >-
  Kubernetes 기반 DevSecOps 자동화 콘솔. 다크 우선, 데이터 밀집 운영 도구.
  중립 표면 + 액센트 1개에 집중하고, 상태는 색과 아이콘·텍스트를 함께 쓴다.
colors:
  # ─────────────────────────────────────────────────────────────
  # 대표 역할(canonical) — 다크가 기본 테마이므로 다크 값을 가리킨다.
  # design.md 스펙은 단일 스킴을 전제하므로, 스킴별 값은 아래 light-*/dark-* 에 둔다.
  # ─────────────────────────────────────────────────────────────
  primary: "{colors.dark-primary}"
  secondary: "{colors.dark-text-secondary}"
  tertiary: "{colors.dark-accent-alt}"
  neutral: "{colors.dark-surface}"

  # ─────────────────────────────────────────────────────────────
  # Light scheme — 카드는 순백, 페이지는 한 톤 낮게. 깊이는 그림자로 만든다.
  # ─────────────────────────────────────────────────────────────
  light-bg: "#f4f6f8"
  light-surface: "#ffffff"
  light-surface-sunken: "#eef2f6"
  light-divider: "#cbd5e1"
  light-divider-strong: "#b9c3cf"
  light-text: "#0f172a"
  light-text-secondary: "#475569"
  light-text-muted: "#5f6f85"
  light-primary: "#4338ca"
  light-on-primary: "#ffffff"
  light-success: "#047857"
  light-warning: "#a15c07"
  light-error: "#c81e1e"
  light-info: "#1d4ed8"
  light-accent-alt: "#6d28d9"

  # ─────────────────────────────────────────────────────────────
  # Dark scheme — 깊이는 표면 밝기 차로 만든다. 그림자를 쓰지 않는다.
  # ─────────────────────────────────────────────────────────────
  dark-bg: "#0a0a0a"
  dark-surface: "#0f1419"
  dark-surface-raised: "#161d26"
  dark-divider: "#2d3748"
  dark-divider-strong: "#4a5568"
  dark-text: "#f1f5f9"
  dark-text-secondary: "#94a3b8"
  dark-text-muted: "#8496a9"
  dark-primary: "#8f9bff"
  dark-on-primary: "#0a0a0a"
  dark-success: "#3ddc97"
  dark-warning: "#f5b544"
  dark-error: "#ff8080"
  dark-info: "#6aa8fb"
  dark-accent-alt: "#c4b5fd"

  # ─────────────────────────────────────────────────────────────
  # Scrim — 모달·드롭다운 뒤에 깔리는 차폐막. 표면이 아니므로 별도 토큰이다.
  # ─────────────────────────────────────────────────────────────
  light-scrim: "rgba(15, 23, 42, 0.45)"
  dark-scrim: "rgba(0, 0, 0, 0.7)"

  # ─────────────────────────────────────────────────────────────
  # Brand — 면(배경)으로만 쓴다. 텍스트·보더·아이콘 색으로 쓰지 않는다.
  # ─────────────────────────────────────────────────────────────
  brand-gold: "#ffd700"
  brand-gold-end: "#f59e0b"
  on-brand-gold: "#1a1d29"

typography:
  h1:
    fontFamily: Inter
    fontSize: 2rem
    fontWeight: 800
    lineHeight: 1.25
    letterSpacing: -0.01em
  h2:
    fontFamily: Inter
    fontSize: 1.125rem
    fontWeight: 700
    lineHeight: 1.3
  h3:
    fontFamily: Inter
    fontSize: 0.875rem
    fontWeight: 700
    lineHeight: 1.4
  body-md:
    fontFamily: Inter
    fontSize: 0.875rem
    fontWeight: 400
    lineHeight: 1.6
  body-sm:
    fontFamily: Inter
    fontSize: 0.8125rem
    fontWeight: 400
    lineHeight: 1.5
  label-sm:
    fontFamily: Inter
    fontSize: 0.75rem
    fontWeight: 600
    lineHeight: 1.5
  overline:
    fontFamily: Inter
    fontSize: 0.6875rem
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: 0.06em
  code:
    fontFamily: Fira Code
    fontSize: 0.8125rem
    fontWeight: 400
    lineHeight: 1.5

spacing:
  xs: 4px
  sm: 8px
  md: 12px
  lg: 16px
  xl: 24px
  2xl: 32px
  3xl: 48px

# 깊이. design.md 스펙에는 그림자 카테고리가 없어서 확장 키로 둔다
# (스펙은 모르는 내용을 보존하며, lint 도 통과한다).
# 라이트는 그림자로, 다크는 표면 밝기 차로 깊이를 만든다 → §Elevation & Depth
elevation:
  flat: none
  raised: 0 1px 2px rgba(15, 23, 42, 0.06)
  overlay: 0 8px 24px rgba(15, 23, 42, 0.12)
  raised-dark: none
  overlay-dark: none

rounded:
  sm: 6px
  md: 10px
  lg: 12px
  full: 9999px

components:
  button-primary:
    backgroundColor: "{colors.light-primary}"
    textColor: "{colors.light-on-primary}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.md}"
    padding: 10px
  button-secondary:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-primary}"
    rounded: "{rounded.md}"
    padding: 10px
  button-outline:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.md}"
    padding: 10px
  button-danger:
    backgroundColor: "{colors.light-error}"
    textColor: "{colors.light-on-primary}"
    rounded: "{rounded.md}"
    padding: 10px
  card:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.lg}"
    padding: 16px
  input:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.sm}"
    padding: 8px
    height: 36px
  badge-success:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-success}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-warning:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-warning}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-error:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-error}"
    rounded: "{rounded.full}"
    padding: 4px
  table-header:
    backgroundColor: "{colors.light-surface-sunken}"
    textColor: "{colors.light-text-secondary}"
    typography: "{typography.overline}"
    height: 36px
  table-row:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    typography: "{typography.body-sm}"
    height: 40px

  # ── 다크 변형 ──────────────────────────────────────────────
  # 스펙이 변형을 별도 엔트리로 표현하도록 규정하므로, 두 스킴의 컴포넌트 값을
  # 각각 명시한다. 같은 hex 를 두 테마에 재사용할 수 없다는 원칙(§Colors)의 증거다.
  button-primary-dark:
    backgroundColor: "{colors.dark-primary}"
    textColor: "{colors.dark-on-primary}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.md}"
    padding: 10px
  button-secondary-dark:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-primary}"
    rounded: "{rounded.md}"
    padding: 10px
  button-outline-dark:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-text}"
    rounded: "{rounded.md}"
    padding: 10px
  button-danger-dark:
    backgroundColor: "{colors.dark-error}"
    textColor: "{colors.dark-on-primary}"
    rounded: "{rounded.md}"
    padding: 10px
  button-brand:
    backgroundColor: "{colors.brand-gold}"
    textColor: "{colors.on-brand-gold}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.md}"
    padding: 10px
  page-dark:
    backgroundColor: "{colors.dark-bg}"
    textColor: "{colors.dark-text}"
  page:
    backgroundColor: "{colors.light-bg}"
    textColor: "{colors.light-text}"
  card-dark:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-text}"
    rounded: "{rounded.lg}"
    padding: 16px
  overlay-dark:
    backgroundColor: "{colors.dark-surface-raised}"
    textColor: "{colors.dark-text}"
    rounded: "{rounded.lg}"
    padding: 16px
  overlay:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.lg}"
    padding: 16px
  input-dark:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-text}"
    rounded: "{rounded.sm}"
    padding: 8px
    height: 36px
  divider:
    backgroundColor: "{colors.light-divider}"
    height: 1px
  divider-hover:
    backgroundColor: "{colors.light-divider-strong}"
    height: 1px
  divider-dark:
    backgroundColor: "{colors.dark-divider}"
    height: 1px
  divider-hover-dark:
    backgroundColor: "{colors.dark-divider-strong}"
    height: 1px
  badge-success-dark:
    textColor: "{colors.dark-success}"
    backgroundColor: "{colors.dark-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-warning-dark:
    textColor: "{colors.dark-warning}"
    backgroundColor: "{colors.dark-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-error-dark:
    textColor: "{colors.dark-error}"
    backgroundColor: "{colors.dark-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-info:
    textColor: "{colors.light-info}"
    backgroundColor: "{colors.light-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-info-dark:
    textColor: "{colors.dark-info}"
    backgroundColor: "{colors.dark-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-role:
    textColor: "{colors.light-accent-alt}"
    backgroundColor: "{colors.light-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-role-dark:
    textColor: "{colors.dark-accent-alt}"
    backgroundColor: "{colors.dark-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-inactive:
    textColor: "{colors.light-text-muted}"
    backgroundColor: "{colors.light-surface-sunken}"
    rounded: "{rounded.full}"
    padding: 4px
  badge-inactive-dark:
    textColor: "{colors.dark-text-muted}"
    backgroundColor: "{colors.dark-surface}"
    rounded: "{rounded.full}"
    padding: 4px
  table-header-dark:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-text-secondary}"
    typography: "{typography.overline}"
    height: 36px
  table-row-dark:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-text}"
    typography: "{typography.body-sm}"
    height: 40px
  scrim:
    backgroundColor: "{colors.light-scrim}"
  scrim-dark:
    backgroundColor: "{colors.dark-scrim}"
  brand-cta-gradient:
    backgroundColor: "{colors.brand-gold-end}"
    textColor: "{colors.on-brand-gold}"
    rounded: "{rounded.md}"
    padding: 10px
---

## Overview

Nullus 는 Kubernetes 위에서 DevSecOps 스택을 설치·운영하는 콘솔이다. 사용자는 하루의
상당 시간을 이 화면에서 목록을 훑고 상태를 판단하며 보낸다. 그래서 이 디자인 시스템의
1순위는 아름다움이 아니라 **한 화면에서 판단이 끝나는 것**이다.

톤은 "신뢰감 있는 운영 도구"다. 채도를 낮춘 중립 표면 위에 액센트 하나(Indigo)만 두고,
색은 의미가 있을 때만 쓴다. 장식적 그라데이션과 다중 액센트를 쓰지 않는다 — 색이 흔해지면
상태 색이 상태로 읽히지 않는다.

**다크가 기본이고 라이트는 1급 지원이다.** DevOps 도구 관행상 다크를 기본으로 두지만,
라이트가 "다크 값을 그대로 쓰는 이등 테마"가 되는 것을 금지한다. 실제로 그렇게 방치된 결과
라이트 테마에서 화면이 와이어프레임처럼 보였고 그게 이 문서가 생긴 이유다.

이 문서는 [google-labs-code/design.md](https://github.com/google-labs-code/design.md) 스펙을
따른다. **YAML front matter 의 토큰이 규범이고 본문은 그 이유와 적용법이다.**
코드는 이 파일의 파생물이다 — `src/theme/` 가 여기서 MUI 테마, Tailwind 브릿지,
AG Grid 테마를 만든다. 값을 바꿀 곳은 이 파일 하나다.

## Colors

### 표면(surface)과 깊이

**두 테마가 깊이를 다른 수단으로 만든다.** 이게 이 팔레트의 핵심 결정이다.

- **라이트**: 카드는 순백(`light-surface` `#ffffff`), 페이지는 한 톤 낮게
  (`light-bg` `#f4f6f8`). 대비 1.08:1 로 은은하지만 면이 분명히 나뉜다.
  추가 깊이는 그림자로 준다(§Elevation & Depth).
- **다크**: 페이지가 가장 어둡고(`dark-bg` `#0a0a0a`) 카드가 한 단 밝다
  (`dark-surface` `#0f1419`). 그림자는 쓰지 않는다 — 어두운 배경에서 그림자는 보이지 않는다.

> 개편 전에는 라이트의 카드와 페이지 배경이 **같은 색(#f8fafc)** 이었다. 대비 1.00:1.
> 카드가 배경에 녹아 사라지고 유일한 구분선인 보더만 남았는데, 그 보더가 `#1f2937`
> (거의 검정, 14.03:1)이라 "흰 종이에 검은 선" = 와이어프레임처럼 보였다.

### 경계선(divider)

경계선은 **은은해야 한다.** 카드 대비 1.2~1.6:1 을 목표로 한다.
본문 텍스트만큼 튀는 경계선은 정보가 아니라 소음이다.

- `light-divider` `#cbd5e1` — 카드 대비 1.48:1
- `dark-divider` `#2d3748` — 카드 대비 1.54:1

`*-divider-strong` 은 호버·포커스처럼 **상호작용으로 강조할 때만** 쓴다.

### 텍스트 3단

| 토큰 | 용도 | 라이트 대비 | 다크 대비 |
|------|------|-------------|-----------|
| `*-text` | 제목, 값, 본문 | 17.85:1 | 16.90:1 |
| `*-text-secondary` | 설명, 라벨, 컬럼 헤더 | 7.58:1 | 7.22:1 |
| `*-text-muted` | 보조 메타(타임스탬프, 힌트) | 5.12:1 | 6.10:1 |

3단이 끝이다. 4단째를 만들지 않는다 — 더 흐리게 하면 AA 를 넘길 수 없다.
모든 값은 카드와 페이지 배경 **양쪽에서** 4.5:1 을 넘는다.

### 상태색은 테마별로 다른 톤을 쓴다

같은 hex 를 두 테마에 재사용할 수 없다. 어두운 배경에는 밝은 톤(400대), 밝은 배경에는
어두운 톤(700~800대)이 필요하다. 이걸 지키지 않아서 개편 전 라이트 테마의 상태색이
1.6~2.9:1 로 무너져 있었다.

| 의미 | 라이트 | 다크 | 쓰는 곳 |
|------|--------|------|---------|
| primary | `#4338ca` | `#8f9bff` | 주요 액션, 링크, 선택 상태 |
| success | `#047857` | `#3ddc97` | Connected, 배포 성공, Running |
| warning | `#a15c07` | `#f5b544` | Pending, 경고, 호환성 주의 |
| error | `#c81e1e` | `#ff8080` | 실패, 삭제, 장애 |
| info | `#1d4ed8` | `#6aa8fb` | 정보 배지, 차트 기본 계열 |
| accent-alt | `#6d28d9` | `#c4b5fd` | 역할·권한 등 보조 분류 |

라이트 최솟값 4.79:1, 다크 최솟값 7.32:1 — 전부 AA 통과.

### 브랜드 골드

`brand-gold` `#ffd700` 은 **로고와 면(배경)으로만** 쓴다.
흰 배경에서 골드는 1.40:1 이라 텍스트·보더·아이콘 색으로 쓸 수 없다.
골드 배경 위 텍스트는 `on-brand-gold` `#1a1d29` (11.96:1).

주요 액션(CTA)의 색은 골드가 아니라 `*-primary` 다. 화면마다 CTA 가 골드와 인디고로
갈리는 것이 통일감을 깨는 큰 축이었다.

## Typography

| 토큰 | 크기 | 굵기 | 용도 |
|------|------|------|------|
| `h1` | 2rem / 32px | 800 | 페이지 제목 (화면당 1개) |
| `h2` | 1.125rem / 18px | 700 | 섹션 제목 |
| `h3` | 0.875rem / 14px | 700 | 카드 제목, 필드 그룹 |
| `body-md` | 0.875rem / 14px | 400 | 기본 본문 |
| `body-sm` | 0.8125rem / 13px | 400 | 표 셀, 밀집 영역 |
| `label-sm` | 0.75rem / 12px | 600 | 버튼, 배지, 폼 라벨 |
| `overline` | 0.6875rem / 11px | 600 | 표 헤더, 섹션 오버라인 (대문자 + 자간) |
| `code` | 0.8125rem / 13px | 400 | YAML, 스크립트, 식별자 |

폰트는 UI 에 **Inter**, 한글에 **Pretendard**, 코드에 **Fira Code**.
`body-md` 14px 가 기준선이다. 데이터 밀집 화면에서 16px 는 한 화면에 담기는 행 수를
줄이는데, 그건 정보 손실과 같다.

한글과 영문이 섞이는 화면이므로 `letterSpacing` 을 임의로 좁히지 않는다.
`h1` 의 `-0.01em` 과 `overline` 의 `0.06em` 만 의도된 예외다.

## Layout

| 토큰 | 값 | 용도 |
|------|------|------|
| `--sidebar-width` | 240px | 사이드바 펼침 |
| `--sidebar-collapsed` | 64px | 사이드바 접힘 |
| `--header-height` | 56px | 상단 헤더 |
| `--page-padding` | 32px | 페이지 좌우 여백 |
| `--card-radius` | `{rounded.lg}` | 카드 모서리 |
| `--card-padding` | `{spacing.lg}` | 카드 내부 여백 |
| `--grid-gap` | `{spacing.md}` | 카드 그리드 간격 |

간격은 `spacing` 스케일(4 / 8 / 12 / 16 / 24 / 32 / 48)만 쓴다. 스케일 밖의 값
(`13px`, `18px`, `7px`)을 쓰지 않는다 — 개편 전 `--card-padding: 18px`,
`--grid-gap: 14px` 처럼 스케일 밖 값이 섞여 있으면 화면마다 리듬이 어긋난다.

반응형은 데스크톱 우선이다. 1024px 미만에서 사이드바가 접히고, 768px 미만은
지원하되 최적화 대상이 아니다 (운영 콘솔 특성).

## Elevation & Depth

**라이트는 그림자로, 다크는 밝기로 깊이를 만든다.** 같은 그림자를 두 테마에 쓰지 않는다.

| 단계 | 용도 | 라이트 | 다크 |
|------|------|--------|------|
| `flat` | 페이지 배경 | `light-bg`, 그림자 없음 | `dark-bg`, 그림자 없음 |
| `raised` | 카드, 패널, 사이드바 | `light-surface` + `0 1px 2px rgba(15,23,42,.06)` + `light-divider` 1px | `dark-surface` + `dark-divider` 1px, 그림자 없음 |
| `overlay` | 모달, 드롭다운, 팝오버 | `light-surface` + `0 8px 24px rgba(15,23,42,.12)` | `dark-surface-raised` `#161d26` + 오버레이 `rgba(0,0,0,.7)` |

개편 전에는 elevation 토큰이 **하나도 없었다**. 다크는 표면 밝기 차로 어찌어찌 버텼지만
라이트는 깊이를 표현할 수단이 아예 없어서 평평한 와이어프레임이 됐다.

깊이를 3단으로 제한한다. 4단째(카드 안의 카드 안의 카드)가 필요하면 그건 elevation
문제가 아니라 정보 구조 문제다.

## Shapes

| 토큰 | 값 | 용도 |
|------|------|------|
| `rounded.sm` | 6px | 입력, 배지, 작은 버튼 |
| `rounded.md` | 10px | 버튼, 셀렉트 |
| `rounded.lg` | 12px | 카드, 패널, 모달 |
| `rounded.full` | 9999px | 상태 Chip, 아바타, 토글 |

모서리는 4단이 끝이다. 같은 카드 안에서 서로 다른 반경을 섞지 않는다.

아이콘은 **lucide-react 하나만** 쓴다. `@mui/icons-material` 을 도입하지 않는다 —
아이콘 세트가 둘이면 같은 뜻의 아이콘이 화면마다 달라진다.
아이콘 컨테이너는 38x38, 반경 10px, 배경은 해당 기능 색의 15% 알파.

## Components

컴포넌트 층은 **MUI v9** 다. 동작·접근성(포커스 트랩, 키보드 내비, ARIA)은 MUI 를 그대로
쓰고, 표면 스타일(색·모양·밀도)만 이 문서의 토큰으로 재정의한다.

표는 **AG Grid Community(MIT)** 하나만 쓴다. `@mui/x-data-grid` 를 도입하지 않는다.

### 밀도 규칙

MUI 기본값은 여백이 넉넉하다. 그대로 쓰면 한 화면에 들어가던 행이 줄어드는데,
**그건 정보 손실이다.** 그래서 다음을 기본값으로 조인다:

- 모든 입력·셀렉트·버튼은 `size="small"`
- 표 행 높이 40px, 헤더 36px
- 카드 내부 여백 16px (`spacing.lg`)

### 상태 표시

상태는 **색 + 아이콘 + 텍스트**를 항상 함께 쓴다. 색만으로 표시하지 않는다.

| 상태 | 색 토큰 | 아이콘 |
|------|---------|--------|
| Connected / Success / Running | `*-success` | `CheckCircle` |
| Pending / Warning | `*-warning` | `Clock` |
| Error / Failed | `*-error` | `AlertCircle` |
| Inactive / Unknown | `*-text-muted` | `MinusCircle` |

상태 배지는 `StatusBadge` 하나만 쓴다. 화면마다 배지를 다시 만들지 않는다.

## Do's and Don'ts

### Do

- ✅ 색은 항상 토큰으로 쓴다. 값이 필요하면 이 파일을 고친다.
- ✅ 상태는 색 + 아이콘 + 텍스트를 함께 쓴다.
- ✅ 라이트/다크 각각의 톤을 쓴다. 상태색은 테마별로 다른 hex 다.
- ✅ 간격은 `spacing` 스케일 안에서만 고른다.
- ✅ 표는 `DataTable`, 정적·레이아웃 표만 MUI `Table`.
- ✅ 밀도를 높이려면 여백을 줄인다.
- ✅ 새 화면을 만들 때 이 파일을 먼저 읽는다.

### Don't

- ❌ TSX 에 hex 를 박지 않는다. (ESLint 로 금지한다)
- ❌ 골드를 텍스트·보더·아이콘 색으로 쓰지 않는다. 흰 배경에서 1.40:1 이다.
- ❌ 라이트 테마에 400대 톤(`#34d399`, `#fbbf24`, `#818cf8`)을 텍스트로 쓰지 않는다.
  각각 1.84 / 1.60 / 2.85:1 이다.
- ❌ 라이트 보더에 `gray-800` 이상 어두운 값을 쓰지 않는다. 와이어프레임처럼 보인다.
- ❌ 다크 테마에 그림자를 쓰지 않는다. 보이지 않는데 렌더 비용만 든다.
- ❌ 그리드·차트·아이콘 라이브러리를 추가하지 않는다.
- ❌ **정보를 줄여서 여백을 만들지 않는다.** 밀도와 정보량은 별개다.
- ❌ 텍스트 4단째를 만들지 않는다. AA 를 넘길 수 없다.
