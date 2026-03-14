import { useState } from 'react'
import { GitBranch, Plus, Search, Play } from 'lucide-react'
import { usePipelines, useCreatePipeline, useDeployPipeline } from '../api/cicd-api'
import type { Pipeline, PipelineStatus, AppType, CreatePipelineRequest } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'

const MOCK_PIPELINES: Pipeline[] = [
  {
    id: 'p1',
    name: 'api-server-pipeline',
    appType: 'web-backend',
    clusterId: 'c1',
    clusterName: 'prod-cluster',
    status: 'success',
    lastDeployedAt: '2026-03-13T10:00:00Z',
    createdAt: '2026-02-01T00:00:00Z',
  },
  {
    id: 'p2',
    name: 'frontend-pipeline',
    appType: 'web-frontend',
    clusterId: 'c2',
    clusterName: 'staging-cluster',
    status: 'running',
    lastDeployedAt: '2026-03-14T08:30:00Z',
    createdAt: '2026-02-10T00:00:00Z',
  },
  {
    id: 'p3',
    name: 'data-batch-pipeline',
    appType: 'batch-job',
    clusterId: 'c3',
    clusterName: 'dev-cluster',
    status: 'failed',
    lastDeployedAt: '2026-03-12T14:00:00Z',
    createdAt: '2026-03-01T00:00:00Z',
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
  return new Date(iso).toLocaleDateString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

const selectStyle: React.CSSProperties = {
  background: 'rgba(255,255,255,0.04)',
  border: '1px solid var(--color-border-default)',
  borderRadius: '8px',
  padding: '9px 12px',
  fontSize: '14px',
  color: 'var(--color-text-primary)',
  cursor: 'pointer',
}

export function CicdListPage() {
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [createModal, setCreateModal] = useState(false)
  const [form, setForm] = useState<CreatePipelineRequest>({ name: '', appType: 'web-backend', clusterId: '' })

  const { data: apiData } = usePipelines({ status: statusFilter || undefined, search: search || undefined })
  const pipelines = apiData?.items ?? MOCK_PIPELINES
  const createPipeline = useCreatePipeline()
  const deployPipeline = useDeployPipeline()

  const filtered = pipelines.filter((p) => {
    const q = search.toLowerCase()
    const matchesSearch = !search || p.name.toLowerCase().includes(q) || p.clusterName.toLowerCase().includes(q)
    const matchesStatus = !statusFilter || p.status === statusFilter
    return matchesSearch && matchesStatus
  })

  const handleCreate = () => {
    createPipeline.mutate(form, {
      onSuccess: () => {
        setCreateModal(false)
        setForm({ name: '', appType: 'web-backend', clusterId: '' })
      },
    })
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
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '24px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
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
            <GitBranch size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              CI/CD Pipelines
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              CI/CD 파이프라인 목록
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={() => setCreateModal(true)}>
          <Plus size={15} />
          New Pipeline
        </Button>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '10px', marginBottom: '16px', flexWrap: 'wrap' }}>
        <div style={{ position: 'relative', flex: '1 1 240px', maxWidth: '320px' }}>
          <Search
            size={13}
            style={{
              position: 'absolute',
              left: '10px',
              top: '50%',
              transform: 'translateY(-50%)',
              color: 'var(--color-text-secondary)',
              pointerEvents: 'none',
            }}
          />
          <Input
            placeholder="파이프라인 검색..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ paddingLeft: '30px' }}
          />
        </div>
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
              {['이름', '앱 타입', '클러스터', '상태', '최근 배포', 'Actions'].map((h) => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((p) => {
              const st = STATUS_STYLES[p.status] ?? STATUS_STYLES.pending
              return (
                <tr
                  key={p.id}
                  style={{ transition: 'background var(--transition-fast)' }}
                  onMouseEnter={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)' }}
                  onMouseLeave={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent' }}
                >
                  <td style={tdStyle}><span style={{ fontWeight: 600 }}>{p.name}</span></td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)' }}>{p.appType}</td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)' }}>{p.clusterName}</td>
                  <td style={tdStyle}>
                    <span style={{ padding: '3px 9px', borderRadius: '6px', background: st.bg, color: st.color, fontSize: '12px', fontWeight: 600 }}>
                      {st.label}
                    </span>
                  </td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px' }}>
                    {formatDate(p.lastDeployedAt)}
                  </td>
                  <td style={tdStyle}>
                    <div style={{ display: 'flex', gap: '6px' }}>
                      <Button
                        variant="secondary"
                        size="sm"
                        loading={deployPipeline.isPending}
                        onClick={() => deployPipeline.mutate(p.id)}
                      >
                        <Play size={11} />
                        Deploy
                      </Button>
                      <Button variant="ghost" size="sm">View</Button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>

        {filtered.length === 0 && (
          <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
            파이프라인이 없습니다.
          </div>
        )}
      </div>

      {/* Create Pipeline Modal */}
      <Modal
        open={createModal}
        onClose={() => setCreateModal(false)}
        title="New Pipeline"
        footer={
          <>
            <Button variant="outline" size="sm" onClick={() => setCreateModal(false)}>Cancel</Button>
            <Button
              variant="primary"
              size="sm"
              loading={createPipeline.isPending}
              onClick={handleCreate}
              disabled={!form.name || !form.clusterId}
            >
              Create
            </Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <Input
            label="파이프라인 이름"
            placeholder="예: api-server-pipeline"
            value={form.name}
            onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
          />
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>앱 타입</label>
            <select
              value={form.appType}
              onChange={(e) => setForm((p) => ({ ...p, appType: e.target.value as AppType }))}
              style={selectStyle}
            >
              <option value="web-backend">Web Backend</option>
              <option value="web-frontend">Web Frontend</option>
              <option value="batch-job">Batch Job</option>
            </select>
          </div>
          <Input
            label="클러스터 ID"
            placeholder="예: c1"
            value={form.clusterId}
            onChange={(e) => setForm((p) => ({ ...p, clusterId: e.target.value }))}
          />
        </div>
      </Modal>
    </div>
  )
}
