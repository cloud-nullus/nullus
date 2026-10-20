// 보안 대시보드의 설치 OSS 탭(설계 §4.4) — admin · devops 만 본다.
//
// 보고용이다. 앱 탭의 게이트와 달리 설치 · 배포를 막지 않고, 고칠 사람도 다르다(OSS 업그레이드).
// 그래서 숫자를 앱 쪽과 합치지 않고 탭을 나눈다.
//
// A · B · C 는 이미지 수다(취약점 건수가 아니다). 숫자는 수정판이 있는 것만 센다 — 업스트림
// 이미지는 사용자가 고칠 수 없는 경우가 많다.
//
// "스캔 안 함" 은 0 으로 보이지 않는다. 숫자 칸을 비우고 사유를 그대로 적는다. 행을 고르면
// 스택 상세의 "이미지 취약점" 탭으로 간다 — 이미지별 상세는 그 탭이 갖는다.

import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import type { ColumnDef } from '@tanstack/react-table'
import { Info } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { Badge } from '../../../components/ui/badge'
import { DataTable } from '../../../components/shared/data-table'
import { TONE_CLASS } from '../../../components/shared/gate-badge'
import { cn } from '../../../lib/utils'
import type { StackSecurityReport, StackSecurityRow } from '../types/security-posture'
import { daysSince, formatRelativeTime, shortImageRef, stackImageScansHref } from '../utils/security-posture'

const DASH = '—'

/** 스캔한 스택만 숫자를 갖는다. 그 밖(스캔 안 함 · 대기)은 숫자를 모른다. */
const isScanned = (row: StackSecurityRow) => row.state === 'scanned'

function num(row: StackSecurityRow, value: number | undefined) {
  return isScanned(row) && value !== undefined ? String(value) : DASH
}

function StateCell({ row }: { row: StackSecurityRow }) {
  const { t } = useTranslation()
  if (row.state === 'not_scanned') {
    const label =
      row.reason === 'scanner_not_installed'
        ? t('securityDashboardPage.oss.state.scannerNotInstalled')
        : t('securityDashboardPage.oss.state.notScanned')
    return (
      <Badge className={TONE_CLASS.neutral} title={row.reason || undefined}>
        {label}
      </Badge>
    )
  }
  if (row.state === 'pending') {
    return <Badge className={TONE_CLASS.neutral}>{t('securityDashboardPage.oss.state.pending')}</Badge>
  }
  if (!isScanned(row)) {
    // 모르는 상태는 서버 값 그대로 — 스캔됨으로 보이지 않게.
    return <Badge className={TONE_CLASS.neutral}>{row.state || DASH}</Badge>
  }
  return (
    <span className="inline-flex flex-wrap items-center gap-1 text-[12px]">
      <span className="text-[var(--color-text-primary)]">{t('securityDashboardPage.oss.state.scanned')}</span>
      {row.failedImages !== undefined && row.failedImages > 0 && (
        <span className="text-[var(--color-warning)]">
          · {t('securityDashboardPage.oss.state.failed', { count: row.failedImages })}
        </span>
      )}
      {row.airgap && (
        // 에어갭은 반입한 DB 로 스캔한다. 반입 뒤의 CVE 는 다음 번들을 들여야 보인다.
        <span
          className="inline-flex items-center gap-0.5 text-[var(--color-text-secondary)]"
          title={t('stackList.imageScans.airgapDB')}
        >
          · {t('securityDashboardPage.oss.state.airgap')}
          <Info {...iconProps('xs')} />
        </span>
      )}
    </span>
  )
}

