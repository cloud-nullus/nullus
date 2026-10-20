// 보안 단계의 판정 배지 — 시크릿 탐지 · 의존성 스캔 · SAST · 이미지 스캔이 같은 어휘를 쓴다.
//
// 처음에는 cicd 기능 폴더에 이미지 스캔(ImageScanGateBadge)과 SAST(SASTGateBadge) 배지가
// 따로 있었다. 판정 어휘(pass · warn · block · error)도 색 규칙도 같은데 두 벌이었고, 보안
// 대시보드(관측성)가 네 종류를 한 줄에 놓으면서 관측 기능이 cicd 기능 폴더를 import 해야
// 하는 상황이 됐다. 그래서 shared 로 옮겨 한 벌로 합쳤다(Security Dashboard 설계 §6).
//
// 색 규칙
//   - block 은 위험색, warn 은 경고색, pass 는 성공색이다.
//   - error 는 스캔을 하지 못했다는 뜻이라 중립색이다. 초록이면 통과로, 빨강이면
//     취약점 발견으로 읽힌다 — 둘 다 사실이 아니다.
//   - 취약점 DB 가 오래됐으면(db_stale) pass 라도 초록을 쓰지 않는다. 최근 취약점을 모르는
//     DB 로 얻은 통과는 깨끗한 통과가 아니다.
//   - 의존성 스캔이 검사 대상 0 개로 통과했으면 초록이 아니라 "대상 없음" 이다. 락파일이 없어
//     아무것도 검사하지 않은 것을 통과로 보이지 않는다.

import { useTranslation } from 'react-i18next'
import { CircleAlert, ShieldAlert, ShieldCheck, ShieldX, type LucideIcon } from 'lucide-react'
import { iconProps } from '../ui/icon'
import { Badge } from '../ui/badge'
import { StatusIcon } from '../ui/status-icon'
import type { ImageScanGateResult } from '../../types'

/** 판정 어휘. 이미지 스캔이 처음 정했고 네 단계가 같이 쓴다. */
export type GateResult = ImageScanGateResult

export const TONE_CLASS = {
  success: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]',
  warning: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]',
  danger: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]',
  neutral:
    'bg-[color-mix(in_srgb,_var(--color-text-secondary)_15%,_transparent)] text-[var(--color-text-secondary)]',
} as const

export type GateTone = keyof typeof TONE_CLASS

export const GATE_TONE: Record<GateResult, GateTone> = {
  pass: 'success',
  warn: 'warning',
  block: 'danger',
  error: 'neutral',
}

const GATE_ICON: Record<GateResult, LucideIcon> = {
  pass: ShieldCheck,
  warn: ShieldAlert,
  block: ShieldX,
  error: CircleAlert,
}

/**
 * 판정 하나. 라벨은 호출부가 정한다 — 같은 block 이라도 이미지 스캔은 "정책으로 차단",
 * SAST 는 "Quality Gate 로 차단" 처럼 무엇에 걸렸는지가 다르다.
 */
export function GateBadge({
  result,
  label,
  tone,
  title,
}: {
  result: GateResult
  label: string
  /** 판정의 기본 색을 덮는다(DB 가 낡은 통과, 대상 없는 통과). */
  tone?: GateTone
  title?: string
}) {
  // 모르는 판정은 판정 불가로 그린다. 초록으로 떨어뜨리면 통과로 읽힌다.
  const Icon = GATE_ICON[result] ?? CircleAlert
  const resolvedTone = tone ?? GATE_TONE[result] ?? 'neutral'
  return (
    <Badge pill className={TONE_CLASS[resolvedTone]} title={title}>
      <Icon {...iconProps('xs')} />
      {label}
    </Badge>
  )
}

/** 취약점 DB 가 30일보다 오래됐거나 날짜를 모른다. */
export function DbStaleBadge({ label, title }: { label?: string; title?: string }) {
  const { t } = useTranslation()
  return (
    <Badge pill className={TONE_CLASS.warning} title={title}>
      {/* 경고 글리프는 상태 아이콘 단일 출처에서 받는다. 배지가 이미 경고색을 칠했다. */}
      <StatusIcon tone="warning" size="xs" inheritColor />
      {label ?? t('cicdListPage.imageScan.dbStale')}
    </Badge>
  )
}

