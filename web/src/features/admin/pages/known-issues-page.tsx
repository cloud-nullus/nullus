import { AlertTriangle } from 'lucide-react'
import { useKnownIssues } from '../api/admin-api'
import type { KnownIssueSeverity, KnownIssueStatus } from '../../../types'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

const SEVERITY_BADGE: Record<KnownIssueSeverity, string> = {
  high: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  medium: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
  low: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
}

const STATUS_BADGE: Record<KnownIssueStatus, string> = {
  open: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  acknowledged: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
  planned: 'bg-[rgba(34,197,94,0.15)] text-[#34d399]',
}

export function KnownIssuesPage() {
  const { data, isLoading } = useKnownIssues()
  const items = data?.items ?? []

  return (
    <div>
      <Breadcrumb items={[{ label: 'Known Issues' }]} />

      <div className="mb-7 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(245,158,11,0.15)] text-[#f59e0b]">
          <AlertTriangle size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Known Issues
          </h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
            현재 버전의 제한사항과 우회 방법을 확인합니다.
          </p>
        </div>
      </div>

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[rgba(255,255,255,0.02)]">
              {['ID', 'Severity', 'Title', 'Status', 'Workaround'].map((header) => (
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
