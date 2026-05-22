import { useEffect, useMemo, useState, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Network, Plus, CheckCircle, Clock, AlertCircle, MinusCircle, Upload } from 'lucide-react'
import { useCluster, useClusters, useCreateCluster, useDeleteCluster, useUpdateCluster, useVerifyCluster, useVerifyClusterDraft } from '../api/admin-api'
import type { Cluster, ClusterStatus, ClusterType, CloudProvider } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

const STATUS_CONFIG: Record<ClusterStatus, { icon: React.ReactNode; badgeClassName: string; panelClassName: string }> = {
  connected: {
    icon: <CheckCircle size={14} />,
    badgeClassName: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
    panelClassName: 'border-[#22c55e40] bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
  },
  pending: {
    icon: <Clock size={14} />,
    badgeClassName: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
    panelClassName: 'border-[#f59e0b40] bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
  },
  error: {
    icon: <AlertCircle size={14} />,
    badgeClassName: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
    panelClassName: 'border-[#ef444440] bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
  },
  inactive: {
    icon: <MinusCircle size={14} />,
    badgeClassName: 'bg-[rgba(100,116,139,0.15)] text-[#64748b]',
    panelClassName: 'border-[#64748b40] bg-[rgba(100,116,139,0.15)] text-[#64748b]',
  },
  unreachable: {
    icon: <AlertCircle size={14} />,
    badgeClassName: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
    panelClassName: 'border-[#f59e0b40] bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
  },
  auth_failed: {
    icon: <AlertCircle size={14} />,
    badgeClassName: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
    panelClassName: 'border-[#ef444440] bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
  },
}

function getStatusLabel(t: TFunction, status: ClusterStatus) {
  if (status === 'connected') return t('clusterPage.status.connected', 'Connected')
  if (status === 'pending') return t('clusterPage.status.pending', 'Pending')
  if (status === 'error') return t('clusterPage.status.error', 'Error')
  if (status === 'inactive') return t('clusterPage.status.inactive', 'Inactive')
  if (status === 'unreachable') return t('clusterPage.status.unreachable', 'Unreachable')
  return t('clusterPage.status.authFailed', 'Auth Failed')
}

function getConnectionHint(t: TFunction, status: ClusterStatus): { text: string; className: string } {
  switch (status) {
    case 'connected':
      return { text: t('clusterPage.connection.connectedDetail', 'Cluster API is reachable and authentication is valid.'), className: 'text-[#22c55e]' }
    case 'auth_failed':
      return { text: t('clusterPage.connection.authFailed', 'Authentication failed. Recheck credentials/kubeconfig.'), className: 'text-[#ef4444]' }
    case 'error':
      return { text: t('clusterPage.connection.error', 'Connection error. Check endpoint and network path.'), className: 'text-[#ef4444]' }
    case 'unreachable':
      return { text: t('clusterPage.connection.unreachable', 'Endpoint unreachable. Verify DNS, firewall, and cluster API reachability.'), className: 'text-[#f59e0b]' }
    case 'pending':
      return { text: t('clusterPage.connection.pending', 'Pending verification'), className: 'text-[#f59e0b]' }
    case 'inactive':
      return { text: t('clusterPage.connection.inactive', 'Inactive'), className: 'text-[#64748b]' }
  }
}

function normalizeClusterStatus(rawStatus: string | undefined | null, fallback: ClusterStatus = 'pending'): ClusterStatus {
  const normalized = (rawStatus ?? '').trim().toLowerCase()
  if (normalized === 'connected') return 'connected'
  if (normalized === 'pending') return 'pending'
  if (normalized === 'error') return 'error'
  if (normalized === 'inactive') return 'inactive'
  if (normalized === 'unreachable') return 'unreachable'
  if (normalized === 'auth_failed' || normalized === 'auth-failed' || normalized === 'authfailed') return 'auth_failed'
  return fallback
}

const selectClassName = 'rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'

