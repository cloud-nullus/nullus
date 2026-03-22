import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Bell, Plus, Search } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useAlertRules, useCreateAlertRule, useUpdateAlertRule, useDeleteAlertRule } from '../api/observability-api'
import type { AlertRule, AlertChannel, AlertSeverity, CreateAlertRuleRequest } from '../api/observability-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { NativeSelect } from '../../../components/ui/native-select'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'

type AlertRuleWithSeverity = AlertRule & { severity: AlertSeverity }

const CHANNEL_BADGE: Record<AlertChannel, { className: string }> = {
  slack: { className: 'bg-[rgba(99,102,241,0.12)] text-[#a5b4fc]' },
  email: { className: 'bg-[rgba(16,185,129,0.12)] text-[#34d399]' },
}

const SEVERITY_BADGE: Record<AlertSeverity, { className: string; label: string }> = {
  critical: { className: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]', label: 'Critical' },
  warning: { className: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]', label: 'Warning' },
  info: { className: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]', label: 'Info' },
}

interface AlertRuleForm {
  name: string
  severity: AlertSeverity
  condition: string
  threshold: number
  channel: AlertChannel
  enabled: boolean
}

const alertRuleSchema = z.object({
  name: z.string().min(1, '이름을 입력하세요'),
  severity: z.enum(['critical', 'warning', 'info']),
  condition: z.string().min(1, '조건을 입력하세요'),
  threshold: z.number().gt(0, '임계값은 0보다 커야 합니다'),
  channel: z.enum(['slack', 'email']),
  enabled: z.boolean(),
})

const ALERT_RULE_DEFAULTS: AlertRuleForm = {
  name: '',
  severity: 'warning',
  condition: '',
  threshold: 1,
  channel: 'slack',
  enabled: true,
}


