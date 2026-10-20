// 앱 행을 고르면 오른쪽에 붙는 패널(설계 §4.3).
//
// 요약만 보인다. 실행 단위 상세(이미지 스캔 상세 · SAST 상세 · 취약점 목록 · 시크릿 탐지 위치)는
// CI/CD 실행 기록이 이미 갖고 있다 — 상세가 두 곳에 있으면 한쪽이 낡는다(결정 8). 그래서 각
// 단계의 실행 번호와 "실행 기록 열기" 가 그 화면으로 딥링크한다.

import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { ArrowRight, ExternalLink } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { SeverityCounts } from '../../../components/shared/severity-counts'
import { SecretRotateWarning } from '../../../components/shared/secret-rotate-warning'
import { SECURITY_STAGES, type SecurityApp, type SecurityCell, type SecurityStage } from '../types/security-posture'
import {
  formatRelativeTime,
  runHistoryHref,
  safeExternalHref,
  shortImageRef,
} from '../utils/security-posture'
import { CellDbBadge, SecurityCellBadge, isNoTargetsPass } from './security-cell'

const STAGE_LABEL: Record<SecurityStage, string> = {
  secret_scan: 'securityDashboardPage.stage.secret_scan',
  dependency_scan: 'securityDashboardPage.stage.dependency_scan',
  sast: 'securityDashboardPage.stage.sast',
  image_scan: 'securityDashboardPage.stage.image_scan',
}

function StageSummary({ stage, cell }: { stage: SecurityStage; cell: SecurityCell }) {
  const { t } = useTranslation()
  const muted = 'm-0 text-[12px] text-[var(--color-text-secondary)]'

  if (cell.state === 'not_applied') return <p className={muted}>{t('securityDashboardPage.detail.notApplied')}</p>
  if (cell.state === 'unrecorded') return <p className={muted}>{t('securityDashboardPage.detail.unrecorded')}</p>

  const lines: ReactNode[] = []
  if (cell.state === 'error') {
    lines.push(<p key="error" className={muted}>{t('securityDashboardPage.detail.error')}</p>)
  }

  switch (stage) {
    case 'secret_scan': {
      if (cell.findings !== undefined && cell.findings > 0) {
        // 규칙 이름만 보인다. 비밀값은 API 에도 없다.
        const rules = (cell.rules ?? []).map((rule) => rule.ruleId).filter(Boolean)
        lines.push(
          <p key="findings" className="m-0 text-[12px] text-[var(--color-text-primary)]">
            <span className="font-semibold text-[var(--color-error)]">
              {t('securityDashboardPage.cell.findings', { count: cell.findings })}
            </span>
            {rules.length > 0 && <span className="font-mono"> — {rules.join(' · ')}</span>}
          </p>,
        )
      }
      if (cell.state === 'block' || cell.state === 'warn') {
        lines.push(<SecretRotateWarning key="rotate" />)
      }
      break
    }
    case 'dependency_scan': {
      if (isNoTargetsPass(stage, cell)) {
        lines.push(<p key="no-targets" className={muted}>{t('cicdListPage.dependencyScan.noTargetsHint')}</p>)
        break
      }
      lines.push(
        <span key="counts" className="flex flex-wrap items-center gap-1.5 text-[12px] text-[var(--color-text-secondary)]">
          {cell.state !== 'error' && <SeverityCounts compact counts={cell.counts} />}
          {cell.targets !== undefined && (
            <span>· {t('securityDashboardPage.detail.targets', { count: cell.targets })}</span>
          )}
        </span>,
      )
      break
    }
    case 'sast': {
      const conditions = cell.failedConditions ?? []
      const metrics: string[] = []
      if (cell.vulnerabilities !== undefined) {
        metrics.push(t('securityDashboardPage.cell.vulnerabilities', { count: cell.vulnerabilities }))
      }
      if (cell.securityHotspots !== undefined) {
        metrics.push(t('securityDashboardPage.cell.hotspots', { count: cell.securityHotspots }))
      }
      if (conditions.length > 0) {
        lines.push(
          <p key="conditions" className="m-0 text-[12px] text-[var(--color-text-primary)]">
            <span className="text-[var(--color-text-secondary)]">{t('securityDashboardPage.detail.failedConditions')} </span>
            {conditions
              .map((metric) => t([`cicdListPage.sast.metricNames.${metric}`], { defaultValue: metric }))
              .join(' · ')}
          </p>,
        )
      }
      if (metrics.length > 0) {
        lines.push(<p key="metrics" className={muted}>{metrics.join(' · ')}</p>)
      }
      break
    }
    case 'image_scan': {
      lines.push(
        <span key="counts" className="flex flex-wrap items-center gap-1.5 text-[12px]">
          {cell.state !== 'error' && <SeverityCounts compact counts={cell.counts} />}
          {cell.image && (
            <code title={cell.image} className="break-all font-mono text-[11px] text-[var(--color-text-secondary)]">
              {shortImageRef(cell.image)}
            </code>
          )}
        </span>,
      )
      break
    }
  }

  if (lines.length === 0) return null
  return <div className="flex flex-col gap-1">{lines}</div>
}

