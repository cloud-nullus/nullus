# Nullus 디자인 시스템

**작성일**: 2026-03-14
**기반**: proto4 와이어프레임 분석 + UI/UX Pro Max 디자인 시스템 생성
**스타일**: Dark Mode (OLED) + Data-Dense Dashboard

---

## 1. 스타일 방향

| 항목 | 선택 | 근거 |
|------|------|------|
| **기본 스타일** | Dark Mode (OLED) | proto4가 이미 다크 테마 기반, DevOps 도구 관행 |
| **보조 스타일** | Data-Dense Dashboard | 모니터링/목록 페이지의 정보 밀도 최적화 |
| **접근성** | WCAG AA 이상 | 텍스트 대비 4.5:1, 포커스 링 필수 |
| **모드** | Dark 기본 / Light 지원 | proto4에 테마 토글 존재 |

---

## 2. 컬러 팔레트

### 2.1 다크 테마 (기본)

| 용도 | 색상 코드 | Tailwind 근사값 | proto4 출처 |
|------|-----------|-----------------|-------------|
| **배경 (Base)** | `#0a0a0a` | `neutral-950` | `body` 배경 그라데이션 시작 |
| **배경 (Gradient End)** | `#1a1a2e` → `#16213e` | — | `body` 그라데이션 |
| **배경 (Surface/Card)** | `#0f1419` | — | 카드 `background` |
| **보더** | `#2d3748` | `gray-700` | 카드 `border` |
| **텍스트 (Primary)** | `#f1f5f9` | `slate-100` | 제목/본문 |
| **텍스트 (Secondary)** | `#64748b` | `slate-500` | 설명/보조 텍스트 |
| **텍스트 (Muted)** | `#475569` | `slate-600` | 최소 가독성 보장 |

### 2.2 브랜드 & 액센트

| 용도 | 색상 코드 | Tailwind | 사용처 |
|------|-----------|----------|--------|
| **브랜드 Gold** | `#ffd700` → `#f59e0b` | `amber-500` | 로고, 주요 CTA 그라데이션 |
| **Indigo (주 액센트)** | `#6366f1` / `#818cf8` / `#a5b4fc` | `indigo-500/400/300` | 버튼, 링크, 선택 상태, nav-section |
| **Green (성공)** | `#22c55e` / `#34d399` | `green-500/400` | Connected, 성공 상태, 완료 |
| **Amber (경고)** | `#f59e0b` / `#fbbf24` | `amber-500/400` | Pending, 경고 상태 |
| **Red (위험)** | `#ef4444` / `#f87171` | `red-500/400` | Error, 삭제, 장애 |
| **Blue (정보)** | `#3b82f6` / `#60a5fa` | `blue-500/400` | 정보성 배지, 차트 |
| **Purple (보조)** | `#8b5cf6` / `#c4b5fd` | `violet-500/300` | 역할 관리, 보조 기능 |

### 2.3 기능별 색상 매핑 (proto4 기준)

| 기능 영역 | 아이콘 배경 | 아이콘 색상 |
|-----------|------------|------------|
| DevSecOps 스택 | `rgba(99,102,241,0.15)` | `#818cf8` (indigo) |
| Golden Path 템플릿 | `rgba(16,185,129,0.15)` | `#34d399` (emerald) |
| CI/CD 파이프라인 | `rgba(245,158,11,0.15)` | `#fbbf24` (amber) |
| 버전 호환성 | `rgba(239,68,68,0.15)` | `#f87171` (red) |
| 모니터링 | `rgba(59,130,246,0.15)` | `#60a5fa` (blue) |
| 권한 관리 | `rgba(139,92,246,0.15)` | `#c4b5fd` (violet) |

### 2.4 라이트 테마

| 용도 | 색상 코드 |
|------|-----------|
| **배경 (Base)** | `#ffffff` |
| **배경 (Surface)** | `#f8fafc` |
| **보더** | `#e2e8f0` |
| **텍스트 (Primary)** | `#0f172a` |
| **텍스트 (Secondary)** | `#475569` |

---

## 3. 타이포그래피

### 3.1 폰트 패밀리

| 용도 | 폰트 | 폴백 |
|------|------|------|
| **UI/제목** | Inter | -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif |
| **한글 보조** | Pretendard | 'Malgun Gothic', sans-serif |
| **코드/YAML** | Fira Code | 'Cascadia Code', 'JetBrains Mono', monospace |

### 3.2 Google Fonts Import

```css
@import url('https://fonts.googleapis.com/css2?family=Fira+Code:wght@400;500;600;700&family=Inter:wght@300;400;500;600;700;800&display=swap');
```

Pretendard는 CDN 별도:
```css
@import url('https://cdn.jsdelivr.net/gh/orioncactus/pretendard/dist/web/static/pretendard.css');
```

