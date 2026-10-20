// 보안 대시보드의 순수 함수 — 응답 정규화 · 묶기 · 딥링크 · 표기.
//
// 판정과 순서는 서버 도메인이 정한다(rank · 정렬, 설계 §2.3). 여기서는 받은 것을 화면 모델로
// 옮기고 묶기만 한다 — 화면이 rank 를 다시 계산하기 시작하면 서버와 화면의 "문제 있는 앱" 이
// 서로 달라진다.

import { normalizeVulnerabilityCounts } from '../../../lib/vulnerability-counts'
import {
  SECURITY_STAGES,
  type SecretRuleCount,
  type SecurityApp,
  type SecurityAppGroup,
  type SecurityAppsQuery,
  type SecurityAppsReport,
  type SecurityCell,
  type SecurityCellState,
  type SecurityStage,
  type SecurityStacksQuery,
  type StackSecurityReport,
  type StackSecurityRow,
  type StackWorstImage,
} from '../types/security-posture'

type Raw = Record<string, unknown>

const isObject = (value: unknown): value is Raw =>
  value !== null && typeof value === 'object' && !Array.isArray(value)

const asObject = (value: unknown): Raw => (isObject(value) ? value : {})

/** 숫자일 때만 옮긴다. 빠졌거나 null 이면 모른다 — 0 이 아니다. */
const optionalNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined

const optionalString = (value: unknown): string | undefined =>
  typeof value === 'string' && value !== '' ? value : undefined

const stringOf = (value: unknown): string => (typeof value === 'string' ? value : '')

const CELL_STATES: ReadonlySet<string> = new Set([
  'pass',
  'warn',
  'block',
  'error',
  'unrecorded',
  'not_applied',
])

/**
 * 셀 하나. 모르는 state(또는 셀이 통째로 빠진 경우)는 error(판정 불가)로 둔다 — pass 로
 * 떨어뜨리면 초록 통과로 보이고, not_applied 로 떨어뜨리면 단계가 없는 것으로 보인다.
 */
export function normalizeSecurityCell(raw: unknown): SecurityCell {
  const cell = asObject(raw)
  const state = String(cell.state ?? '')
  const rules = Array.isArray(cell.rules)
    ? cell.rules.filter(isObject).map(
        (rule): SecretRuleCount => ({
          ruleId: stringOf(rule.rule_id ?? rule.ruleId),
          count: optionalNumber(rule.count) ?? 0,
        }),
      )
    : undefined
  const failedConditions = Array.isArray(cell.failed_conditions)
    ? cell.failed_conditions.filter((c): c is string => typeof c === 'string')
    : undefined
  return {
    state: CELL_STATES.has(state) ? (state as SecurityCellState) : 'error',
    deploymentId: optionalString(cell.deployment_id),
    version: optionalString(cell.version),
    at: optionalString(cell.at),
    staleRun: cell.stale_run === true,
    findings: optionalNumber(cell.findings),
    rules,
    targets: optionalNumber(cell.targets),
    counts: normalizeVulnerabilityCounts(cell.counts),
    dbStale: cell.db_stale === true,
    dbUpdatedAt: optionalString(cell.db_updated_at),
    vulnerabilities: optionalNumber(cell.vulnerabilities),
    securityHotspots: optionalNumber(cell.security_hotspots),
    failedConditions,
    image: optionalString(cell.image),
    link: optionalString(cell.link),
  }
}

function normalizeApp(raw: Raw): SecurityApp {
  const cells = asObject(raw.cells)
  const latest = isObject(raw.latest_run) ? raw.latest_run : null
  return {
    pipelineId: stringOf(raw.pipeline_id),
    name: stringOf(raw.name),
    stackId: stringOf(raw.stack_id),
    rank: optionalNumber(raw.rank) ?? 0,
    latestRun: latest
      ? {
          deploymentId: stringOf(latest.deployment_id),
          version: stringOf(latest.version),
          status: stringOf(latest.status),
          startedAt: optionalString(latest.started_at),
        }
      : null,
    cells: Object.fromEntries(
      SECURITY_STAGES.map((stage) => [stage, normalizeSecurityCell(cells[stage])]),
    ) as Record<SecurityStage, SecurityCell>,
  }
}

