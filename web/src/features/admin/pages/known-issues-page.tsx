import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useKnownIssues } from '../api/admin-api'
import type { KnownIssueSeverity, KnownIssueStatus } from '../../../types'
import { cn } from '../../../lib/utils'
import { PageHeader } from '../../../components/layout/page-header'

const SEVERITY_BADGE: Record<KnownIssueSeverity, string> = {
  high: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]',
  medium: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]',
  low: 'bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]',
}

const STATUS_BADGE: Record<KnownIssueStatus, string> = {
  open: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]',
  acknowledged: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]',
  planned: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]',
}

export function KnownIssuesPage() {
  const { t } = useTranslation()
  const { data, isLoading } = useKnownIssues()
  const items = data?.items ?? []
  const tableHeaders = [
    t('knownIssuesPage.table.id', 'ID'),
    t('knownIssuesPage.table.severity', 'Severity'),
    t('knownIssuesPage.table.title', 'Title'),
    t('knownIssuesPage.table.status', 'Status'),
    t('knownIssuesPage.table.workaround', 'Workaround'),
  ]

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('knownIssuesPage.breadcrumb.current', 'Known Issues') }]}
        icon={<AlertTriangle size={16} />}
        tone="warning"
        title={t('knownIssuesPage.title', 'Known Issues')}
        subtitle={t('knownIssuesPage.description', 'Check current version limitations and available workarounds.')}
      />

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]">
              {tableHeaders.map((header) => (
                <th
                  key={header}
                  className="px-3.5 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]"
                >
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr>
                <td
                  colSpan={5}
                  className="border-t border-[var(--color-border-default)] px-3.5 py-8 text-center text-sm text-[var(--color-text-secondary)]"
                >
                  Loading known issues...
                </td>
              </tr>
            )}

            {!isLoading && items.length === 0 && (
              <tr>
                <td
                  colSpan={5}
                  className="border-t border-[var(--color-border-default)] px-3.5 py-8 text-center text-sm text-[var(--color-text-secondary)]"
                >
                  No known issues.
                </td>
              </tr>
            )}

            {!isLoading && items.map((item) => (
              <tr key={item.id}>
                <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm font-semibold text-[var(--color-text-primary)]">
                  {item.id}
                </td>
                <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm text-[var(--color-text-primary)]">
                  <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold capitalize', SEVERITY_BADGE[item.severity])}>
                    {item.severity}
                  </span>
                </td>
                <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm text-[var(--color-text-primary)]">
                  <div className="font-semibold">{item.title}</div>
                  <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{item.description}</div>
                </td>
                <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm text-[var(--color-text-primary)]">
                  <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold capitalize', STATUS_BADGE[item.status])}>
                    {item.status}
                  </span>
                </td>
                <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm text-[var(--color-text-secondary)]">
                  {item.workaround}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
