import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { GitBranch, Plus, Search, Play } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { usePipelines, useDeployPipeline } from '../api/cicd-api'
import type { Pipeline, PipelineStatus } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'

const STATUS_STYLES: Record<PipelineStatus, { bg: string; color: string; label: string }> = {
  running: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa', label: 'Running' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Success' },
  failed: { bg: 'rgba(239,68,68,0.15)', color: '#ef4444', label: 'Failed' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Pending' },
  cancelled: { bg: 'rgba(100,116,139,0.15)', color: '#64748b', label: 'Cancelled' },
}

function formatDate(iso: string | null) {
  if (!iso) return '-'
  return new Date(iso).toLocaleDateString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

const selectClassName =
  'cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'

const MOCK_PIPELINES: Pipeline[] = [
  { id: 'frontend-web', name: 'frontend-web', appType: 'web-frontend' as const, clusterId: 'c1', clusterName: 'prod-k8s', status: 'success' as const, lastDeployedAt: '2026-03-03T14:28:00Z', createdAt: '2026-01-15T00:00:00Z' },
  { id: 'backend-api', name: 'backend-api', appType: 'web-backend' as const, clusterId: 'c1', clusterName: 'prod-k8s', status: 'failed' as const, lastDeployedAt: '2026-03-01T16:30:00Z', createdAt: '2026-01-20T00:00:00Z' },
  { id: 'ml-service', name: 'ml-service', appType: 'web-backend' as const, clusterId: 'c2', clusterName: 'dev-k8s', status: 'failed' as const, lastDeployedAt: '2026-02-28T11:05:00Z', createdAt: '2026-02-01T00:00:00Z' },
  { id: 'batch-runner', name: 'batch-runner', appType: 'batch-job' as const, clusterId: 'c1', clusterName: 'prod-k8s', status: 'running' as const, lastDeployedAt: '2026-03-03T10:00:00Z', createdAt: '2026-02-10T00:00:00Z' },
]

export function CicdListPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [expandedPipelineId, setExpandedPipelineId] = useState<string | null>(null)

  const { data: apiData } = usePipelines({ status: statusFilter || undefined, search: search || undefined })
  const pipelines = apiData?.items ?? MOCK_PIPELINES
  const deployPipeline = useDeployPipeline()

  const filtered = pipelines.filter((p) => {
    const q = search.toLowerCase()
    const matchesSearch = !search || p.name.toLowerCase().includes(q) || p.clusterName.toLowerCase().includes(q)
    const matchesStatus = !statusFilter || p.status === statusFilter
    return matchesSearch && matchesStatus
  })

  const columns: ColumnDef<Pipeline, unknown>[] = [
    {
      accessorKey: 'name',
      header: '이름',
      cell: ({ row }) => <span className="font-semibold">{row.original.name}</span>,
    },
    {
      accessorKey: 'appType',
      header: '앱 타입',
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.appType}</span>,
    },
    {
      accessorKey: 'clusterName',
      header: '클러스터',
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.clusterName}</span>,
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
      accessorKey: 'lastDeployedAt',
      header: '최근 배포',
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.lastDeployedAt)}</span>,
    },
    {
      id: 'actions',
      header: 'Actions',
      enableSorting: false,
      cell: ({ row }) => {
        const expanded = expandedPipelineId === row.original.id
        return (
          <div className="flex gap-1.5">
            <Button
              variant="secondary"
              size="sm"
              loading={deployPipeline.isPending}
              onClick={(event) => {
                event.stopPropagation()
                deployPipeline.mutate(row.original.id)
              }}
              type="button"
            >
              <Play size={11} />
              Deploy
            </Button>
            <Button
              variant="ghost"
              size="sm"
              type="button"
              onClick={(event) => {
                event.stopPropagation()
                setExpandedPipelineId((prev) => (prev === row.original.id ? null : row.original.id))
              }}
            >
              {expanded ? 'Hide' : 'View'}
            </Button>
          </div>
        )
      },
    },
  ]

  const expandedPipeline = filtered.find((pipeline) => pipeline.id === expandedPipelineId) ?? null

  return (
    <div>
      <Breadcrumb items={[{ label: 'CI/CD List' }]} />

      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <GitBranch size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              CI/CD List
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              CI/CD 파이프라인 목록
            </p>
          </div>
        </div>
        <Button
          variant="primary"
          size="md"
          onClick={() => navigate('/cicd/templates')}
          type="button"
        >
          <Plus size={15} />
          New Pipeline
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={filtered}
        getRowKey={(row) => row.id}
        emptyMessage="파이프라인이 없습니다."
        toolbar={
          <>
            <div className="relative max-w-[320px] flex-[1_1_240px]">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <Input
                placeholder="파이프라인 검색..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-[30px]"
              />
            </div>
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className={`${selectClassName} [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]`}>
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Status</option>
              <option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Success</option>
              <option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Running</option>
              <option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Pending</option>
              <option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Failed</option>
              <option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Cancelled</option>
            </select>
          </>
        }
      />

      {expandedPipeline && (
        <div className="mt-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-[14px]">
          <div className="grid grid-cols-[repeat(3,minmax(120px,1fr))] gap-3">
            <div>
              <div className="mb-1 text-[11px] text-[var(--color-text-secondary)]">Pipeline ID</div>
              <div className="font-mono text-[13px] text-[var(--color-text-primary)]">{expandedPipeline.id}</div>
            </div>
            <div>
              <div className="mb-1 text-[11px] text-[var(--color-text-secondary)]">Cluster ID</div>
              <div className="font-mono text-[13px] text-[var(--color-text-primary)]">{expandedPipeline.clusterId}</div>
            </div>
            <div>
              <div className="mb-1 text-[11px] text-[var(--color-text-secondary)]">Created At</div>
              <div className="text-[13px] text-[var(--color-text-primary)]">{formatDate(expandedPipeline.createdAt)}</div>
            </div>
          </div>
        </div>
      )}

    </div>
  )
}