function normalizeGroup(raw: Raw): SecurityAppGroup {
  return {
    stackId: stringOf(raw.stack_id),
    stackName: stringOf(raw.stack_name),
    clusterId: stringOf(raw.cluster_id),
    clusterName: stringOf(raw.cluster_name),
    apps: optionalNumber(raw.apps),
    blocked: optionalNumber(raw.blocked),
    secrets: optionalNumber(raw.secrets),
    critical: optionalNumber(raw.critical),
    undetermined: optionalNumber(raw.undetermined),
    problems: optionalNumber(raw.problems),
  }
}

/** GET /observability/security/apps 응답을 화면 모델로. */
export function normalizeSecurityAppsReport(raw: unknown): SecurityAppsReport {
  const body = asObject(raw)
  const summary = asObject(body.summary)
  const coverage = asObject(body.coverage)
  return {
    generatedAt: optionalString(body.generated_at),
    truncated: body.truncated === true,
    summary: {
      apps: optionalNumber(summary.apps),
      withSecurityStage: optionalNumber(summary.with_security_stage),
      blocked: optionalNumber(summary.blocked),
      secrets: optionalNumber(summary.secrets),
      critical: optionalNumber(summary.critical),
      undetermined: optionalNumber(summary.undetermined),
    },
    coverage: Object.fromEntries(
      SECURITY_STAGES.flatMap((stage) => {
        const value = optionalNumber(coverage[stage])
        return value === undefined ? [] : [[stage, value]]
      }),
    ),
    groups: (Array.isArray(body.groups) ? body.groups : []).filter(isObject).map(normalizeGroup),
    apps: (Array.isArray(body.apps) ? body.apps : []).filter(isObject).map(normalizeApp),
  }
}

function normalizeWorstImage(raw: unknown): StackWorstImage | null {
  if (!isObject(raw)) return null
  const image = stringOf(raw.image)
  if (!image) return null
  return {
    image,
    critical: optionalNumber(raw.critical),
    high: optionalNumber(raw.high),
    exposed: raw.exposed === true,
  }
}

function normalizeStackRow(raw: Raw): StackSecurityRow {
  return {
    stackId: stringOf(raw.stack_id),
    stackName: stringOf(raw.stack_name),
    clusterId: stringOf(raw.cluster_id),
    clusterName: stringOf(raw.cluster_name),
    state: stringOf(raw.state),
    reason: stringOf(raw.reason),
    airgap: raw.airgap === true,
    images: optionalNumber(raw.images),
    failedImages: optionalNumber(raw.failed_images),
    sharedImages: optionalNumber(raw.shared_images),
    gradeAImages: optionalNumber(raw.grade_a_images),
    gradeBImages: optionalNumber(raw.grade_b_images),
    gradeCImages: optionalNumber(raw.grade_c_images),
    fixable: normalizeVulnerabilityCounts(raw.fixable),
    worstImage: normalizeWorstImage(raw.worst_image),
    scannedAt: optionalString(raw.scanned_at),
    dbUpdatedAt: optionalString(raw.db_updated_at),
    dbStale: raw.db_stale === true,
  }
}

/** GET /observability/security/stacks 응답을 화면 모델로. */
export function normalizeStackSecurityReport(raw: unknown): StackSecurityReport {
  const body = asObject(raw)
  const summary = asObject(body.summary)
  return {
    generatedAt: optionalString(body.generated_at),
    summary: {
      stacks: optionalNumber(summary.stacks),
      scannedStacks: optionalNumber(summary.scanned_stacks),
      notScannedStacks: optionalNumber(summary.not_scanned_stacks),
      actionImages: optionalNumber(summary.action_images),
      gradeAImages: optionalNumber(summary.grade_a_images),
      gradeBImages: optionalNumber(summary.grade_b_images),
    },
    stacks: (Array.isArray(body.stacks) ? body.stacks : []).filter(isObject).map(normalizeStackRow),
  }
}