export function SecurityOssTable({ report, locale }: { report: StackSecurityReport; locale: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const gradeHeader = (label: string, hint: string) => (
    <span title={hint} className="cursor-help underline decoration-dotted underline-offset-2">
      {label}
    </span>
  )

  const columns: ColumnDef<StackSecurityRow, unknown>[] = [
    {
      id: 'stack',
      header: t('securityDashboardPage.oss.columns.stack'),
      cell: ({ row }) => <span className="font-semibold">{row.original.stackName || row.original.stackId}</span>,
    },
    {
      id: 'cluster',
      header: t('securityDashboardPage.oss.columns.cluster'),
      cell: ({ row }) => (
        <span className="text-[12px] text-[var(--color-text-secondary)]">{row.original.clusterName || DASH}</span>
      ),
    },
    {
      id: 'status',
      header: t('securityDashboardPage.oss.columns.status'),
      cell: ({ row }) => <StateCell row={row.original} />,
    },
    {
      id: 'images',
      header: t('securityDashboardPage.oss.columns.images'),
      cell: ({ row }) => <span className="tabular-nums">{num(row.original, row.original.images)}</span>,
    },
    {
      id: 'gradeA',
      header: () =>
        gradeHeader(t('securityDashboardPage.oss.columns.gradeA'), t('securityDashboardPage.oss.gradeAHint')),
      cell: ({ row }) => {
        const value = row.original.gradeAImages
        return (
          <span className={cn('tabular-nums', isScanned(row.original) && value ? 'font-semibold text-[var(--color-error)]' : '')}>
            {num(row.original, value)}
          </span>
        )
      },
    },
    {
      id: 'gradeB',
      header: () =>
        gradeHeader(t('securityDashboardPage.oss.columns.gradeB'), t('securityDashboardPage.oss.gradeBHint')),
      cell: ({ row }) => {
        const value = row.original.gradeBImages
        return (
          <span className={cn('tabular-nums', isScanned(row.original) && value ? 'font-semibold text-[var(--color-error)]' : '')}>
            {num(row.original, value)}
          </span>
        )
      },
    },
    {
      id: 'gradeC',
      header: () =>
        gradeHeader(t('securityDashboardPage.oss.columns.gradeC'), t('securityDashboardPage.oss.gradeCHint')),
      cell: ({ row }) => {
        const value = row.original.gradeCImages
        return (
          <span className={cn('tabular-nums', isScanned(row.original) && value ? 'text-[var(--color-warning)]' : '')}>
            {num(row.original, value)}
          </span>
        )
      },
    },
    {
      // 수정판 있는 CRITICAL · HIGH 를 한 칸에 둔다. 칸을 나누면 표가 넓어져 오른쪽 칸이 화면 밖으로 밀린다.
      id: 'fixable',
      header: t('securityDashboardPage.oss.columns.fixable'),
      cell: ({ row }) => {
        const critical = row.original.fixable?.critical
        const high = row.original.fixable?.high
        if (!isScanned(row.original) || (critical === undefined && high === undefined)) {
          return <span className="text-[var(--color-text-muted)]">{DASH}</span>
        }
        return (
          <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-[12px] tabular-nums">
            <span className={cn(critical ? 'font-semibold text-[var(--color-error)]' : '')}>C{critical ?? DASH}</span>
            <span className={cn(high ? 'text-[var(--color-warning)]' : '')}>H{high ?? DASH}</span>
          </span>
        )
      },
    },
    {
      id: 'worst',
      header: t('securityDashboardPage.oss.columns.worst'),
      cell: ({ row }) => {
        const worst = row.original.worstImage
        if (!isScanned(row.original) || !worst) return <span className="text-[var(--color-text-muted)]">{DASH}</span>
        return (
          // 이미지 이름이 길다(레지스트리 경로 · 태그). 칸을 넓히지 않도록 이름과 건수를 두 줄로 나누고,
          // 이름은 잘라 전체를 툴팁에 둔다 — 한 줄로 이으면 표가 넓어져 마지막 스캔 · DB 칸이 화면 밖으로 밀린다.
          <span className="flex flex-col gap-0.5 text-[12px]">
            <code title={worst.image} className="block max-w-[180px] truncate font-mono">
              {shortImageRef(worst.image)}
            </code>
            <span className="inline-flex items-center gap-1.5">
              {worst.critical !== undefined && (
                <span className="font-semibold text-[var(--color-error)]">C{worst.critical}</span>
              )}
              {worst.high !== undefined && <span className="text-[var(--color-warning)]">H{worst.high}</span>}
              {worst.exposed && (
                <Badge className={TONE_CLASS.danger}>{t('securityDashboardPage.oss.exposed')}</Badge>
              )}
            </span>
          </span>
        )
      },
    },
    {
      // 마지막 스캔과 취약점 DB 나이를 한 칸 두 줄로 둔다 — 둘 다 "이 숫자가 얼마나 새것인가" 다.
      id: 'lastScan',
      header: t('securityDashboardPage.oss.columns.lastScan'),
      cell: ({ row }) => {
        if (!isScanned(row.original) || !row.original.scannedAt) {
          return <span className="text-[var(--color-text-muted)]">{DASH}</span>
        }
        const days = daysSince(row.original.dbUpdatedAt)
        return (
          <span className="flex flex-col gap-0.5 whitespace-nowrap text-[12px] text-[var(--color-text-secondary)]">
            <span>{formatRelativeTime(row.original.scannedAt, locale)}</span>
            {days !== undefined && (
              <span
                className={cn(row.original.dbStale && 'font-semibold text-[var(--color-warning)]')}
                title={row.original.dbStale ? t('cicdListPage.imageScan.dbStaleHint') : undefined}
              >
                {row.original.airgap
                  ? t('securityDashboardPage.oss.dbImported', { days })
                  : t('securityDashboardPage.oss.dbAge', { days })}
              </span>
            )}
          </span>
        )
      },
    },
  ]

  return (
    <DataTable
      flush
      columns={columns}
      data={report.stacks}
      getRowKey={(row) => row.stackId}
      onRowClick={(row) => navigate(stackImageScansHref(row.stackId))}
      emptyMessage={t('securityDashboardPage.oss.empty.noStacks')}
      toolbar={
        <div className="flex w-full flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-[var(--color-text-secondary)]">
          <span className="font-semibold text-[var(--color-text-primary)]">
            {t('securityDashboardPage.oss.summary', {
              scanned: report.summary.scannedStacks ?? DASH,
              notScanned: report.summary.notScannedStacks ?? DASH,
            })}
          </span>
          <span>{t('securityDashboardPage.oss.reportOnly')}</span>
          <span className="ml-auto">{t('securityDashboardPage.oss.openHint')}</span>
        </div>
      }
    />
  )
}
