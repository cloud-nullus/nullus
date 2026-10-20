// 보안 대시보드 앱 표의 셀 하나 — 그 앱 main 의 그 단계 최신 기록(설계 §2.1 · §2.2).
//
// 여섯 상태 중 뒤의 셋(판정 불가 · 기록 없음 · 미적용)은 초록으로 보이지 않는다. 의존성
// 스캔이 검사 대상 0 개로 통과한 것도 초록이 아니라 회색 "대상 없음" 이다 — 000070 의 교훈:
// 돌지 않은 것을 성공으로 보여주지 않는다.
//
// 숫자는 아는 것만 그린다. 응답에서 빠진 숫자는 모르는 것이지 0 이 아니다.

import { useTranslation } from 'react-i18next'
import { Badge } from '../../../components/ui/badge'
import { DbStaleBadge, GateBadge, type GateResult } from '../../../components/shared/gate-badge'
import { cn } from '../../../lib/utils'
import type { SecurityCell, SecurityStage } from '../types/security-posture'
import { daysSince } from '../utils/security-posture'

const CELL_GATE_LABEL: Record<GateResult, string> = {
  pass: 'securityDashboardPage.cell.pass',
  warn: 'securityDashboardPage.cell.warn',
  block: 'securityDashboardPage.cell.block',
  error: 'securityDashboardPage.cell.error',
}

/** 의존성 스캔이 아무것도 검사하지 않고 통과했다. */
export const isNoTargetsPass = (stage: SecurityStage, cell: SecurityCell) =>
  stage === 'dependency_scan' && cell.state === 'pass' && cell.targets === 0

/** 셀의 상태 배지. 숫자 · DB · 실행 번호는 붙이지 않는다(상세 패널도 이 배지를 쓴다). */
export function SecurityCellBadge({ stage, cell }: { stage: SecurityStage; cell: SecurityCell }) {
  const { t } = useTranslation()
  switch (cell.state) {
    case 'pass':
      if (isNoTargetsPass(stage, cell)) {
        return (
          <GateBadge
            result="pass"
            tone="neutral"
            label={t('securityDashboardPage.cell.noTargets')}
            title={t('cicdListPage.dependencyScan.noTargetsHint')}
          />
        )
      }
      // 낡은 DB 로 얻은 통과는 깨끗한 통과가 아니다 — 경고색이다(이미지 스캔과 같은 규칙).
      return (
        <GateBadge
          result="pass"
          label={t(CELL_GATE_LABEL.pass)}
          tone={cell.dbStale ? 'warning' : undefined}
        />
      )
    case 'warn':
    case 'block':
    case 'error':
      return (
        <GateBadge
          result={cell.state}
          label={t(CELL_GATE_LABEL[cell.state])}
          title={cell.state === 'error' ? t('securityDashboardPage.detail.error') : undefined}
        />
      )
    case 'unrecorded':
      // 점선 테두리 — 단계는 있는데 main 결과가 아직 없다. 통과도 실패도 아니다.
      return (
        <Badge
          pill
          className="border border-dashed border-[var(--color-border-hover)] bg-transparent text-[var(--color-text-secondary)]"
          title={t('securityDashboardPage.detail.unrecorded')}
        >
          {t('securityDashboardPage.cell.unrecorded')}
        </Badge>
      )
    case 'not_applied':
    default:
      return (
        <span
          role="img"
          aria-label={t('securityDashboardPage.cell.notApplied')}
          title={t('securityDashboardPage.detail.notApplied')}
          className="text-[13px] text-[var(--color-text-muted)]"
        >
          —
        </span>
      )
  }
}