/** 조회 파라미터. 빈 값은 보내지 않는다 — 빈 cluster_id 는 "전체" 이고 서버 기본값과 같다. */
export function securityAppsParams(query: SecurityAppsQuery): Record<string, string> {
  const params: Record<string, string> = {}
  if (query.clusterId) params.cluster_id = query.clusterId
  if (query.stackId) params.stack_id = query.stackId
  const q = query.q?.trim()
  if (q) params.q = q
  if (query.problemOnly) params.problem_only = 'true'
  return params
}

export function securityStacksParams(query: SecurityStacksQuery): Record<string, string> {
  const params: Record<string, string> = {}
  if (query.clusterId) params.cluster_id = query.clusterId
  if (query.stackId) params.stack_id = query.stackId
  return params
}

/** "문제 있는 것만" 이 남기는 rank(설계 §2.3: 1~4). */
export const isProblemRank = (rank: number) => rank >= 1 && rank <= 4

export interface SecurityAppSection {
  group: SecurityAppGroup
  apps: SecurityApp[]
}

/**
 * 앱을 스택 묶음으로 나눈다. 묶음 순서는 서버의 groups 순서, 묶음 안의 앱 순서는 서버의
 * apps 순서(rank · 정렬) 그대로다.
 *
 * groups 에 없는 스택의 앱(서버가 묶음을 빠뜨린 경우)은 버리지 않고 뒤에 묶음을 만들어
 * 붙인다 — 앱이 화면에서 조용히 사라지는 것이 묶음 머리 숫자가 비는 것보다 나쁘다.
 */
export function groupAppsByStack(report: Pick<SecurityAppsReport, 'groups' | 'apps'>): SecurityAppSection[] {
  const byStack = new Map<string, SecurityApp[]>()
  for (const app of report.apps) {
    const list = byStack.get(app.stackId)
    if (list) list.push(app)
    else byStack.set(app.stackId, [app])
  }
  const sections: SecurityAppSection[] = report.groups.map((group) => ({
    group,
    apps: byStack.get(group.stackId) ?? [],
  }))
  const known = new Set(report.groups.map((group) => group.stackId))
  for (const [stackId, apps] of byStack) {
    if (known.has(stackId)) continue
    sections.push({
      group: { stackId, stackName: '', clusterId: '', clusterName: '', apps: undefined, problems: undefined },
      apps,
    })
  }
  return sections
}

/**
 * 묶음을 처음에 펼칠지. 문제가 있는 묶음은 열고, 없는 묶음은 접는다(설계 §4.2).
 * 문제 수를 모르면 연다 — 접어 두면 모르는 것이 "문제 없음" 처럼 숨는다.
 */
export const isGroupOpenByDefault = (group: SecurityAppGroup) =>
  group.problems === undefined || group.problems > 0

/**
 * "문제 있는 것만" 이 가린 앱 수. 서버가 거른 뒤라 범위 전체 수(summary.apps)와 받은 수의
 * 차이다. 전체 수를 모르면 셀 수 없다(undefined) — 0 으로 두면 "가린 것 없음" 으로 읽힌다.
 */
export function hiddenAppCount(report: Pick<SecurityAppsReport, 'summary' | 'apps'>, problemOnly: boolean) {
  if (!problemOnly) return 0
  if (report.summary.apps === undefined) return undefined
  return Math.max(0, report.summary.apps - report.apps.length)
}

export interface ScopeOption {
  id: string
  name: string
  clusterId?: string
}

/**
 * 범위 필터의 선택지. 앱 응답의 묶음(과 admin · devops 면 설치 OSS 응답의 스택)에서 만든다.
 *
 * 기존 ClusterStackFilter 는 /admin/clusters · /stacks 를 읽는데 둘 다 개발자에게 닫혀 있다
 * (/admin 은 admin 만). 보안 대시보드는 세 역할이 다 쓰므로 이 화면이 이미 받은 응답에서
 * 선택지를 만든다 — 조직 범위도 서버가 이미 걸렀다.
 */