export function AlertRulesPage() {
  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId] = useState('')
  const [search, setSearch] = useState('')
  const { clusters, filteredStacks, selectedCluster, selectedStack } = useClusterStackFilterState(selectedClusterId, selectedStackId)
  const { data: apiData } = useAlertRules()
  const [localRules, setLocalRules] = useState<AlertRuleWithSeverity[]>([])
  const rules = useMemo<AlertRuleWithSeverity[]>(() => {
    if (!apiData?.items) return localRules
    return apiData.items.map((rule) => ({ ...rule, severity: (rule as AlertRuleWithSeverity).severity ?? 'warning' }))
  }, [apiData?.items, localRules])

  const createRule = useCreateAlertRule()
  const updateRule = useUpdateAlertRule()
  const deleteRule = useDeleteAlertRule()

  const [ruleModalOpen, setRuleModalOpen] = useState(false)
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null)
  const [deleteRuleId, setDeleteRuleId] = useState<string | null>(null)
  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isValid, isSubmitting },
  } = useForm<AlertRuleForm>({
    resolver: zodResolver(alertRuleSchema),
    defaultValues: ALERT_RULE_DEFAULTS,
    mode: 'onChange',
  })

  const resetForm = () => {
    reset(ALERT_RULE_DEFAULTS)
    setEditingRuleId(null)
  }

  const openCreateModal = () => {
    resetForm()
    setRuleModalOpen(true)
  }

  const openEditModal = (rule: AlertRuleWithSeverity) => {
    setEditingRuleId(rule.id)
    reset({
      name: rule.name,
      severity: rule.severity,
      condition: rule.condition,
      threshold: Number(rule.threshold.replace('%', '')) || 1,
      channel: rule.channel,
      enabled: rule.enabled,
    })
    setRuleModalOpen(true)
  }

  const submitRule = (form: AlertRuleForm) => {
    const payload: CreateAlertRuleRequest = {
      name: form.name,
      condition: form.condition,
      threshold: String(form.threshold),
      channel: form.channel,
    }

    if (editingRuleId) {
      updateRule.mutate(
        { id: editingRuleId, data: payload },
        {
          onSuccess: () => {
            setRuleModalOpen(false)
            resetForm()
          },
        }
      )
      setLocalRules((prev) =>
        prev.map((rule) => (rule.id === editingRuleId ? { ...rule, ...payload, severity: form.severity } : rule))
      )
      return
    }

    createRule.mutate(payload, {
      onSuccess: () => {
        setRuleModalOpen(false)
        resetForm()
      },
    })
    setLocalRules((prev) => [
      {
        id: `local-${Date.now()}`,
        name: form.name,
        severity: form.severity,
        condition: payload.condition,
        threshold: payload.threshold,
        channel: payload.channel,
        enabled: form.enabled,
        createdAt: new Date().toISOString(),
      },
      ...prev,
    ])
  }

  const handleDelete = () => {
    if (!deleteRuleId) return
    deleteRule.mutate(deleteRuleId, {
      onSuccess: () => {
        setDeleteRuleId(null)
      },
    })
    setLocalRules((prev) => prev.filter((rule) => rule.id !== deleteRuleId))
    setDeleteRuleId(null)
  }

  const handleToggle = (rule: AlertRule) => {
    updateRule.mutate({ id: rule.id, data: { enabled: !rule.enabled } })
  }

  const handleClusterChange = (clusterId: string) => {
    setSelectedClusterId(clusterId)
    setSelectedStackId('')
  }

  const handleStackChange = (stackId: string) => {
    setSelectedStackId(stackId)
  }

  const extractMetric = (condition: string): string => {
    const [metric = ''] = condition.trim().split(' ')
    return metric
  }

  const columns: ColumnDef<AlertRuleWithSeverity, unknown>[] = [
    {
      accessorKey: 'name',
      header: '이름',
      cell: ({ row }) => <span className="font-semibold">{row.original.name}</span>,
    },
    {
      accessorKey: 'severity',
      header: '심각도',
      cell: ({ row }) => (
        <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-bold', SEVERITY_BADGE[row.original.severity].className)}>
          {SEVERITY_BADGE[row.original.severity].label}
        </span>
      ),
    },
    {
      accessorKey: 'condition',
      header: '조건',
      cell: ({ row }) => <span className="font-mono text-xs text-[var(--color-text-secondary)]">{row.original.condition}</span>,
    },
    {
      accessorKey: 'threshold',
      header: '임계값',
      cell: ({ row }) => <span className="text-[13px] [font-family:'Fira_Code',monospace]">{row.original.threshold}</span>,
    },
    {
      accessorKey: 'channel',
      header: '채널',
      cell: ({ row }) => {
        const ch = CHANNEL_BADGE[row.original.channel]
        return (
          <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', ch.className)}>
            {row.original.channel}
          </span>
        )
      },
    },
    {
      accessorKey: 'enabled',
      header: '활성',
      cell: ({ row }) => {
        const rule = row.original
        return (
          <div className="inline-flex items-center gap-2">
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation()
                handleToggle(rule)
              }}
              className={cn(
                'relative h-5 w-9 cursor-pointer rounded-[10px] border-0 p-0 transition-colors duration-150',
                rule.enabled ? 'bg-[#6366f1]' : 'bg-[rgba(255,255,255,0.12)]'
              )}
            >
              <div
                className={cn(
                  'absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white transition-transform duration-150',
                  rule.enabled && 'translate-x-4'
                )}
              />
            </button>
            <span className={cn('text-xs', rule.enabled ? 'text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]')}>
              {rule.enabled ? 'On' : 'Off'}
            </span>
          </div>
        )
      },
    },
    {
      id: 'actions',
      header: 'Actions',
      enableSorting: false,
      cell: ({ row }) => (
        <div className="flex gap-1.5">
          <Button variant="ghost" size="sm" type="button" onClick={() => openEditModal(row.original)}>Edit</Button>
          <Button variant="danger" size="sm" type="button" onClick={() => setDeleteRuleId(row.original.id)}>Delete</Button>
        </div>
      ),
    },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: 'Alert Rules' }]} />

      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(239,68,68,0.15)] text-[#f87171]">
            <Bell size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Alert Rules
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              알림 규칙 목록 및 관리
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={openCreateModal} type="button">
          <Plus size={15} />
          New Rule
        </Button>
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
        data={rules.filter(
          (r) => !search || r.name.toLowerCase().includes(search.toLowerCase()) || extractMetric(r.condition).toLowerCase().includes(search.toLowerCase())
        )}
        getRowKey={(row) => row.id}
        emptyMessage="알림 규칙이 없습니다."
        toolbar={
          <div className="relative ml-auto">
            <Search
              size={13}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
            />
            <input
              placeholder="규칙명 / 메트릭 검색..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
            />
          </div>
        }
      />

      <Modal
        open={ruleModalOpen}
        onClose={() => {
          setRuleModalOpen(false)
          resetForm()
        }}
        title={editingRuleId ? 'Edit Alert Rule' : 'New Alert Rule'}
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setRuleModalOpen(false)
                resetForm()
              }}
              type="button"
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={createRule.isPending || updateRule.isPending || isSubmitting}
              onClick={handleSubmit(submitRule)}
              disabled={!isValid || isSubmitting}
              type="button"
            >
              {editingRuleId ? 'Save' : 'Create'}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3.5">
          {/* 이름 */}
          <div>
            <Input
              label="이름"
              placeholder="예: High CPU Alert"
              {...register('name')}
            />
            {errors.name && <span className="mt-1 block text-xs text-[#ef4444]">{errors.name.message}</span>}
          </div>

          {/* 심각도 */}
          <NativeSelect label="심각도" {...register('severity')}>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
            <option value="info">Info</option>
          </NativeSelect>

          {/* 조건 */}
          <div>
            <Input
              label="조건"
              placeholder="예: cpu_usage > 80%"
              {...register('condition')}
            />
            {errors.condition && <span className="mt-1 block text-xs text-[#ef4444]">{errors.condition.message}</span>}
          </div>

          {/* 임계값 */}
          <div>
            <Input
              label="임계값"
              type="number"
              placeholder="예: 80"
              {...register('threshold', { valueAsNumber: true })}
            />
            {errors.threshold && <span className="mt-1 block text-xs text-[#ef4444]">{errors.threshold.message}</span>}
          </div>

          {/* 채널 */}
          <NativeSelect label="채널" {...register('channel')}>
            <option value="slack">Slack</option>
            <option value="email">Email</option>
          </NativeSelect>

          {/* 활성 */}
          <div className="flex flex-col gap-1">
            <span className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">활성</span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setValue('enabled', !watch('enabled'), { shouldValidate: true })}
                className={cn(
                  'relative h-5 w-9 cursor-pointer rounded-[10px] border-0 p-0 transition-colors duration-150',
                  watch('enabled') ? 'bg-[#6366f1]' : 'bg-[rgba(255,255,255,0.12)]'
                )}
              >
                <div
                  className={cn(
                    'absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white transition-transform duration-150',
                    watch('enabled') && 'translate-x-4'
                  )}
                />
              </button>
              <span className={cn('text-sm', watch('enabled') ? 'text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]')}>
                {watch('enabled') ? 'On' : 'Off'}
              </span>
            </div>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteRuleId !== null}
        onClose={() => setDeleteRuleId(null)}
        onConfirm={handleDelete}
        title="Delete Alert Rule"
        description="이 알림 규칙을 삭제하면 더 이상 알림이 발생하지 않습니다. 계속하시겠습니까?"
        confirmLabel="Delete"
        loading={deleteRule.isPending}
      />
    </div>
  )
}
