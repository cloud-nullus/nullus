import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { List, Plus, Search, ChevronUp, ChevronDown } from 'lucide-react'
import { useStacks } from '../api/stack-api'
import type { Stack } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'

type SortField = 'name' | 'templateName' | 'clusterName' | 'status' | 'createdAt'
type SortDir = 'asc' | 'desc'

const STATUS_STYLES: Record<string, { bg: string; color: string; label: string }> = {
  running: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa', label: 'Running' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Success' },
  failed: { bg: 'rgba(239,68,68,0.15)', color: '#ef4444', label: 'Failed' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Pending' },
  cancelled: { bg: 'rgba(100,116,139,0.15)', color: '#64748b', label: 'Cancelled' },
}

const MOCK_STACKS: Stack[] = [
  {
    id: 's1',
    name: 'prod-gitlab-stack',
    templateId: 'gitlab-all-in-one',
    templateName: 'GitLab All-in-One',
    clusterId: 'c1',
    clusterName: 'prod-cluster',
    status: 'success',
    createdAt: '2026-03-10T09:00:00Z',
    updatedAt: '2026-03-10T09:25:00Z',
  },
  {
    id: 's2',
    name: 'staging-argocd',
    templateId: 'gitlab-argocd',
    templateName: 'GitLab + ArgoCD',
    clusterId: 'c2',
    clusterName: 'staging-cluster',
    status: 'running',
    createdAt: '2026-03-12T14:00:00Z',
    updatedAt: '2026-03-12T14:05:00Z',
  },
  {
    id: 's3',
    name: 'dev-github-stack',
    templateId: 'github-argocd',
    templateName: 'GitHub + ArgoCD',
    clusterId: 'c3',
    clusterName: 'dev-cluster',
    status: 'pending',
    createdAt: '2026-03-14T08:00:00Z',
    updatedAt: '2026-03-14T08:01:00Z',
  },
]

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit' })
}

export function StackListPage() {
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [sort, setSort] = useState<{ field: SortField; dir: SortDir }>({ field: 'createdAt', dir: 'desc' })

  const { data: apiData } = useStacks({ search, status: statusFilter || undefined })
  const stacks = apiData?.items ?? MOCK_STACKS

  const filtered = stacks
    .filter((s) => {
      const q = search.toLowerCase()
      const matchesSearch =
        !search ||
        s.name.toLowerCase().includes(q) ||
        s.templateName.toLowerCase().includes(q) ||
        s.clusterName.toLowerCase().includes(q)
      const matchesStatus = !statusFilter || s.status === statusFilter
      return matchesSearch && matchesStatus
    })
    .sort((a, b) => {
      const mul = sort.dir === 'asc' ? 1 : -1
      const av = a[sort.field] ?? ''
      const bv = b[sort.field] ?? ''
      return av < bv ? -mul : av > bv ? mul : 0
    })

  const handleSort = (field: SortField) => {
    setSort((prev) =>
      prev.field === field ? { field, dir: prev.dir === 'asc' ? 'desc' : 'asc' } : { field, dir: 'asc' }
    )
  }

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sort.field !== field) return null
    return sort.dir === 'asc' ? <ChevronUp size={12} /> : <ChevronDown size={12} />
  }

  const thStyle: React.CSSProperties = {
    padding: '10px 14px',
    textAlign: 'left',
    fontSize: '11px',
    fontWeight: 600,
    color: 'var(--color-text-secondary)',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    cursor: 'pointer',
    userSelect: 'none',
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
            <List size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              Stack List
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              배포된 DevSecOps 스택 목록
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={() => navigate('/stack/install')}>
          <Plus size={15} />
          New Stack
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
            placeholder="스택 검색..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ paddingLeft: '30px' }}
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          style={{
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '8px',
            padding: '9px 12px',
            fontSize: '14px',
            color: 'var(--color-text-primary)',
            cursor: 'pointer',
          }}
        >
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
              {(
                [
                  ['name', '스택 이름'],
                  ['templateName', '템플릿'],
                  ['clusterName', '클러스터'],
                  ['status', '상태'],
                  ['createdAt', '생성일'],
                ] as [SortField, string][]
              ).map(([field, label]) => (
                <th key={field} style={thStyle} onClick={() => handleSort(field)}>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                    {label}
                    <SortIcon field={field} />
                  </span>
                </th>
              ))}
              <th style={{ ...thStyle, cursor: 'default' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((stack) => {
              const statusStyle = STATUS_STYLES[stack.status] ?? STATUS_STYLES.pending
              return (
                <tr
                  key={stack.id}
                  style={{ transition: 'background var(--transition-fast)' }}
                  onMouseEnter={(e) => {
                    ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)'
                  }}
                  onMouseLeave={(e) => {
                    ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent'
                  }}
                >
                  <td style={tdStyle}>
                    <span style={{ fontWeight: 600 }}>{stack.name}</span>
                  </td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)' }}>{stack.templateName}</td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)' }}>{stack.clusterName}</td>
                  <td style={tdStyle}>
                    <span
                      style={{
                        padding: '3px 9px',
                        borderRadius: '6px',
                        background: statusStyle.bg,
                        color: statusStyle.color,
                        fontSize: '12px',
                        fontWeight: 600,
                      }}
                    >
                      {statusStyle.label}
                    </span>
                  </td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px' }}>
                    {formatDate(stack.createdAt)}
                  </td>
                  <td style={tdStyle}>
                    <div style={{ display: 'flex', gap: '6px' }}>
                      <Button variant="ghost" size="sm">
                        View
                      </Button>
                      <Button variant="danger" size="sm">
                        Delete
                      </Button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>

        {filtered.length === 0 && (
          <div
            style={{
              textAlign: 'center',
              padding: '48px 0',
              color: 'var(--color-text-secondary)',
              fontSize: '14px',
            }}
          >
            스택이 없습니다.{' '}
            <button
              onClick={() => navigate('/stack/install')}
              style={{
                background: 'none',
                border: 'none',
                color: '#a5b4fc',
                cursor: 'pointer',
                textDecoration: 'underline',
                fontSize: '14px',
              }}
            >
              새 스택 만들기
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
