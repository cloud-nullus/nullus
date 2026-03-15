import { useState } from 'react'
import { History, GitCompare, RotateCcw } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { DataTable } from '../../../components/shared/data-table'
import type { StackHistoryEntry, StackVersionDiff } from '../api/stack-api'

const MOCK_HISTORY: StackHistoryEntry[] = [
  {
    id: 'h1',
    stackId: 's1',
    version: 5,
    changedBy: 'alice@nullus.io',
    changedAt: '2026-03-14T09:30:00Z',
    reason: 'ArgoCD 버전 업그레이드 (2.9 → 2.10)',
    snapshot: { argocd: '2.10.0', gitlab: '16.9.0', harbor: '2.10.0' },
  },
  {
    id: 'h2',
    stackId: 's1',
    version: 4,
    changedBy: 'bob@nullus.io',
    changedAt: '2026-03-12T14:00:00Z',
    reason: 'Harbor 스토리지 용량 증설',
    snapshot: { argocd: '2.9.0', gitlab: '16.9.0', harbor: '2.10.0' },
  },
  {
    id: 'h3',
    stackId: 's1',
    version: 3,
    changedBy: 'alice@nullus.io',
    changedAt: '2026-03-10T11:20:00Z',
    reason: 'GitLab 보안 패치 적용',
    snapshot: { argocd: '2.9.0', gitlab: '16.9.0', harbor: '2.9.0' },
  },
  {
    id: 'h4',
    stackId: 's1',
    version: 2,
    changedBy: 'carol@nullus.io',
    changedAt: '2026-03-05T08:00:00Z',
    reason: '리소스 할당 조정',
    snapshot: { argocd: '2.9.0', gitlab: '16.8.0', harbor: '2.9.0' },
  },
  {
    id: 'h5',
    stackId: 's1',
    version: 1,
    changedBy: 'alice@nullus.io',
    changedAt: '2026-03-01T10:00:00Z',
    reason: '초기 스택 설치',
    snapshot: { argocd: '2.8.0', gitlab: '16.8.0', harbor: '2.9.0' },
  },
]

const MOCK_DIFF: StackVersionDiff = {
  fromVersion: 4,
  toVersion: 5,
  added: [{ key: 'argocd.notifications', value: 'enabled' }],
  removed: [],
  changed: [{ key: 'argocd.version', from: '2.9.0', to: '2.10.0' }],
}

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
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [diffEntry, setDiffEntry] = useState<StackHistoryEntry | null>(null)
  const [rollbackEntry, setRollbackEntry] = useState<StackHistoryEntry | null>(null)
  const [rollbackLoading, setRollbackLoading] = useState(false)

  const handleRollbackConfirm = () => {
    if (!rollbackEntry) return
    setRollbackLoading(true)
    setTimeout(() => {
      setRollbackLoading(false)
      setRollbackEntry(null)
    }, 1500)
  }

  const columns: ColumnDef<StackHistoryEntry, unknown>[] = [
    {
      accessorKey: 'version',
      header: '버전',
      cell: ({ row }) => {
        const entry = row.original
        const isCurrent = entry.id === MOCK_HISTORY[0]?.id
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
        const index = MOCK_HISTORY.findIndex((item) => item.id === entry.id)
        const hasPrev = index < MOCK_HISTORY.length - 1
        return (
          <div className="flex gap-1.5">
            {hasPrev && (
              <Button
                variant="secondary"
                size="sm"
                onClick={(event) => {
                  event.stopPropagation()
                  setDiffEntry(entry)
                }}
                type="button"
              >
                <GitCompare size={13} />
                Diff
              </Button>
            )}
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

  const expandedEntry = MOCK_HISTORY.find((entry) => entry.id === expandedId) ?? null

  return (
    <div>
      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
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

      <DataTable
        columns={columns}
        data={MOCK_HISTORY}
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

      {/* Diff Modal */}
      <Modal
        open={!!diffEntry}
        onClose={() => setDiffEntry(null)}
        title={diffEntry ? `Diff: v${diffEntry.version - 1} → v${diffEntry.version}` : ''}
        wide
      >
        {diffEntry && (
          <div className="flex flex-col gap-3">
            {MOCK_DIFF.changed.map((item) => (
              <div key={item.key} className="font-mono text-[13px]">
                <span className="mr-2 text-[var(--color-text-secondary)]">{item.key}:</span>
                <span
                  className="mr-1.5 rounded bg-[rgba(239,68,68,0.12)] px-2 py-0.5 text-[#f87171] line-through"
                >
                  - {item.from}
                </span>
                <span
                  className="rounded bg-[rgba(34,197,94,0.12)] px-2 py-0.5 text-[#4ade80]"
                >
                  + {item.to}
                </span>
              </div>
            ))}
            {MOCK_DIFF.added.map((item) => (
              <div key={item.key} className="font-mono text-[13px]">
                <span
                  className="rounded bg-[rgba(34,197,94,0.12)] px-2 py-0.5 text-[#4ade80]"
                >
                  + {item.key}: {item.value}
                </span>
              </div>
            ))}
            {MOCK_DIFF.removed.map((item) => (
              <div key={item.key} className="font-mono text-[13px]">
                <span
                  className="rounded bg-[rgba(239,68,68,0.12)] px-2 py-0.5 text-[#f87171] line-through"
                >
                  - {item.key}: {item.value}
                </span>
              </div>
            ))}
            {MOCK_DIFF.added.length === 0 && MOCK_DIFF.removed.length === 0 && MOCK_DIFF.changed.length === 0 && (
              <p className="m-0 text-sm text-[var(--color-text-secondary)]">변경 사항이 없습니다.</p>
            )}
          </div>
        )}
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
        loading={rollbackLoading}
      />
    </div>
  )
}
