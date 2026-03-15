import { useState } from 'react'
import { History, GitCompare, RotateCcw } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useStackHistory, useRollbackStack, useStackVersionDiff } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
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

export function StackHistoryPage() {
  const stackId = 's1'
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [compareOpen, setCompareOpen] = useState(false)
  const [versionA, setVersionA] = useState(4)
  const [versionB, setVersionB] = useState(5)
  const [rollbackEntry, setRollbackEntry] = useState<StackHistoryEntry | null>(null)
  const { data: historyData } = useStackHistory(stackId)
  const entries = Array.isArray(historyData) ? historyData : []
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
      { stackId, version: rollbackEntry.version },
      { onSuccess: () => setRollbackEntry(null) }
    )
  }

  const columns: ColumnDef<StackHistoryEntry, unknown>[] = [
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
        onRowClick={(row) => setExpandedId((prev) => (prev === row.id ? null : row.id))}
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
      <ConfirmDialog
        open={!!rollbackEntry}
        onClose={() => setRollbackEntry(null)}
        onConfirm={handleRollbackConfirm}
        title={`v${rollbackEntry?.version ?? ''}로 롤백`}
        description={`스택을 v${rollbackEntry?.version ?? ''}으로 롤백합니다. 현재 설정이 변경되며 이 작업은 되돌릴 수 없습니다.`}
        confirmLabel="Rollback"
        confirmText={`v${rollbackEntry?.version ?? ''}`}
        loading={rollbackMutation.isPending}
      />
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