const clusterSchema = z
  .object({
    name: z.string().min(2, 'Name must be at least 2 characters'),
    types: z.array(z.enum(['pipeline', 'target'])).min(1, 'Select at least one cluster type'),
    cloudProvider: z.enum(['aws', 'azure', 'gcp', 'oci', 'ibm_cloud', 'alibaba_cloud', 'tencent_cloud', 'naver_cloud', 'kt_cloud', 'nhn_cloud', 'on_premise']),
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
  types: [],
  cloudProvider: 'on_premise',
  endpoint: '',
  kubeconfig: '',
  isEdit: false,
}

const CLUSTER_DETAIL_META: Record<ClusterType, { namespace: string; authMethod: string }> = {
  pipeline: {
    namespace: 'nullus-system',
    authMethod: 'Kubeconfig (ServiceAccount)',
  },
  target: {
    namespace: 'default',
    authMethod: 'Kubeconfig',
  },
}

const CLUSTER_TYPE_OPTIONS: Array<{ value: ClusterType; key: string; fallback: string }> = [
  { value: 'pipeline', key: 'clusterPage.type.pipeline', fallback: 'DevSecOps Stack Cluster' },
  { value: 'target', key: 'clusterPage.type.target', fallback: 'Target Cluster' },
]

const CLOUD_PROVIDER_OPTIONS: Array<{ value: CloudProvider; label: string }> = [
  { value: 'aws', label: 'AWS' },
  { value: 'azure', label: 'Azure' },
  { value: 'gcp', label: 'GCP' },
  { value: 'oci', label: 'OCI' },
  { value: 'ibm_cloud', label: 'IBM Cloud' },
  { value: 'alibaba_cloud', label: 'Alibaba Cloud' },
  { value: 'tencent_cloud', label: 'Tencent Cloud' },
  { value: 'naver_cloud', label: 'Naver Cloud' },
  { value: 'kt_cloud', label: 'KT Cloud' },
  { value: 'nhn_cloud', label: 'NHN Cloud' },
  { value: 'on_premise', label: 'On-Premise' },
]

function resolveClusterTypes(cluster: Pick<Cluster, 'type' | 'types'>): ClusterType[] {
  const types = Array.isArray(cluster.types) && cluster.types.length > 0 ? cluster.types : (cluster.type ? [cluster.type] : [])
  return Array.from(new Set(types))
}

function getPrimaryClusterType(types: ClusterType[]): ClusterType {
  return types.includes('pipeline') ? 'pipeline' : 'target'
}

function getClusterTypeLabel(t: TFunction, type: ClusterType) {
  const option = CLUSTER_TYPE_OPTIONS.find((item) => item.value === type)
  return t(option?.key ?? 'clusterPage.type.target', option?.fallback ?? 'Target Cluster')
}

function formatClusterTypes(t: TFunction, types: ClusterType[]) {
  return types.map((type) => getClusterTypeLabel(t, type)).join(' / ')
}

function formatCloudProvider(provider: CloudProvider | undefined) {
  return CLOUD_PROVIDER_OPTIONS.find((item) => item.value === provider)?.label ?? 'On-Premise'
}

function getClusterNamespaces(types: ClusterType[]) {
  return Array.from(new Set(types.map((type) => CLUSTER_DETAIL_META[type].namespace))).join(' / ')
}

function getClusterAuthMethods(types: ClusterType[]) {
  return Array.from(new Set(types.map((type) => CLUSTER_DETAIL_META[type].authMethod))).join(' / ')
}

export function ClusterPage() {
  const { t } = useTranslation()
  const { data: clustersData, isLoading } = useClusters()
  const clusters = useMemo(() => clustersData?.items ?? [], [clustersData?.items])
  const createCluster = useCreateCluster()
  const updateCluster = useUpdateCluster()
  const deleteCluster = useDeleteCluster()
  const verifyCluster = useVerifyCluster()
  const verifyClusterDraft = useVerifyClusterDraft()

  const [selected, setSelected] = useState<Cluster | null>(clusters[0] ?? null)
  const [registerModal, setRegisterModal] = useState(false)
  const [editingClusterId, setEditingClusterId] = useState<string | null>(null)
  const [deleteClusterId, setDeleteClusterId] = useState<string | null>(null)
  const [deleteClusterError, setDeleteClusterError] = useState<string | null>(null)
  const [isVerifyingConnection, setIsVerifyingConnection] = useState(false)
  const [verifyConnectionResult, setVerifyConnectionResult] = useState<'success' | 'error' | null>(null)
  const [draftVerifyStatus, setDraftVerifyStatus] = useState<ClusterStatus | 'error' | null>(null)
  const [draftVerifyMessage, setDraftVerifyMessage] = useState<string | null>(null)
  const [statusOverrides, setStatusOverrides] = useState<Record<string, ClusterStatus>>({})
  const [fileUploadError, setFileUploadError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const editPrefilledClusterIdRef = useRef<string | null>(null)
  const { data: editingClusterDetail, isFetching: isFetchingEditingCluster } = useCluster(editingClusterId ?? '', registerModal && !!editingClusterId)

  useEffect(() => {
    if (clusters.length === 0) {
      setSelected(null)
      return
    }

    if (!selected || !clusters.some((cluster) => cluster.id === selected.id)) {
      setSelected(clusters[0])
    }
  }, [clusters, selected])

  useEffect(() => {
    setVerifyConnectionResult(null)
  }, [selected?.id])

  useEffect(() => {
    if (clusters.length === 0) {
      if (Object.keys(statusOverrides).length > 0) {
        setStatusOverrides({})
      }
      return
    }

    setStatusOverrides((prev) => {
      if (Object.keys(prev).length === 0) return prev
      let changed = false
      const next = { ...prev }

      Object.entries(prev).forEach(([clusterId, overriddenStatus]) => {
        const cluster = clusters.find((item) => item.id === clusterId)
        if (!cluster) {
          delete next[clusterId]
          changed = true
          return
        }
        if (cluster.status === overriddenStatus) {
          delete next[clusterId]
          changed = true
        }
      })

      return changed ? next : prev
    })
  }, [clusters, statusOverrides])

  const getEffectiveStatus = (cluster: Pick<Cluster, 'id' | 'status'>): ClusterStatus => statusOverrides[cluster.id] ?? cluster.status
  const selectedCluster = selected ? clusters.find((cluster) => cluster.id === selected.id) ?? selected : null

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    getValues,
    formState: { errors, isValid, isSubmitting },
  } = useForm<ClusterFormData>({
    resolver: zodResolver(clusterSchema),
    defaultValues: CLUSTER_DEFAULTS,
    mode: 'onChange',
  })

  useEffect(() => {
    if (!registerModal || !editingClusterId || !editingClusterDetail) return
    if (editPrefilledClusterIdRef.current === editingClusterId) return

    reset({
      name: editingClusterDetail.name,
      types: resolveClusterTypes(editingClusterDetail),
      cloudProvider: editingClusterDetail.cloudProvider,
      endpoint: editingClusterDetail.endpoint,
      kubeconfig: editingClusterDetail.kubeconfig ?? '',
      isEdit: true,
    })
    editPrefilledClusterIdRef.current = editingClusterId
  }, [registerModal, editingClusterId, editingClusterDetail, reset])

  const closeRegisterModal = () => {
    setRegisterModal(false)
    setEditingClusterId(null)
    editPrefilledClusterIdRef.current = null
    reset(CLUSTER_DEFAULTS)
    setDraftVerifyStatus(null)
    setDraftVerifyMessage(null)
  }

  const verifySavedClusterAndClose = (clusterId: string | null | undefined) => {
    if (!clusterId) {
      closeRegisterModal()
      return
    }

    verifyCluster.mutate(clusterId, {
      onSuccess: () => {
        setStatusOverrides((prev) => ({ ...prev, [clusterId]: 'connected' }))
        closeRegisterModal()
      },
      onError: () => {
        closeRegisterModal()
      },
    })
  }

  const handleRegister = (form: ClusterFormData) => {
    const endpoint = form.endpoint?.trim() ?? ''
    const types = Array.from(new Set(form.types))
    const payload = {
      name: form.name,
      type: getPrimaryClusterType(types),
      types,
      cloudProvider: form.cloudProvider,
      endpoint,
    }

    if (editingClusterId) {
      const updatePayload = form.kubeconfig.trim()
        ? { ...payload, kubeconfig: form.kubeconfig }
        : payload

      updateCluster.mutate(
        { id: editingClusterId, data: updatePayload },
        {
          onSuccess: () => {
            verifySavedClusterAndClose(editingClusterId)
          },
        }
      )
      return
    }

    createCluster.mutate({ ...payload, kubeconfig: form.kubeconfig }, {
      onSuccess: (createdCluster) => {
        verifySavedClusterAndClose(createdCluster?.id)
      },
    })
  }

  const openCreateModal = () => {
    setEditingClusterId(null)
    editPrefilledClusterIdRef.current = null
    reset(CLUSTER_DEFAULTS)
    setDraftVerifyStatus(null)
    setDraftVerifyMessage(null)
    setRegisterModal(true)
  }

  const openEditModal = () => {
    if (!selectedCluster) return
    setEditingClusterId(selectedCluster.id)
    editPrefilledClusterIdRef.current = null
    reset({
      name: selectedCluster.name,
      types: resolveClusterTypes(selectedCluster),
      cloudProvider: selectedCluster.cloudProvider,
      endpoint: selectedCluster.endpoint,
      kubeconfig: selectedCluster.kubeconfig ?? '',
      isEdit: true,
    })
    setDraftVerifyStatus(null)
    setDraftVerifyMessage(null)
    setRegisterModal(true)
  }

  const selectedTypes = watch('types') ?? []
  const watchedEndpoint = watch('endpoint')
  const watchedKubeconfig = watch('kubeconfig')
  const selectedTypesKey = selectedTypes.join('|')
  const isSubmitBlockedByVerification = draftVerifyStatus !== 'connected'

  useEffect(() => {
    if (!registerModal) return
    setDraftVerifyStatus(null)
    setDraftVerifyMessage(null)
  }, [registerModal, watchedEndpoint, watchedKubeconfig, selectedTypesKey])

  const handleDeleteCluster = () => {
    if (!deleteClusterId) return

    deleteCluster.mutate(deleteClusterId, {
      onSuccess: () => {
        setDeleteClusterError(null)
        setDeleteClusterId(null)
      },
      onError: (err) => {
        const message =
          typeof err === 'object' &&
          err !== null &&
          'message' in err &&
          typeof err.message === 'string'
            ? err.message
            : 'Failed to delete cluster'
        setDeleteClusterError(message)
      },
    })
  }

  const handleVerifyConnection = () => {
    if (!selectedCluster || isVerifyingConnection) return
    const selectedClusterId = selectedCluster.id
    const selectedClusterStatus = getEffectiveStatus(selectedCluster)
    setIsVerifyingConnection(true)
    setVerifyConnectionResult(null)
    verifyCluster.mutate(selectedClusterId, {
      onSuccess: (result) => {
        const verifiedStatus = normalizeClusterStatus(result?.status, selectedClusterStatus)
        setStatusOverrides((prev) => ({ ...prev, [selectedClusterId]: verifiedStatus }))
        setVerifyConnectionResult('success')
        setIsVerifyingConnection(false)
      },
      onError: () => {
        setStatusOverrides((prev) => ({ ...prev, [selectedClusterId]: 'error' }))
        setVerifyConnectionResult('error')
        setIsVerifyingConnection(false)
      },
    })
  }

  const handleVerifyConnectionInModal = () => {
    if (!registerModal || verifyClusterDraft.isPending) return

    const form = getValues()
    const endpoint = form.endpoint?.trim() ?? ''
    const kubeconfig = form.kubeconfig?.trim() ?? ''

    if (!kubeconfig) {
      setDraftVerifyStatus('error')
      setDraftVerifyMessage(t('clusterPage.connection.verifyDraftRequired', 'Kubeconfig is required before verification.'))
      return
    }

    setDraftVerifyStatus(null)
    setDraftVerifyMessage(null)
    verifyClusterDraft.mutate(
      { endpoint, kubeconfig },
      {
        onSuccess: (result) => {
          const verifiedStatus = normalizeClusterStatus(result?.status, 'pending')
          setDraftVerifyStatus(verifiedStatus)
          if (verifiedStatus === 'connected') {
            setDraftVerifyMessage(t('clusterPage.connection.verifySuccess', 'Connection verified successfully.'))
            return
          }
          setDraftVerifyMessage(t('clusterPage.connection.verifyFailed', 'Connection failed. Check endpoint/kubeconfig.'))
        },
        onError: () => {
          setDraftVerifyStatus('error')
          setDraftVerifyMessage(t('clusterPage.connection.verifyFailed', 'Connection failed. Check endpoint/kubeconfig.'))
        },
      }
    )
  }

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    setFileUploadError(null)

    const validExtensions = ['.yaml', '.yml', '.conf']
    const fileName = file.name.toLowerCase()
    const hasValidExtension = validExtensions.some((ext) => fileName.endsWith(ext))

    if (!hasValidExtension) {
      setFileUploadError('Invalid file type. Please upload a .yaml, .yml, or .conf file.')
      if (fileInputRef.current) fileInputRef.current.value = ''
      return
    }

    const MAX_FILE_SIZE = 1048576
    if (file.size > MAX_FILE_SIZE) {
      setFileUploadError('File size exceeds 1MB limit.')
      if (fileInputRef.current) fileInputRef.current.value = ''
      return
    }

    const reader = new FileReader()
    reader.onload = (e) => {
      const content = e.target?.result as string
      if (content) {
        setValue('kubeconfig', content)
      }
    }
    reader.onerror = () => {
      setFileUploadError('Failed to read file.')
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
    reader.readAsText(file)
  }

  return (
    <div>
      <Breadcrumb items={[{ label: t('sidebar.clusterManagement', 'Cluster Management') }]} />

      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(59,130,246,0.15)] text-[#60a5fa]">
            <Network size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('sidebar.clusterManagement', 'Cluster Management')}
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              {t('clusterPage.description', 'Register and manage Kubernetes clusters.')}
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={openCreateModal} type="button">
          <Plus size={15} />
          {t('clusterPage.actions.registerCluster', 'Register Cluster')}
        </Button>
      </div>

      <div className="h-[640px]">
        <ListDetailPanel
          listWidth={280}
          listContent={
            <>
              <div className="border-b border-[var(--color-border-default)] px-4 py-3 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                {t('clusterPage.list.clusters', 'Clusters')} ({clusters.length})
              </div>
              {isLoading && (
                <div className="px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]">
                  {t('clusterPage.list.loading', 'Loading clusters...')}
                </div>
              )}
              {!isLoading && clusters.map((cluster) => {
                const effectiveStatus = getEffectiveStatus(cluster)
                const st = STATUS_CONFIG[effectiveStatus]
                const isSelected = selected?.id === cluster.id
                const clusterTypes = resolveClusterTypes(cluster)
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
                        {getStatusLabel(t, effectiveStatus)}
                      </span>
                    </div>
                    <div className="text-xs text-[var(--color-text-secondary)]">
                      {formatClusterTypes(t, clusterTypes)} · {formatCloudProvider(cluster.cloudProvider)}
                    </div>
                  </button>
                )
              })}
            </>
          }
          detailContent={
            selectedCluster ? (
              <div className="min-w-0 p-4">
                {(() => {
                  const selectedClusterTypes = resolveClusterTypes(selectedCluster)
                  const effectiveSelectedStatus = getEffectiveStatus(selectedCluster)
                  const connectionHint = getConnectionHint(t, effectiveSelectedStatus)
                  const showBaseHint = !verifyConnectionResult && !isVerifyingConnection && effectiveSelectedStatus !== 'connected'

                  return (
                    <>
                <div className="mb-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <div className="mb-[18px] flex flex-wrap items-center justify-between gap-2.5">
                    <h2 className="m-0 text-base font-bold text-[var(--color-text-primary)]">
                      {selectedCluster.name}
                    </h2>
                    <div className="flex gap-2">
                      <Button variant="secondary" size="sm" onClick={openEditModal} type="button">{t('clusterPage.actions.edit', 'Edit')}</Button>
                      <Button variant="danger" size="sm" onClick={() => setDeleteClusterId(selectedCluster.id)} type="button">{t('common.delete', 'Delete')}</Button>
                    </div>
                  </div>
                    <div className="grid grid-cols-2 gap-4">
                      {[
                        [t('clusterPage.detail.clusterName', 'Cluster Name'), selectedCluster.name],
                        [t('clusterPage.detail.type', 'Type'), formatClusterTypes(t, selectedClusterTypes)],
                        [t('clusterPage.detail.cloudProvider', 'Cloud Provider'), formatCloudProvider(selectedCluster.cloudProvider)],
                        [t('clusterPage.detail.namespace', 'Namespace'), getClusterNamespaces(selectedClusterTypes)],
                        [t('clusterPage.detail.endpoint', 'Endpoint'), selectedCluster.endpoint],
                        [t('clusterPage.detail.authMethod', 'Auth Method'), getClusterAuthMethods(selectedClusterTypes)],
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
                    {t('clusterPage.connection.title', 'Connection Status')}
                  </h3>
                  {(() => {
                    const st = STATUS_CONFIG[effectiveSelectedStatus]
                    const baseHintMessage = showBaseHint ? connectionHint.text : ''
                    const statusMessage = isVerifyingConnection
                      ? t('clusterPage.connection.verifying', 'Verifying connection...')
                      : verifyConnectionResult === 'success'
                        ? t('clusterPage.connection.verifySuccess', 'Connection verified successfully.')
                        : verifyConnectionResult === 'error'
                          ? t('clusterPage.connection.verifyFailed', 'Connection failed. Check endpoint/kubeconfig.')
                          : baseHintMessage
                    const statusMessageClass = isVerifyingConnection
                      ? 'text-[var(--color-text-secondary)]'
                      : verifyConnectionResult === 'success'
                        ? 'text-[#22c55e]'
                        : verifyConnectionResult === 'error'
                          ? 'text-[#ef4444]'
                          : connectionHint.className

                    return (
                      <div className="flex flex-col gap-2.5 sm:flex-row sm:items-start sm:justify-between">
                        <div className="flex min-w-0 flex-wrap items-center gap-2.5">
                          <div className={cn('inline-flex w-fit items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-semibold', st.panelClassName)}>
                            {st.icon}
                            {getStatusLabel(t, effectiveSelectedStatus)}
                          </div>
                          {statusMessage && <span className={cn('text-xs', statusMessageClass)}>{statusMessage}</span>}
                        </div>
                        <Button
                          variant="outline"
                          size="sm"
                          type="button"
                          loading={isVerifyingConnection}
                          onClick={handleVerifyConnection}
                        >
                          {t('clusterPage.actions.verifyConnection', 'Verify Connection')}
                        </Button>
                      </div>
                    )
                  })()}
                </div>

                <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <h3 className="mb-3 mt-0 text-sm font-bold text-[var(--color-text-primary)]">
                    {t('clusterPage.organizationAccess', 'Organization Access')}
                  </h3>
                  <div className="flex flex-wrap gap-1.5">
                    {selectedCluster.organizationIds.map((oid) => (
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
          emptyDetailMessage={t('clusterPage.emptyDetail', 'Select a cluster.')}
        />
      </div>

      <Modal
        open={registerModal}
        onClose={closeRegisterModal}
        title={editingClusterId ? t('clusterPage.modal.editTitle', 'Edit Cluster') : t('clusterPage.modal.registerTitle', 'Register Cluster')}
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={closeRegisterModal}
              type="button"
            >
              {t('common.cancel', 'Cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={createCluster.isPending || updateCluster.isPending || verifyCluster.isPending || isSubmitting}
              onClick={handleSubmit(handleRegister)}
              disabled={!isValid || isSubmitting || isSubmitBlockedByVerification}
              type="button"
            >
              {editingClusterId ? t('common.save', 'Save') : t('clusterPage.actions.register', 'Register')}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3.5">
          <Input
            label={t('clusterPage.form.clusterName', 'Cluster Name')}
            placeholder={t('clusterPage.form.clusterNamePlaceholder', 'e.g. prod-cluster')}
            {...register('name')}
          />
          {errors.name && <span className="text-xs text-[#ef4444]">{errors.name.message}</span>}
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-[var(--color-text-secondary)]">
              {t('clusterPage.form.clusterType', 'Cluster Type')}
            </span>
            <div className="grid gap-2 sm:grid-cols-2">
              {CLUSTER_TYPE_OPTIONS.map((option) => {
                const checked = selectedTypes.includes(option.value)
                return (
                  <label
                    key={option.value}
                    className={cn(
                      'flex cursor-pointer items-center gap-2.5 rounded-lg border px-3 py-2.5 text-sm transition-colors',
                      checked
                        ? 'border-[rgba(99,102,241,0.45)] bg-[rgba(99,102,241,0.12)] text-[var(--color-text-primary)]'
                        : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] text-[var(--color-text-secondary)]'
                    )}
                  >
                    <input type="checkbox" value={option.value} {...register('types')} className="h-4 w-4" />
                    <span className="whitespace-pre-line">{t(option.key, option.fallback)}</span>
                  </label>
                )
              })}
            </div>
          </div>
          {errors.types && <span className="text-xs text-[#ef4444]">{errors.types.message}</span>}
          <NativeSelect label={t('clusterPage.form.cloudProvider', 'Cloud Provider')} {...register('cloudProvider')} className={selectClassName}>
            {CLOUD_PROVIDER_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </NativeSelect>
          <Input
            label={t('clusterPage.form.endpoint', 'Endpoint')}
            placeholder={t('clusterPage.form.endpointPlaceholder', 'e.g. https://prod.k8s.nullus.io')}
            {...register('endpoint')}
          />
           {errors.endpoint && <span className="text-xs text-[#ef4444]">{errors.endpoint.message}</span>}
           <div className="flex flex-col gap-1">
             <label htmlFor="kubeconfig-file" className="text-xs font-medium text-[var(--color-text-secondary)]">
               {t('clusterPage.form.uploadKubeconfig', 'Upload kubeconfig File')}
             </label>
             <div className="flex items-center gap-2">
               <input
                 ref={fileInputRef}
                 id="kubeconfig-file"
                 type="file"
                 accept=".yaml,.yml,.conf"
                 onChange={handleFileUpload}
                 className="hidden"
               />
               <button
                 type="button"
                 onClick={() => fileInputRef.current?.click()}
                 className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[rgba(255,255,255,0.08)]"
               >
                 <Upload size={14} />
                 {t('clusterPage.form.chooseFile', 'Choose File')}
               </button>
               {fileInputRef.current?.files?.[0] && (
                 <span className="text-xs text-[var(--color-text-secondary)]">
                   {fileInputRef.current.files[0].name}
                 </span>
               )}
             </div>
             {fileUploadError && <span className="text-xs text-[#ef4444]">{fileUploadError}</span>}
           </div>
           <div className="flex flex-col gap-1">
             <label htmlFor="cluster-kubeconfig" className="text-xs font-medium text-[var(--color-text-secondary)]">
               kubeconfig (YAML)
             </label>
             {editingClusterId && isFetchingEditingCluster && (
               <span className="text-xs text-[var(--color-text-secondary)]">
                 {t('clusterPage.form.loadingCurrentKubeconfig', 'Loading current kubeconfig...')}
               </span>
             )}
             <textarea
               id="cluster-kubeconfig"
               {...register('kubeconfig')}
               placeholder={t('clusterPage.form.kubeconfigPlaceholder', 'Paste kubeconfig content...')}
               rows={8}
               className="resize-y rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-xs text-[var(--color-text-primary)] outline-none [font-family:'Fira_Code',monospace]"
             />
           </div>
           {errors.kubeconfig && <span className="text-xs text-[#ef4444]">{errors.kubeconfig.message}</span>}

          <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3">
            <div className="mb-2 flex items-center justify-between gap-2">
              <span className="text-xs font-semibold text-[var(--color-text-primary)]">
                {t('clusterPage.connection.title', 'Connection Status')}
              </span>
              <Button
                variant="outline"
                size="sm"
                type="button"
                loading={verifyClusterDraft.isPending}
                onClick={handleVerifyConnectionInModal}
              >
                {t('clusterPage.actions.verifyConnection', 'Verify Connection')}
              </Button>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <span
                className={cn(
                  'inline-flex items-center rounded-md border px-2 py-1 font-semibold',
                  draftVerifyStatus === 'connected'
                    ? 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.12)] text-[#22c55e]'
                    : draftVerifyStatus === 'error'
                      ? 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.12)] text-[#ef4444]'
                      : 'border-[var(--color-border-default)] bg-[rgba(148,163,184,0.1)] text-[var(--color-text-secondary)]'
                )}
              >
                {draftVerifyStatus === 'connected'
                  ? t('clusterPage.status.connected', 'Connected')
                  : draftVerifyStatus === 'error'
                    ? t('clusterPage.status.error', 'Error')
                    : t('clusterPage.status.pending', 'Pending')}
              </span>
              <span
                className={cn(
                  draftVerifyStatus === 'connected'
                    ? 'text-[#22c55e]'
                    : draftVerifyStatus === 'error'
                      ? 'text-[#ef4444]'
                      : 'text-[var(--color-text-secondary)]'
                )}
              >
                {draftVerifyMessage ?? t('clusterPage.connection.verifyBeforeSubmit', 'Verify connection at the final step before saving.')}
              </span>
            </div>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteClusterId !== null}
        onClose={() => {
          setDeleteClusterId(null)
          setDeleteClusterError(null)
        }}
        onConfirm={handleDeleteCluster}
        title={t('clusterPage.confirm.deleteTitle', 'Delete Cluster')}
        description={t('clusterPage.confirm.deleteDescription', 'Deleting this cluster may affect connected pipelines and deployment data. Continue?')}
        confirmLabel={t('common.delete', 'Delete')}
        loading={deleteCluster.isPending}
        customContent={
          deleteClusterError ? (
            <p className="m-0 rounded-md border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.12)] px-3 py-2 text-xs text-[#fca5a5]">
              {deleteClusterError}
            </p>
          ) : undefined
        }
      />
    </div>
  )
}
