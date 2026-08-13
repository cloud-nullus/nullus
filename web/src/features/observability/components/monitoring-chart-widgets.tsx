import React from "react"
import { StatusIcon, STATUS_TOKEN, type StatusTone } from '../../../components/ui/status-icon'
import { IconTile } from '../../../components/ui/icon-tile'
import type { ToolHealthStatus } from "../api/observability-api"

// ─── Shared chart style helpers ───────────────────────────────────────────────
export const CHART_STYLE = {
  bg: 'var(--color-surface-base)',
  // 격자는 데이터보다 뒤로 물러나야 한다. 예전 값(15%)에 점선까지 겹쳐
  // 격자가 선보다 눈에 띄었다.
  grid: 'color-mix(in srgb, var(--color-text-secondary) 10%, transparent)',
  tick: { fill: 'var(--color-text-secondary)', fontSize: 11 },
  tooltip: {
    background: 'var(--color-surface-card)',
    border: '1px solid var(--color-border-default)',
    borderRadius: 8,
    boxShadow: 'var(--shadow-overlay)',
    // 글자는 글자 토큰을 입는다. border 토큰을 쓰고 있어 툴팁 본문이
    // 배경에 묻혔다.
    color: 'var(--color-text-primary)',
    fontSize: 12,
  },
} as const

/**
 * 차트 계열색 — 정체성을 나타내는 카테고리 팔레트.
 *
 * 상태색(success/warning/error)을 계열로 돌려쓰지 않는다. 예전에는 Limit 을
 * `--color-warning` 으로 그렸는데, 한도는 정상 설정값인데도 *경고* 로 읽혔다.
 * 상태색은 상태에만 남겨 둔다.
 *
 * 값은 DESIGN.md 가 소유하고 색각 이상 분리도·표면 대비 검증을 통과한 것이다.
 */
export const CHART_SERIES = {
  request: 'var(--color-chart-1)',
  limit: 'var(--color-chart-2)',
  current: 'var(--color-chart-3)',
} as const

/**
 * 모든 recharts 범례에 그대로 펼쳐 넣는다.
 *
 * recharts 는 범례 항목을 이름 알파벳순으로 정렬한다. 그래서 "CPU (Request /
 * Limit / Current)" 라고 써 붙인 차트의 범례가 Current · Limit · Request 로 나오고,
 * 배포 막대는 success/failed 를 선언해도 failed 가 먼저 온다 — 제목이 약속한
 * 순서와 범례가 어긋난다. itemSorter 를 무력화해 선언 순서를 지킨다.
 */
export const CHART_LEGEND_PROPS = {
  itemSorter: () => 0,
  // 범례 글자도 글자 토큰을 입는다 — border 토큰이라 읽기 힘들었다.
  wrapperStyle: { color: 'var(--color-text-secondary)', fontSize: 11 },
} as const

// 도구 상태도 레지스트리에서 받는다. 여기 warning 에 CircleAlert 가 박혀 있어서
// 같은 "경고" 가 다른 화면에서는 삼각형인데 이 카드만 원이었다.
export const TOOL_STATUS: Record<ToolHealthStatus, { icon: React.ReactNode; style: React.CSSProperties; label: string }> = {
  running: { icon: <StatusIcon tone="success" size="xs" inheritColor />, style: toneSurface('success'), label: 'Running' },
  warning: { icon: <StatusIcon tone="warning" size="xs" inheritColor />, style: toneSurface('warning'), label: 'Warning' },
  error: { icon: <StatusIcon tone="error" size="xs" inheritColor />, style: toneSurface('error'), label: 'Error' },
}

/** tone 의 면 색 — 배경 15% 알파 + 글자는 원본. 토큰 이름을 조립하므로 style 이다. */
function toneSurface(tone: StatusTone): React.CSSProperties {
  const token = `var(${STATUS_TOKEN[tone]})`
  return { backgroundColor: `color-mix(in srgb, ${token} 15%, transparent)`, color: token }
}

// ─── Shared chart panel wrapper ───────────────────────────────────────────────
export function ChartPanel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-[10px] border border-[var(--color-border-default)] p-3" style={{ background: CHART_STYLE.bg }}>
      <div className="mb-2 text-[13px] font-bold text-[var(--color-text-primary)]">{title}</div>
      {children}
    </div>
  )
}

// ─── KPI card ────────────────────────────────────────────────────────────────
export function KpiCard({ label, value, icon, color, token, bar }: { label: string; value: string; icon: React.ReactNode; color: string; token: string; bar: number }) {
  return (
    <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
      <div className="mb-2.5 flex items-center gap-2.5">
        <IconTile token={token}>{icon}</IconTile>
        <span className="text-xs font-medium text-[var(--color-text-secondary)]">{label}</span>
      </div>
      <div className="text-[28px] font-extrabold leading-none text-[var(--color-text-primary)]">{value}</div>
      <div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)]">
        <svg className="h-full w-full" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
          <rect width={Math.max(0, Math.min(100, bar))} height="6" rx="3" fill={color} />
        </svg>
      </div>
    </div>
  )
}
