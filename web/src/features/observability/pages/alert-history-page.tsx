import { useState } from 'react'
import { BellRing } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useAlertHistory } from '../api/observability-api'
import type { AlertHistoryEntry, AlertSeverity } from '../api/observability-api'
import { Input } from '../../../components/ui/input'
import { DataTable } from '../../../components/shared/data-table'
import { cn } from '../../../lib/utils'

const MOCK_ALERT_HISTORY: AlertHistoryEntry[] = [
  { id: 'ah1', ruleName: 'High CPU', severity: 'critical', message: 'CPU usage exceeded 80% on prod-cluster', firedAt: '2026-03-14T07:30:00Z', resolvedAt: '2026-03-14T07:45:00Z' },
  { id: 'ah2', ruleName: 'Memory Warning', severity: 'warning', message: 'Memory usage at 85% on staging-cluster', firedAt: '2026-03-13T15:00:00Z', resolvedAt: '2026-03-13T15:20:00Z' },
  { id: 'ah3', ruleName: 'Pod CrashLoop', severity: 'critical', message: 'Pod api-server-xyz in CrashLoopBackOff', firedAt: '2026-03-12T11:00:00Z', resolvedAt: null },
  { id: 'ah4', ruleName: 'High CPU', severity: 'info', message: 'CPU usage returned to normal', firedAt: '2026-03-11T09:00:00Z', resolvedAt: '2026-03-11T09:01:00Z' },
]

const SEVERITY_BADGE: Record<AlertSeverity, { className: string; label: string }> = {
  critical: { className: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]', label: 'Critical' },
  warning: { className: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]', label: 'Warning' },
  info: { className: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]', label: 'Info' },
}

const selectClassName = 'cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'

function formatDate(iso: string | null) {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function AlertHistoryPage() {
  const [severityFilter, setSeverityFilter] = useState<AlertSeverity | ''>('')
  const [search, setSearch] = useState('')
  const [dateRange, setDateRange] = useState<'24h' | '7d' | '30d' | 'all'>('7d')

  const { data: apiData } = useAlertHistory(severityFilter ? { severity: severityFilter } : undefined)
  const history = apiData?.items ?? MOCK_ALERT_HISTORY

  const filtered = history.filter((entry) => {
    if (severityFilter && entry.severity !== severityFilter) return false

    if (search && !entry.ruleName.toLowerCase().includes(search.toLowerCase())) return false

    if (dateRange === 'all') return true

    const now = Date.now()
    const fromByRange: Record<'24h' | '7d' | '30d', number> = {
      '24h': now - 24 * 60 * 60 * 1000,
      '7d': now - 7 * 24 * 60 * 60 * 1000,
      '30d': now - 30 * 24 * 60 * 60 * 1000,
    }
    return new Date(entry.firedAt).getTime() >= fromByRange[dateRange]
  })

  const columns: ColumnDef<AlertHistoryEntry, unknown>[] = [
    {
      accessorKey: 'ruleName',
      header: '알림명',
      cell: ({ row }) => <span className="font-semibold">{row.original.ruleName}</span>,
    },
    {
      accessorKey: 'severity',
      header: '심각도',
      cell: ({ row }) => {
        const sev = SEVERITY_BADGE[row.original.severity]
        return (
          <span className={cn('rounded-md px-[9px] py-[3px] text-xs font-semibold', sev.className)}>
            {sev.label}
          </span>
        )
      },
    },
    {
      accessorKey: 'message',
      header: '메시지',
      cell: ({ row }) => <span className="max-w-[360px] text-[13px] text-[var(--color-text-secondary)]">{row.original.message}</span>,
    },
    {
      accessorKey: 'firedAt',
      header: '발생 시간',
      cell: ({ row }) => <span className="whitespace-nowrap text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.firedAt)}</span>,
    },
    {
      accessorKey: 'resolvedAt',
      header: '해결 시간',
      cell: ({ row }) =>
        row.original.resolvedAt ? (
          <span className="whitespace-nowrap text-[13px] text-[#22c55e]">{formatDate(row.original.resolvedAt)}</span>
        ) : (
          <span className="whitespace-nowrap text-[13px] text-[#f87171]">미해결</span>
        ),
    },
  ]

  return (
    <div>
      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(245,158,11,0.15)] text-[#fbbf24]">
          <BellRing size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Alert History
          </h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
            알림 발생 이력
          </p>
        </div>
      </div>

      {/* Filters */}
      <div className="mb-4 flex flex-wrap gap-2.5">
        <div className="min-w-[220px] max-w-[320px] flex-[1_1_220px]">
          <Input
            placeholder="Rule name 검색..."
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value as AlertSeverity | '')} className={selectClassName}>
          <option value="">All Severity</option>
          <option value="critical">Critical</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
        </select>
        <div className="flex gap-1.5">
          {[
            { id: '24h', label: 'Last 24h' },
            { id: '7d', label: 'Last 7d' },
            { id: '30d', label: 'Last 30d' },
            { id: 'all', label: 'All' },
          ].map((item) => {
            const active = dateRange === item.id
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setDateRange(item.id as '24h' | '7d' | '30d' | 'all')}
                className={cn(
                  'cursor-pointer rounded-[7px] border px-2.5 py-1.5 text-xs font-semibold',
                  active
                    ? 'border-[rgba(59,130,246,0.5)] bg-[rgba(59,130,246,0.15)] text-[#93c5fd]'
                    : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]'
                )}
              >
                {item.label}
              </button>
            )
          })}
        </div>
      </div>

      <DataTable columns={columns} data={filtered} getRowKey={(row) => row.id} emptyMessage="알림 이력이 없습니다." />
    </div>
  )
}