export function buildScopeOptions(
  groups: SecurityAppGroup[],
  stacks: StackSecurityRow[] = [],
): { clusters: ScopeOption[]; stacks: ScopeOption[]; hasNoStack: boolean } {
  const clusters = new Map<string, ScopeOption>()
  const stackOptions = new Map<string, ScopeOption>()
  let hasNoStack = false
  const add = (stackId: string, stackName: string, clusterId: string, clusterName: string) => {
    if (clusterId && !clusters.has(clusterId)) {
      clusters.set(clusterId, { id: clusterId, name: clusterName || clusterId })
    }
    if (!stackId) {
      hasNoStack = true
      return
    }
    if (!stackOptions.has(stackId)) {
      stackOptions.set(stackId, { id: stackId, name: stackName || stackId, clusterId: clusterId || undefined })
    }
  }
  for (const group of groups) add(group.stackId, group.stackName, group.clusterId, group.clusterName)
  for (const stack of stacks) add(stack.stackId, stack.stackName, stack.clusterId, stack.clusterName)
  const byName = (a: ScopeOption, b: ScopeOption) => a.name.localeCompare(b.name)
  return {
    clusters: [...clusters.values()].sort(byName),
    stacks: [...stackOptions.values()].sort(byName),
    hasNoStack,
  }
}

/** CI/CD 실행 기록 딥링크(설계 §4.3 · §8 G5). 실행을 모르면 파이프라인의 기록 탭까지만 연다. */
export function runHistoryHref(pipelineId: string, deploymentId?: string): string {
  const params = new URLSearchParams({ pipeline: pipelineId, tab: 'history' })
  if (deploymentId) params.set('run', deploymentId)
  return `/cicd/list?${params.toString()}`
}

/** 스택 상세의 "이미지 취약점" 탭 딥링크(설계 §4.4). */
export function stackImageScansHref(stackId: string): string {
  const params = new URLSearchParams({ stack: stackId, tab: 'image-scans' })
  return `/stack/list?${params.toString()}`
}

/** 외부 링크는 http(s) 만 쓴다. CI 가 준 값이라 javascript: 같은 것이 올 수 있다. */
export function safeExternalHref(uri: string | undefined): string | undefined {
  return uri && /^https?:\/\//i.test(uri) ? uri : undefined
}

const DAY_MS = 24 * 60 * 60 * 1000

/** 날짜로부터 지난 일수. 모르면 undefined. */
export function daysSince(iso: string | undefined, now: number = Date.now()): number | undefined {
  if (!iso) return undefined
  const at = new Date(iso).getTime()
  if (Number.isNaN(at)) return undefined
  return Math.max(0, Math.floor((now - at) / DAY_MS))
}

/** "2시간 전" · "2 hours ago". 화면 언어를 따른다. */
export function formatRelativeTime(iso: string | undefined, locale: string, now: number = Date.now()): string {
  if (!iso) return '—'
  const at = new Date(iso).getTime()
  if (Number.isNaN(at)) return '—'
  const diffSeconds = Math.round((at - now) / 1000)
  const abs = Math.abs(diffSeconds)
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  if (abs < 60) return rtf.format(diffSeconds, 'second')
  if (abs < 3600) return rtf.format(Math.round(diffSeconds / 60), 'minute')
  if (abs < 86400) return rtf.format(Math.round(diffSeconds / 3600), 'hour')
  return rtf.format(Math.round(diffSeconds / 86400), 'day')
}

/** 이미지 참조가 길다(레지스트리 경로 + 다이제스트). 좁은 자리에는 마지막 경로 조각만. */
export function shortImageRef(image: string): string {
  const [name, digest] = image.split('@', 2)
  const shortDigest = digest && digest.length > 19 ? `${digest.slice(0, 19)}…` : digest
  const parts = name.split('/')
  const last = parts[parts.length - 1] || name
  return shortDigest ? `${last}@${shortDigest}` : last
}
