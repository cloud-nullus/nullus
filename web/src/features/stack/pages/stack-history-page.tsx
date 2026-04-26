import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router-dom'
import { ChevronDown, ChevronUp, History, GitCompare, RotateCcw, Search, AlertTriangle, Terminal } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import type { ColumnDef } from '@tanstack/react-table'
import { useStacks, useStackHistory, useRollbackStack, useStackVersionDiff } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Modal } from '../../../components/ui/modal'
import { DataTable } from '../../../components/shared/data-table'
import type { StackHistoryEntry, StackVersionDiff } from '../api/stack-api'
import { VersionDiff } from '../components/version-diff'
import { formatDateTime, resolveLocale } from '../../../lib/locale'


export function StackHistoryPage() {
   const { t, i18n } = useTranslation()
   const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
   const { data: stacksData } = useStacks()
   const navigate = useNavigate()
   const { stackId: routeStackId } = useParams<{ stackId?: string }>()
   const [expandedId, setExpandedId] = useState<string | null>(null)
   const [search, setSearch] = useState('')
   const [clusterFilter, setClusterFilter] = useState('')

   const stacks = stacksData?.items ?? []
   const clusterOptions = Array.from(new Set(stacks.map((stack) => stack.clusterName).filter(Boolean))).sort()
   const visibleStacks = clusterFilter ? stacks.filter((stack) => stack.clusterName === clusterFilter) : stacks
   const fallbackStackId = visibleStacks[0]?.id ?? ''
   const stackId = routeStackId ?? fallbackStackId
   const currentRouteMissingFromOptions = !!routeStackId && !visibleStacks.some((stack) => stack.id === routeStackId)
   const [compareOpen, setCompareOpen] = useState(false)
   const [versionA, setVersionA] = useState(0)
   const [versionB, setVersionB] = useState(0)
   const [rollbackEntry, setRollbackEntry] = useState<StackHistoryEntry | null>(null)
   const [preservePVC, setPreservePVC] = useState(true)
   const [deleteConfirmText, setDeleteConfirmText] = useState('')

  useEffect(() => {
    if (!routeStackId && fallbackStackId) {
      navigate(`/stack/history/${fallbackStackId}`, { replace: true })
      return
    }

    if (
      routeStackId &&
      clusterFilter &&
      visibleStacks.length > 0 &&
      !visibleStacks.some((stack) => stack.id === routeStackId)
    ) {
      navigate(`/stack/history/${visibleStacks[0].id}`, { replace: true })
    }
  }, [clusterFilter, fallbackStackId, routeStackId, visibleStacks, navigate])

  useEffect(() => {
    setVersionA(0)
    setVersionB(0)
  }, [stackId])

  const { data: historyData } = useStackHistory(stackId)
  const allEntries = Array.isArray(historyData) ? historyData : []
  const entries = search.trim()
    ? allEntries.filter(
        (e) =>
          e.changedBy.toLowerCase().includes(search.toLowerCase()) ||
          e.reason.toLowerCase().includes(search.toLowerCase())
      )
    : allEntries
  const rollbackMutation = useRollbackStack()

  const versionOptions = entries.map((entry) => entry.version).sort((a, b) => b - a)

  useEffect(() => {
    if (versionOptions.length >= 2 && versionA === 0 && versionB === 0) {
      setVersionA(versionOptions[1])
      setVersionB(versionOptions[0])
    }
  }, [versionOptions, versionA, versionB])

  const compareEntryA = entries.find((entry) => entry.version === versionA) ?? null
  const compareEntryB = entries.find((entry) => entry.version === versionB) ?? null

  const { data: apiDiff } = useStackVersionDiff(stackId, versionA, versionB)
  const fallbackDiff = buildSnapshotDiff(compareEntryA?.snapshot ?? {}, compareEntryB?.snapshot ?? {})
  const diff = apiDiff ?? fallbackDiff

   const handleRollbackConfirm = () => {
     if (!rollbackEntry) return
     rollbackMutation.mutate(
       { stackId, version: rollbackEntry.version, preservePVC },
       { onSuccess: () => {
         setRollbackEntry(null)
         setPreservePVC(true)
         setDeleteConfirmText('')
       } }
     )
   }

  const columns: ColumnDef<StackHistoryEntry, unknown>[] = [
    {
      id: 'expand',
      header: '',
      enableSorting: false,
      cell: ({ row }) => {
        const isExpanded = expandedId === row.original.id
        return (
          <Button
            variant={isExpanded ? 'secondary' : 'ghost'}
            size="sm"
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              setExpandedId((prev) => (prev === row.original.id ? null : row.original.id))
            }}
          >
            {isExpanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
          </Button>
        )
      },
    },
    {
      id: 'stackName',
      header: t('stackHistoryPage.table.stackName', 'Stack Name'),
      enableSorting: false,
      cell: ({ row }) => {
        const name = stacks.find((s) => s.id === row.original.stackId)?.name ?? row.original.stackId
        return <span className="font-semibold text-[var(--color-text-primary)]">{name}</span>
      },
    },
    {
      id: 'cluster',
      header: t('stackHistoryPage.table.cluster', 'Cluster'),
      enableSorting: false,
      cell: ({ row }) => {
        const cluster = stacks.find((s) => s.id === row.original.stackId)?.clusterName ?? '-'
        return <span className="text-[13px] text-[var(--color-text-secondary)]">{cluster}</span>
      },
    },
    {
      accessorKey: 'version',
      header: t('stackHistoryPage.table.version', 'Version'),
      cell: ({ row }) => {
        const entry = row.original
        const isCurrent = entry.id === entries[0]?.id
        return (
          <span className="inline-flex items-center gap-1.5 font-mono text-[13px] font-semibold text-[#a5b4fc]">
            v{entry.version}
            {isCurrent && (
              <span className="rounded bg-[rgba(34,197,94,0.15)] px-1.5 py-[1px] font-inherit text-[10px] text-[#22c55e]">
                {t('stackHistoryPage.current', 'CURRENT')}
              </span>
            )}
          </span>
        )
      },
    },
    {
      accessorKey: 'changedBy',
      header: t('stackHistoryPage.table.changedBy', 'Changed By'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{row.original.changedBy}</span>,
    },
    {
      accessorKey: 'changedAt',
      header: t('stackHistoryPage.table.changedAt', 'Changed At'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDateTime(row.original.changedAt, locale)}</span>,
    },
    {
      accessorKey: 'reason',
      header: t('stackHistoryPage.table.reason', 'Reason'),
    },
      {
        id: 'actions',
        header: t('stackHistoryPage.table.actions', 'Actions'),
        enableSorting: false,
        cell: ({ row }) => {
          const entry = row.original
          const index = entries.findIndex((item) => item.id === entry.id)
          return (
            <div className="flex gap-1.5">
              <Button
                variant="outline"
                size="sm"
                onClick={(event) => {
                  event.stopPropagation()
                  navigate(`/stack/logs/${entry.stackId}`)
                }}
                type="button"
              >
                <Terminal size={13} />
                {t('stackHistoryPage.actions.log', 'Log')}
              </Button>
              {index !== 0 && (
                <Button
                variant="danger"
                size="sm"
                onClick={(event) => {
                  event.stopPropagation()
                  setRollbackEntry(entry)
                }}
                type="button"
              >
                <RotateCcw size={13} />
                {t('stackHistoryPage.actions.rollback', 'Rollback')}
              </Button>
            )}
          </div>
        )
      },
    },
  ]

  const expandedEntry = entries.find((entry) => entry.id === expandedId) ?? null

  return (
    <div>
      <Breadcrumb items={[{ label: t('sidebar.stackHistory', 'Stack History') }]} />

      {/* Page header */}
      <div className="mb-7 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <History size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('stackHistoryPage.title', 'Stack History')}
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              {t('stackHistoryPage.description', 'Stack change history and version management')}
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={() => setCompareOpen(true)}>
          <GitCompare size={15} />
          {t('stackHistoryPage.actions.compareVersions', 'Compare Versions')}
        </Button>
      </div>

      <div className="mb-4 max-w-[360px]">
        <label className="mb-1.5 block text-xs font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
          {t('stackHistoryPage.stackSelect', 'Stack')}
        </label>
        <NativeSelect
          value={stackId}
          onChange={(event) => navigate(`/stack/history/${event.target.value}`)}
          disabled={!stackId && visibleStacks.length === 0}
          className="w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-sm text-[var(--color-text-primary)]"
        >
          {currentRouteMissingFromOptions && routeStackId && (
            <option value={routeStackId}>{routeStackId}</option>
          )}
          {visibleStacks.map((stack) => (
            <option key={stack.id} value={stack.id}>
              {stack.name}
            </option>
          ))}
        </NativeSelect>
      </div>

      <DataTable
        columns={columns}
        data={entries}
        getRowKey={(row) => row.id}
        toolbar={
          <>
            <NativeSelect
              value={clusterFilter}
              onChange={(event) => setClusterFilter(event.target.value)}
              className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)]"
            >
              <option value="">{t('stackHistoryPage.filters.allClusters', 'All Clusters')}</option>
              {clusterOptions.map((clusterName) => (
                <option key={clusterName} value={clusterName}>{clusterName}</option>
              ))}
            </NativeSelect>
            <div className="relative ml-auto">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <input
                placeholder={t('stackHistoryPage.searchPlaceholder', 'Search by changed by / reason...')}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
              />
            </div>
          </>
        }
      />

      {expandedEntry && (
        <div className="mt-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(0,0,0,0.2)] px-5 py-4">
          <p className="mb-2.5 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            {t('stackHistoryPage.snapshot', 'Configuration Snapshot')} (v{expandedEntry.version})
          </p>
          <div className="flex flex-wrap gap-2.5">
            {Object.entries(expandedEntry.snapshot ?? {}).map(([k, v]) => (
              <div
                key={k}
                className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-[14px] py-2 font-mono text-xs"
              >
                <span className="text-[var(--color-text-secondary)]">{k}: </span>
                <span className="text-[#a5b4fc]">{String(v)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <Modal
        open={compareOpen}
        onClose={() => setCompareOpen(false)}
        title={`${t('stackHistoryPage.actions.compareVersions', 'Compare Versions')} (v${versionA} ↔ v${versionB})`}
        wide
      >
        <div className="flex flex-col gap-4">
          <div className="grid gap-2 md:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-xs text-[var(--color-text-secondary)]">
              {t('stackHistoryPage.compare.versionA', 'Version A')}
              <NativeSelect
                value={versionA}
                onChange={(event) => setVersionA(Number(event.target.value))}
                className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-sm text-[var(--color-text-primary)]"
              >
                {versionOptions.map((version) => (
                  <option key={`a-${version}`} value={version}>{`v${version}`}</option>
                ))}
              </NativeSelect>
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-[var(--color-text-secondary)]">
              {t('stackHistoryPage.compare.versionB', 'Version B')}
              <NativeSelect
                value={versionB}
                onChange={(event) => setVersionB(Number(event.target.value))}
                className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-sm text-[var(--color-text-primary)]"
              >
                {versionOptions.map((version) => (
                  <option key={`b-${version}`} value={version}>{`v${version}`}</option>
                ))}
              </NativeSelect>
            </label>
          </div>

          {compareEntryA && compareEntryB && (
            <VersionDiff
              versionA={versionA}
              versionB={versionB}
              configA={compareEntryA.snapshot}
              configB={compareEntryB.snapshot}
              diff={diff}
            />
          )}
        </div>
      </Modal>

       {/* Rollback confirm */}
       <Modal
         open={!!rollbackEntry}
         onClose={() => {
           setRollbackEntry(null)
           setPreservePVC(true)
           setDeleteConfirmText('')
         }}
         title={`${t('stackHistoryPage.actions.rollback', 'Rollback')} v${rollbackEntry?.version ?? ''}`}
         footer={
           <>
             <Button
               variant="outline"
               size="md"
               onClick={() => {
                 setRollbackEntry(null)
                 setPreservePVC(true)
                 setDeleteConfirmText('')
               }}
               disabled={rollbackMutation.isPending}
             >
               {t('common.cancel', 'Cancel')}
             </Button>
             <Button
               variant="danger"
               size="md"
               onClick={handleRollbackConfirm}
               disabled={!preservePVC && deleteConfirmText !== 'DELETE' || rollbackMutation.isPending}
               loading={rollbackMutation.isPending}
             >
               {t('stackHistoryPage.actions.rollback', 'Rollback')}
             </Button>
           </>
         }
       >
         <div className="flex flex-col gap-4">
           <div className="flex items-start gap-3">
             <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-[rgba(239,68,68,0.15)] text-[#f87171]">
               <AlertTriangle size={20} />
             </div>
             <p className="m-0 text-sm leading-[1.6] text-[var(--color-text-secondary)]">
               {t('stackHistoryPage.rollback.description', 'Rollback this stack to the selected version. Current configuration will change and this action cannot be undone.')}
             </p>
           </div>

           <div className="mt-4">
             <p className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">{t('stackHistoryPage.rollback.dataRetention', 'Data Retention Options')}</p>
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
                 <span>{t('stackHistoryPage.rollback.safeMode', 'Safe Mode — Preserve data')}</span>
               </label>
               <label className="flex items-center gap-2 text-sm cursor-pointer">
                 <input
                   type="radio"
                   name="pvcMode"
                   value="clean"
                   checked={!preservePVC}
                   onChange={() => setPreservePVC(false)}
                 />
                 <span>{t('stackHistoryPage.rollback.cleanMode', 'Clean Mode — Delete volumes')}</span>
               </label>
             </div>
             {!preservePVC && (
               <div className="mt-3">
                 <div className="rounded-lg border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-sm text-[#ef4444]">
                   {t('stackHistoryPage.rollback.cleanWarning', 'This action permanently deletes Persistent Volumes.')}
                 </div>
                 <input
                   type="text"
                   placeholder={t('stackHistoryPage.rollback.confirmDeletePlaceholder', 'Type "DELETE" to confirm')}
                   value={deleteConfirmText}
                   onChange={(e) => setDeleteConfirmText(e.target.value)}
                   className="mt-2 w-full rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] outline-none"
                 />
               </div>
             )}
           </div>
         </div>
       </Modal>
    </div>
  )
}

function buildSnapshotDiff(
  snapshotA: Record<string, unknown>,
  snapshotB: Record<string, unknown>
): StackVersionDiff {
  const flatA = flattenObject(snapshotA)
  const flatB = flattenObject(snapshotB)
  const keys = new Set([...Object.keys(flatA), ...Object.keys(flatB)])

  const added: Record<string, unknown> = {}
  const removed: Record<string, unknown> = {}
  const changed: Record<string, [unknown, unknown]> = {}

  keys.forEach((key) => {
    const hasA = Object.hasOwn(flatA, key)
    const hasB = Object.hasOwn(flatB, key)

    if (!hasA && hasB) {
      added[key] = flatB[key]
      return
    }
    if (hasA && !hasB) {
      removed[key] = flatA[key]
      return
    }

    if (flatA[key] !== flatB[key]) {
      changed[key] = [flatA[key], flatB[key]]
    }
  })

  return { added, removed, changed }
}

function flattenObject(source: Record<string, unknown>, prefix = ''): Record<string, unknown> {
  const out: Record<string, unknown> = {}

  Object.entries(source).forEach(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key
    if (isPlainObject(value)) {
      Object.assign(out, flattenObject(value, path))
      return
    }
    out[path] = value
  })

  return out
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
