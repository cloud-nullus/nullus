// 보안 대시보드의 앱 표(설계 §4.2).
//
// 행 순서는 서버가 정한 위험순(rank) 그대로다. 이 표는 순서를 바꾸지 않고 스택별로 묶기만
// 한다 — 그래서 정렬 · 페이지를 갖는 DataTable 이 아니라 손으로 짠 <table> 이다. 묶음 머리가
// 행 사이에 끼어야 하고, 페이지로 자르면 묶음이 두 쪽에 걸친다. 표면(머리 · 셀)은 DataTable 과
// 같은 table-chrome 값을 쓴다.
//
// 묶음은 문제가 있으면 열고 없으면 접어서 연다. 스택 하나를 고르면 묶음을 풀고 평평하게 보인다.

import { useTranslation } from 'react-i18next'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { tableHeadRowClass, tdClass, thClass } from '../../../components/shared/table-chrome'
import { cn } from '../../../lib/utils'
import {
  SECURITY_STAGES,
  type SecurityApp,
  type SecurityAppGroup,
  type SecurityAppsReport,
  type SecurityStage,
} from '../types/security-posture'
import { formatRelativeTime, groupAppsByStack, isGroupOpenByDefault } from '../utils/security-posture'
import { SecurityCellView } from './security-cell'

const STAGE_LABEL: Record<SecurityStage, string> = {
  secret_scan: 'securityDashboardPage.stage.secret_scan',
  dependency_scan: 'securityDashboardPage.stage.dependency_scan',
  sast: 'securityDashboardPage.stage.sast',
  image_scan: 'securityDashboardPage.stage.image_scan',
}

/** 묶음 상태의 키. 스택 없는 앱의 stackId 는 '' 라 그대로 키로 쓰면 읽기 어렵다. */
const groupKey = (group: SecurityAppGroup) => group.stackId || '__no_stack__'

interface SecurityAppsTableProps {
  report: SecurityAppsReport
  /** 스택 하나(또는 "스택 없음")를 골랐다 — 묶지 않는다. */
  grouped: boolean
  problemOnly: boolean
  selectedAppId: string | null
  onSelect: (app: SecurityApp) => void
  /** 사용자가 펼치거나 접은 묶음. 없으면 기본(문제 있으면 펼침). */
  groupOpen: Record<string, boolean>
  onToggleGroup: (key: string, open: boolean) => void
  locale: string
  /** 표가 비었을 때의 문구(필터 결과 없음 · 문제 있는 앱 없음). */
  emptyMessage: string
}

