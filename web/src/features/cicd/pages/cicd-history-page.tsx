import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ChevronDown, ChevronUp, History, Search } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import type { ColumnDef } from '@tanstack/react-table'
import { useDeployments } from '../api/cicd-api'
import type { Deployment, PipelineStatus } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { DataTable } from '../../../components/shared/data-table'

const STATUS_STYLES: Record<string, { bg: string; color: string }> = {
  running: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e' },
  failed: { bg: 'rgba(239,68,68,0.15)', color: '#ef4444' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b' },
  cancelled: { bg: 'rgba(100,116,139,0.15)', color: '#64748b' },
}

function formatDate(iso: string | null, locale = 'en-US') {
  if (!iso) return '-'
  return new Date(iso).toLocaleString(locale, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function getPipelineStatusLabel(t: (key: string, defaultValue?: string) => string, status: string) {
  if (status === 'running') return t('cicd.status.running', 'Running')
  if (status === 'success') return t('cicd.status.success', 'Success')
  if (status === 'failed') return t('cicd.status.failed', 'Failed')
  if (status === 'pending') return t('cicd.status.pending', 'Pending')
  if (status === 'cancelled') return t('cicd.status.cancelled', 'Cancelled')
  return status
}
export function CicdHistoryPage() {
  const { t, i18n } = useTranslation()
  const locale = i18n.resolvedLanguage?.startsWith('ko') ? 'ko-KR' : 'en-US'
  const [searchParams] = useSearchParams()
  const pipelineFilter = searchParams.get('pipeline') ?? ''
  const [statusFilter, setStatusFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [expandedDeploymentId, setExpandedDeploymentId] = useState<string | null>(null)
  const [search, setSearch] = useState('')

  const { data: apiData } = useDeployments({
    pipelineId: pipelineFilter || undefined,
    status: statusFilter as PipelineStatus || undefined,
  })
  const deployments = apiData?.items ?? []

  const filtered = deployments.filter((d) => {
    const matchesPipeline = !pipelineFilter || d.pipelineId === pipelineFilter
    const matchesStatus = !statusFilter || d.status === statusFilter
    const matchesType = !typeFilter || d.pipelineName.includes(typeFilter)
    const matchesSearch = !search || d.pipelineName.toLowerCase().includes(search.toLowerCase()) || d.triggeredBy.toLowerCase().includes(search.toLowerCase())
    return matchesPipeline && matchesStatus && matchesType && matchesSearch
  })
  const columns: ColumnDef<Deployment, unknown>[] = [
    {
      id: 'expand',
      header: '',
      enableSorting: false,
      cell: ({ row }) => {
        const isExpanded = expandedDeploymentId === row.original.id
        return (
          <Button
            variant={isExpanded ? 'secondary' : 'ghost'}
            size="sm"
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              setExpandedDeploymentId((prev) =>
                prev === row.original.id ? null : row.original.id
              )
            }}
          >
            {isExpanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
          </Button>
        )
      },
    },
    {
      accessorKey: 'pipelineName',
      header: t('cicdHistoryPage.table.pipeline', 'Pipeline'),
      cell: ({ row }) => <span className="font-semibold">{row.original.pipelineName}</span>,
    },
    {
      accessorKey: 'version',
      header: t('cicdHistoryPage.table.version', 'Version'),
      cell: ({ row }) => <span className="font-mono text-[13px]">{row.original.version}</span>,
    },
    {
      accessorKey: 'status',
      header: t('cicdHistoryPage.table.status', 'Status'),
      cell: ({ row }) => {
        const st = STATUS_STYLES[row.original.status] ?? STATUS_STYLES.pending
        return (
          <span className="rounded-md px-[9px] py-[3px] text-xs font-semibold" style={{ backgroundColor: st.bg, color: st.color }}>
            {getPipelineStatusLabel(t, row.original.status)}
          </span>
        )
      },
    },
    {
      accessorKey: 'triggeredBy',
      header: t('cicdHistoryPage.table.triggeredBy', 'Deployer'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{row.original.triggeredBy}</span>,
    },
    {
      accessorKey: 'startedAt',
      header: t('cicdHistoryPage.table.startedAt', 'Started At'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.startedAt, locale)}</span>,
    },
    {
      accessorKey: 'completedAt',
      header: t('cicdHistoryPage.table.completedAt', 'Completed At'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.completedAt, locale)}</span>,
    },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: t('sidebar.cicdList', 'CI/CD List'), path: '/cicd/list' }, { label: t('cicdHistoryPage.title', 'CI/CD History') }]} />

      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div
          className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(245,158,11,0.15)] text-[#fbbf24]"
        >
          <History size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            {t('cicdHistoryPage.title', 'CI/CD History')}
          </h1>
          <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
            {t('cicdHistoryPage.description', 'CI/CD Deployment History')}
          </p>
        </div>
      </div>

      {pipelineFilter && (
        <div className="mb-4 flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(59,130,246,0.08)] px-3 py-2 text-sm">
          <span className="text-[var(--color-text-secondary)]">{t('cicdHistoryPage.filteredByPipeline', 'Filtered by pipeline:')}</span>
          <span className="font-semibold text-[var(--color-text-primary)]">
            {deployments.find((d) => d.pipelineId === pipelineFilter)?.pipelineName ?? pipelineFilter}
          </span>
          <button
            type="button"
            onClick={() => {
              const newParams = new URLSearchParams(searchParams)
              newParams.delete('pipeline')
              window.history.replaceState({}, '', `${window.location.pathname}?${newParams.toString()}`)
              window.location.reload()
            }}
            className="ml-auto cursor-pointer text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
          >
            {t('cicdHistoryPage.clearFilter', 'Clear filter')}
          </button>
        </div>
      )}

      <DataTable
        columns={columns}
        data={filtered}
        getRowKey={(row) => row.id}
        expandedRowId={expandedDeploymentId}
        renderExpanded={(deployment) => (
          <div className="bg-[rgba(0,0,0,0.2)] px-5 py-4">
            <p className="mb-3 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
              Deployment Detail
            </p>
            <div className="grid grid-cols-2 gap-x-8 gap-y-2 sm:grid-cols-3">
              {[
                { label: 'Pipeline', value: deployment.pipelineName },
                { label: 'Version', value: deployment.version },
                { label: 'Triggered By', value: deployment.triggeredBy || '-' },
                { label: 'Status', value: getPipelineStatusLabel(t, deployment.status) },
                { label: 'Started At', value: formatDate(deployment.startedAt, locale) },
                { label: 'Completed At', value: formatDate(deployment.completedAt, locale) },
              ].map(({ label, value }) => (
                <div key={label} className="flex gap-2 text-[13px]">
                  <span className="w-[100px] shrink-0 text-[var(--color-text-muted)]">{label}</span>
                  <span className="text-[var(--color-text-primary)]">{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        emptyMessage={t('cicdHistoryPage.empty', 'No deployment history.')}
        toolbar={
          <>
            <NativeSelect value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]">
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicdHistoryPage.filters.allTypes', 'All Types')}</option>
              <option value="api" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">API</option>
              <option value="frontend" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Frontend</option>
              <option value="batch" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Batch</option>
            </NativeSelect>
            <NativeSelect value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]">
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicdHistoryPage.filters.allStatus', 'All Status')}</option>
              <option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.success', 'Success')}</option>
              <option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.running', 'Running')}</option>
              <option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.pending', 'Pending')}</option>
              <option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.failed', 'Failed')}</option>
              <option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.cancelled', 'Cancelled')}</option>
            </NativeSelect>
            <div className="relative ml-auto">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <input
                placeholder={t('cicdHistoryPage.searchPlaceholder', 'Search pipeline / deployer...')}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
              />
            </div>
          </>
        }
      />
    </div>
  )
}
