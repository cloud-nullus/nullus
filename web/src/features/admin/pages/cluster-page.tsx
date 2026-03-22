import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Network, Plus, CheckCircle, Clock, AlertCircle, MinusCircle } from 'lucide-react'
import { useClusters, useCreateCluster, useDeleteCluster, useUpdateCluster, useVerifyCluster } from '../api/admin-api'
import type { Cluster, ClusterStatus } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

const STATUS_CONFIG: Record<ClusterStatus, { icon: React.ReactNode; badgeClassName: string; panelClassName: string; label: string }> = {
  connected: {
    icon: <CheckCircle size={14} />,
    badgeClassName: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
    panelClassName: 'border-[#22c55e40] bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
    label: 'Connected',
  },
  pending: {
    icon: <Clock size={14} />,
    badgeClassName: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
    panelClassName: 'border-[#f59e0b40] bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
    label: 'Pending',
  },
  error: {
    icon: <AlertCircle size={14} />,
    badgeClassName: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
    panelClassName: 'border-[#ef444440] bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
    label: 'Error',
  },
  inactive: {
    icon: <MinusCircle size={14} />,
    badgeClassName: 'bg-[rgba(100,116,139,0.15)] text-[#64748b]',
    panelClassName: 'border-[#64748b40] bg-[rgba(100,116,139,0.15)] text-[#64748b]',
    label: 'Inactive',
  },
}

const selectClassName = 'rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'

const clusterSchema = z
  .object({
    name: z.string().min(2, 'Name must be at least 2 characters'),
    type: z.enum(['kubernetes', 'eks', 'gke', 'aks', 'k3s', 'pipeline', 'target']),
    endpoint: z.string().optional().refine((value) => !value || z.url().safeParse(value).success, 'Invalid URL'),
    kubeconfig: z.string(),
    isEdit: z.boolean(),
  })
  .superRefine((data, ctx) => {
    const kubeconfigLength = data.kubeconfig.trim().length
    if (!data.isEdit && kubeconfigLength < 10) {
      ctx.addIssue({
        code: 'custom',
        path: ['kubeconfig'],
        message: 'Kubeconfig is required and must be at least 10 characters',
      })
    }
    if (data.isEdit && kubeconfigLength > 0 && kubeconfigLength < 10) {
      ctx.addIssue({
        code: 'custom',
        path: ['kubeconfig'],
        message: 'Kubeconfig must be at least 10 characters',
      })
    }
  })

type ClusterFormData = z.infer<typeof clusterSchema>

const CLUSTER_DEFAULTS: ClusterFormData = {
  name: '',
  type: 'kubernetes',
  endpoint: '',
  kubeconfig: '',
  isEdit: false,
}

const CLUSTER_DETAIL_META: Record<Cluster['type'], { purpose: string; namespace: string; authMethod: string }> = {
  kubernetes: {
    purpose: 'Pipeline',
    namespace: 'nullus-system',
    authMethod: 'Kubeconfig (ServiceAccount)',
  },
  eks: {
    purpose: 'Application',
    namespace: 'default',
    authMethod: 'IAM + Kubeconfig',
  },
  gke: {
    purpose: 'Application',
    namespace: 'default',
    authMethod: 'Workload Identity',
  },
  aks: {
    purpose: 'Application',
    namespace: 'default',
    authMethod: 'AAD + Kubeconfig',
  },
  k3s: {
    purpose: 'Edge / Lightweight',
    namespace: 'default',
    authMethod: 'Kubeconfig',
  },
  pipeline: {
    purpose: 'Pipeline',
    namespace: 'nullus-system',
    authMethod: 'Kubeconfig (ServiceAccount)',
  },
  target: {
    purpose: 'Application',
    namespace: 'default',
    authMethod: 'Kubeconfig',
  },
}

