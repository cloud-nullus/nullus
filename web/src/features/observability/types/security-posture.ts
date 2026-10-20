// 보안 대시보드의 화면 모델(Security Dashboard 설계 §2 · §5.2).
//
// 두 축을 섞지 않는다 — 앱(파이프라인 보안, 배포를 막는 게이트)과 설치 OSS(스택 이미지,
// 막지 않는 보고)는 응답도 타입도 따로다. 숫자를 합치지 않는다.
//
// 숫자 칸은 모르면 undefined 다. 응답에서 빠졌거나 null 인 숫자를 0 으로 채우면 "문제 0건"
// 으로 읽힌다 — 이 화면이 가장 피해야 하는 오해다.

import type { VulnerabilityCounts } from '../../../types'

/** 앱 표의 열 — 파이프라인 단계 순서(시크릿 → 의존성 → SAST → 이미지)를 따른다. */
export type SecurityStage = 'secret_scan' | 'dependency_scan' | 'sast' | 'image_scan'

export const SECURITY_STAGES: readonly SecurityStage[] = ['secret_scan', 'dependency_scan', 'sast', 'image_scan']

/**
 * 셀 상태(설계 §2.2). 뒤의 셋은 초록이 아니다.
 * - error: 판정 불가(스캐너 장애 · 리포트 없음)
 * - unrecorded: 단계는 있는데 main 결과가 아직 없다
 * - not_applied: 파이프라인에 단계가 없다(시크릿은 스택 정책 off 도)
 */
export type SecurityCellState = 'pass' | 'warn' | 'block' | 'error' | 'unrecorded' | 'not_applied'

export interface SecretRuleCount {
  ruleId: string
  count: number
}

/** 셀 하나 = 그 앱 main 의 그 단계 최신 기록. 앱의 최신 실행과 다를 수 있다(staleRun). */
export interface SecurityCell {
  state: SecurityCellState
  deploymentId?: string
  /** 실행 번호 표기("#38"). */
  version?: string
  at?: string
  /** 이 셀의 실행이 앱의 최신 실행이 아니다 — 최신 실행에서는 이 단계의 결과가 없다. */
  staleRun: boolean
  /** 시크릿 탐지 건수. */
  findings?: number
  rules?: SecretRuleCount[]
  /** 의존성 스캔의 검사 대상 수. 0 이면 통과여도 "대상 없음" 이다. */
  targets?: number
  /** 의존성 · 이미지의 심각도 건수(정책 기준). */
  counts?: VulnerabilityCounts
  dbStale: boolean
  /** DB 날짜. 계약에는 없지만 오면 "DB 34d" 처럼 나이를 적는다. */
  dbUpdatedAt?: string
  vulnerabilities?: number
  securityHotspots?: number
  failedConditions?: string[]
  image?: string
  /** 리포트(시크릿 · 의존성 · 이미지) 또는 SonarQube(SAST) 주소. */
  link?: string
}

export interface SecurityLatestRun {
  deploymentId: string
  version: string
  status: string
  startedAt?: string
}

/** 앱 한 행 = 파이프라인 하나. rank · 순서는 서버가 정한다 — 화면은 받은 순서를 그대로 쓴다. */
export interface SecurityApp {
  pipelineId: string
  name: string
  /** '' 이면 스택 없는 앱(직접 배포 · 스택이 지워진 앱). */
  stackId: string
  rank: number
  latestRun: SecurityLatestRun | null
  cells: Record<SecurityStage, SecurityCell>
}

/** 스택 하나의 묶음 머리. 범위 필터만 따르고 "문제 있는 것만" 은 따르지 않는다. */
export interface SecurityAppGroup {
  stackId: string
  stackName: string
  clusterId: string
  clusterName: string
  apps?: number
  blocked?: number
  secrets?: number
  critical?: number
  undetermined?: number
  problems?: number
}

/** KPI M1–M4 의 재료. */
export interface SecurityAppsSummary {
  apps?: number
  withSecurityStage?: number
  blocked?: number
  secrets?: number
  critical?: number
  undetermined?: number
}

export interface SecurityAppsReport {
  generatedAt?: string
  /** 앱 상한(1000)에 닿았다. 화면이 알려야 한다. */
  truncated: boolean
  summary: SecurityAppsSummary
  /** 단계별 적용 앱 수 — 열 머리에 `SAST 12/24` 로 보인다. */
  coverage: Partial<Record<SecurityStage, number>>
  groups: SecurityAppGroup[]
  apps: SecurityApp[]
}

/** 설치 OSS — 스택 하나의 스캔 상태. scanned · pending · not_scanned(그 밖은 서버 값 그대로). */
export type StackScanState = 'scanned' | 'pending' | 'not_scanned' | (string & {})

export interface StackWorstImage {
  image: string
  critical?: number
  high?: number
  exposed: boolean
}

export interface StackSecurityRow {
  stackId: string
  stackName: string
  clusterId: string
  clusterName: string
  state: StackScanState
  /** not_scanned 의 사유(scanner_not_installed …). */
  reason: string
  airgap: boolean
  images?: number
  failedImages?: number
  sharedImages?: number
  /** A: 수정판 있는 CRITICAL + 외부 노출 · B: 같은 것 + 내부 · C: CRITICAL 없이 수정판 있는 HIGH(이미지 수). */
  gradeAImages?: number
  gradeBImages?: number
  gradeCImages?: number
  /** 수정판이 있는 것만 센 건수. */
  fixable?: VulnerabilityCounts
  worstImage: StackWorstImage | null
  scannedAt?: string
  dbUpdatedAt?: string
  dbStale: boolean
}

/** KPI M5 의 재료. */
export interface StackSecuritySummary {
  stacks?: number
  scannedStacks?: number
  notScannedStacks?: number
  actionImages?: number
  gradeAImages?: number
  gradeBImages?: number
}

export interface StackSecurityReport {
  generatedAt?: string
  summary: StackSecuritySummary
  stacks: StackSecurityRow[]
}

/** 앱 축 조회 범위. 비어 있으면 조직 전체다. stackId 'none' 은 스택 없는 앱이다. */
export interface SecurityAppsQuery {
  clusterId?: string
  stackId?: string
  q?: string
  problemOnly: boolean
}

export interface SecurityStacksQuery {
  clusterId?: string
  stackId?: string
}
