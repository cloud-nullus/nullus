import React from "react"
import { IconTile } from '../../../components/ui/icon-tile'
import type { ToolHealthStatus } from "../api/observability-api"
import { CircleAlert, CircleCheck, CircleX } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'

// ─── Shared chart style helpers ───────────────────────────────────────────────
export const CHART_STYLE = {
  bg: 'var(--color-surface-base)',
  grid: 'color-mix(in srgb, var(--color-text-secondary) 15%, transparent)',
  tick: { fill: 'var(--color-text-secondary)', fontSize: 11 },
  tooltip: { background: 'var(--color-surface-base)', border: '1px solid var(--color-border-default)', color: 'var(--color-border-default)' },
}

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
  wrapperStyle: { color: 'var(--color-border-default)', fontSize: 11 },
} as const

export const TOOL_STATUS: Record<ToolHealthStatus, { icon: React.ReactNode; cls: string; label: string }> = {
  running: { icon: <CircleCheck {...iconProps('xs')} />, cls: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]', label: 'Running' },
  warning: { icon: <CircleAlert {...iconProps('xs')} />, cls: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]', label: 'Warning' },
  error: { icon: <CircleX {...iconProps('xs')} />, cls: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]', label: 'Error' },
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
