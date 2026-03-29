import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { BellRing, ChevronDown, ChevronUp, Search } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useAlertHistory } from '../api/observability-api'
import type { AlertHistoryEntry, AlertSeverity } from '../api/observability-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'
import { formatDateTime, resolveLocale } from '../../../lib/locale'

const SEVERITY_BADGE: Record<AlertSeverity, { className: string }> = {
  critical: { className: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]' },
  warning: { className: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]' },
  info: { className: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]' },
}

function getSeverityLabel(t: (key: string, defaultValue?: string) => string, severity: AlertSeverity) {
  if (severity === 'critical') return t('observability.severity.critical', 'Critical')
  if (severity === 'warning') return t('observability.severity.warning', 'Warning')
  return t('observability.severity.info', 'Info')
}

export function AlertHistoryPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId] = useState('')
  const [expandedAlertId, setExpandedAlertId] = useState<string | null>(null)
  const [severityFilter, setSeverityFilter] = useState<AlertSeverity | ''>('')
  const [search, setSearch] = useState('')
  const [dateRange, setDateRange] = useState<'24h' | '7d' | '30d' | 'all'>('7d')
  const { clusters, filteredStacks, selectedCluster, selectedStack } = useClusterStackFilterState(selectedClusterId, selectedStackId)

  const { data: apiData } = useAlertHistory(severityFilter ? { severity: severityFilter } : undefined)
  const history = apiData?.items ?? []

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
      header: t('alertHistoryPage.table.ruleName', 'Rule Name'),
      cell: ({ row }) => <span className="font-semibold">{row.original.ruleName}</span>,
    },
    {
      accessorKey: 'severity',
      header: t('alertHistoryPage.table.severity', 'Severity'),
      cell: ({ row }) => {
        const sev = SEVERITY_BADGE[row.original.severity]
        return (
          <span className={cn('rounded-md px-[9px] py-[3px] text-xs font-semibold', sev.className)}>
            {getSeverityLabel(t, row.original.severity)}
          </span>
        )
      },
    },
    {
      accessorKey: 'message',
      header: t('alertHistoryPage.table.message', 'Message'),
      cell: ({ row }) => <span className="max-w-[360px] text-[13px] text-[var(--color-text-secondary)]">{row.original.message}</span>,
    },
    {
      accessorKey: 'firedAt',
      header: t('alertHistoryPage.table.firedAt', 'Fired At'),
      cell: ({ row }) => <span className="whitespace-nowrap text-[13px] text-[var(--color-text-secondary)]">{formatDateTime(row.original.firedAt, locale)}</span>,
    },
    {
      accessorKey: 'resolvedAt',
      header: t('alertHistoryPage.table.resolvedAt', 'Resolved At'),
      cell: ({ row }) =>
        row.original.resolvedAt ? (
          <span className="whitespace-nowrap text-[13px] text-[#22c55e]">{formatDateTime(row.original.resolvedAt, locale)}</span>
        ) : (
          <span className="whitespace-nowrap text-[13px] text-[#f87171]">{t('alertHistoryPage.unresolved', 'Unresolved')}</span>
        ),
    },
  ]

  const handleClusterChange = (clusterId: string) => {
    setSelectedClusterId(clusterId)
    setSelectedStackId('')
  }

  const handleStackChange = (stackId: string) => {
    setSelectedStackId(stackId)
  }

  return (
    <div>
      <Breadcrumb items={[{ label: t('observability.alertHistory', 'Alert History') }]} />

      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(245,158,11,0.15)] text-[#fbbf24]">
          <BellRing size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            {t('observability.alertHistory', 'Alert History')}
          </h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
            {t('observability.alertHistoryDesc', 'Alert occurrence history')}
          </p>
        </div>
      </div>

      <ClusterStackFilter
        selectedClusterId={selectedClusterId}
        selectedStackId={selectedStackId}
        onClusterChange={handleClusterChange}
        onStackChange={handleStackChange}
        onClear={() => { setSelectedClusterId(''); setSelectedStackId('') }}
        clusters={clusters}
        filteredStacks={filteredStacks}
        selectedCluster={selectedCluster}
        selectedStack={selectedStack}
      />

      <DataTable
        columns={columns}
        data={filtered}
        getRowKey={(row) => row.id}
        emptyMessage={t('alertHistoryPage.empty', 'No alert history found.')}
        toolbar={
          <>
            <NativeSelect value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value as AlertSeverity | '')} className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]">
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('alertHistoryPage.filters.allSeverity', 'All Severity')}</option>
              <option value="critical" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('observability.severity.critical', 'Critical')}</option>
              <option value="warning" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('observability.severity.warning', 'Warning')}</option>
              <option value="info" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('observability.severity.info', 'Info')}</option>
            </NativeSelect>
            <div className="flex gap-1.5">
              {[
                { id: '24h', label: t('alertHistoryPage.filters.last24h', 'Last 24h') },
                { id: '7d', label: t('alertHistoryPage.filters.last7d', 'Last 7d') },
                { id: '30d', label: t('alertHistoryPage.filters.last30d', 'Last 30d') },
                { id: 'all', label: t('alertHistoryPage.filters.all', 'All') },
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
                placeholder={t('alertHistoryPage.searchPlaceholder', 'Search rule name...')}
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
            {t('alertHistoryPage.detail.title', 'Alert Detail')}
          </p>
          <div className="grid grid-cols-2 gap-x-8 gap-y-2">
            {[
              { label: t('alertHistoryPage.detail.rule', 'Rule'), value: expandedAlert.ruleName },
              { label: t('alertHistoryPage.detail.severity', 'Severity'), value: expandedAlert.severity },
              { label: t('alertHistoryPage.detail.firedAt', 'Fired At'), value: formatDateTime(expandedAlert.firedAt, locale) },
              { label: t('alertHistoryPage.detail.resolvedAt', 'Resolved At'), value: expandedAlert.resolvedAt ? formatDateTime(expandedAlert.resolvedAt, locale) : t('alertHistoryPage.unresolved', 'Unresolved') },
            ].map(({ label, value }) => (
              <div key={label} className="flex gap-2 text-[13px]">
                <span className="w-[80px] shrink-0 text-[var(--color-text-muted)]">{label}</span>
                <span className="text-[var(--color-text-primary)]">{value}</span>
              </div>
            ))}
            <div className="col-span-2 flex gap-2 text-[13px]">
              <span className="w-[80px] shrink-0 text-[var(--color-text-muted)]">{t('alertHistoryPage.detail.message', 'Message')}</span>
              <span className="text-[var(--color-text-primary)]">{expandedAlert.message}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