### 3.3 타입 스케일

| 용도 | 크기 | 굵기 | 행간 | proto4 기준 |
|------|------|------|------|-------------|
| **H1 (페이지 제목)** | 36px | 800 | 1.2 | 홈 "Nullus Platform" |
| **H2 (섹션 제목)** | 18px | 700 | 1.3 | "핵심 기능" 등 |
| **H3 (카드 제목)** | 14px | 700 | 1.4 | 카드 내 제목 |
| **Body** | 14px | 400 | 1.6 | 일반 텍스트 |
| **Small/Label** | 12px | 500~600 | 1.5 | 배지, 라벨 |
| **Caption** | 11px | 400 | 1.4 | 보조 설명 |
| **Code** | 13px | 400 | 1.5 | YAML/스크립트 미리보기 |

---

## 4. 레이아웃 토큰

### 4.1 전역 레이아웃

```
┌──────────────────────────────────────────────────────┐
│ Sidebar (240px / 64px 접힘)  │  메인 영역             │
│ ┌──────────────────────────┐ │ ┌──────────────────┐  │
│ │ 로고 + 토글 버튼         │ │ │ 헤더 (56px)      │  │
│ │ 역할 전환기              │ │ │ 언어/테마/사용자  │  │
│ │ ────────────────         │ │ ├──────────────────┤  │
│ │ 데브섹옵스 스택 ▾        │ │ │                  │  │
│ │   스택 템플릿             │ │ │  페이지 콘텐츠    │  │
│ │   스택 설치               │ │ │                  │  │
│ │   스택 목록               │ │ │                  │  │
│ │   스택 이력               │ │ │                  │  │
│ │   스택 버전 관리          │ │ │                  │  │
│ │ CI/CD ▾                  │ │ │                  │  │
│ │ 관측성 ▾                 │ │ │                  │  │
│ │ 관리 ▾                   │ │ │                  │  │
│ │ ────────────────         │ │ │                  │  │
│ │ 로그아웃                  │ │ │                  │  │
│ └──────────────────────────┘ │ └──────────────────┘  │
└──────────────────────────────────────────────────────┘
```

### 4.2 디자인 토큰

| 토큰 | 값 | 용도 |
|------|------|------|
| `--sidebar-width` | `240px` | 사이드바 펼침 |
| `--sidebar-collapsed` | `64px` | 사이드바 접힘 |
| `--header-height` | `56px` | 상단 헤더 |
| `--card-radius` | `12px` | 카드 모서리 |
| `--card-padding` | `18px` | 카드 내부 여백 |
| `--page-padding` | `48px` | 페이지 좌우 여백 |
| `--grid-gap` | `14px` | 카드 그리드 간격 |
| `--icon-size` | `38px` | 기능 아이콘 컨테이너 |
| `--icon-radius` | `10px` | 아이콘 컨테이너 모서리 |
| `--transition` | `200ms ease` | 기본 전환 시간 |
| `--z-sidebar` | `40` | 사이드바 z-index |
| `--z-modal` | `50` | 모달 z-index |
| `--z-toast` | `60` | 토스트 알림 z-index |

---

## 5. 컴포넌트 스펙

### 5.1 버튼

| 변형 | 배경 | 텍스트 | 보더 | 용도 |
|------|------|--------|------|------|
| **Primary (Gold)** | `linear-gradient(135deg, #ffd700, #f59e0b)` | `#1a1d29` | 없음 | 주요 CTA (Stack 시작, Deploy) |
| **Secondary (Indigo)** | `rgba(99,102,241,0.15)` | `#a5b4fc` | `rgba(99,102,241,0.3)` | 보조 액션 |
| **Outline** | 투명 | `#e2e8f0` | `#2d3748` | 일반 액션 |
| **Danger** | `rgba(239,68,68,0.15)` | `#f87171` | `rgba(239,68,68,0.3)` | 삭제, 위험 액션 |
| **Ghost** | 투명 | `#64748b` | 없음 | 아이콘 버튼, 토글 |

공통: `border-radius: 10px`, `padding: 12px 24px`, `font-weight: 600~700`, `cursor: pointer`, `transition: 200ms`

### 5.2 카드

```css
.card {
  background: #0f1419;
  border: 1px solid #2d3748;
  border-radius: 12px;
  padding: 18px;
  transition: border-color 200ms ease;
}
.card:hover {
  border-color: #4a5568;
}
```

### 5.3 모달

| 속성 | 일반 | 와이드 |
|------|------|--------|
| 최대 너비 | 480px | 800px |
| 배경 | `#0f1419` | `#0f1419` |
| 오버레이 | `rgba(0,0,0,0.7)` | `rgba(0,0,0,0.7)` |
| 용도 | 등록/수정 폼 | Deploy Script, K8s Preview |

