import { useState } from 'react'
import { BellRing, ChevronDown, ChevronUp, Search } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useAlertHistory } from '../api/observability-api'
import type { AlertHistoryEntry, AlertSeverity } from '../api/observability-api'
import { Button } from '../../../components/ui/button'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

type ObsTab = 'stack' | 'cicd'

const CICD_MOCK_HISTORY: AlertHistoryEntry[] = [
  {
    id: 'cicd-alert-1',
    ruleName: 'Build Failure Spike',
    severity: 'critical',
    message: 'Build failure rate exceeded 15% in last 10 minutes.',
    firedAt: '2026-03-08T02:14:00Z',
    resolvedAt: '2026-03-08T02:29:00Z',
  },
  {
    id: 'cicd-alert-2',
    ruleName: 'Pipeline Queue Delay',
    severity: 'warning',
    message: 'Queue wait time is above 120 seconds.',
    firedAt: '2026-03-07T15:22:00Z',
    resolvedAt: null,
  },
  {
    id: 'cicd-alert-3',
    ruleName: 'Rollback Triggered',
    severity: 'info',
    message: 'Automatic rollback executed for backend-api deployment.',
    firedAt: '2026-03-07T10:03:00Z',
    resolvedAt: '2026-03-07T10:11:00Z',
  },
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
  const [activeTab, setActiveTab] = useState<ObsTab>('stack')
  const [expandedAlertId, setExpandedAlertId] = useState<string | null>(null)
  const [severityFilter, setSeverityFilter] = useState<AlertSeverity | ''>('')
  const [search, setSearch] = useState('')
  const [dateRange, setDateRange] = useState<'24h' | '7d' | '30d' | 'all'>('7d')

  const { data: apiData } = useAlertHistory(severityFilter ? { severity: severityFilter } : undefined)
  const history = activeTab === 'cicd' ? CICD_MOCK_HISTORY : (apiData?.items ?? [])

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

  const expandedAlert = filtered.find((e) => e.id === expandedAlertId) ?? null

  const columns: ColumnDef<AlertHistoryEntry, unknown>[] = [
    {
      id: 'expand',
      header: '',
      enableSorting: false,
      cell: ({ row }) => {
        const isExpanded = expandedAlertId === row.original.id
        return (
          <Button
            variant={isExpanded ? 'secondary' : 'ghost'}
            size="sm"
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              setExpandedAlertId((prev) => (prev === row.original.id ? null : row.original.id))
            }}
          >
            {isExpanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
          </Button>
        )
      },
    },
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
      <Breadcrumb items={[{ label: 'Alert History' }]} />

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

      <div className="mb-5 flex gap-1.5">
        {(['stack', 'cicd'] as const).map((tab) => {
          const active = activeTab === tab
          return (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              className={cn(
                'cursor-pointer rounded-[7px] border px-3 py-[5px] text-xs font-bold',
                active
                  ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]'
              )}
            >
              {tab === 'stack' ? 'Stack' : 'CI/CD'}
            </button>
          )
        })}
      </div>

      <DataTable
        columns={columns}
        data={filtered}
        getRowKey={(row) => row.id}
        emptyMessage="알림 이력이 없습니다."
        toolbar={
          <>
            <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value as AlertSeverity | '')} className={`${selectClassName} [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]`}>
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Severity</option>
              <option value="critical" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Critical</option>
              <option value="warning" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Warning</option>
              <option value="info" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Info</option>
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
            <div className="relative ml-auto">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <input
                placeholder="Rule name 검색..."
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
              />
            </div>
          </>
        }
      />

      {expandedAlert && (
        <div className="mt-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(0,0,0,0.2)] px-5 py-4">
          <p className="mb-3 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            Alert Detail
          </p>
          <div className="grid grid-cols-2 gap-x-8 gap-y-2">
            {[
              { label: 'Rule', value: expandedAlert.ruleName },
              { label: 'Severity', value: expandedAlert.severity },
              { label: 'Fired At', value: new Date(expandedAlert.firedAt).toLocaleString('ko-KR') },
              { label: 'Resolved At', value: expandedAlert.resolvedAt ? new Date(expandedAlert.resolvedAt).toLocaleString('ko-KR') : '미해결' },
            ].map(({ label, value }) => (
              <div key={label} className="flex gap-2 text-[13px]">
                <span className="w-[80px] shrink-0 text-[var(--color-text-muted)]">{label}</span>
                <span className="text-[var(--color-text-primary)]">{value}</span>
              </div>
            ))}
            <div className="col-span-2 flex gap-2 text-[13px]">
              <span className="w-[80px] shrink-0 text-[var(--color-text-muted)]">Message</span>
              <span className="text-[var(--color-text-primary)]">{expandedAlert.message}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
