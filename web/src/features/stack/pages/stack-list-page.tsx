import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { List, Plus, Search } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useStacks } from '../api/stack-api'
import type { Stack } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { DataTable } from '../../../components/shared/data-table'

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

  const { data: apiData } = useStacks({ search, status: statusFilter || undefined })
  const stacks = apiData?.items ?? MOCK_STACKS

  const filtered = stacks.filter((s) => {
    const q = search.toLowerCase()
    const matchesSearch =
      !search ||
      s.name.toLowerCase().includes(q) ||
      s.templateName.toLowerCase().includes(q) ||
      s.clusterName.toLowerCase().includes(q)
    const matchesStatus = !statusFilter || s.status === statusFilter
    return matchesSearch && matchesStatus
  })

  const columns: ColumnDef<Stack, unknown>[] = [
    {
      accessorKey: 'name',
      header: '스택 이름',
      cell: ({ row }) => <span className="font-semibold">{row.original.name}</span>,
    },
    {
      accessorKey: 'templateName',
      header: '템플릿',
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.templateName}</span>,
    },
    {
      accessorKey: 'clusterName',
      header: '클러스터',
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.clusterName}</span>,
    },
    {
      accessorKey: 'status',
      header: '상태',
      cell: ({ row }) => {
        const statusStyle = STATUS_STYLES[row.original.status] ?? STATUS_STYLES.pending
        return (
          <span
            className="rounded-md px-[9px] py-[3px] text-xs font-semibold"
            style={{ backgroundColor: statusStyle.bg, color: statusStyle.color }}
          >
            {statusStyle.label}
          </span>
        )
      },
    },
    {
      accessorKey: 'createdAt',
      header: '생성일',
      cell: ({ row }) => (
        <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.createdAt)}</span>
      ),
    },
    {
      id: 'actions',
      header: 'Actions',
      enableSorting: false,
      cell: () => (
        <div className="flex gap-1.5">
          <Button variant="ghost" size="sm" type="button">
            View
          </Button>
          <Button variant="danger" size="sm" type="button">
            Delete
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div>
      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <List size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Stack List
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
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
      <div className="mb-4 flex flex-wrap gap-2.5">
        <div className="relative max-w-[320px] flex-[1_1_240px]">
          <Search
            size={13}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
          />
          <Input
            placeholder="스택 검색..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-[30px]"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
        >
          <option value="">All Status</option>
          <option value="success">Success</option>
          <option value="running">Running</option>
          <option value="pending">Pending</option>
          <option value="failed">Failed</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      <DataTable columns={columns} data={filtered} getRowKey={(row) => row.id} emptyMessage="스택이 없습니다." />
    </div>
  )
}
