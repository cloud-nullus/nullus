import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronDown, ChevronUp, History } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import type { ColumnDef } from '@tanstack/react-table'
import { useStackAlerts } from '../api/observability-api'
import type { StackAlert, StackAlertState } from '../api/observability-api'
import { Button } from '../../../components/ui/button'
import { DataTable } from '../../../components/shared/data-table'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { cn } from '../../../lib/utils'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'
import { formatDateTime, resolveLocale } from '../../../lib/locale'
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from '../../../components/ui/search-input'

type DateRange = '24h' | '7d' | '30d' | 'all'

const RANGE_MS: Record<Exclude<DateRange, 'all'>, number> = {
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
  '30d': 30 * 24 * 60 * 60 * 1000,
}

const pillClass = (active: boolean) =>
  cn(
    'cursor-pointer rounded-[7px] border px-2.5 py-1.5 text-xs font-semibold',
    active
      ? 'border-[color-mix(in_srgb,_var(--color-info)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]'
      : 'border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] text-[var(--color-text-secondary)]',
  )

/**
 * 알림 이력 — 스택 심각 경보 (nullus-plan#87).
 *
 * 스택마다 Prometheus 가 kube-prometheus-stack 기본 규칙으로 평가한 심각(critical) 경보를 플랫폼이
 * Alertmanager 에서 가져와 남긴 것이다. 예전 화면은 평가되지 않는 규칙의 데모 이력을 보여 줬다.
 */
