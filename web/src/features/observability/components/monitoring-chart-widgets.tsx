import React from "react"
import type { ToolHealthStatus } from "../api/observability-api"
import { CheckCircle, AlertCircle, XCircle } from "lucide-react"
import { cn } from "../../../lib/utils"

// ─── Shared chart style helpers ───────────────────────────────────────────────
export const CHART_STYLE = {
  bg: '#0b1220',
  grid: 'rgba(148,163,184,0.15)',
  tick: { fill: '#94a3b8', fontSize: 11 },
  tooltip: { background: '#111827', border: '1px solid #374151', color: '#e5e7eb' },
}

export const TOOL_STATUS: Record<ToolHealthStatus, { icon: React.ReactNode; cls: string; label: string }> = {
  running: { icon: <CheckCircle size={13} />, cls: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Running' },
  warning: { icon: <AlertCircle size={13} />, cls: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Warning' },
  error: { icon: <XCircle size={13} />, cls: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', label: 'Error' },
}

// ─── Shared chart panel wrapper ───────────────────────────────────────────────
export function ChartPanel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-[10px] border border-[var(--color-border-default)] p-3" style={{ background: CHART_STYLE.bg }}>
      <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">{title}</div>
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
      <div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[rgba(255,255,255,0.08)]">
        <svg className="h-full w-full" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
          <rect width={Math.max(0, Math.min(100, bar))} height="6" rx="3" fill={color} />
        </svg>
      </div>
    </div>
  )
}
