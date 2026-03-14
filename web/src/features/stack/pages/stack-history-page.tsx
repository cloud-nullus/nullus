import { useState } from 'react'
import { History, GitCompare, RotateCcw, ChevronDown, ChevronRight } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
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

  const thStyle: React.CSSProperties = {
    padding: '10px 14px',
    textAlign: 'left',
    fontSize: '11px',
    fontWeight: 600,
    color: 'var(--color-text-secondary)',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    whiteSpace: 'nowrap',
  }

  const tdStyle: React.CSSProperties = {
    padding: '12px 14px',
    fontSize: '14px',
    color: 'var(--color-text-primary)',
    borderTop: '1px solid var(--color-border-default)',
  }

  const handleRollbackConfirm = () => {
    if (!rollbackEntry) return
    setRollbackLoading(true)
    setTimeout(() => {
      setRollbackLoading(false)
      setRollbackEntry(null)
    }, 1500)
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '28px' }}>
        <div
          style={{
            width: 'var(--icon-size)',
            height: 'var(--icon-size)',
            background: 'rgba(99,102,241,0.15)',
            borderRadius: 'var(--icon-radius)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#818cf8',
          }}
        >
          <History size={18} />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
            Stack History
          </h1>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
            스택 변경 이력 및 버전 관리
          </p>
        </div>
      </div>

      {/* Table */}
      <div
        style={{
          background: 'var(--color-surface-card)',
          border: '1px solid var(--color-border-default)',
          borderRadius: 'var(--card-radius)',
          overflow: 'hidden',
        }}
      >
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: 'rgba(255,255,255,0.02)' }}>
              <th style={{ ...thStyle, width: '32px' }} />
              {['버전', '변경자', '변경 시간', '변경 사유'].map((h) => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
              <th style={{ ...thStyle, cursor: 'default' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {MOCK_HISTORY.map((entry, idx) => {
              const isExpanded = expandedId === entry.id
              const hasPrev = idx < MOCK_HISTORY.length - 1
              return (
                <>
                  <tr
                    key={entry.id}
                    style={{ transition: 'background var(--transition-fast)', cursor: 'pointer' }}
                    onClick={() => setExpandedId(isExpanded ? null : entry.id)}
                    onMouseEnter={(e) => {
                      ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)'
                    }}
                    onMouseLeave={(e) => {
                      ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent'
                    }}
                  >
                    <td style={{ ...tdStyle, padding: '12px 14px 12px 16px', color: 'var(--color-text-secondary)' }}>
                      {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                    </td>
                    <td style={tdStyle}>
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '6px',
                          fontFamily: 'Fira Code, monospace',
                          fontSize: '13px',
                          fontWeight: 600,
                          color: '#a5b4fc',
                        }}
                      >
                        v{entry.version}
                        {idx === 0 && (
                          <span
                            style={{
                              fontSize: '10px',
                              background: 'rgba(34,197,94,0.15)',
                              color: '#22c55e',
                              padding: '1px 6px',
                              borderRadius: '4px',
                              fontFamily: 'inherit',
                            }}
                          >
                            CURRENT
                          </span>
                        )}
                      </span>
                    </td>
                    <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px' }}>
                      {entry.changedBy}
                    </td>
                    <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px' }}>
                      {formatDate(entry.changedAt)}
                    </td>
                    <td style={tdStyle}>{entry.reason}</td>
                    <td style={tdStyle} onClick={(e) => e.stopPropagation()}>
                      <div style={{ display: 'flex', gap: '6px' }}>
                        {hasPrev && (
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => setDiffEntry(entry)}
                          >
                            <GitCompare size={13} />
                            Diff
                          </Button>
                        )}
                        {idx !== 0 && (
                          <Button
                            variant="danger"
                            size="sm"
                            onClick={() => setRollbackEntry(entry)}
                          >
                            <RotateCcw size={13} />
                            Rollback
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                  {isExpanded && (
                    <tr key={`${entry.id}-detail`}>
                      <td colSpan={6} style={{ borderTop: '1px solid var(--color-border-default)', padding: 0 }}>
                        <div
                          style={{
                            background: 'rgba(0,0,0,0.2)',
                            padding: '16px 20px',
                          }}
                        >
                          <p style={{ margin: '0 0 10px', fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                            설정 스냅샷 (v{entry.version})
                          </p>
                          <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                            {Object.entries(entry.snapshot).map(([k, v]) => (
                              <div
                                key={k}
                                style={{
                                  background: 'rgba(255,255,255,0.04)',
                                  border: '1px solid var(--color-border-default)',
                                  borderRadius: '8px',
                                  padding: '8px 14px',
                                  fontFamily: 'Fira Code, monospace',
                                  fontSize: '12px',
                                }}
                              >
                                <span style={{ color: 'var(--color-text-secondary)' }}>{k}: </span>
                                <span style={{ color: '#a5b4fc' }}>{String(v)}</span>
                              </div>
                            ))}
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Diff Modal */}
      <Modal
        open={!!diffEntry}
        onClose={() => setDiffEntry(null)}
        title={diffEntry ? `Diff: v${diffEntry.version - 1} → v${diffEntry.version}` : ''}
        wide
      >
        {diffEntry && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            {MOCK_DIFF.changed.map((item) => (
              <div key={item.key} style={{ fontFamily: 'Fira Code, monospace', fontSize: '13px' }}>
                <span style={{ color: 'var(--color-text-secondary)', marginRight: '8px' }}>{item.key}:</span>
                <span
                  style={{
                    background: 'rgba(239,68,68,0.12)',
                    color: '#f87171',
                    padding: '2px 8px',
                    borderRadius: '4px',
                    textDecoration: 'line-through',
                    marginRight: '6px',
                  }}
                >
                  - {item.from}
                </span>
                <span
                  style={{
                    background: 'rgba(34,197,94,0.12)',
                    color: '#4ade80',
                    padding: '2px 8px',
                    borderRadius: '4px',
                  }}
                >
                  + {item.to}
                </span>
              </div>
            ))}
            {MOCK_DIFF.added.map((item) => (
              <div key={item.key} style={{ fontFamily: 'Fira Code, monospace', fontSize: '13px' }}>
                <span
                  style={{
                    background: 'rgba(34,197,94,0.12)',
                    color: '#4ade80',
                    padding: '2px 8px',
                    borderRadius: '4px',
                  }}
                >
                  + {item.key}: {item.value}
                </span>
              </div>
            ))}
            {MOCK_DIFF.removed.map((item) => (
              <div key={item.key} style={{ fontFamily: 'Fira Code, monospace', fontSize: '13px' }}>
                <span
                  style={{
                    background: 'rgba(239,68,68,0.12)',
                    color: '#f87171',
                    padding: '2px 8px',
                    borderRadius: '4px',
                    textDecoration: 'line-through',
                  }}
                >
                  - {item.key}: {item.value}
                </span>
              </div>
            ))}
            {MOCK_DIFF.added.length === 0 && MOCK_DIFF.removed.length === 0 && MOCK_DIFF.changed.length === 0 && (
              <p style={{ color: 'var(--color-text-secondary)', fontSize: '14px', margin: 0 }}>변경 사항이 없습니다.</p>
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