function StageRow({ app, stage }: { app: SecurityApp; stage: SecurityStage }) {
  const { t } = useTranslation()
  const cell = app.cells[stage]
  const href = safeExternalHref(cell.link)
  const hasDb = stage === 'dependency_scan' || stage === 'image_scan'
  return (
    <section
      aria-label={t(STAGE_LABEL[stage])}
      className="flex flex-col gap-1.5 border-b border-[var(--color-border-default)] py-2.5 last:border-b-0"
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className="w-[84px] shrink-0 text-[12px] font-semibold text-[var(--color-text-secondary)]">
          {t(STAGE_LABEL[stage])}
        </span>
        <SecurityCellBadge stage={stage} cell={cell} />
        {hasDb && <CellDbBadge cell={cell} />}
        <span className="ml-auto flex items-center gap-2 text-[12px]">
          {cell.version && cell.deploymentId && (
            // 이 단계의 결과를 낸 실행으로 간다 — 앱의 최신 실행과 다를 수 있다.
            <Link
              to={runHistoryHref(app.pipelineId, cell.deploymentId)}
              aria-label={t('securityDashboardPage.detail.openRun', { version: cell.version })}
              title={
                cell.staleRun
                  ? t('securityDashboardPage.cell.staleRun', {
                      latest: app.latestRun?.version ?? '—',
                      version: cell.version,
                    })
                  : undefined
              }
              className={
                cell.staleRun
                  ? 'text-[var(--color-text-muted)] hover:underline'
                  : 'text-[var(--color-text-secondary)] hover:underline'
              }
            >
              {cell.version}
            </Link>
          )}
          {href && (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-[var(--color-primary)] hover:underline"
            >
              {stage === 'sast' ? t('securityDashboardPage.detail.sonarqube') : t('securityDashboardPage.detail.report')}
              <ExternalLink {...iconProps('xs')} />
            </a>
          )}
        </span>
      </div>
      <div className="pl-[92px]">
        <StageSummary stage={stage} cell={cell} />
      </div>
    </section>
  )
}

export function SecurityAppDetail({
  app,
  stackName,
  locale,
}: {
  app: SecurityApp
  /** 묶음 머리의 스택 이름. 스택 없는 앱이면 "스택 없음". */
  stackName: string
  locale: string
}) {
  const { t } = useTranslation()
  const run = app.latestRun
  const runStatus = run ? t([`cicd.status.${run.status}`], { defaultValue: run.status }) : ''
  return (
    <div className="flex flex-col p-3" data-testid="security-app-detail">
      <div className="mb-1">
        <h2 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">{app.name}</h2>
        <p className="m-0 mt-0.5 text-[12px] text-[var(--color-text-secondary)]">
          {stackName}
          {' · '}
          {run
            ? `${t('securityDashboardPage.detail.latestRun', { version: run.version, status: runStatus })} · ${formatRelativeTime(run.startedAt, locale)}`
            : t('securityDashboardPage.detail.noRun')}
        </p>
      </div>

      <div className="flex flex-col">
        {SECURITY_STAGES.map((stage) => (
          <StageRow key={stage} app={app} stage={stage} />
        ))}
      </div>

      <Link
        to={runHistoryHref(app.pipelineId, run?.deploymentId || undefined)}
        className="mt-3 inline-flex items-center gap-1 self-start rounded-[var(--radius-sm)] border border-[var(--color-border-default)] px-2.5 py-1.5 text-[12px] font-semibold text-[var(--color-primary)] no-underline hover:bg-[color-mix(in_srgb,_var(--color-primary)_8%,_transparent)]"
      >
        {t('securityDashboardPage.detail.openRunHistory')}
        <ArrowRight {...iconProps('xs')} />
      </Link>
    </div>
  )
}
