import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router-dom'
import { ChevronDown, ChevronUp, GitCompare, History, RotateCcw, Terminal, TriangleAlert } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import type { ColumnDef } from '@tanstack/react-table'
import { useStacks, useStackHistory, useRollbackStack, useStackVersionDiff } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Select } from '../../../components/ui/select'
import { Modal } from '../../../components/ui/modal'
import { DataTable } from '../../../components/shared/data-table'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import type { StackHistoryEntry, StackVersionDiff } from '../api/stack-api'
import { VersionDiff } from '../components/version-diff'
import { formatDateTime, resolveLocale } from '../../../lib/locale'
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from '../../../components/ui/search-input'
import { TextInput } from '../../../components/ui/text-input'


export function StackHistoryPage() {
   const { t, i18n } = useTranslation()
   const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
   const { data: stacksData } = useStacks({ include_deleted: true })
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
   const selectedStack = stacks.find((stack) => stack.id === stackId) ?? null
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
  // 이력은 최신이 위다. API 는 오름차순으로 주므로 여기서 뒤집는다.
  const allEntries = (Array.isArray(historyData) ? [...historyData] : []).sort(
    (a, b) => b.version - a.version,
  )
  const entries = search.trim()
    ? allEntries.filter(
        (e) =>
          e.changedBy.toLowerCase().includes(search.toLowerCase()) ||
          e.reason.toLowerCase().includes(search.toLowerCase())
      )
    : allEntries

  // 현재 버전은 가장 높은 번호다. 목록의 첫 행으로 보면 안 된다 — 정렬이나
  // 검색 결과에 따라 첫 행이 달라지고, API 가 오름차순이던 시절에는 v1 에
  // "현재" 가 붙고 정작 현재인 v2 가 롤백 대상으로 나왔다.
  const currentVersion = allEntries.length > 0 ? allEntries[0].version : null
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
            {isExpanded ? <ChevronUp {...iconProps('xs')} /> : <ChevronDown {...iconProps('xs')} />}
          </Button>
        )
      },
    },
    // 스택 이름과 클러스터는 열로 두지 않는다. 이 화면은 위에서 고른 스택 하나의
    // 이력만 보여주므로 행마다 같은 값이 반복되고, 그 두 열이 252px 를 먹어
    // "작업" 열이 칸 밖으로 밀려났다. 값은 선택기 옆 맥락 줄에 한 번만 적는다.
    {
      accessorKey: 'version',
      header: t('stackHistoryPage.table.version', 'Version'),
      cell: ({ row }) => {
        const entry = row.original
        const isCurrent = entry.version === currentVersion
        return (
          <span className="inline-flex items-center gap-1.5 font-mono text-[13px] font-semibold text-[var(--color-primary)]">
            v{entry.version}
            {isCurrent && (
              <span className="rounded bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] px-1.5 py-[1px] font-inherit text-[10px] text-[var(--color-success)]">
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
          // 지금 돌고 있는 버전으로는 되돌릴 것이 없다.
          const isCurrent = entry.version === currentVersion
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
                <Terminal {...iconProps('xs')} />
                {t('stackHistoryPage.actions.log', 'Log')}
              </Button>
              {!isCurrent && (
                <Button
                variant="danger"
                size="sm"
                onClick={(event) => {
                  event.stopPropagation()
                  setRollbackEntry(entry)
                }}
                type="button"
              >
                <RotateCcw {...iconProps('xs')} />
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
      <PageHeader
        breadcrumb={[{ label: t('sidebar.stackHistory', 'Stack History') }]}
        icon={<History {...iconProps('sm')} />}
        tone="primary"
        title={t('stackHistoryPage.title', 'Stack History')}
        subtitle={t('stackHistoryPage.description', 'Stack change history and version management')}
        actions={
          <Button variant="primary" size="md" onClick={() => setCompareOpen(true)}>
            <GitCompare {...iconProps('sm')} />
            {t('stackHistoryPage.actions.compareVersions', 'Compare Versions')}
          </Button>
        }
      />

      <div className="mb-4 flex flex-wrap items-end gap-x-6 gap-y-3">
        <div className="w-full max-w-[360px]">
          <label className="mb-1.5 block text-xs font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
            {t('stackHistoryPage.stackSelect', 'Stack')}
          </label>
          <Select
            value={stackId}
            onChange={(event) => navigate(`/stack/history/${event.target.value}`)}
            disabled={!stackId && visibleStacks.length === 0}
            className="w-full"
          >
            {currentRouteMissingFromOptions && routeStackId && (
              <option value={routeStackId}>{routeStackId}</option>
            )}
            {visibleStacks.map((stack) => (
              <option key={stack.id} value={stack.id}>
                {stack.name}
              </option>
            ))}
          </Select>
        </div>

        {/* 행마다 반복하던 클러스터를 여기 한 번만 적는다. 스택 이름은 위
            선택기가 이미 보여주므로 다시 쓰지 않는다. */}
        {selectedStack && (
          <div className="pb-2 text-[13px] text-[var(--color-text-secondary)]">
            {t('stackHistoryPage.table.cluster', 'Cluster')}{' '}
            <span className="font-semibold text-[var(--color-text-primary)]">
              {selectedStack.clusterName || '-'}
            </span>
          </div>
        )}
      </div>

      {/* 스냅샷은 표 아래가 아니라 오른쪽에 붙는다. 아래로 펼치면 행을 고를
          때마다 표가 밀려 내려가 방금 고른 행을 잃는다 (DESIGN.md §Layout). */}
      <ListDetailPanel
        // 스냅샷은 pipeline.cd_tool.name 같은 경로와 값을 함께 보여준다.
        // 340px 로는 거의 모든 줄이 접혀 읽기 어려웠다.
        detailWidth={420}
        emptyDetailMessage={t('stackHistoryPage.selectEntry', 'Select a change to view its configuration snapshot.')}
        detailContent={
          expandedEntry && (
            <div className="p-3">
              <p className="mb-2.5 mt-0 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                {t('stackHistoryPage.snapshot', 'Configuration Snapshot')} (v{expandedEntry.version})
              </p>
              {/* 스냅샷은 중첩 객체다. 최상위 키만 찍으면 값이 전부
                  [object Object] 로 나온다 — 아래 비교(diff)가 쓰는 것과 같은
                  평탄화를 써서 잎 값까지 보여준다. */}
              <div className="flex flex-col gap-1.5">
                {Object.entries(flattenObject(expandedEntry.snapshot ?? {})).map(([path, value]) => (
                  <div
                    key={path}
                    className="rounded-[var(--radius-sm)] border border-[var(--color-border-default)] bg-[var(--color-surface-sunken)] px-2.5 py-1.5 font-mono text-xs"
                  >
                    <div className="break-all text-[var(--color-text-secondary)]">{path}</div>
                    <div className="break-all text-[var(--color-primary)]">{formatSnapshotValue(value)}</div>
                  </div>
                ))}
              </div>
            </div>
          )
        }
        listContent={
          <DataTable
            flush
            columns={columns}
            data={entries}
            getRowKey={(row) => row.id}
            onRowClick={(row) => setExpandedId(row.id)}
            toolbar={
              <>
                <Select
                  value={clusterFilter}
                  onChange={(event) => setClusterFilter(event.target.value)}
                >
                  <option value="">{t('stackHistoryPage.filters.allClusters', 'All Clusters')}</option>
                  {clusterOptions.map((clusterName) => (
                    <option key={clusterName} value={clusterName}>{clusterName}</option>
                  ))}
                </Select>
                <SearchInput
                  wrapperClassName="ml-auto w-[220px]"
                  placeholder={t('stackHistoryPage.searchPlaceholder', 'Search by changed by / reason...')}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </>
        }
      />
        }
      />

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
              <Select
                value={versionA}
                onChange={(event) => setVersionA(Number(event.target.value))}
              >
                {versionOptions.map((version) => (
                  <option key={`a-${version}`} value={version}>{`v${version}`}</option>
                ))}
              </Select>
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-[var(--color-text-secondary)]">
              {t('stackHistoryPage.compare.versionB', 'Version B')}
              <Select
                value={versionB}
                onChange={(event) => setVersionB(Number(event.target.value))}
              >
                {versionOptions.map((version) => (
                  <option key={`b-${version}`} value={version}>{`v${version}`}</option>
                ))}
              </Select>
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
             <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]">
               <TriangleAlert {...iconProps('md')} />
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
                 <div className="rounded-lg border border-[color-mix(in_srgb,_var(--color-error)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_8%,_transparent)] px-3 py-2 text-sm text-[var(--color-error)]">
                   {t('stackHistoryPage.rollback.cleanWarning', 'This action permanently deletes Persistent Volumes.')}
                 </div>
                 <TextInput
                   type="text"
                   placeholder={t('stackHistoryPage.rollback.confirmDeletePlaceholder', 'Type "DELETE" to confirm')}
                   value={deleteConfirmText}
                   onChange={(e) => setDeleteConfirmText(e.target.value)}
                   className="mt-2 w-full"
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

/**
 * 잎 값을 화면에 쓸 문자열로 바꾼다.
 *
 * 빈 값을 빈 문자열로 두면 줄이 통째로 사라진 것처럼 보인다 — "설정은 있는데
 * 값이 비었다" 와 "그 설정이 없다" 는 다르므로 눈에 보이게 적는다.
 */
function formatSnapshotValue(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (Array.isArray(value)) return value.length > 0 ? value.map(String).join(', ') : '[]'
  if (value === '') return '""'
  return String(value)
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
