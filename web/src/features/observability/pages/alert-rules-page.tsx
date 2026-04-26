import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useTranslation } from 'react-i18next'
import { Bell, Pencil, Plus, Search } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useAlertRule, useAlertRules, useCreateAlertRule, useUpdateAlertRule, useDeleteAlertRule } from '../api/observability-api'
import type { AlertRule, AlertChannel, CreateAlertRuleRequest } from '../api/observability-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { NativeSelect } from '../../../components/ui/native-select'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'
import { useClusterStackFilterState } from '../components/cluster-stack-filter'

const CHANNEL_BADGE: Record<AlertChannel, { className: string }> = {
  slack: { className: 'bg-[rgba(99,102,241,0.12)] text-[#a5b4fc]' },
  email: { className: 'bg-[rgba(16,185,129,0.12)] text-[#34d399]' },
}

interface AlertRuleForm {
  name: string
  metricName: string
  warningThreshold: number
  criticalThreshold: number
  channel: AlertChannel
  enabled: boolean
}

const alertRuleSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  metricName: z.string().min(1, 'Metric name is required'),
  warningThreshold: z.number().gt(0, 'Warning threshold must be greater than 0'),
  criticalThreshold: z.number().gt(0, 'Critical threshold must be greater than 0'),
  channel: z.enum(['slack', 'email']),
  enabled: z.boolean(),
}).refine((data) => data.criticalThreshold >= data.warningThreshold, {
  message: 'Critical threshold must be greater than or equal to warning threshold',
  path: ['criticalThreshold'],
})

const ALERT_RULE_DEFAULTS: AlertRuleForm = {
  name: '',
  metricName: '',
  warningThreshold: 1,
  criticalThreshold: 1,
  channel: 'slack',
  enabled: true,
}