/** CRITICAL · HIGH 두 칸만(`C2 H5`). 행에서는 그 둘이 조치를 가른다 — 나머지는 상세 패널에 있다. */
function CriticalHigh({ critical, high }: { critical: number; high: number }) {
  const { t } = useTranslation()
  const chip = 'inline-flex items-center rounded-[var(--radius-sm)] px-1 py-0.5 text-[11px] font-semibold tabular-nums'
  const zero = 'text-[var(--color-text-muted)]'
  return (
    <span className="inline-flex items-center gap-0.5">
      <span
        aria-label={`${t('common.vulnerability.critical')} ${critical}`}
        title={`${t('common.vulnerability.critical')} ${critical}`}
        className={cn(chip, critical > 0 ? 'text-[var(--color-error)]' : zero)}
      >
        <span aria-hidden="true">C{critical}</span>
      </span>
      <span
        aria-label={`${t('common.vulnerability.high')} ${high}`}
        title={`${t('common.vulnerability.high')} ${high}`}
        className={cn(chip, high > 0 ? 'text-[var(--color-warning)]' : zero)}
      >
        <span aria-hidden="true">H{high}</span>
      </span>
    </span>
  )
}

/** 셀 안의 짧은 숫자. 시크릿 = 탐지 건수, 의존성 · 이미지 = C/H, SAST = 취약 · 핫스팟. */
export function SecurityCellNumbers({ stage, cell }: { stage: SecurityStage; cell: SecurityCell }) {
  const { t } = useTranslation()
  if (cell.state === 'not_applied' || cell.state === 'unrecorded') return null
  switch (stage) {
    case 'secret_scan':
      if (cell.findings === undefined || cell.findings === 0) return null
      return (
        <span className="text-[11px] font-semibold text-[var(--color-error)]">
          {t('securityDashboardPage.cell.findings', { count: cell.findings })}
        </span>
      )
    case 'dependency_scan':
    case 'image_scan':
      if (!cell.counts || isNoTargetsPass(stage, cell)) return null
      return <CriticalHigh critical={cell.counts.critical} high={cell.counts.high} />
    case 'sast': {
      const parts: string[] = []
      if (cell.vulnerabilities !== undefined) {
        parts.push(t('securityDashboardPage.cell.vulnerabilities', { count: cell.vulnerabilities }))
      }
      if (cell.securityHotspots !== undefined) {
        parts.push(t('securityDashboardPage.cell.hotspots', { count: cell.securityHotspots }))
      }
      if (parts.length === 0) return null
      return <span className="text-[11px] text-[var(--color-text-secondary)]">{parts.join(' · ')}</span>
    }
    default:
      return null
  }
}

/**
 * 취약점 DB 가 낡았다는 배지. 날짜를 알면 나이(`DB 34d`)를, 모르면 "DB 오래됨" 을 적는다.
 * 이미지 · 의존성 스캔에만 해당한다.
 */
export function CellDbBadge({ cell, now }: { cell: SecurityCell; now?: number }) {
  const { t } = useTranslation()
  if (!cell.dbStale) return null
  const days = daysSince(cell.dbUpdatedAt, now)
  return (
    <DbStaleBadge
      label={days !== undefined ? t('securityDashboardPage.cell.dbAge', { days }) : undefined}
      title={t('cicdListPage.imageScan.dbStaleHint')}
    />
  )
}

/** 표의 셀 전체: 상태 · 숫자 · DB · (최신 실행이 아니면) 흐린 실행 번호. */
export function SecurityCellView({
  stage,
  cell,
  latestVersion,
}: {
  stage: SecurityStage
  cell: SecurityCell
  /** 앱의 최신 실행 번호. 툴팁에서 "최근 실행 #42 에는 이 단계의 결과가 없다" 를 말한다. */
  latestVersion?: string
}) {
  const { t } = useTranslation()
  const hasDb = stage === 'dependency_scan' || stage === 'image_scan'
  return (
    <span className="inline-flex flex-wrap items-center gap-1" data-cell-state={cell.state}>
      <SecurityCellBadge stage={stage} cell={cell} />
      <SecurityCellNumbers stage={stage} cell={cell} />
      {hasDb && <CellDbBadge cell={cell} />}
      {cell.staleRun && cell.version && (
        <span
          className="text-[11px] text-[var(--color-text-muted)]"
          title={t('securityDashboardPage.cell.staleRun', {
            latest: latestVersion ?? '—',
            version: cell.version,
          })}
        >
          {cell.version}
        </span>
      )}
    </span>
  )
}