export function AlertHistoryPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId] = useState('')
  const [expandedAlertId, setExpandedAlertId] = useState<string | null>(null)
  const [state, setState] = useState<StackAlertState>('all')
  const [search, setSearch] = useState('')
  const [dateRange, setDateRange] = useState<DateRange>('7d')
  const { clusters, filteredStacks, selectedCluster, selectedStack } = useClusterStackFilterState(
    selectedClusterId,
    selectedStackId,
  )

  const { data } = useStackAlerts(state)
  const alerts = useMemo(() => data ?? [], [data])

  const filtered = useMemo(() => {
    const clusterStackIds = selectedClusterId ? new Set(filteredStacks.map((s) => s.id)) : null
    const q = search.trim().toLowerCase()
    const now = Date.now()
    return alerts.filter((a) => {
      if (selectedStackId && a.stack_id !== selectedStackId) return false
      if (clusterStackIds && !clusterStackIds.has(a.stack_id)) return false
      if (q && ![a.alertname, a.stack_name, a.summary].some((v) => v?.toLowerCase().includes(q))) return false
      // 그 기간에 걸쳐 있었는지로 본다 — 진행 중인 경보는 언제 시작했든 지금도 걸쳐 있다.
      const lastActive = a.resolved_at ? new Date(a.resolved_at).getTime() : now
      if (dateRange !== 'all' && lastActive < now - RANGE_MS[dateRange]) return false
      return true
    })
  }, [alerts, selectedClusterId, selectedStackId, filteredStacks, search, dateRange])

  const expandedAlert = filtered.find((a) => a.id === expandedAlertId) ?? null

  const columns: ColumnDef<StackAlert, unknown>[] = [
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
            aria-label={t('alertHistoryPage.detail.title', 'Alert Detail')}
            onClick={(e) => {
              e.stopPropagation()
              setExpandedAlertId((prev) => (prev === row.original.id ? null : row.original.id))
            }}
          >
            {isExpanded ? <ChevronUp {...iconProps('xs')} /> : <ChevronDown {...iconProps('xs')} />}
          </Button>
        )
      },
    },
    {
      accessorKey: 'alertname',
      header: t('alertHistoryPage.table.alert', 'Alert'),
      cell: ({ row }) => <span className="font-semibold">{row.original.alertname}</span>,
    },
    {
      accessorKey: 'stack_name',
      header: t('alertHistoryPage.table.stack', 'Stack'),
      cell: ({ row }) => <span className="text-[13px]">{row.original.stack_name}</span>,
    },
    {
      accessorKey: 'target',
      header: t('alertHistoryPage.table.target', 'Target'),
      cell: ({ row }) => (
        <span className="font-mono text-xs text-[var(--color-text-secondary)]">{row.original.target || '—'}</span>
      ),
    },
    {
      accessorKey: 'summary',
      header: t('alertHistoryPage.table.message', 'Message'),
      cell: ({ row }) => (
        <span className="max-w-[360px] text-[13px] text-[var(--color-text-secondary)]">{row.original.summary}</span>
      ),
    },
    {
      accessorKey: 'started_at',
      header: t('alertHistoryPage.table.firedAt', 'Fired At'),
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-[13px] text-[var(--color-text-secondary)]">
          {formatDateTime(row.original.started_at, locale)}
        </span>
      ),
    },
    {
      accessorKey: 'resolved_at',
      header: t('alertHistoryPage.table.resolvedAt', 'Resolved At'),
      cell: ({ row }) =>
        row.original.resolved_at ? (
          <span className="whitespace-nowrap text-[13px] text-[var(--color-success)]">
            {formatDateTime(row.original.resolved_at, locale)}
          </span>
        ) : (
          <span className="whitespace-nowrap rounded-md bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] px-[9px] py-[3px] text-xs font-semibold text-[var(--color-error)]">
            {t('alertHistoryPage.active', 'Active')}
          </span>
        ),
    },
  ]

  const detailRows = expandedAlert
    ? [
        { label: t('alertHistoryPage.detail.alert', 'Alert'), value: expandedAlert.alertname },
        { label: t('alertHistoryPage.detail.stack', 'Stack'), value: expandedAlert.stack_name },
        { label: t('alertHistoryPage.detail.target', 'Target'), value: expandedAlert.target || '—' },
        { label: t('alertHistoryPage.detail.severity', 'Severity'), value: expandedAlert.severity },
        { label: t('alertHistoryPage.detail.firedAt', 'Fired At'), value: formatDateTime(expandedAlert.started_at, locale) },
        {
          label: t('alertHistoryPage.detail.resolvedAt', 'Resolved At'),
          value: expandedAlert.resolved_at
            ? formatDateTime(expandedAlert.resolved_at, locale)
            : t('alertHistoryPage.active', 'Active'),
        },
        {
          label: t('alertHistoryPage.detail.lastNotified', 'Last notified'),
          value: expandedAlert.last_notified_at ? formatDateTime(expandedAlert.last_notified_at, locale) : '—',
        },
        { label: t('alertHistoryPage.detail.message', 'Message'), value: expandedAlert.summary },
        { label: t('alertHistoryPage.detail.description', 'Description'), value: expandedAlert.description || '—' },
      ]
    : []

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('observability.alertHistory', 'Alert History') }]}
        icon={<History {...iconProps('sm')} />}
        tone="warning"
        title={t('observability.alertHistory', 'Alert History')}
        subtitle={t('observability.alertHistoryDesc', 'Alert occurrence history')}
      />

      <ClusterStackFilter
        selectedClusterId={selectedClusterId}
        selectedStackId={selectedStackId}
        onClusterChange={(clusterId) => {
          setSelectedClusterId(clusterId)
          setSelectedStackId('')
        }}
        onStackChange={setSelectedStackId}
        onClear={() => {
          setSelectedClusterId('')
          setSelectedStackId('')
        }}
        clusters={clusters}
        filteredStacks={filteredStacks}
        selectedCluster={selectedCluster}
        selectedStack={selectedStack}
      />

      {/* 상세는 표 아래가 아니라 오른쪽에 붙는다. 아래로 펼치면 행을 고를 때마다
          표가 밀려 내려가 방금 고른 행을 잃는다 (DESIGN.md §Layout). */}
      <ListDetailPanel
        detailWidth={360}
        emptyDetailMessage={t('alertHistoryPage.selectAlert', 'Select an alert to view its detail.')}
        detailContent={
          expandedAlert && (
            <div className="p-3">
              <p className="mb-2.5 mt-0 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                {t('alertHistoryPage.detail.title', 'Alert Detail')}
              </p>
              <div className="flex flex-col gap-1.5">
                {detailRows.map(({ label, value }) => (
                  <div key={label} className="flex gap-2 text-[13px]">
                    <span className="w-[84px] shrink-0 text-[var(--color-text-muted)]">{label}</span>
                    <span className="min-w-0 break-words text-[var(--color-text-primary)]">{value}</span>
                  </div>
                ))}
                {expandedAlert.runbook_url && (
                  <a
                    href={expandedAlert.runbook_url}
                    target="_blank"
                    rel="noreferrer"
                    className="mt-1 text-[13px] text-[var(--color-primary)] underline"
                  >
                    {t('alertHistoryPage.detail.runbook', 'Open runbook')}
                  </a>
                )}
                <div className="mt-2">
                  <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                    {t('alertHistoryPage.detail.labels', 'Labels')}
                  </p>
                  <ul className="m-0 list-none p-0 font-mono text-xs text-[var(--color-text-secondary)]">
                    {Object.entries(expandedAlert.labels ?? {})
                      .sort(([a], [b]) => a.localeCompare(b))
                      .map(([k, v]) => (
                        <li key={k}>{`${k}=${v}`}</li>
                      ))}
                  </ul>
                </div>
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
            emptyMessage={t(
              'alertHistoryPage.empty',
              "No critical alerts from your stacks. Each stack's Prometheus evaluates the default rules, and firing alerts appear here.",
            )}
            toolbar={
              <>
                <div className="flex gap-1.5">
                  {(['all', 'open'] as const).map((s) => (
                    <button key={s} type="button" onClick={() => setState(s)} className={pillClass(state === s)}>
                      {s === 'open'
                        ? t('alertHistoryPage.filters.activeOnly', 'Active only')
                        : t('alertHistoryPage.filters.allStates', 'All')}
                    </button>
                  ))}
                </div>
                <div className="flex gap-1.5">
                  {(
                    [
                      { id: '24h', label: t('alertHistoryPage.filters.last24h', 'Last 24h') },
                      { id: '7d', label: t('alertHistoryPage.filters.last7d', 'Last 7d') },
                      { id: '30d', label: t('alertHistoryPage.filters.last30d', 'Last 30d') },
                      { id: 'all', label: t('alertHistoryPage.filters.all', 'All time') },
                    ] as const
                  ).map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => setDateRange(item.id)}
                      className={pillClass(dateRange === item.id)}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
                <SearchInput
                  wrapperClassName="ml-auto w-[240px]"
                  placeholder={t('alertHistoryPage.searchPlaceholder', 'Search alert, stack, or message...')}
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
