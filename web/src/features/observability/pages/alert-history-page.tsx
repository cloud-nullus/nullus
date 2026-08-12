import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { BellRing, ChevronDown, ChevronUp } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useAlertHistory } from '../api/observability-api'
import type { AlertHistoryEntry, AlertSeverity } from '../api/observability-api'
import { Button } from '../../../components/ui/button'
import { Select } from '../../../components/ui/select'
import { DataTable } from '../../../components/shared/data-table'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { cn } from '../../../lib/utils'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'
import { formatDateTime, resolveLocale } from '../../../lib/locale'
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from '../../../components/ui/search-input'

const SEVERITY_BADGE: Record<AlertSeverity, { className: string }> = {
  critical: { className: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]' },
  warning: { className: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]' },
  info: { className: 'bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]' },
}

function getSeverityLabel(t: TFunction, severity: AlertSeverity) {
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
          <span className="whitespace-nowrap text-[13px] text-[var(--color-success)]">{formatDateTime(row.original.resolvedAt, locale)}</span>
        ) : (
          <span className="whitespace-nowrap text-[13px] text-[var(--color-error)]">{t('alertHistoryPage.unresolved', 'Unresolved')}</span>
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
      <PageHeader
        breadcrumb={[{ label: t('observability.alertHistory', 'Alert History') }]}
        icon={<BellRing size={16} />}
        tone="warning"
        title={t('observability.alertHistory', 'Alert History')}
        subtitle={t('observability.alertHistoryDesc', 'Alert occurrence history')}
      />

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

      {/* 상세는 표 아래가 아니라 오른쪽에 붙는다. 아래로 펼치면 행을 고를 때마다
          표가 밀려 내려가 방금 고른 행을 잃는다 (DESIGN.md §Layout). */}
      <ListDetailPanel
        detailWidth={340}
        emptyDetailMessage={t('alertHistoryPage.selectAlert', 'Select an alert to view its detail.')}
        detailContent={
          expandedAlert && (
            <div className="p-3">
              <p className="mb-2.5 mt-0 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                {t('alertHistoryPage.detail.title', 'Alert Detail')}
              </p>
              <div className="flex flex-col gap-1.5">
                {[
                  { label: t('alertHistoryPage.detail.rule', 'Rule'), value: expandedAlert.ruleName },
                  { label: t('alertHistoryPage.detail.severity', 'Severity'), value: expandedAlert.severity },
                  { label: t('alertHistoryPage.detail.firedAt', 'Fired At'), value: formatDateTime(expandedAlert.firedAt, locale) },
                  { label: t('alertHistoryPage.detail.resolvedAt', 'Resolved At'), value: expandedAlert.resolvedAt ? formatDateTime(expandedAlert.resolvedAt, locale) : t('alertHistoryPage.unresolved', 'Unresolved') },
                  { label: t('alertHistoryPage.detail.message', 'Message'), value: expandedAlert.message },
                ].map(({ label, value }) => (
                  <div key={label} className="flex gap-2 text-[13px]">
                    <span className="w-[76px] shrink-0 text-[var(--color-text-muted)]">{label}</span>
                    <span className="min-w-0 break-words text-[var(--color-text-primary)]">{value}</span>
                  </div>
                ))}
              </div>
            </div>
          )
        }
        listContent={
          <DataTable
            flush
            columns={columns}
            data={filtered}
            getRowKey={(row) => row.id}
            onRowClick={(row) => setExpandedAlertId(row.id)}
            emptyMessage={t('alertHistoryPage.empty', 'No alert history found.')}
            toolbar={
              <>
                <Select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value as AlertSeverity | '')} >
                  <option value="">{t('alertHistoryPage.filters.allSeverity', 'All Severity')}</option>
                  <option value="critical">{t('observability.severity.critical', 'Critical')}</option>
                  <option value="warning">{t('observability.severity.warning', 'Warning')}</option>
                  <option value="info">{t('observability.severity.info', 'Info')}</option>
                </Select>
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
                            ? 'border-[color-mix(in_srgb,_var(--color-info)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]'
                            : 'border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] text-[var(--color-text-secondary)]'
                        )}
                      >
                        {item.label}
                      </button>
                    )
                  })}
                </div>
                <SearchInput
                  wrapperClassName="ml-auto w-[220px]"
                  placeholder={t('alertHistoryPage.searchPlaceholder', 'Search rule name...')}
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                />
              </>
        }
      />
        }
      />
    </div>
  )
}