export function ClusterPage() {
  const { data: clustersData, isLoading } = useClusters()
  const clusters = clustersData?.items ?? []
  const createCluster = useCreateCluster()
  const updateCluster = useUpdateCluster()
  const deleteCluster = useDeleteCluster()
  const verifyCluster = useVerifyCluster()

  const [selected, setSelected] = useState<Cluster | null>(clusters[0] ?? null)
  const [registerModal, setRegisterModal] = useState(false)
  const [editingClusterId, setEditingClusterId] = useState<string | null>(null)
  const [deleteClusterId, setDeleteClusterId] = useState<string | null>(null)
  const [isVerifyingConnection, setIsVerifyingConnection] = useState(false)
  const [verifyConnectionResult, setVerifyConnectionResult] = useState<'success' | 'error' | null>(null)

  useEffect(() => {
    if (clusters.length === 0) {
      setSelected(null)
      return
    }

    if (!selected || !clusters.some((cluster) => cluster.id === selected.id)) {
      setSelected(clusters[0])
    }
  }, [clusters, selected])

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isValid, isSubmitting },
  } = useForm<ClusterFormData>({
    resolver: zodResolver(clusterSchema),
    defaultValues: CLUSTER_DEFAULTS,
    mode: 'onChange',
  })

  const handleRegister = (form: ClusterFormData) => {
    if (editingClusterId) {
      const updatePayload = form.kubeconfig.trim()
        ? { name: form.name, type: form.type, kubeconfig: form.kubeconfig }
        : { name: form.name, type: form.type }

      updateCluster.mutate(
        { id: editingClusterId, data: updatePayload },
        {
          onSuccess: () => {
            setRegisterModal(false)
            setEditingClusterId(null)
            reset(CLUSTER_DEFAULTS)
          },
        }
      )
      return
    }

    createCluster.mutate({ name: form.name, type: form.type, kubeconfig: form.kubeconfig }, {
      onSuccess: () => {
        setRegisterModal(false)
        reset(CLUSTER_DEFAULTS)
      },
    })
  }

  const openCreateModal = () => {
    setEditingClusterId(null)
    reset(CLUSTER_DEFAULTS)
    setRegisterModal(true)
  }

  const openEditModal = () => {
    if (!selected) return
    setEditingClusterId(selected.id)
    reset({
      name: selected.name,
      type: selected.type,
      endpoint: selected.endpoint,
      kubeconfig: '',
      isEdit: true,
    })
    setRegisterModal(true)
  }

  const handleDeleteCluster = () => {
    if (!deleteClusterId) return

    deleteCluster.mutate(deleteClusterId, {
      onSuccess: () => {
        setDeleteClusterId(null)
      },
    })
    setDeleteClusterId(null)
  }

  const handleVerifyConnection = () => {
    if (!selected || isVerifyingConnection) return
    setIsVerifyingConnection(true)
    setVerifyConnectionResult(null)
    verifyCluster.mutate(selected.id, {
      onSuccess: () => {
        setVerifyConnectionResult('success')
        setIsVerifyingConnection(false)
      },
      onError: () => {
        setVerifyConnectionResult('error')
        setIsVerifyingConnection(false)
      },
    })
  }

  return (
    <div>
      <Breadcrumb items={[{ label: 'Cluster Management' }]} />

      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(59,130,246,0.15)] text-[#60a5fa]">
            <Network size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Cluster Management
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              쿠버네티스 클러스터를 등록하고 관리합니다.
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={openCreateModal} type="button">
          <Plus size={15} />
          Register Cluster
        </Button>
      </div>

      <div className="h-[640px]">
        <ListDetailPanel
          listWidth={280}
          listContent={
            <>
              <div className="border-b border-[var(--color-border-default)] px-4 py-3 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                Clusters ({clusters.length})
              </div>
              {isLoading && (
                <div className="px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]">
                  Loading clusters...
                </div>
              )}
              {!isLoading && clusters.map((cluster) => {
                const st = STATUS_CONFIG[cluster.status]
                const isSelected = selected?.id === cluster.id
                const meta = CLUSTER_DETAIL_META[cluster.type]
                return (
                  <button
                    key={cluster.id}
                    type="button"
                    onClick={() => setSelected(cluster)}
                    className={cn(
                      'w-full cursor-pointer border-0 border-b border-l-[3px] border-b-[var(--color-border-default)] px-4 py-3.5 text-left transition-all duration-150',
                      isSelected
                        ? 'border-l-[#6366f1] bg-[rgba(99,102,241,0.1)]'
                        : 'border-l-transparent bg-transparent'
                    )}
                  >
                    <div className="mb-1 flex items-center justify-between">
                      <span className={cn('text-sm font-semibold', isSelected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                        {cluster.name}
                      </span>
                      <span className={cn('flex items-center gap-1 rounded-[5px] px-[7px] py-0.5 text-[11px] font-semibold', st.badgeClassName)}>
                        {st.icon}
                        {st.label}
                      </span>
                    </div>
                    <div className="text-xs text-[var(--color-text-secondary)]">
                      {meta ? `${meta.purpose} · ${meta.namespace}` : cluster.type.toUpperCase()}
                    </div>
                  </button>
                )
              })}
            </>
          }
          detailContent={
            selected ? (
              <div className="min-w-0 p-4">
                {(() => {
                  const detailMeta = CLUSTER_DETAIL_META[selected.type]
                  const connected = selected.status === 'connected'

                  return (
                    <>
                <div className="mb-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <div className="mb-[18px] flex flex-wrap items-center justify-between gap-2.5">
                    <h2 className="m-0 text-base font-bold text-[var(--color-text-primary)]">
                      {selected.name}
                    </h2>
                    <div className="flex gap-2">
                      <Button variant="secondary" size="sm" onClick={openEditModal} type="button">Edit</Button>
                      <Button variant="danger" size="sm" onClick={() => setDeleteClusterId(selected.id)} type="button">Delete</Button>
                    </div>
                  </div>
                    <div className="grid grid-cols-2 gap-4">
                      {[
                        ['클러스터 이름', selected.name],
                        ['유형', detailMeta?.purpose ?? selected.type.toUpperCase()],
                        ['네임스페이스', detailMeta?.namespace ?? 'Not Configured'],
                        ['엔드포인트', selected.endpoint],
                        ['Auth Method', detailMeta?.authMethod ?? 'Kubeconfig'],
                      ].map(([label, val]) => (
                        <div key={label}>
                        <div className="mb-1 text-[11px] uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
                          {label}
                        </div>
                        <div className="break-all text-sm font-semibold text-[var(--color-text-primary)]">
                          {val}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="mb-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <h3 className="mb-3.5 mt-0 text-sm font-bold text-[var(--color-text-primary)]">
                    연결 상태
                  </h3>
                  {(() => {
                    const st = STATUS_CONFIG[selected.status]
                    return (
                      <div className="flex flex-wrap items-center justify-between gap-2.5">
                        <div className={cn('inline-flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-semibold', st.panelClassName)}>
                          {st.icon}
                          {st.label}
                        </div>
                        <Button
                          variant="outline"
                          size="sm"
                          type="button"
                          loading={isVerifyingConnection}
                          onClick={handleVerifyConnection}
                        >
                          Verify Connection
                        </Button>
                        {verifyConnectionResult && (
                          <span className={cn('text-xs', verifyConnectionResult === 'success' ? 'text-[#22c55e]' : 'text-[#ef4444]')}>
                            {verifyConnectionResult === 'success' ? 'Connection verified successfully.' : 'Connection failed. Check endpoint/kubeconfig.'}
                          </span>
                        )}
                        {!verifyConnectionResult && !isVerifyingConnection && (
                          <span className={cn('text-xs', connected ? 'text-[#22c55e]' : 'text-[#f59e0b]')}>
                            {connected ? 'Connected' : 'Not Configured'}
                          </span>
                        )}
                      </div>
                    )
                  })()}
                </div>

                <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <h3 className="mb-3 mt-0 text-sm font-bold text-[var(--color-text-primary)]">
                    Organization Access
                  </h3>
                  <div className="flex flex-wrap gap-1.5">
                    {selected.organizationIds.map((oid) => (
                      <span key={oid} className="rounded-md bg-[rgba(139,92,246,0.12)] px-2.5 py-1 text-xs font-medium text-[#c4b5fd]">
                        {oid}
                      </span>
                    ))}
                  </div>
                </div>
                    </>
                  )
                })()}
              </div>
            ) : null
          }
          emptyDetailMessage="클러스터를 선택하세요."
        />
      </div>

      <Modal
        open={registerModal}
        onClose={() => {
          setRegisterModal(false)
          setEditingClusterId(null)
          reset(CLUSTER_DEFAULTS)
        }}
        title={editingClusterId ? 'Edit Cluster' : 'Register Cluster'}
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setRegisterModal(false)
                setEditingClusterId(null)
                reset(CLUSTER_DEFAULTS)
              }}
              type="button"
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={createCluster.isPending || updateCluster.isPending || isSubmitting}
              onClick={handleSubmit(handleRegister)}
              disabled={!isValid || isSubmitting}
              type="button"
            >
              {editingClusterId ? 'Save' : 'Register'}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3.5">
          <Input
            label="클러스터 이름"
            placeholder="예: prod-cluster"
            {...register('name')}
          />
          {errors.name && <span className="text-xs text-[#ef4444]">{errors.name.message}</span>}
          <NativeSelect label="클러스터 타입" {...register('type')} className={selectClassName}>
              <option value="kubernetes">Kubernetes</option>
              <option value="eks">AWS EKS</option>
              <option value="gke">GCP GKE</option>
              <option value="aks">Azure AKS</option>
              <option value="k3s">K3s</option>
              <option value="pipeline">Pipeline Cluster</option>
              <option value="target">Target Cluster</option>
            </NativeSelect>
          <Input
            label="엔드포인트"
            placeholder="예: https://prod.k8s.nullus.io"
            {...register('endpoint')}
          />
          {errors.endpoint && <span className="text-xs text-[#ef4444]">{errors.endpoint.message}</span>}
          <div className="flex flex-col gap-1">
            <label htmlFor="cluster-kubeconfig" className="text-xs font-medium text-[var(--color-text-secondary)]">
              kubeconfig (YAML)
            </label>
            <textarea
              id="cluster-kubeconfig"
              {...register('kubeconfig')}
              placeholder="kubeconfig 내용을 붙여넣으세요..."
              rows={8}
              className="resize-y rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-xs text-[var(--color-text-primary)] outline-none [font-family:'Fira_Code',monospace]"
            />
          </div>
          {errors.kubeconfig && <span className="text-xs text-[#ef4444]">{errors.kubeconfig.message}</span>}
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteClusterId !== null}
        onClose={() => setDeleteClusterId(null)}
        onConfirm={handleDeleteCluster}
        title="Delete Cluster"
        description="선택한 클러스터를 삭제하면 연결된 파이프라인과 배포 정보가 영향을 받을 수 있습니다. 계속하시겠습니까?"
        confirmLabel="Delete"
        loading={deleteCluster.isPending}
      />
    </div>
  )
}
