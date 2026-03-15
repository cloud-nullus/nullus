import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { GitBranch, Plus, Search, Play } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { usePipelines, useCreatePipeline, useDeployPipeline } from '../api/cicd-api'
import type { Pipeline, PipelineStatus, AppType } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { DataTable } from '../../../components/shared/data-table'

const MOCK_PIPELINES: Pipeline[] = [
  {
    id: 'p1',
    name: 'api-server-pipeline',
    appType: 'web-backend',
    clusterId: 'c1',
    clusterName: 'prod-cluster',
    status: 'success',
    lastDeployedAt: '2026-03-13T10:00:00Z',
    createdAt: '2026-02-01T00:00:00Z',
  },
  {
    id: 'p2',
    name: 'frontend-pipeline',
    appType: 'web-frontend',
    clusterId: 'c2',
    clusterName: 'staging-cluster',
    status: 'running',
    lastDeployedAt: '2026-03-14T08:30:00Z',
    createdAt: '2026-02-10T00:00:00Z',
  },
  {
    id: 'p3',
    name: 'data-batch-pipeline',
    appType: 'batch-job',
    clusterId: 'c3',
    clusterName: 'dev-cluster',
    status: 'failed',
    lastDeployedAt: '2026-03-12T14:00:00Z',
    createdAt: '2026-03-01T00:00:00Z',
  },
]

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

const createPipelineSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  template: z.enum(['web-backend', 'web-frontend', 'batch-job'], { message: 'Template is required' }),
  clusterId: z.string().min(1, 'Cluster is required'),
})

type CreatePipelineFormData = z.infer<typeof createPipelineSchema>

const CREATE_PIPELINE_DEFAULTS: CreatePipelineFormData = {
  name: '',
  template: 'web-backend',
  clusterId: '',
}

export function CicdListPage() {
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [createModal, setCreateModal] = useState(false)
  const [expandedPipelineId, setExpandedPipelineId] = useState<string | null>(null)
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isValid, isSubmitting },
  } = useForm<CreatePipelineFormData>({
    resolver: zodResolver(createPipelineSchema),
    defaultValues: CREATE_PIPELINE_DEFAULTS,
    mode: 'onChange',
  })

  const { data: apiData } = usePipelines({ status: statusFilter || undefined, search: search || undefined })
  const pipelines = apiData?.items ?? MOCK_PIPELINES
  const createPipeline = useCreatePipeline()
  const deployPipeline = useDeployPipeline()

  const filtered = pipelines.filter((p) => {
    const q = search.toLowerCase()
    const matchesSearch = !search || p.name.toLowerCase().includes(q) || p.clusterName.toLowerCase().includes(q)
    const matchesStatus = !statusFilter || p.status === statusFilter
    return matchesSearch && matchesStatus
  })

  const handleCreate = (data: CreatePipelineFormData) => {
    createPipeline.mutate({ name: data.name, appType: data.template as AppType, clusterId: data.clusterId }, {
      onSuccess: () => {
        setCreateModal(false)
        reset(CREATE_PIPELINE_DEFAULTS)
      },
    })
  }

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
              CI/CD Pipelines
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              CI/CD 파이프라인 목록
            </p>
          </div>
        </div>
        <Button
          variant="primary"
          size="md"
          onClick={() => {
            reset(CREATE_PIPELINE_DEFAULTS)
            setCreateModal(true)
          }}
          type="button"
        >
          <Plus size={15} />
          New Pipeline
        </Button>
      </div>

      {/* Filters */}
      <div className="mb-4 flex flex-wrap gap-2.5">
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
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className={selectClassName}>
          <option value="">All Status</option>
          <option value="success">Success</option>
          <option value="running">Running</option>
          <option value="pending">Pending</option>
          <option value="failed">Failed</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      <DataTable columns={columns} data={filtered} getRowKey={(row) => row.id} emptyMessage="파이프라인이 없습니다." />

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

      {/* Create Pipeline Modal */}
      <Modal
        open={createModal}
        onClose={() => {
          setCreateModal(false)
          reset(CREATE_PIPELINE_DEFAULTS)
        }}
        title="New Pipeline"
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setCreateModal(false)
                reset(CREATE_PIPELINE_DEFAULTS)
              }}
              type="button"
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={createPipeline.isPending || isSubmitting}
              onClick={handleSubmit(handleCreate)}
              disabled={!isValid || isSubmitting}
              type="button"
            >
              Create
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-[14px]">
          <Input
            label="파이프라인 이름"
            placeholder="예: api-server-pipeline"
            {...register('name')}
          />
          {errors.name && <span className="text-xs text-[#ef4444]">{errors.name.message}</span>}
          <div className="flex flex-col gap-1">
            <label htmlFor="pipeline-app-type" className="text-xs font-medium text-[var(--color-text-secondary)]">앱 타입</label>
            <select
              id="pipeline-app-type"
              {...register('template')}
              className={selectClassName}
            >
              <option value="web-backend">Web Backend</option>
              <option value="web-frontend">Web Frontend</option>
              <option value="batch-job">Batch Job</option>
            </select>
          </div>
          {errors.template && <span className="text-xs text-[#ef4444]">{errors.template.message}</span>}
          <Input
            label="클러스터 ID"
            placeholder="예: c1"
            {...register('clusterId')}
          />
          {errors.clusterId && <span className="text-xs text-[#ef4444]">{errors.clusterId.message}</span>}
        </div>
      </Modal>
    </div>
  )
}
