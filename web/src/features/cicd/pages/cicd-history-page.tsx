import { useState } from 'react'
import { ChevronDown, ChevronUp, History, Search } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import type { ColumnDef } from '@tanstack/react-table'
import { useDeployments } from '../api/cicd-api'
import type { Deployment, PipelineStatus } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { DataTable } from '../../../components/shared/data-table'

const STATUS_STYLES: Record<PipelineStatus, { bg: string; color: string; label: string }> = {
  running: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa', label: 'Running' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Success' },
  failed: { bg: 'rgba(239,68,68,0.15)', color: '#ef4444', label: 'Failed' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Pending' },
  cancelled: { bg: 'rgba(100,116,139,0.15)', color: '#64748b', label: 'Cancelled' },
}

function formatDate(iso: string | null) {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

const MOCK_DEPLOYMENTS: Deployment[] = [
  { id: 'd1', pipelineId: 'frontend-web', pipelineName: 'frontend-web', version: 'v1.2.3', status: 'success' as const, triggeredBy: 'kim.dev', startedAt: '2026-03-03T14:22:00Z', completedAt: '2026-03-03T14:28:00Z' },
  { id: 'd2', pipelineId: 'frontend-web', pipelineName: 'frontend-web', version: 'v1.2.2', status: 'success' as const, triggeredBy: 'lee.devops', startedAt: '2026-03-02T11:00:00Z', completedAt: '2026-03-02T11:20:00Z' },
  { id: 'd3', pipelineId: 'backend-api', pipelineName: 'backend-api', version: 'v2.1.0', status: 'failed' as const, triggeredBy: 'park.dev', startedAt: '2026-03-01T16:25:00Z', completedAt: '2026-03-01T16:30:00Z' },
  { id: 'd4', pipelineId: 'batch-runner', pipelineName: 'batch-runner', version: 'v1.3.1', status: 'success' as const, triggeredBy: 'choi.devops', startedAt: '2026-03-03T10:00:00Z', completedAt: '2026-03-03T10:08:00Z' },
]

export function CicdHistoryPage() {
  const [statusFilter, setStatusFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [expandedDeploymentId, setExpandedDeploymentId] = useState<string | null>(null)
  const [search, setSearch] = useState('')

  const { data: apiData } = useDeployments({ status: statusFilter as PipelineStatus || undefined })
  const deployments = apiData?.items ?? MOCK_DEPLOYMENTS

  const filtered = deployments.filter((d) => {
    const matchesStatus = !statusFilter || d.status === statusFilter
    const matchesType = !typeFilter || d.pipelineName.includes(typeFilter)
    const matchesSearch = !search || d.pipelineName.toLowerCase().includes(search.toLowerCase()) || d.triggeredBy.toLowerCase().includes(search.toLowerCase())
    return matchesStatus && matchesType && matchesSearch
  })

  const selectClassName =
    'cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'

  const expandedDeployment = filtered.find((d) => d.id === expandedDeploymentId) ?? null

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
      header: '파이프라인',
      cell: ({ row }) => <span className="font-semibold">{row.original.pipelineName}</span>,
    },
    {
      accessorKey: 'version',
      header: '버전',
      cell: ({ row }) => <span className="font-mono text-[13px]">{row.original.version}</span>,
    },
    {
      accessorKey: 'status',
      header: '상태',
      cell: ({ row }) => {
        const st = STATUS_STYLES[row.original.status] ?? STATUS_STYLES.pending
        return (
          <span className="rounded-md px-[9px] py-[3px] text-xs font-semibold" style={{ backgroundColor: st.bg, color: st.color }}>
            {st.label}
          </span>
        )
      },
    },
    {
      accessorKey: 'triggeredBy',
      header: '배포자',
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{row.original.triggeredBy}</span>,
    },
    {
      accessorKey: 'startedAt',
      header: '시작 시간',
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.startedAt)}</span>,
    },
    {
      accessorKey: 'completedAt',
      header: '완료 시간',
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.completedAt)}</span>,
    },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: 'CI/CD History' }]} />

      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div
          className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(245,158,11,0.15)] text-[#fbbf24]"
        >
          <History size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            CI/CD History
          </h1>
          <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
            CI/CD 배포 이력
          </p>
        </div>
      </div>

      <DataTable
        columns={columns}
        data={filtered}
        getRowKey={(row) => row.id}
        emptyMessage="배포 이력이 없습니다."
        toolbar={
          <>
            <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} className={`${selectClassName} [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]`}>
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Types</option>
              <option value="api" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">API</option>
              <option value="frontend" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Frontend</option>
              <option value="batch" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Batch</option>
            </select>
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className={`${selectClassName} [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]`}>
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Status</option>
              <option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Success</option>
              <option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Running</option>
              <option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Pending</option>
              <option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Failed</option>
              <option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Cancelled</option>
            </select>
            <div className="relative ml-auto">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <input
                placeholder="파이프라인 / 배포자 검색..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
              />
            </div>
          </>
        }
      />

      {expandedDeployment && (
        <div className="mt-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(0,0,0,0.2)] px-5 py-4">
          <p className="mb-3 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            Deployment Detail
          </p>
          <div className="grid grid-cols-2 gap-x-8 gap-y-2">
            {[
              { label: 'Pipeline', value: expandedDeployment.pipelineName },
              { label: 'Version', value: expandedDeployment.version },
              { label: 'Triggered By', value: expandedDeployment.triggeredBy },
              { label: 'Status', value: expandedDeployment.status },
              { label: 'Started At', value: formatDate(expandedDeployment.startedAt) },
              { label: 'Completed At', value: formatDate(expandedDeployment.completedAt) },
            ].map(({ label, value }) => (
              <div key={label} className="flex gap-2 text-[13px]">
                <span className="w-[100px] shrink-0 text-[var(--color-text-muted)]">{label}</span>
                <span className="text-[var(--color-text-primary)]">{value}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
