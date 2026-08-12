import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ChevronRight, History } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useDeployments } from '../api/cicd-api'
import type { Deployment, PipelineStatus } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Select } from '../../../components/ui/select'
import { DataTable } from '../../../components/shared/data-table'
import { formatDateTime, resolveLocale } from '../../../lib/locale'
import { getPipelineStatusLabel } from '../utils/pipeline-status'
import { StatusBadge, toneForStatus } from '../../../components/shared/status-badge'
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from '../../../components/ui/search-input'
import { thClass } from '../../../components/shared/table-chrome'
import { cn } from '../../../lib/utils'
export function CicdHistoryPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const [searchParams] = useSearchParams()
  const pipelineFilter = searchParams.get('pipeline') ?? ''
  const [statusFilter, setStatusFilter] = useState('')
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string | null>(null)
  const [search, setSearch] = useState('')

  const { data: apiData } = useDeployments({
    pipelineId: pipelineFilter || undefined,
    status: statusFilter as PipelineStatus || undefined,
  })
  const deployments = apiData?.items ?? []

  const filtered = deployments.filter((d) => {
    const matchesPipeline = !pipelineFilter || d.pipelineId === pipelineFilter
    const matchesStatus = !statusFilter || d.status === statusFilter
    const matchesSearch = !search || d.pipelineName.toLowerCase().includes(search.toLowerCase()) || d.triggeredBy.toLowerCase().includes(search.toLowerCase())
    return matchesPipeline && matchesStatus && matchesSearch
  })
  // 필터링 결과에서 선택 행을 찾는다. 필터가 바뀌어 선택 행이 목록에서 빠지면
  // 상세도 자동으로 사라진다 — 목록에 없는 항목의 상세가 남아 있으면 안 된다.
  const selectedDetail = filtered.find((d) => d.id === selectedDeploymentId) ?? null

  const columns: ColumnDef<Deployment, unknown>[] = [
    {
      id: 'select',
      header: '',
      enableSorting: false,
      cell: ({ row }) => {
        const isSelected = selectedDeploymentId === row.original.id
        return (
          <Button
            variant={isSelected ? 'secondary' : 'ghost'}
            size="sm"
            type="button"
            aria-pressed={isSelected}
            aria-label={t('cicdHistoryPage.actions.viewDetail', 'View detail')}
            onClick={(e) => {
              e.stopPropagation()
              setSelectedDeploymentId((prev) => (prev === row.original.id ? null : row.original.id))
            }}
          >
            <ChevronRight size={13} />
          </Button>
        )
      },
    },
    {
      accessorKey: 'pipelineName',
      header: t('cicdHistoryPage.table.pipeline', 'Pipeline'),
      cell: ({ row }) => <span className="font-semibold">{row.original.pipelineName}</span>,
    },
    {
      accessorKey: 'version',
      header: t('cicdHistoryPage.table.version', 'Version'),
      cell: ({ row }) => <span className="font-mono text-[13px]">{row.original.version}</span>,
    },
    {
      accessorKey: 'status',
      header: t('cicdHistoryPage.table.status', 'Status'),
      cell: ({ row }) => (
        <StatusBadge
          tone={toneForStatus(row.original.status)}
          label={getPipelineStatusLabel(t, row.original.status)}
        />
      ),
    },
    {
      accessorKey: 'triggeredBy',
      header: t('cicdHistoryPage.table.triggeredBy', 'Deployer'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{row.original.triggeredBy}</span>,
    },
    {
      accessorKey: 'startedAt',
      header: t('cicdHistoryPage.table.startedAt', 'Started At'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDateTime(row.original.startedAt, locale)}</span>,
    },
    {
      accessorKey: 'completedAt',
      header: t('cicdHistoryPage.table.completedAt', 'Completed At'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDateTime(row.original.completedAt, locale)}</span>,
    },
  ]

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('sidebar.cicdList', 'CI/CD List'), path: '/cicd/list' }, { label: t('cicdHistoryPage.title', 'CI/CD History') }]}
        icon={<History size={16} />}
        tone="warning"
        title={t('cicdHistoryPage.title', 'CI/CD History')}
        subtitle={t('cicdHistoryPage.description', 'CI/CD Deployment History')}
      />

      {pipelineFilter && (
        <div className="mb-4 flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-info)_8%,_transparent)] px-3 py-2 text-sm">
          <span className="text-[var(--color-text-secondary)]">{t('cicdHistoryPage.filteredByPipeline', 'Filtered by pipeline:')}</span>
          <span className="font-semibold text-[var(--color-text-primary)]">
            {deployments.find((d) => d.pipelineId === pipelineFilter)?.pipelineName ?? pipelineFilter}
          </span>
          <button
            type="button"
            onClick={() => {
              const newParams = new URLSearchParams(searchParams)
              newParams.delete('pipeline')
              window.history.replaceState({}, '', `${window.location.pathname}?${newParams.toString()}`)
              window.location.reload()
            }}
            className="ml-auto cursor-pointer text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
          >
            {t('cicdHistoryPage.clearFilter', 'Clear filter')}
          </button>
        </div>
      )}

      <DataTable
        columns={columns}
        data={filtered}
        getRowKey={(row) => row.id}
        onRowClick={(row) => setSelectedDeploymentId(row.id)}
        emptyMessage={t('cicdHistoryPage.empty', 'No deployment history.')}
        toolbar={
          <>
            <Select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} >
              <option value="">{t('cicdHistoryPage.filters.allStatus', 'All Status')}</option>
              <option value="success">{t('cicd.status.success', 'Success')}</option>
              <option value="running">{t('cicd.status.running', 'Running')}</option>
              <option value="pending">{t('cicd.status.pending', 'Pending')}</option>
              <option value="failed">{t('cicd.status.failed', 'Failed')}</option>
              <option value="cancelled">{t('cicd.status.cancelled', 'Cancelled')}</option>
            </Select>
            <SearchInput
              wrapperClassName="ml-auto w-[220px]"
              placeholder={t('cicdHistoryPage.searchPlaceholder', 'Search pipeline / deployer...')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </>
        }
      />

      {/*
        서브 테이블 — 메인 테이블에서 선택한 배포의 상세.

        이전에는 행 안에서 펼치는 방식이었는데, 그 패널이 보여주던 6개 필드가
        메인 테이블 컬럼과 완전히 같아서 새 정보가 0 이었다. 또 행 확장은 28개 화면
        중 이 한 곳뿐인 일회성 패턴이었고, AG Grid 의 Master/Detail 은 Enterprise 전용이다.
        메인/서브 분리는 Community 순정 구성이면서 앱의 다른 3화면이 쓰는
        분할 상세 패턴과도 맞는다. (기획안 D5)

        실제 하위 엔티티(배포 단계 steps)는 GET /cicd/deployments/{id} 에 이미 있다.
        그걸 여기에 붙이는 것은 기능 추가라 별 티켓으로 분리했다(기획안 Phase 3-b).
      */}
      {selectedDetail && (
        <section className="mt-4 overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
          <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-[14px] py-3">
            <h2 className="m-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
              {t('cicdHistoryPage.detail.title', 'Deployment Detail')}
            </h2>
            <Button variant="ghost" size="sm" type="button" onClick={() => setSelectedDeploymentId(null)}>
              {t('cicdHistoryPage.detail.close', 'Close')}
            </Button>
          </div>
          <table className="w-full border-collapse">
            <thead>
              <tr>
                <th className={cn(thClass, "w-[180px]")}>
                  {t('cicdHistoryPage.detail.field', 'Field')}
                </th>
                <th className={cn(thClass)}>
                  {t('cicdHistoryPage.detail.value', 'Value')}
                </th>
              </tr>
            </thead>
            <tbody>
              {[
                { label: t('cicdHistoryPage.table.pipeline', 'Pipeline'), value: selectedDetail.pipelineName },
                { label: t('cicdHistoryPage.table.version', 'Version'), value: selectedDetail.version },
                // 상세는 개편 전 라벨('Triggered By')을 그대로 쓴다. 메인 컬럼은 'Deployer' 라
                // 둘이 어긋나 있지만, 문자열을 없애는 건 정보 유실이라 통일은 별도 제안으로 남긴다.
                { label: t('cicdHistoryPage.detail.triggeredBy', 'Triggered By'), value: selectedDetail.triggeredBy || '-' },
                { label: t('cicdHistoryPage.table.status', 'Status'), value: getPipelineStatusLabel(t, selectedDetail.status) },
                { label: t('cicdHistoryPage.table.startedAt', 'Started At'), value: formatDateTime(selectedDetail.startedAt, locale) },
                { label: t('cicdHistoryPage.table.completedAt', 'Completed At'), value: formatDateTime(selectedDetail.completedAt, locale) },
              ].map(({ label, value }) => (
                <tr key={label}>
                  <td className="border-t border-[var(--color-border-default)] px-[14px] py-2.5 text-[13px] text-[var(--color-text-muted)]">
                    {label}
                  </td>
                  <td className="border-t border-[var(--color-border-default)] px-[14px] py-2.5 text-[13px] text-[var(--color-text-primary)]">
                    {value}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  )
}
