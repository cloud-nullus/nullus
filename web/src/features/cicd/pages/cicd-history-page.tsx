import { useState } from 'react'
import { useForm } from 'react-hook-form'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { ChevronDown, ChevronUp, History, RotateCcw, Search } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import type { ColumnDef } from '@tanstack/react-table'
import { useDeployments, useRollbackDeployment } from '../api/cicd-api'
import type { Deployment, PipelineStatus } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Modal } from '../../../components/ui/modal'
import { DataTable } from '../../../components/shared/data-table'
import { useAppToast } from '../../../hooks/use-toast'

const STATUS_STYLES: Record<string, { bg: string; color: string; label: string }> = {
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


const rollbackFormSchema = z.object({
  confirmText: z
    .string()
    .trim()
    .refine((value) => value === 'ROLLBACK', '확인을 위해 ROLLBACK을 입력하세요.'),
})

interface RollbackFormValues {
  confirmText: string
}

export function CicdHistoryPage() {
  const [statusFilter, setStatusFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [expandedDeploymentId, setExpandedDeploymentId] = useState<string | null>(null)
  const [rollbackTarget, setRollbackTarget] = useState<Deployment | null>(null)
  const [search, setSearch] = useState('')
  const [preservePVC, setPreservePVC] = useState(true)
  const [deleteConfirmText, setDeleteConfirmText] = useState('')
  const toast = useAppToast()

  const { data: apiData } = useDeployments({ status: statusFilter as PipelineStatus || undefined })
  const rollbackMutation = useRollbackDeployment()
  const deployments = apiData?.items ?? []
  const {
    register,
    reset,
    watch,
    handleSubmit,
    formState: { errors },
  } = useForm<RollbackFormValues>({
    resolver: zodResolver(rollbackFormSchema) as Resolver<RollbackFormValues>,
    defaultValues: { confirmText: '' },
    mode: 'onChange',
  })

  const filtered = deployments.filter((d) => {
    const matchesStatus = !statusFilter || d.status === statusFilter
    const matchesType = !typeFilter || d.pipelineName.includes(typeFilter)
    const matchesSearch = !search || d.pipelineName.toLowerCase().includes(search.toLowerCase()) || d.triggeredBy.toLowerCase().includes(search.toLowerCase())
    return matchesStatus && matchesType && matchesSearch
  })


  const expandedDeployment = filtered.find((d) => d.id === expandedDeploymentId) ?? null

  const getPreviousVersion = (target: Deployment) => {
    const currentIndex = deployments.findIndex((item) => item.id === target.id)
    if (currentIndex < 0) return null
    return deployments.slice(currentIndex + 1).find((item) => item.pipelineId === target.pipelineId) ?? null
  }

  const closeRollbackModal = () => {
    reset({ confirmText: '' })
    setRollbackTarget(null)
    setPreservePVC(true)
    setDeleteConfirmText('')
  }

  const previousVersion = rollbackTarget ? getPreviousVersion(rollbackTarget) : null
  const canRollbackConfirm = watch('confirmText').trim() === 'ROLLBACK' && (preservePVC || deleteConfirmText === 'DELETE')

  const submitRollback = handleSubmit(() => {
    if (!rollbackTarget) return
    rollbackMutation.mutate(
      { pipelineId: rollbackTarget.pipelineId, deploymentId: rollbackTarget.id, preservePVC },
      {
        onSuccess: () => {
          toast.success(`배포를 ${rollbackTarget.version} 기준으로 롤백했습니다.`)
          closeRollbackModal()
        },
        onError: () => {
          toast.error('롤백에 실패했습니다. 잠시 후 다시 시도해주세요.')
        },
      }
    )
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
    {
      id: 'actions',
      header: 'Actions',
      enableSorting: false,
      cell: ({ row }) => {
        const canRollback = row.original.status === 'success' || row.original.status === 'failed'
        if (!canRollback) return null
        return (
          <Button
            variant="danger"
            size="sm"
            type="button"
            data-testid="rollback-btn"
            onClick={(event) => {
              event.stopPropagation()
              reset({ confirmText: '' })
              setRollbackTarget(row.original)
            }}
          >
            <RotateCcw size={13} />
            Rollback
          </Button>
        )
      },
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
            <NativeSelect value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]">
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Types</option>
              <option value="api" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">API</option>
              <option value="frontend" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Frontend</option>
              <option value="batch" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Batch</option>
            </NativeSelect>
            <NativeSelect value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]">
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Status</option>
              <option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Success</option>
              <option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Running</option>
              <option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Pending</option>
              <option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Failed</option>
              <option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Cancelled</option>
            </NativeSelect>
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

      <Modal
        open={!!rollbackTarget}
        onClose={closeRollbackModal}
        title="배포 롤백 확인"
        footer={
          <>
            <Button variant="outline" size="md" onClick={closeRollbackModal} disabled={rollbackMutation.isPending}>
              Cancel
            </Button>
            <Button
              variant="danger"
              size="md"
              type="button"
              data-testid="rollback-confirm"
              onClick={submitRollback}
              disabled={!canRollbackConfirm || rollbackMutation.isPending}
              loading={rollbackMutation.isPending}
            >
              Rollback
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-4">
          <p className="m-0 text-sm leading-[1.6] text-[var(--color-text-secondary)]">
            선택한 배포로 롤백하면 현재 버전이 이전 상태로 교체됩니다. 이 작업은 되돌릴 수 없습니다.
          </p>

          <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] p-3 text-sm">
            <div className="mb-1.5 flex justify-between gap-3">
              <span className="text-[var(--color-text-secondary)]">Current Version</span>
              <span className="font-mono text-[var(--color-text-primary)]">{rollbackTarget?.version ?? '-'}</span>
            </div>
            <div className="flex justify-between gap-3">
              <span className="text-[var(--color-text-secondary)]">Previous Version</span>
              <span className="font-mono text-[var(--color-text-primary)]">{previousVersion?.version ?? '-'}</span>
            </div>
          </div>

          <div className="mt-4">
            <p className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">데이터 보존 옵션</p>
            <div className="flex flex-col gap-2">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="radio"
                  name="pvcMode"
                  value="safe"
                  checked={preservePVC}
                  onChange={() => {
                    setPreservePVC(true)
                    setDeleteConfirmText('')
                  }}
                />
                <span>Safe Mode — 데이터 보존</span>
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="radio"
                  name="pvcMode"
                  value="clean"
                  checked={!preservePVC}
                  onChange={() => setPreservePVC(false)}
                />
                <span>Clean Mode — 볼륨 삭제</span>
              </label>
            </div>
            {!preservePVC && (
              <div className="mt-3">
                <div className="rounded-lg border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-sm text-[#ef4444]">
                  이 작업은 Persistent Volume을 영구 삭제합니다
                </div>
                <input
                  type="text"
                  placeholder='확인하려면 "DELETE" 입력'
                  value={deleteConfirmText}
                  onChange={(e) => setDeleteConfirmText(e.target.value)}
                  className="mt-2 w-full rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] outline-none"
                />
              </div>
            )}
          </div>

          <div>
            <p className="mb-2 mt-0 text-[13px] text-[var(--color-text-secondary)]">
              확인하려면{' '}
              <code className="rounded bg-[rgba(255,255,255,0.08)] px-1.5 py-0.5 font-mono text-xs text-[#f87171]">
                ROLLBACK
              </code>
              을(를) 입력하세요.
            </p>
            <input
              type="text"
              placeholder="ROLLBACK"
              className="box-border w-full rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] font-mono text-sm text-[var(--color-text-primary)] outline-none"
              {...register('confirmText')}
            />
            {errors.confirmText && (
              <p className="mb-0 mt-1.5 text-xs text-[#f87171]">{errors.confirmText.message}</p>
            )}
          </div>
        </div>
      </Modal>
    </div>
  )
}
