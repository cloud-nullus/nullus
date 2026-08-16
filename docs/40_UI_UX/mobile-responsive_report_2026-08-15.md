# Mobile / Responsive 점검 결과서 (2026-08-15)

- **EPIC**: [cloud-nullus/nullus-plan#36](https://github.com/cloud-nullus/nullus-plan/issues/36) — 모바일/반응형 점검 1차
- **브랜치**: `feat/ui/mobile-responsive-audit` (별도 GitHub 이슈 없이 브랜치로 추적)
- **점검 도구**: `web/scripts/responsive-audit.mjs` (Playwright headless)
- **점검 일시**: 2026-08-15

---

## 1. 점검 방법

- 로그인(mock auth, `admin@nullus.dev`) 후 주요 9개 화면을 3개 뷰포트로 렌더해 자동 점검.
- **뷰포트**: `mobile-sm(360px)` · `mobile(390px)` · `tablet(768px)`
- **감지 신호 2종**
  1. **가로 오버플로우** — `document.scrollWidth > clientWidth` (가로 스크롤 발생)
  2. **사이드바 미collapse** — 모바일 폭(<768px)에서 `<aside>`가 뷰포트의 30% 초과 점유 → 본문이 좁은 칸에 짓눌리는데 문서 폭은 안 넘어 (1)로는 안 잡히는 유형
- ⭐ = EPIC #36이 명시한 우선 점검 화면(home / stack install / cicd list)

## 2. 결과 요약

| 지표 | 값 |
|---|---|
| 총 점검 | 27건 (9화면 × 3뷰포트) |
| **이슈** | **16건** |
| 가로 오버플로우 | **0건** (양호) |
| 사이드바 미collapse | 16건 (인증 8화면 × 모바일 2뷰포트) |

- **가로 스크롤 깨짐은 없음** — 문서 폭이 뷰포트를 넘지 않아 이 축은 양호.
- **핵심 문제는 사이드바가 모바일에서 접히지 않는 것** — 자동 오버플로우 지표로는 안 잡혀 스크린샷/사이드바 폭 검사로 확인.

## 3. 핵심 발견 — 사이드바 모바일 미collapse (앱 전역)

**증상**: 모바일 폭(360/390px)에서 좌측 사이드바가 240px 그대로 펼쳐진 채라, 본문이 약 120~150px 좁은 칸으로 짓눌린다. home의 히어로 텍스트가 한 단어씩 세로로 쪼개지고("Nullus Platfo…" 잘림), stack install의 폼·`Save Draft` 버튼이 잘린다.

**영향 범위**: 사이드바를 쓰는 **모든 인증 페이지**(home, stack install, cicd list, stack/cicd templates, stack list, monitoring, admin 등). 단일 공통 레이아웃이 원인이라 화면마다 별개 버그가 아니다.

**근본 원인** (코드):
- `web/src/components/layout/sidebar.tsx:39-41` — 루트 `<aside>`가 `shrink-0` flex 형제라 항상 폭을 차지하고, 폭은 `collapsed ? --sidebar-collapsed : --sidebar-width` 뿐. **반응형 breakpoint(`md:`/`hidden`/`matchMedia`)가 전혀 없다.**
- `web/src/stores/sidebar-store.ts` — `collapsed`는 localStorage 기반 **수동 토글**(햄버거)이고 기본 펼침. 뷰포트를 인지하지 않는다.
- `web/src/theme/tokens.generated.css:67-68` — `--sidebar-width: 240px`, `--sidebar-collapsed: 48px`.
- 결과: 390px 화면에서 240px 사이드바 + ~150px 본문.

**증거 스크린샷** (재점검 시 재생성): `web/.responsive-audit/home__mobile.png`, `stack-install__mobile.png`, `cicd-list__mobile.png`

## 4. 수정안 — B 채택 (1차 최소 수정), A는 2차 후속

EPIC 범위가 "**최소 CSS 수정**"이라 두 안을 검토했다. 사이드바는 UX가 걸린 공통 컴포넌트다.

- **(A) 오프캔버스 드로어 — 권장**: `<md`에서 `<aside>`를 `absolute`(off-canvas)로 빼고 본문이 전체 폭을 쓰게 한다. 햄버거로 오버레이 + 백드롭 토글. 표준 반응형 패턴이나 변경이 레이아웃(`sidebar.tsx`/`header.tsx`/`AppLayout`)에 걸친다.
- **(B) 모바일 기본 collapsed — 퀵윈**: 모바일 진입 시 사이드바를 48px 레일로 기본 접기(`collapsed` 초기값을 뷰포트로 결정). 변경 최소(스토어 1곳)지만 레일 48px는 남고 라벨이 가려진다.

> **적용 결정: B (1차).** EPIC의 "최소 CSS 수정"·1차 범위에 맞춰 B를 적용했다 — fix는 별도 브랜치 `feat/ui/mobile-sidebar-responsive`(`web/src/stores/sidebar-store.ts` 단독). A(오프캔버스 드로어)는 2차 후속으로 남긴다.
>
> 자동 점검 스크립트는 두 안 모두를 회귀 검사한다 — 수정 후 `aside` 폭이 뷰포트의 30% 밑으로 내려가면 `사이드바 미collapse`가 사라진다.

## 5. 재현 / 지속 점검 방법

```bash
# 사전: 웹 dev + API 기동 (mock auth 로그인)
cd web
RESPONSIVE_BASE_URL=http://localhost:5173 npm run responsive:audit

# CI 게이트(이슈 있으면 exit 1)
npm run responsive:audit:check
```

- 산출물: `web/.responsive-audit/report-<date>.md` · `report-<date>.json` · 화면별 스크린샷 (해당 폴더는 gitignore).
- 환경변수: `RESPONSIVE_BASE_URL`(기본 5173), `RESPONSIVE_EMAIL`/`RESPONSIVE_PASSWORD`.
- **CI 연동(선택)**: 웹 dev·API·DB가 뜬 잡에서 `npm run responsive:audit:check`를 실행하면 반응형 회귀를 PR에서 차단할 수 있다. 전체 스택 기동이 필요해 무거우므로 nightly 잡 권장.

## 6. 부록 — 수정 전 전체 결과표

| 화면 | mobile-sm(360) | mobile(390) | tablet(768) |
|---|---|---|---|
| login | ✅ | ✅ | ✅ |
| home ⭐ | ❌ 사이드바 | ❌ 사이드바 | ✅ |
| stack-install ⭐ | ❌ 사이드바 | ❌ 사이드바 | ✅ |
| cicd-list ⭐ | ❌ 사이드바 | ❌ 사이드바 | ✅ |
| stack-templates | ❌ 사이드바 | ❌ 사이드바 | ✅ |
| stack-list | ❌ 사이드바 | ❌ 사이드바 | ✅ |
| cicd-templates | ❌ 사이드바 | ❌ 사이드바 | ✅ |
| monitoring | ❌ 사이드바 | ❌ 사이드바 | ✅ |
| admin-org | ❌ 사이드바 | ❌ 사이드바 | ✅ |

*가로 오버플로우: 전 화면 0px. 이슈는 모두 사이드바 미collapse.*

## 7. 수정 후 재점검 (2026-08-15)

B(모바일 기본 collapse) 적용 후 동일 스크립트로 재점검 → **이슈 0건 / 27건**.

| 뷰포트 | 수정 전 `aside` | 수정 후 `aside` | 판정 |
|---|---|---|---|
| mobile-sm(360) | 240px (본문 짓눌림) | **48px** | ✅ 해소 |
| mobile(390) | 240px | **48px** | ✅ 해소 |
| tablet(768) | 240px | 240px (유지) | ✅ 정상 |

- 인증 8화면 × 모바일 2뷰포트 = **16건 → 0건**. login·tablet은 이전과 동일하게 정상.
- 육안 확인: home 모바일에서 사이드바가 48px 레일로 접히고 본문이 전체 폭을 사용 — 히어로·버튼·Support Tools 정상 렌더.
- **fix 위치**: 브랜치 `feat/ui/mobile-sidebar-responsive`, 커밋 `f47d882` (`web/src/stores/sidebar-store.ts` 단독). 본 결과서(PR #148)와는 분리되어 있어 독립 머지 가능.