export function SecurityAppsTable({
  report,
  grouped,
  problemOnly,
  selectedAppId,
  onSelect,
  groupOpen,
  onToggleGroup,
  locale,
  emptyMessage,
}: SecurityAppsTableProps) {
  const { t } = useTranslation()
  const total = report.summary.apps
  const columnCount = SECURITY_STAGES.length + 2

  const groupSummary = (group: SecurityAppGroup) => {
    const parts: string[] = []
    if (group.apps !== undefined) parts.push(t('securityDashboardPage.group.apps', { count: group.apps }))
    if (group.blocked) parts.push(t('securityDashboardPage.group.blocked', { count: group.blocked }))
    if (group.secrets) parts.push(t('securityDashboardPage.group.secrets', { count: group.secrets }))
    if (group.critical) parts.push(t('securityDashboardPage.group.critical', { count: group.critical }))
    if (group.undetermined) {
      parts.push(t('securityDashboardPage.group.undetermined', { count: group.undetermined }))
    }
    // 문제 수가 0 이라고 확실할 때만 "문제 없음" 이다. 모르면 적지 않는다.
    if (group.problems === 0) parts.push(t('securityDashboardPage.group.noProblems'))
    return parts.join(' · ')
  }

  const renderRow = (app: SecurityApp) => {
    const selected = app.pipelineId === selectedAppId
    return (
      <tr
        key={app.pipelineId}
        data-app-row={app.pipelineId}
        aria-selected={selected}
        onClick={() => onSelect(app)}
        className={cn(
          'cursor-pointer transition-colors duration-150',
          selected
            ? 'bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)]'
            : 'hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]',
        )}
      >
        <td className={cn(tdClass, grouped && 'pl-7')}>
          {/* 행 전체가 눌리지만 키보드로도 닿게 이름을 단추로 둔다. */}
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation()
              onSelect(app)
            }}
            className="cursor-pointer border-none bg-transparent p-0 text-left text-[13px] font-semibold text-[var(--color-text-primary)] hover:underline"
          >
            {app.name}
          </button>
        </td>
        {SECURITY_STAGES.map((stage) => (
          <td key={stage} className={tdClass}>
            <SecurityCellView stage={stage} cell={app.cells[stage]} latestVersion={app.latestRun?.version} />
          </td>
        ))}
        <td className={cn(tdClass, 'whitespace-nowrap text-[12px] text-[var(--color-text-secondary)]')}>
          {app.latestRun
            ? `${app.latestRun.version} · ${formatRelativeTime(app.latestRun.startedAt, locale)}`
            : t('securityDashboardPage.detail.noRun')}
        </td>
      </tr>
    )
  }

  const sections = grouped ? groupAppsByStack(report) : null

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-max border-collapse">
        <thead>
          <tr className={tableHeadRowClass}>
            <th className={thClass}>{t('securityDashboardPage.columns.app')}</th>
            {SECURITY_STAGES.map((stage) => {
              const applied = report.coverage[stage]
              // 적용률은 KPI 가 아니라 열 머리에 둔다(설계 §3) — `SAST 12/24`.
              const coverage = applied !== undefined && total !== undefined ? ` ${applied}/${total}` : ''
              return (
                <th
                  key={stage}
                  className={thClass}
                  title={
                    coverage
                      ? t('securityDashboardPage.columns.coverageHint', { applied, total })
                      : undefined
                  }
                >
                  {t(STAGE_LABEL[stage])}
                  <span className="font-normal tabular-nums">{coverage}</span>
                </th>
              )
            })}
            <th className={thClass}>{t('securityDashboardPage.columns.latestRun')}</th>
          </tr>
        </thead>
        <tbody>
          {sections
            ? sections.map(({ group, apps }) => {
                const key = groupKey(group)
                const open = groupOpen[key] ?? isGroupOpenByDefault(group)
                const name = group.stackId ? group.stackName || group.stackId : t('securityDashboardPage.noStack')
                return [
                  <tr key={`group-${key}`} data-group={key} className="bg-[var(--color-surface-sunken)]">
                    <td colSpan={columnCount} className={cn(tdClass, 'py-0')}>
                      <button
                        type="button"
                        aria-expanded={open}
                        onClick={() => onToggleGroup(key, !open)}
                        className="flex w-full cursor-pointer items-center gap-1.5 border-none bg-transparent p-0 py-1.5 text-left text-[12px] text-[var(--color-text-secondary)]"
                      >
                        {open ? <ChevronDown {...iconProps('xs')} /> : <ChevronRight {...iconProps('xs')} />}
                        <span className="font-semibold text-[var(--color-text-primary)]">{name}</span>
                        {group.clusterName && (
                          <span>· {t('securityDashboardPage.group.cluster', { name: group.clusterName })}</span>
                        )}
                        <span>— {groupSummary(group)}</span>
                      </button>
                    </td>
                  </tr>,
                  ...(open
                    ? apps.length > 0
                      ? apps.map(renderRow)
                      : problemOnly
                        ? [
                            <tr key={`group-${key}-hidden`}>
                              <td
                                colSpan={columnCount}
                                className={cn(tdClass, 'pl-7 text-[12px] text-[var(--color-text-secondary)]')}
                              >
                                {t('securityDashboardPage.group.allHidden')}
                              </td>
                            </tr>,
                          ]
                        : []
                    : []),
                ]
              })
            : report.apps.map(renderRow)}
        </tbody>
      </table>
      {/* 묶음 머리만 남고 행이 하나도 없을 때도 말해 준다 — 빈 표가 "다 통과" 로 읽히지 않게. */}
      {report.apps.length === 0 && (
        <div className="py-10 text-center text-[13px] text-[var(--color-text-secondary)]">{emptyMessage}</div>
      )}
    </div>
  )
}