export function AlertRulesPage() {
  const { t } = useTranslation()
  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId] = useState('')
  const [search, setSearch] = useState('')
  const { clusters, filteredStacks } = useClusterStackFilterState(selectedClusterId, selectedStackId)
  const { data: apiData, refetch: refetchAlertRules } = useAlertRules()
  const rules = useMemo<AlertRule[]>(() => apiData?.items ?? [], [apiData?.items])

  const createRule = useCreateAlertRule()
  const updateRule = useUpdateAlertRule()
  const deleteRule = useDeleteAlertRule()

  const [ruleModalOpen, setRuleModalOpen] = useState(false)
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null)
  const [deleteRuleId, setDeleteRuleId] = useState<string | null>(null)
  const { data: editingRule, isFetching: isFetchingEditingRule } = useAlertRule(editingRuleId, ruleModalOpen && editingRuleId !== null)
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

  const openEditModal = (rule: AlertRule) => {
    setEditingRuleId(rule.id)
    reset(ALERT_RULE_DEFAULTS)
    setRuleModalOpen(true)
  }

  useEffect(() => {
    if (!editingRule) return
    reset({
      name: editingRule.name,
      metricName: editingRule.metric_name,
      warningThreshold: Number(editingRule.warning_threshold) || 1,
      criticalThreshold: Number(editingRule.critical_threshold ?? editingRule.threshold) || 1,
      channel: editingRule.channel,
      enabled: editingRule.enabled,
    })
  }, [editingRule, reset])

  const submitRule = async (form: AlertRuleForm) => {
    const payload: CreateAlertRuleRequest = {
      name: form.name,
      metric_name: form.metricName,
      warning_threshold: form.warningThreshold,
      critical_threshold: form.criticalThreshold,
      channel: form.channel,
      enabled: form.enabled,
    }

    if (editingRuleId) {
      await updateRule.mutateAsync({ id: editingRuleId, data: payload })
      await refetchAlertRules()
      setRuleModalOpen(false)
      resetForm()
      return
    }

    await createRule.mutateAsync(payload)
    await refetchAlertRules()
    setRuleModalOpen(false)
    resetForm()
  }

  const handleDelete = () => {
    if (!deleteRuleId) return
    deleteRule.mutate(deleteRuleId, {
      onSuccess: () => {
        setDeleteRuleId(null)
      },
    })
  }

  const handleToggle = (rule: AlertRule) => {
    updateRule.mutate({
      id: rule.id,
      data: { enabled: !rule.enabled },
    })
  }

  const handleClusterChange = (clusterId: string) => {
    setSelectedClusterId(clusterId)
    setSelectedStackId('')
  }

  const handleStackChange = (stackId: string) => {
    setSelectedStackId(stackId)
  }

  const columns: ColumnDef<AlertRule, unknown>[] = [
    {
      accessorKey: 'enabled',
      header: t('alertRulesPage.table.active', 'Active'),
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
                rule.enabled ? 'bg-[#6366f1]' : 'bg-[rgba(255,255,255,0.12)]',
              )}
            >
              <div
                className={cn(
                  'absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white transition-transform duration-150',
                  rule.enabled && 'translate-x-4',
                )}
              />
            </button>
            <span className={cn('text-xs', rule.enabled ? 'text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]')}>
              {rule.enabled ? t('alertRulesPage.switch.on', 'On') : t('alertRulesPage.switch.off', 'Off')}
            </span>
          </div>
        )
      },
    },
    {
      accessorKey: 'name',
      header: t('alertRulesPage.table.name', 'Name'),
      cell: ({ row }) => <span className="font-semibold">{row.original.name}</span>,
    },
    {
      accessorKey: 'metric_name',
      header: t('alertRulesPage.table.metric', 'Metric'),
      cell: ({ row }) => <span className="font-mono text-xs text-[var(--color-text-secondary)]">{row.original.metric_name}</span>,
    },
    {
      accessorKey: 'condition',
      header: t('alertRulesPage.table.condition', 'Condition'),
      cell: ({ row }) => <span className="font-mono text-xs text-[var(--color-text-secondary)]">{row.original.condition}</span>,
    },
    {
      id: 'thresholds',
      header: t('alertRulesPage.table.thresholds', 'Thresholds'),
      cell: ({ row }) => (
        <div className="flex flex-col text-[12px] [font-family:'Fira_Code',monospace]">
          <span>{t('alertRulesPage.threshold.warning', 'Warning')}: {row.original.warning_threshold}</span>
          <span>{t('alertRulesPage.threshold.critical', 'Critical')}: {row.original.critical_threshold}</span>
        </div>
      ),
    },
    {
      accessorKey: 'channel',
      header: t('alertRulesPage.table.channel', 'Channel'),
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
      id: 'actions',
      header: t('alertRulesPage.table.actions', 'Actions'),
      enableSorting: false,
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" type="button" onClick={() => openEditModal(row.original)}>
            <Pencil size={12} />
            {t('alertRulesPage.actions.edit', 'Edit')}
          </Button>
          <Button variant="danger" size="sm" type="button" onClick={() => setDeleteRuleId(row.original.id)}>
            {t('alertRulesPage.actions.delete', 'Delete')}
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: 'Alert Rules' }]} />

      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(239,68,68,0.15)] text-[#f87171]">
            <Bell size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('observability.alertRules', 'Alert Rules')}
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              {t('observability.alertRulesDesc', 'Alert rule list and management')}
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={openCreateModal} type="button">
          <Plus size={15} />
          {t('observability.newRule', 'New Rule')}
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={rules.filter(
          (r) => !search
            || r.name.toLowerCase().includes(search.toLowerCase())
            || r.metric_name.toLowerCase().includes(search.toLowerCase()),
        )}
        getRowKey={(row) => row.id}
        emptyMessage={t('alertRulesPage.empty', 'No alert rules found.')}
        toolbar={(
          <div className="flex w-full flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2.5">
              <NativeSelect
                aria-label={t('clusterStackFilter.clusterLabel', 'Cluster')}
                value={selectedClusterId}
                onChange={(event) => handleClusterChange(event.target.value)}
                className="min-w-[200px]"
              >
                <option value="">{t('clusterStackFilter.selectCluster', '— Select Cluster —')}</option>
                {clusters.map((cluster) => (
                  <option key={cluster.id} value={cluster.id}>{cluster.name}</option>
                ))}
              </NativeSelect>
              <NativeSelect
                aria-label={t('clusterStackFilter.stackLabel', 'Stack')}
                value={selectedStackId}
                onChange={(event) => handleStackChange(event.target.value)}
                className="min-w-[200px]"
              >
                <option value="">{t('clusterStackFilter.selectStack', '— Select Stack —')}</option>
                {filteredStacks.map((stack) => (
                  <option key={stack.id} value={stack.id}>{stack.name}</option>
                ))}
              </NativeSelect>
              {(selectedClusterId || selectedStackId) && (
                <button
                  type="button"
                  onClick={() => {
                    setSelectedClusterId('')
                    setSelectedStackId('')
                  }}
                  className="text-xs text-[var(--color-text-secondary)] hover:text-red-400"
                >
                  {t('clusterStackFilter.clear', 'Clear')}
                </button>
              )}
            </div>

            <div className="relative ml-auto">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <input
                placeholder={t('alertRulesPage.searchPlaceholder', 'Search by rule or metric...')}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
              />
            </div>
          </div>
        )}
      />

      <Modal
        open={ruleModalOpen}
        onClose={() => {
          setRuleModalOpen(false)
          resetForm()
        }}
        title={editingRuleId ? t('alertRulesPage.modal.editTitle', 'Edit Alert Rule') : t('alertRulesPage.modal.newTitle', 'New Alert Rule')}
        footer={(
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
              {t('common.cancel', 'Cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={createRule.isPending || updateRule.isPending || isSubmitting}
              onClick={handleSubmit(submitRule)}
              disabled={!isValid || isSubmitting || isFetchingEditingRule}
              type="button"
            >
              {editingRuleId ? t('common.save', 'Save') : t('alertRulesPage.actions.create', 'Create')}
            </Button>
          </>
        )}
      >
        <div className="flex flex-col gap-3.5">
          {editingRuleId && isFetchingEditingRule ? (
            <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-2 text-sm text-[var(--color-text-secondary)]">
              {t('alertRulesPage.modal.loadingFromDb', 'Loading latest alert rule from DB...')}
            </div>
          ) : null}
          <div>
            <Input
              label={t('alertRulesPage.form.name', 'Name')}
              placeholder={t('alertRulesPage.form.namePlaceholder', 'ex) High CPU Alert')}
              {...register('name')}
            />
            {errors.name && <span className="mt-1 block text-xs text-[#ef4444]">{errors.name.message}</span>}
          </div>

          <div>
            <Input
              label={t('alertRulesPage.form.metricName', 'Metric Name')}
              placeholder={t('alertRulesPage.form.metricNamePlaceholder', 'ex) cpu_usage')}
              {...register('metricName')}
            />
            {errors.metricName && <span className="mt-1 block text-xs text-[#ef4444]">{errors.metricName.message}</span>}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Input
                label={t('alertRulesPage.form.warningThreshold', 'Warning Threshold')}
                type="number"
                placeholder={t('alertRulesPage.form.warningThresholdPlaceholder', 'ex) 70')}
                {...register('warningThreshold', { valueAsNumber: true })}
              />
              {errors.warningThreshold && <span className="mt-1 block text-xs text-[#ef4444]">{errors.warningThreshold.message}</span>}
            </div>
            <div>
              <Input
                label={t('alertRulesPage.form.criticalThreshold', 'Critical Threshold')}
                type="number"
                placeholder={t('alertRulesPage.form.criticalThresholdPlaceholder', 'ex) 85')}
                {...register('criticalThreshold', { valueAsNumber: true })}
              />
              {errors.criticalThreshold && <span className="mt-1 block text-xs text-[#ef4444]">{errors.criticalThreshold.message}</span>}
            </div>
          </div>

          <NativeSelect label={t('alertRulesPage.form.channel', 'Channel')} {...register('channel')}>
            <option value="slack">Slack</option>
            <option value="email">Email</option>
          </NativeSelect>

          <div className="flex flex-col gap-1">
            <span className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">{t('alertRulesPage.form.active', 'Active')}</span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setValue('enabled', !watch('enabled'), { shouldValidate: true })}
                className={cn(
                  'relative h-5 w-9 cursor-pointer rounded-[10px] border-0 p-0 transition-colors duration-150',
                  watch('enabled') ? 'bg-[#6366f1]' : 'bg-[rgba(255,255,255,0.12)]',
                )}
              >
                <div
                  className={cn(
                    'absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white transition-transform duration-150',
                    watch('enabled') && 'translate-x-4',
                  )}
                />
              </button>
              <span className={cn('text-sm', watch('enabled') ? 'text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]')}>
                {watch('enabled') ? t('alertRulesPage.switch.on', 'On') : t('alertRulesPage.switch.off', 'Off')}
              </span>
            </div>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteRuleId !== null}
        onClose={() => setDeleteRuleId(null)}
        onConfirm={handleDelete}
        title={t('alertRulesPage.confirm.deleteTitle', 'Delete Alert Rule')}
        description={t('alertRulesPage.confirm.deleteDescription', 'Deleting this rule stops future alerts from being triggered by it. Continue?')}
        confirmLabel={t('common.delete', 'Delete')}
        loading={deleteRule.isPending}
      />
    </div>
  )
}
