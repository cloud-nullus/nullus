import React from "react"
import type { ToolHealthStatus } from "../api/observability-api"
import { CheckCircle, AlertCircle, XCircle } from "lucide-react"
import { cn } from "../../../lib/utils"

// ─── Shared chart style helpers ───────────────────────────────────────────────
export const CHART_STYLE = {
  bg: 'var(--color-surface-base)',
  grid: 'color-mix(in srgb, var(--color-text-secondary) 15%, transparent)',
  tick: { fill: 'var(--color-text-secondary)', fontSize: 11 },
  tooltip: { background: 'var(--color-surface-base)', border: '1px solid var(--color-border-default)', color: 'var(--color-border-default)' },
}

export const TOOL_STATUS: Record<ToolHealthStatus, { icon: React.ReactNode; cls: string; label: string }> = {
  running: { icon: <CheckCircle size={13} />, cls: 'bg-[color-mix(in srgb, var(--color-success) 15%, transparent)] text-[var(--color-success)]', label: 'Running' },
  warning: { icon: <AlertCircle size={13} />, cls: 'bg-[color-mix(in srgb, var(--color-warning) 15%, transparent)] text-[var(--color-warning)]', label: 'Warning' },
  error: { icon: <XCircle size={13} />, cls: 'bg-[color-mix(in srgb, var(--color-error) 15%, transparent)] text-[var(--color-error)]', label: 'Error' },
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
export function KpiCard({ label, value, icon, color, iconCls, bar }: { label: string; value: string; icon: React.ReactNode; color: string; iconCls: string; bar: number }) {
  return (
    <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
      <div className="mb-2.5 flex items-center gap-2.5">
        <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', iconCls)}>{icon}</div>
        <span className="text-xs font-medium text-[var(--color-text-secondary)]">{label}</span>
      </div>
      <div className="text-[28px] font-extrabold leading-none text-[var(--color-text-primary)]">{value}</div>
      <div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[color-mix(in srgb, var(--color-text-primary) 8%, transparent)]">
        <svg className="h-full w-full" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
          <rect width={Math.max(0, Math.min(100, bar))} height="6" rx="3" fill={color} />
        </svg>
      </div>
    </div>
  )
}
