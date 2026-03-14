import { useState } from 'react'
import { History } from 'lucide-react'
import { useDeployments } from '../api/cicd-api'
import type { Deployment, PipelineStatus } from '../api/cicd-api'

const MOCK_DEPLOYMENTS: Deployment[] = [
  {
    id: 'd1',
    pipelineId: 'p1',
    pipelineName: 'api-server-pipeline',
    version: 'v1.2.3',
    status: 'success',
    triggeredBy: 'alice@nullus.io',
    startedAt: '2026-03-13T10:00:00Z',
    completedAt: '2026-03-13T10:05:00Z',
  },
  {
    id: 'd2',
    pipelineId: 'p2',
    pipelineName: 'frontend-pipeline',
    version: 'v2.0.1',
    status: 'running',
    triggeredBy: 'bob@nullus.io',
    startedAt: '2026-03-14T08:30:00Z',
    completedAt: null,
  },
  {
    id: 'd3',
    pipelineId: 'p3',
    pipelineName: 'data-batch-pipeline',
    version: 'v0.9.0',
    status: 'failed',
    triggeredBy: 'carol@nullus.io',
    startedAt: '2026-03-12T14:00:00Z',
    completedAt: '2026-03-12T14:02:00Z',
  },
  {
    id: 'd4',
    pipelineId: 'p1',
    pipelineName: 'api-server-pipeline',
    version: 'v1.2.2',
    status: 'success',
    triggeredBy: 'alice@nullus.io',
    startedAt: '2026-03-10T09:00:00Z',
    completedAt: '2026-03-10T09:04:00Z',
  },
]

const STATUS_STYLES: Record<PipelineStatus, { bg: string; color: string; label: string }> = {
  running: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa', label: 'Running' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Success' },
  failed: { bg: 'rgba(239,68,68,0.15)', color: '#ef4444', label: 'Failed' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Pending' },
  cancelled: { bg: 'rgba(100,116,139,0.15)', color: '#64748b', label: 'Cancelled' },
}

function formatDate(iso: string | null) {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function CicdHistoryPage() {
  const [statusFilter, setStatusFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')

  const { data: apiData } = useDeployments({ status: statusFilter as PipelineStatus || undefined })
  const deployments = apiData?.items ?? MOCK_DEPLOYMENTS

  const filtered = deployments.filter((d) => {
    const matchesStatus = !statusFilter || d.status === statusFilter
    const matchesType = !typeFilter || d.pipelineName.includes(typeFilter)
    return matchesStatus && matchesType
  })

  const selectStyle: React.CSSProperties = {
    background: 'rgba(255,255,255,0.04)',
    border: '1px solid var(--color-border-default)',
    borderRadius: '8px',
    padding: '9px 12px',
    fontSize: '14px',
    color: 'var(--color-text-primary)',
    cursor: 'pointer',
  }

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

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '28px' }}>
        <div
          style={{
            width: 'var(--icon-size)',
            height: 'var(--icon-size)',
            background: 'rgba(245,158,11,0.15)',
            borderRadius: 'var(--icon-radius)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fbbf24',
          }}
        >
          <History size={18} />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
            Deployment History
          </h1>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
            CI/CD 배포 이력
          </p>
        </div>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '10px', marginBottom: '16px', flexWrap: 'wrap' }}>
        <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} style={selectStyle}>
          <option value="">All Types</option>
          <option value="api">API</option>
          <option value="frontend">Frontend</option>
          <option value="batch">Batch</option>
        </select>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} style={selectStyle}>
          <option value="">All Status</option>
          <option value="success">Success</option>
          <option value="running">Running</option>
          <option value="pending">Pending</option>
          <option value="failed">Failed</option>
          <option value="cancelled">Cancelled</option>
        </select>
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
              {['파이프라인', '버전', '상태', '배포자', '시작 시간', '완료 시간'].map((h) => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((d) => {
              const st = STATUS_STYLES[d.status] ?? STATUS_STYLES.pending
              return (
                <tr
                  key={d.id}
                  style={{ transition: 'background var(--transition-fast)' }}
                  onMouseEnter={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)' }}
                  onMouseLeave={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent' }}
                >
                  <td style={tdStyle}><span style={{ fontWeight: 600 }}>{d.pipelineName}</span></td>
                  <td style={{ ...tdStyle, fontFamily: 'Fira Code, monospace', fontSize: '13px' }}>{d.version}</td>
                  <td style={tdStyle}>
                    <span style={{ padding: '3px 9px', borderRadius: '6px', background: st.bg, color: st.color, fontSize: '12px', fontWeight: 600 }}>
                      {st.label}
                    </span>
                  </td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px' }}>{d.triggeredBy}</td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px' }}>{formatDate(d.startedAt)}</td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px' }}>{formatDate(d.completedAt)}</td>
                </tr>
              )
            })}
          </tbody>
        </table>

        {filtered.length === 0 && (
          <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
            배포 이력이 없습니다.
          </div>
        )}
      </div>
    </div>
  )
}