### 5.4 상태 배지

| 상태 | 배경 | 텍스트 | 아이콘 |
|------|------|--------|--------|
| Connected | `rgba(34,197,94,0.15)` | `#22c55e` | `check-circle` |
| Pending | `rgba(245,158,11,0.15)` | `#f59e0b` | `clock` |
| Error | `rgba(239,68,68,0.15)` | `#ef4444` | `alert-circle` |
| Inactive | `rgba(100,116,139,0.15)` | `#64748b` | `minus-circle` |

### 5.5 테이블

```css
.table-row {
  height: 48px;
  border-bottom: 1px solid #2d3748;
  transition: background 150ms;
}
.table-row:hover {
  background: rgba(99, 102, 241, 0.05);
}
.table-header {
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
}
```

---

## 6. 아이콘 매핑

proto4에서 사용된 Font Awesome → Lucide React 대응:

| proto4 (Font Awesome) | Lucide React | 용도 |
|-----------------------|--------------|------|
| `fa-cube` | `Box` | Nullus 로고 |
| `fa-cubes` | `Boxes` | DevSecOps 스택 |
| `fa-book-open` | `BookOpen` | 템플릿 |
| `fa-code-branch` | `GitBranch` | CI/CD |
| `fa-shield-alt` | `Shield` | 보안/호환성 |
| `fa-chart-bar` | `BarChart3` | 모니터링 |
| `fa-users-cog` | `UsersCog` | 권한 관리 |
| `fa-rocket` | `Rocket` | Stack 시작 CTA |
| `fa-terminal` | `Terminal` | Deploy Script |
| `fa-dharmachakra` | `CircleDot` | Kubernetes |
| `fa-copy` | `Copy` | 복사 버튼 |
| `fa-times` | `X` | 모달 닫기 |
| `fa-bars` | `Menu` | 사이드바 토글 |
| `fa-arrow-left` | `ArrowLeft` | 뒤로 가기 |
| `fa-user-shield` | `ShieldCheck` | Admin 역할 |
| `fa-hard-hat` | `HardHat` | DevOps 역할 |
| `fa-laptop-code` | `LaptopMinimal` | Developer 역할 |
| `fa-check-circle` | `CheckCircle` | 성공/연결됨 |
| `fa-clock` | `Clock` | 대기 중 |
| `fa-cog` / `fa-cogs` | `Settings` | 설정 |
| `fa-network-wired` | `Network` | 클러스터 |
| `fa-server` | `Server` | 서버/클러스터 |
| `fa-bell` | `Bell` | 알림 |

---

## 7. 반응형 브레이크포인트

| 브레이크포인트 | 너비 | 동작 |
|---------------|------|------|
| **Desktop (기본)** | 1440px+ | 전체 레이아웃 |
| **Laptop** | 1024px~1439px | 사이드바 자동 접힘 |
| **Tablet** | 768px~1023px | 사이드바 오버레이, 1열 그리드 |
| **Mobile** | 375px~767px | 지원하되 최적화 대상 아님 (관리 도구 특성) |

---

## 8. 애니메이션 가이드

| 유형 | 지속 시간 | 이징 | 사용처 |
|------|-----------|------|--------|
| 마이크로 인터랙션 | 150ms | `ease` | 버튼 호버, 배지 상태 변경 |
| 상태 전환 | 200ms | `ease` | 사이드바 접기, 탭 전환 |
| 모달 진입 | 200ms | `ease-out` | 모달 열기 (scale 0.95→1, opacity 0→1) |
| 모달 퇴장 | 150ms | `ease-in` | 모달 닫기 |
| 데이터 로딩 | — | — | 스켈레톤 shimmer (`2s infinite`) |

`prefers-reduced-motion: reduce` 미디어 쿼리 시 모든 애니메이션 비활성화.

---

## 9. 접근성 체크리스트

- [ ] 텍스트 대비 비율 4.5:1 이상 (WCAG AA)
- [ ] 모든 인터랙티브 요소에 `cursor: pointer`
- [ ] 포커스 링 가시성 확보 (`outline: 2px solid #6366f1`)
- [ ] 아이콘 전용 버튼에 `aria-label` 속성
- [ ] 모달 오픈 시 포커스 트랩 + Esc 닫기
- [ ] Tab 순서 = 시각적 순서
- [ ] 폼 입력에 `<label>` + `htmlFor` 연결
- [ ] 상태를 색상만으로 표시하지 않음 (아이콘 + 텍스트 병행)
- [ ] `prefers-reduced-motion` 준수
- [ ] 이미지에 `alt` 텍스트