const IMAGE_SCAN_LABEL: Record<GateResult, string> = {
  pass: 'cicdListPage.imageScan.gate.pass',
  warn: 'cicdListPage.imageScan.gate.warn',
  block: 'cicdListPage.imageScan.gate.block',
  error: 'cicdListPage.imageScan.gate.error',
}

const SAST_LABEL: Record<GateResult, string> = {
  pass: 'cicdListPage.sast.gate.pass',
  warn: 'cicdListPage.sast.gate.warn',
  block: 'cicdListPage.sast.gate.block',
  error: 'cicdListPage.sast.gate.error',
}

const DEPENDENCY_SCAN_LABEL: Record<GateResult, string> = {
  pass: 'cicdListPage.dependencyScan.gate.pass',
  warn: 'cicdListPage.dependencyScan.gate.warn',
  block: 'cicdListPage.dependencyScan.gate.block',
  error: 'cicdListPage.dependencyScan.gate.error',
}

const SECRET_SCAN_LABEL: Record<GateResult, string> = {
  pass: 'cicdListPage.secretScan.gate.pass',
  warn: 'cicdListPage.secretScan.gate.warn',
  block: 'cicdListPage.secretScan.gate.block',
  error: 'cicdListPage.secretScan.gate.error',
}

function labelKey(map: Record<GateResult, string>, result: GateResult): string {
  return map[result] ?? map.error
}

/** 이미지 스캔 판정. DB 가 낡았으면 통과도 경고색이고 "DB 오래됨" 을 붙인다. */
export function ImageScanGateBadge({ scan }: { scan: { gateResult: GateResult; dbStale: boolean } }) {
  const { t } = useTranslation()
  const tone = scan.gateResult === 'pass' && scan.dbStale ? 'warning' : undefined
  return (
    <>
      <GateBadge result={scan.gateResult} label={t(labelKey(IMAGE_SCAN_LABEL, scan.gateResult))} tone={tone} />
      {scan.dbStale && <DbStaleBadge />}
    </>
  )
}

/** 소스 정적 분석(SonarQube Quality Gate) 판정. */
export function SASTGateBadge({ result }: { result: { gateResult: GateResult } }) {
  const { t } = useTranslation()
  return <GateBadge result={result.gateResult} label={t(labelKey(SAST_LABEL, result.gateResult))} />
}

/**
 * 의존성 스캔(Trivy fs) 판정. 이미지 스캔과 같은 DB 규칙에 "대상 없음" 이 더해진다 —
 * 검사 대상 0 개로 통과한 것은 초록이 아니다(설계 §2.2).
 */
export function DependencyScanGateBadge({
  scan,
}: {
  scan: { gateResult: GateResult; dbStale: boolean; targets?: number }
}) {
  const { t } = useTranslation()
  if (scan.gateResult === 'pass' && scan.targets === 0) {
    return (
      <GateBadge
        result="pass"
        tone="neutral"
        label={t('cicdListPage.dependencyScan.noTargets')}
        title={t('cicdListPage.dependencyScan.noTargetsHint')}
      />
    )
  }
  const tone = scan.gateResult === 'pass' && scan.dbStale ? 'warning' : undefined
  return (
    <>
      <GateBadge result={scan.gateResult} label={t(labelKey(DEPENDENCY_SCAN_LABEL, scan.gateResult))} tone={tone} />
      {scan.dbStale && <DbStaleBadge />}
    </>
  )
}

/** 시크릿 탐지(gitleaks) 판정. warn 도 유출이다 — 경고 정책이 배포를 통과시켰을 뿐이다. */
export function SecretScanGateBadge({ scan }: { scan: { gateResult: GateResult } }) {
  const { t } = useTranslation()
  return <GateBadge result={scan.gateResult} label={t(labelKey(SECRET_SCAN_LABEL, scan.gateResult))} />
}
