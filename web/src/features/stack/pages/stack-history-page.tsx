import { useState, useEffect } from 'react'
import { ChevronDown, ChevronUp, History, GitCompare, RotateCcw, Search, AlertTriangle } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import type { ColumnDef } from '@tanstack/react-table'
import { useStacks, useStackHistory, useRollbackStack, useStackVersionDiff } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { DataTable } from '../../../components/shared/data-table'
import type { StackHistoryEntry, StackVersionDiff } from '../api/stack-api'
import { VersionDiff } from '../components/version-diff'

function formatDate(iso: string) {
  return new Date(iso).toLocaleString('ko-KR', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const MOCK_STACKS_FOR_HISTORY = [
  { id: 'production-stack', name: 'production-stack' },
]

const MOCK_STACK_HISTORY: StackHistoryEntry[] = [
  { id: 'h1', stackId: 'production-stack', version: 5, changedBy: 'kim.dev', changedAt: '2026-03-03T14:28:00Z', reason: 'Grafana 버전 업그레이드 (v10.2 → v10.3)', snapshot: { gitlab: 'v16.7', argocd: 'v2.9.3', prometheus: 'v2.49', grafana: 'v10.3' } },
  { id: 'h2', stackId: 'production-stack', version: 4, changedBy: 'lee.devops', changedAt: '2026-02-20T09:15:00Z', reason: 'ArgoCD 보안 패치 적용', snapshot: { gitlab: 'v16.7', argocd: 'v2.9.3', prometheus: 'v2.49', grafana: 'v10.2' } },
  { id: 'h3', stackId: 'production-stack', version: 3, changedBy: 'park.dev', changedAt: '2026-02-10T11:00:00Z', reason: 'Prometheus 설정 변경 (retention 30d)', snapshot: { gitlab: 'v16.7', argocd: 'v2.9.2', prometheus: 'v2.49', grafana: 'v10.2' } },
  { id: 'h4', stackId: 'production-stack', version: 2, changedBy: 'choi.devops', changedAt: '2026-01-25T16:30:00Z', reason: 'GitLab Runner 리소스 증설', snapshot: { gitlab: 'v16.7', argocd: 'v2.9.2', prometheus: 'v2.47', grafana: 'v10.2' } },
  { id: 'h5', stackId: 'production-stack', version: 1, changedBy: 'admin', changedAt: '2026-01-10T00:00:00Z', reason: '초기 스택 배포', snapshot: { gitlab: 'v16.7', argocd: 'v2.9.2', prometheus: 'v2.47', grafana: 'v10.1' } },
]

export function StackHistoryPage() {
   const { data: stacksData } = useStacks()
   const stacks = stacksData?.items ?? MOCK_STACKS_FOR_HISTORY
   const [stackId, setStackId] = useState(stacks[0]?.id ?? '')
   const [expandedId, setExpandedId] = useState<string | null>(null)
   const [search, setSearch] = useState('')
   const [compareOpen, setCompareOpen] = useState(false)
   const [versionA, setVersionA] = useState(4)
   const [versionB, setVersionB] = useState(5)
   const [rollbackEntry, setRollbackEntry] = useState<StackHistoryEntry | null>(null)
   const [preservePVC, setPreservePVC] = useState(true)
   const [deleteConfirmText, setDeleteConfirmText] = useState('')

  useEffect(() => {
    if (stacks.length > 0 && !stackId) {
      setStackId(stacks[0].id)
    }
  }, [stacks, stackId])

  const { data: historyData } = useStackHistory(stackId)
  const allEntries = Array.isArray(historyData) && historyData.length > 0 ? historyData : MOCK_STACK_HISTORY
  const entries = search.trim()
    ? allEntries.filter(
        (e) =>
          e.changedBy.toLowerCase().includes(search.toLowerCase()) ||
          e.reason.toLowerCase().includes(search.toLowerCase())
      )
    : allEntries
  const rollbackMutation = useRollbackStack()

  const versionOptions = entries.map((entry) => entry.version).sort((a, b) => b - a)
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
      header: '스택 이름',
      enableSorting: false,
      cell: ({ row }) => {
        const name = stacks.find((s) => s.id === row.original.stackId)?.name ?? row.original.stackId
        return <span className="font-semibold text-[var(--color-text-primary)]">{name}</span>
      },
    },
    {
      accessorKey: 'version',
      header: '버전',
      cell: ({ row }) => {
        const entry = row.original
        const isCurrent = entry.id === entries[0]?.id
        return (
          <span className="inline-flex items-center gap-1.5 font-mono text-[13px] font-semibold text-[#a5b4fc]">
            v{entry.version}
            {isCurrent && (
              <span className="rounded bg-[rgba(34,197,94,0.15)] px-1.5 py-[1px] font-inherit text-[10px] text-[#22c55e]">
                CURRENT
              </span>
            )}
          </span>
        )
      },
    },
    {
      accessorKey: 'changedBy',
      header: '변경자',
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{row.original.changedBy}</span>,
    },
    {
      accessorKey: 'changedAt',
      header: '변경 시간',
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.changedAt)}</span>,
    },
    {
      accessorKey: 'reason',
      header: '변경 사유',
    },
      {
        id: 'actions',
        header: 'Actions',
        enableSorting: false,
        cell: ({ row }) => {
          const entry = row.original
          const index = entries.findIndex((item) => item.id === entry.id)
          return (
            <div className="flex gap-1.5">
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
                Rollback
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
      <Breadcrumb items={[{ label: 'Stack History' }]} />

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
              Stack History
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              스택 변경 이력 및 버전 관리
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={() => setCompareOpen(true)}>
          <GitCompare size={15} />
          Compare Versions
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={entries}
        getRowKey={(row) => row.id}
        toolbar={
          <div className="relative ml-auto">
            <Search
              size={13}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
            />
            <input
              placeholder="변경자 / 변경 사유 검색..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
            />
          </div>
        }
      />

      {expandedEntry && (
        <div className="mt-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(0,0,0,0.2)] px-5 py-4">
          <p className="mb-2.5 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            설정 스냅샷 (v{expandedEntry.version})
          </p>
          <div className="flex flex-wrap gap-2.5">
            {Object.entries(expandedEntry.snapshot).map(([k, v]) => (
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
        title={`Compare Versions (v${versionA} ↔ v${versionB})`}
        wide
      >
        <div className="flex flex-col gap-4">
          <div className="grid gap-2 md:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-xs text-[var(--color-text-secondary)]">
              Version A
              <select
                value={versionA}
                onChange={(event) => setVersionA(Number(event.target.value))}
                className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-sm text-[var(--color-text-primary)]"
              >
                {versionOptions.map((version) => (
                  <option key={`a-${version}`} value={version}>{`v${version}`}</option>
                ))}
              </select>
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-[var(--color-text-secondary)]">
              Version B
              <select
                value={versionB}
                onChange={(event) => setVersionB(Number(event.target.value))}
                className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-sm text-[var(--color-text-primary)]"
              >
                {versionOptions.map((version) => (
                  <option key={`b-${version}`} value={version}>{`v${version}`}</option>
                ))}
              </select>
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
         title={`v${rollbackEntry?.version ?? ''}로 롤백`}
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
               Cancel
             </Button>
             <Button
               variant="danger"
               size="md"
               onClick={handleRollbackConfirm}
               disabled={!preservePVC && deleteConfirmText !== 'DELETE' || rollbackMutation.isPending}
               loading={rollbackMutation.isPending}
             >
               Rollback
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
               스택을 v{rollbackEntry?.version ?? ''}으로 롤백합니다. 현재 설정이 변경되며 이 작업은 되돌릴 수 없습니다.
             </p>
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
