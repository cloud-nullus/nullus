// 심각도별 취약점 건수.
//
// 파이프라인 실행 이력(cicd)과 스택의 설치 이미지 보고(stack)가 같은 모양으로 쓴다.
// 한 모듈 안에 두면 다른 모듈이 그 내부를 import 해야 하므로 shared 에 둔다.
//
// counts 가 없으면 건수를 모른다는 뜻이다(스캔 오류, 리포트 누락). 그 자리에 0 을
// 그리면 "취약점 없음" 으로 읽히므로 숫자 대신 "건수 모름" 을 보여준다.
//
// 0 인 심각도에는 경보색을 쓰지 않는다. 다섯 칸이 모두 빨강·주황이면 색이 뜻을
// 잃는다 — 색은 실제로 건수가 있는 칸에만 준다.

import { useTranslation } from 'react-i18next'
import { cn } from '../../lib/utils'
import { VULNERABILITY_SEVERITIES } from '../../lib/vulnerability-counts'
import type { VulnerabilityCounts } from '../../types'

const SEVERITY_STYLE: Record<keyof VulnerabilityCounts, { short: string; className: string }> = {
  critical: {
    short: 'C',
    className: 'bg-[color-mix(in_srgb,_var(--color-error)_18%,_transparent)] text-[var(--color-error)]',
  },
  high: {
    short: 'H',
    className: 'bg-[color-mix(in_srgb,_var(--color-warning)_18%,_transparent)] text-[var(--color-warning)]',
  },
  medium: {
    short: 'M',
    className: 'bg-[color-mix(in_srgb,_var(--color-warning)_10%,_transparent)] text-[var(--color-text-primary)]',
  },
  low: {
    short: 'L',
    className: 'bg-[color-mix(in_srgb,_var(--color-info)_12%,_transparent)] text-[var(--color-text-primary)]',
  },
  unknown: {
    short: 'U',
    className: 'bg-[color-mix(in_srgb,_var(--color-text-secondary)_15%,_transparent)] text-[var(--color-text-primary)]',
  },
}

const CHIP_CLASS = 'inline-flex items-center gap-1 rounded-[var(--radius-sm)] px-1.5 py-0.5 text-[11px]'
const ZERO_CLASS = 'bg-[color-mix(in_srgb,_var(--color-text-primary)_5%,_transparent)] text-[var(--color-text-muted)]'

interface SeverityCountsProps {
  /** 없으면 건수를 모른다. 0 이 아니다. */
  counts: VulnerabilityCounts | undefined
  /** 목록 행처럼 좁은 자리. 머리글자(C/H/M/L)만 쓰고, 0 인 Unknown 심각도는 생략한다. */
  compact?: boolean
  className?: string
}

export function SeverityCounts({ counts, compact = false, className }: SeverityCountsProps) {
  const { t } = useTranslation()

  if (!counts) {
    return (
      <span
        className={cn(
          CHIP_CLASS,
          'border border-dashed border-[var(--color-border-default)] text-[var(--color-text-secondary)]',
          className,
        )}
      >
        {t('common.vulnerability.countsUnknown')}
      </span>
    )
  }

  const severities = VULNERABILITY_SEVERITIES.filter(
    (severity) => !compact || severity !== 'unknown' || counts.unknown > 0,
  )

  return (
    <span className={cn('inline-flex flex-wrap items-center gap-1', className)}>
      {severities.map((severity) => {
        const label = t(`common.vulnerability.${severity}`)
        const value = counts[severity]
        const style = SEVERITY_STYLE[severity]
        // 머리글자만 보이는 compact 에서도 스크린리더와 툴팁은 전체 이름을 읽는다.
        return (
          <span
            key={severity}
            aria-label={`${label} ${value}`}
            title={`${label} ${value}`}
            className={cn(CHIP_CLASS, value > 0 ? style.className : ZERO_CLASS)}
          >
            <span aria-hidden="true">{compact ? style.short : label}</span>
            <span aria-hidden="true" className="font-semibold tabular-nums">
              {value}
            </span>
          </span>
        )
      })}
    </span>
  )
}
