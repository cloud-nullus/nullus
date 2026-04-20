import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CheckCircle2, AlertTriangle, XCircle, RefreshCw, Plus, Pencil, Trash2 } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { Button } from '../../../components/ui/button'
import { useCompatibilityMatrix, useDeleteMatrix } from '../../stack/api/stack-api'
import { useClusters, useRefreshDiscovery } from '../api/admin-api'
import { MatrixEditModal } from '../components/matrix-edit-modal'
import type { CompatibilityMatrix, CompatibilityTier } from '../../../types'
import { cn } from '../../../lib/utils'
import {
  isMatrixCompatibleWithCluster,
  matrixArchMismatches,
} from '../../stack/utils/compatibility-arch'

const STATUS_BADGE_CLASS: Record<CompatibilityMatrix['status'], string> = {
  verified: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
  untested: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
  unsupported: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
}

const TIER_BADGE_CLASS: Record<CompatibilityTier, string> = {
  stable: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
  beta: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
  deprecated: 'bg-[rgba(148,163,184,0.15)] text-[#94a3b8]',
}

export function StackVersionsAdminPage() {
  const { t } = useTranslation()
  const { data: matrices, isLoading: matricesLoading, isError: matricesError } = useCompatibilityMatrix()
  const { data: clustersData } = useClusters()
  const refreshDiscovery = useRefreshDiscovery()
  const deleteMatrix = useDeleteMatrix()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [modal, setModal] = useState<{ mode: 'create' | 'edit'; initial?: CompatibilityMatrix } | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<CompatibilityMatrix | null>(null)

  // Determinism: always render by id so server order doesn't leak into UI.
  const sortedMatrices = useMemo(
    () => (matrices ?? []).slice().sort((a, b) => a.id.localeCompare(b.id)),
    [matrices],
  )

  const sortedClusters = useMemo(
    () => (clustersData?.items ?? []).slice().sort((a, b) => a.id.localeCompare(b.id)),
    [clustersData],
  )

  const selectedMatrix = useMemo(
    () => sortedMatrices.find((m) => m.id === (selectedId ?? sortedMatrices[0]?.id)) ?? null,
    [sortedMatrices, selectedId],
  )

  const handleRefresh = async (clusterId: string) => {
    try {
      await refreshDiscovery.mutateAsync(clusterId)
    } catch {
      // Error is surfaced via the mutation state + status column below; do
      // not rethrow — the button should remain usable for retries.
    }
  }

  const listContent = (
    <div>
      <div className="border-b border-[var(--color-border-default)] px-4 py-3">
        <div className="text-sm font-medium text-[var(--color-text-primary)]">
          {t('stackVersionsAdmin.listTitle', 'Compatibility Matrices')}
        </div>
        <div className="text-xs text-[var(--color-text-secondary)]">
          {t('stackVersionsAdmin.listSubtitle', 'Golden Path 3 canonical matrices (Narwhal baseline)')}
        </div>
      </div>
      {matricesLoading && (
        <div className="p-4 text-xs text-[var(--color-text-secondary)]">Loading…</div>
      )}
      {matricesError && (
        <div className="p-4 text-xs text-[#ef4444]">
          {t('stackVersionsAdmin.loadError', 'Failed to load compatibility matrices.')}
        </div>
      )}
      <ul>
        {sortedMatrices.map((m) => {
          const active = selectedMatrix?.id === m.id
          return (
            <li key={m.id}>
              <button
                type="button"
                onClick={() => setSelectedId(m.id)}
                className={cn(
                  'flex w-full flex-col items-start gap-1 border-b border-[var(--color-border-default)] px-4 py-3 text-left transition-colors hover:bg-[rgba(255,255,255,0.03)]',
                  active && 'bg-[rgba(99,102,241,0.08)]',
                )}
                aria-pressed={active}
              >
                <div className="flex w-full items-center justify-between gap-2">
                  <span className="truncate text-sm text-[var(--color-text-primary)]">{m.name}</span>
                  <span
                    className={cn('rounded-full px-2 py-0.5 text-[10px] uppercase', STATUS_BADGE_CLASS[m.status])}
                    aria-label={`status ${m.status}`}
                  >
                    {t(`stackVersionsAdmin.status.${m.status}`, m.status)}
                  </span>
                </div>
                <span className="font-mono text-[11px] text-[var(--color-text-secondary)]">{m.id}</span>
                <span className="text-[11px] text-[var(--color-text-secondary)]">
                  K8s {m.k8sRange} · {m.tools.length} tools
                </span>
              </button>
            </li>
          )
        })}
      </ul>
    </div>
  )

  const renderArchBadges = (archs: string[] | undefined) => {
    if (!archs || archs.length === 0) {
      return (
        <span className="text-xs text-[var(--color-text-secondary)]">
          {t('stackVersionsAdmin.archBadge.unknown', '—')}
        </span>
      )
    }
    return (
      <div className="flex flex-wrap gap-1">
        {archs.map((arch) => (
          <span
            key={arch}
            className="rounded-full bg-[rgba(99,102,241,0.12)] px-2 py-0.5 text-[10px] text-[#818cf8]"
          >
            {t(`stackVersionsAdmin.archBadge.${arch}`, arch)}
          </span>
        ))}
      </div>
    )
  }

  const renderCrossEval = (archs: string[]) => {
    if (!selectedMatrix) return null
    const verdict = isMatrixCompatibleWithCluster(selectedMatrix, archs)
    if (verdict === 'compatible') {
      return (
        <span className="inline-flex items-center gap-1 text-[#22c55e]">
          <CheckCircle2 size={14} />
          {t('stackVersionsAdmin.crossEval.compatible', 'Compatible')}
        </span>
      )
    }
    if (verdict === 'incompatible') {
      const mismatches = matrixArchMismatches(selectedMatrix, archs)
      const title = mismatches
        .map((m) => `${m.toolName}: missing ${m.missingArchs.join(', ')}`)
        .join('\n')
      return (
        <span
          className="inline-flex items-center gap-1 text-[#ef4444]"
          title={title || t('stackVersionsAdmin.crossEval.incompatible', 'Incompatible')}
        >
          <XCircle size={14} />
          {t('stackVersionsAdmin.crossEval.incompatible', 'Incompatible')}
        </span>
      )
    }
    return (
      <span className="inline-flex items-center gap-1 text-[#f59e0b]">
        <AlertTriangle size={14} />
        {t('stackVersionsAdmin.crossEval.unknown', 'Unknown')}
      </span>
    )
  }

  const detailContent = selectedMatrix ? (
    <div className="flex flex-col gap-4 p-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-[var(--color-text-primary)]">{selectedMatrix.name}</h2>
          <p className="font-mono text-xs text-[var(--color-text-secondary)]">{selectedMatrix.id}</p>
        </div>
        <div className="flex items-center gap-2">
          <span className={cn('rounded-full px-2 py-0.5 text-[11px] uppercase', STATUS_BADGE_CLASS[selectedMatrix.status])}>
            {t(`stackVersionsAdmin.status.${selectedMatrix.status}`, selectedMatrix.status)}
          </span>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setModal({ mode: 'edit', initial: selectedMatrix })}
            aria-label={t('stackVersionsAdmin.actions.edit', 'Edit matrix')}
          >
            <Pencil size={12} className="mr-1" />
            {t('stackVersionsAdmin.actions.edit', 'Edit')}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setDeleteTarget(selectedMatrix)}
            aria-label={t('stackVersionsAdmin.actions.delete', 'Delete matrix')}
          >
            <Trash2 size={12} className="mr-1 text-[#ef4444]" />
            {t('stackVersionsAdmin.actions.delete', 'Delete')}
          </Button>
        </div>
      </header>

      <div className="rounded-md border border-[var(--color-border-default)] p-4">
        <div className="text-xs text-[var(--color-text-secondary)]">
          {t('stackVersionsAdmin.k8sRangeLabel', 'Kubernetes version range')}
        </div>
        <div className="mt-1 text-sm text-[var(--color-text-primary)]">{selectedMatrix.k8sRange}</div>
      </div>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">
          {t('stackVersionsAdmin.toolsHeading', 'Tools')}
        </h3>
        <div className="overflow-x-auto rounded-md border border-[var(--color-border-default)]">
          <table className="min-w-full text-left text-xs">
            <thead className="bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]">
              <tr>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.name', 'Name')}</th>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.helm', 'Helm')}</th>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.app', 'App')}</th>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.arch', 'Arch')}</th>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.minK8s', 'Min K8s')}</th>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.tier', 'Tier')}</th>
              </tr>
            </thead>
            <tbody>
              {selectedMatrix.tools.map((tool) => (
                <tr key={`${tool.name}-${tool.helmVersion}`} className="border-t border-[var(--color-border-default)]">
                  <td className="px-3 py-2 text-[var(--color-text-primary)]">{tool.name}</td>
                  <td className="px-3 py-2 font-mono text-[11px]">{tool.helmVersion}</td>
                  <td className="px-3 py-2 font-mono text-[11px]">{tool.appVersion}</td>
                  <td className="px-3 py-2">{renderArchBadges(tool.archSupport)}</td>
                  <td className="px-3 py-2 font-mono text-[11px]">{tool.minK8sVersion || '—'}</td>
                  <td className="px-3 py-2">
                    <span
                      className={cn(
                        'rounded-full px-2 py-0.5 text-[10px] uppercase',
                        TIER_BADGE_CLASS[tool.tier] ?? TIER_BADGE_CLASS.stable,
                      )}
                    >
                      {t(`stackVersionsAdmin.tier.${tool.tier}`, tool.tier)}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">
          {t('stackVersionsAdmin.clustersHeading', 'Clusters')}
        </h3>
        <div className="overflow-x-auto rounded-md border border-[var(--color-border-default)]">
          <table className="min-w-full text-left text-xs">
            <thead className="bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]">
              <tr>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.cluster', 'Cluster')}</th>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.nodeArch', 'Node Architectures')}</th>
                <th className="px-3 py-2">{t('stackVersionsAdmin.col.crossEval', 'Compatibility')}</th>
                <th className="px-3 py-2 text-right">{t('stackVersionsAdmin.col.actions', 'Actions')}</th>
              </tr>
            </thead>
            <tbody>
              {sortedClusters.length === 0 && (
                <tr>
                  <td className="px-3 py-4 text-[var(--color-text-secondary)]" colSpan={4}>
                    {t('stackVersionsAdmin.noClusters', 'No clusters registered.')}
                  </td>
                </tr>
              )}
              {sortedClusters.map((cluster) => (
                <tr key={cluster.id} className="border-t border-[var(--color-border-default)]">
                  <td className="px-3 py-2">
                    <div className="text-[var(--color-text-primary)]">{cluster.name}</div>
                    <div className="font-mono text-[11px] text-[var(--color-text-secondary)]">{cluster.id}</div>
                  </td>
                  <td className="px-3 py-2">{renderArchBadges(cluster.nodeArchitectures)}</td>
                  <td className="px-3 py-2">{renderCrossEval(cluster.nodeArchitectures)}</td>
                  <td className="px-3 py-2 text-right">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => handleRefresh(cluster.id)}
                      disabled={refreshDiscovery.isPending && refreshDiscovery.variables === cluster.id}
                      aria-label={t('stackVersionsAdmin.refreshDiscovery.button', 'Refresh Discovery')}
                    >
                      <RefreshCw size={12} className="mr-1" />
                      {t('stackVersionsAdmin.refreshDiscovery.button', 'Refresh Discovery')}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  ) : null

  const handleConfirmDelete = () => {
    if (!deleteTarget) return
    deleteMatrix.mutate(deleteTarget.id, {
      onSettled: () => setDeleteTarget(null),
      onSuccess: () => {
        if (selectedId === deleteTarget.id) {
          setSelectedId(null)
        }
      },
    })
  }

  return (
    <div className="flex h-full flex-col gap-4 p-6">
      <Breadcrumb
        items={[
          { label: t('stackVersionsAdmin.breadcrumb.devsecops', 'DevSecOps Stack') },
          { label: t('stackVersionsAdmin.breadcrumb.stackVersions', 'Stack Version Management') },
        ]}
      />
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">
            {t('stackVersionsAdmin.title', 'Stack Version Management')}
          </h1>
          <p className="text-sm text-[var(--color-text-secondary)]">
            {t(
              'stackVersionsAdmin.subtitle',
              'Review Narwhal baseline compatibility matrices and verify cluster architecture fit.',
            )}
          </p>
        </div>
        <Button size="sm" onClick={() => setModal({ mode: 'create' })}>
          <Plus size={14} className="mr-1" />
          {t('stackVersionsAdmin.actions.new', 'New matrix')}
        </Button>
      </div>
      <div className="flex-1 min-h-0">
        <ListDetailPanel
          listContent={listContent}
          detailContent={detailContent}
          emptyDetailMessage={t('stackVersionsAdmin.selectMatrix', 'Select a matrix to view details.')}
        />
      </div>

      {modal && (
        <MatrixEditModal
          open
          mode={modal.mode}
          initial={modal.initial}
          onClose={() => setModal(null)}
          onSaved={(id) => {
            setSelectedId(id)
            setModal(null)
          }}
        />
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleConfirmDelete}
        title={t('stackVersionsAdmin.deleteConfirm.title', 'Delete compatibility matrix')}
        description={t(
          'stackVersionsAdmin.deleteConfirm.description',
          'Deleting this matrix means stacks that previously matched it will be treated as untested. Continue?',
        )}
        confirmLabel={t('stackVersionsAdmin.deleteConfirm.confirm', 'Delete')}
        loading={deleteMatrix.isPending}
      />
    </div>
  )
}
