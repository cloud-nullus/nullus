import { type ReactNode, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { CircleAlert, Flame, Info, KeyRound, Package, ShieldCheck, ShieldX } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { StatusIcon } from '../../../components/ui/status-icon'
import { PageHeader } from '../../../components/layout/page-header'
import { Tabs } from '../../../components/ui/tabs'
import { Checkbox } from '../../../components/ui/checkbox'
import { SearchInput } from '../../../components/ui/search-input'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { SkeletonTable } from '../../../components/shared/skeleton'
import { resolveLocale } from '../../../lib/locale'
import { useAuthStore } from '../../../stores/auth-store'
import { useSecurityApps, useSecurityStacks } from '../api/observability-api'
import { ClusterStackFilter } from '../components/cluster-stack-filter'
import { KpiCard } from '../components/monitoring-chart-widgets'
import { SecurityAppDetail } from '../components/security-app-detail'
import { SecurityAppsTable } from '../components/security-apps-table'
import { SecurityOssTable } from '../components/security-oss-table'
import type { SecurityApp, SecurityAppsReport } from '../types/security-posture'
import { buildScopeOptions, hiddenAppCount, MAX_SECURITY_APPS } from '../utils/security-posture'

type SecurityTab = 'apps' | 'oss'

/** 스택 없는 앱을 고르는 값. API 의 stack_id=none 과 같다. */
const NO_STACK = 'none'

const DASH = '—'

/** 검색어를 입력이 멈춘 뒤에 넘긴다 — 글자마다 조직 전체를 다시 읽지 않게. */
function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])
  return debounced
}

const percent = (part: number | undefined, whole: number | undefined) =>
  part !== undefined && whole ? (part / whole) * 100 : undefined

/**
 * 보안 대시보드 — 관측성 › 보안 › 보안 대시보드 (nullus-plan#65, 설계 문서
 * docs/11_기능설계/Nullus_Security_Dashboard_설계.md).
 *
 * 기본 범위는 조직 전체다. 두 축을 섞지 않는다 — 앱(파이프라인 보안, 배포를 막는 게이트)과
 * 설치 OSS(스택 이미지, 막지 않는 보고)를 탭으로 가르고 숫자를 합치지 않는다. 설치 OSS 는
 * admin · devops 만 본다(지금 /stacks 권한과 같다). 개발자에게는 그 탭과 M5 카드를 감추고
 * API 도 부르지 않는다.
 */
export function SecurityDashboardPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const role = useAuthStore((state) => state.role)
  const canSeeOss = role === 'admin' || role === 'devops'

  const [clusterId, setClusterId] = useState('')
  const [stackId, setStackId] = useState('')
  const [search, setSearch] = useState('')
  const q = useDebouncedValue(search.trim(), 300)
  const [problemOnly, setProblemOnly] = useState(true)
  const [tab, setTab] = useState<SecurityTab>('apps')
  const [selectedAppId, setSelectedAppId] = useState<string | null>(null)
  const [groupOpen, setGroupOpen] = useState<Record<string, boolean>>({})

  const appsQuery = useSecurityApps({ clusterId, stackId, q, problemOnly })
  // 범위 필터의 선택지 — 조직 전체 응답의 묶음에서 만든다. 기본 화면과 같은 조회라 캐시를
  // 함께 쓴다(요청이 하나 더 나가지 않는다).
  const scopeQuery = useSecurityApps({ problemOnly: true })
  // 스택 없는 앱에는 설치 OSS 가 없다. 그 범위에서는 스택 축을 부르지 않는다.
  const stacksQuery = useSecurityStacks(
    { clusterId, stackId: stackId === NO_STACK ? undefined : stackId },
    canSeeOss && stackId !== NO_STACK,
  )
  const scopeStacksQuery = useSecurityStacks({}, canSeeOss)

  const report = appsQuery.data
  const stacksReport = stackId === NO_STACK ? undefined : stacksQuery.data

  const options = useMemo(
    () => buildScopeOptions(scopeQuery.data?.groups ?? [], scopeStacksQuery.data?.stacks ?? []),
    [scopeQuery.data, scopeStacksQuery.data],
  )
  const clusterOptions = options.clusters
  const stackOptions = clusterId ? options.stacks.filter((s) => s.clusterId === clusterId) : options.stacks

  // 탭은 역할에 따라 빠진다. 개발자가 설치 OSS 탭에 머물 길이 없게 한다.
  const activeTab: SecurityTab = canSeeOss ? tab : 'apps'

  const grouped = stackId === ''
  const selectedApp = report?.apps.find((app) => app.pipelineId === selectedAppId) ?? null
  const groupNameOf = (app: SecurityApp) => {
    if (!app.stackId) return t('securityDashboardPage.noStack')
    const group = report?.groups.find((g) => g.stackId === app.stackId)
    return group?.stackName || app.stackId
  }

  const summary = report?.summary
  // 보안 단계가 하나도 없으면 M1–M4 는 잴 것이 없다. 0 으로 두면 "문제 없음" 으로 읽힌다.
  const measured = summary?.withSecurityStage !== undefined && summary.withSecurityStage > 0
  const kpiValue = (value: number | undefined) => (measured && value !== undefined ? String(value) : DASH)
  const noStagesHint = summary && !measured ? t('securityDashboardPage.kpi.noStages') : undefined

  const ossSummary = stacksReport?.summary
  const hidden = report ? hiddenAppCount(report, problemOnly) : 0
  const hasScopeFilter = clusterId !== '' || stackId !== '' || q !== ''

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('observability.securityDashboard', 'Security Dashboard') }]}
        icon={<ShieldCheck {...iconProps('sm')} />}
        tone="accent"
        title={t('observability.securityDashboard', 'Security Dashboard')}
        subtitle={t('observability.securityDashboardDesc')}
        actions={
          // 화면이 보여주는 것은 실행 기록 동기화로 들어온 main 의 결과다. 방금 끝난 실행이
          // 안 보이는 이유를 여기서 먼저 말한다.
          <span className="inline-flex items-center gap-1 text-[12px] text-[var(--color-text-secondary)]">
            <Info {...iconProps('xs')} />
            {t('securityDashboardPage.basedOn')}
          </span>
        }
      />

      <ClusterStackFilter
        allOption
        selectedClusterId={clusterId}
        selectedStackId={stackId}
        onClusterChange={(id) => {
          setClusterId(id)
          // 고른 스택이 새 클러스터에 없으면 푼다("스택 없음" 은 클러스터와 무관해 남긴다).
          if (id && stackId && stackId !== NO_STACK && !options.stacks.some((s) => s.id === stackId && s.clusterId === id)) {
            setStackId('')
          }
        }}
        onStackChange={setStackId}
        onClear={() => {
          setClusterId('')
          setStackId('')
        }}
        clusters={clusterOptions}
        filteredStacks={stackOptions}
        selectedCluster={clusterOptions.find((c) => c.id === clusterId)}
        selectedStack={stackOptions.find((s) => s.id === stackId)}
        extraStackOptions={options.hasNoStack ? [{ value: NO_STACK, label: t('securityDashboardPage.noStack') }] : []}
      >
        <SearchInput
          wrapperClassName="w-[220px]"
          aria-label={t('securityDashboardPage.searchPlaceholder')}
          placeholder={t('securityDashboardPage.searchPlaceholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <label className="mb-[7px] inline-flex cursor-pointer items-center gap-1.5 text-[13px] text-[var(--color-text-primary)]">
          <Checkbox checked={problemOnly} onChange={(event) => setProblemOnly(event.target.checked)} />
          {t('securityDashboardPage.problemsOnly')}
        </label>
      </ClusterStackFilter>

      {/* KPI — M1–M4 는 범위 필터를 따르고 "문제 있는 것만" 은 따르지 않는다(설계 §3).
          표를 줄여도 KPI 는 그대로다. M5 는 admin · devops 만. */}
      <div
        data-testid="security-kpis"
        className={`mb-5 grid grid-cols-1 gap-3 sm:grid-cols-2 ${canSeeOss ? 'xl:grid-cols-5' : 'xl:grid-cols-4'}`}
      >
        <KpiCard
          label={t('securityDashboardPage.kpi.blocked')}
          value={
            measured && summary?.blocked !== undefined
              ? `${summary.blocked} / ${summary.withSecurityStage}`
              : DASH
          }
          icon={<ShieldX {...iconProps('sm')} />}
          token="--color-error"
          color="var(--color-error)"
          bar={measured ? percent(summary?.blocked, summary?.withSecurityStage) : undefined}
          hint={noStagesHint ?? t('securityDashboardPage.kpi.blockedHint')}
        />
        <KpiCard
          label={t('securityDashboardPage.kpi.secrets')}
          value={kpiValue(summary?.secrets)}
          icon={<KeyRound {...iconProps('sm')} />}
          token="--color-error"
          color="var(--color-error)"
          bar={measured ? percent(summary?.secrets, summary?.withSecurityStage) : undefined}
          hint={noStagesHint ?? t('securityDashboardPage.kpi.secretsHint')}
        />
        <KpiCard
          label={t('securityDashboardPage.kpi.critical')}
          value={kpiValue(summary?.critical)}
          icon={<Flame {...iconProps('sm')} />}
          token="--color-warning"
          color="var(--color-warning)"
          bar={measured ? percent(summary?.critical, summary?.withSecurityStage) : undefined}
          hint={noStagesHint ?? t('securityDashboardPage.kpi.criticalHint')}
        />
        <KpiCard
          label={t('securityDashboardPage.kpi.undetermined')}
          value={kpiValue(summary?.undetermined)}
          icon={<CircleAlert {...iconProps('sm')} />}
          token="--color-text-secondary"
          color="var(--color-text-secondary)"
          bar={measured ? percent(summary?.undetermined, summary?.withSecurityStage) : undefined}
          hint={noStagesHint ?? t('securityDashboardPage.kpi.undeterminedHint')}
        />
        {canSeeOss && (
          <KpiCard
            label={t('securityDashboardPage.kpi.oss')}
            value={
              ossSummary?.actionImages !== undefined
                ? t('securityDashboardPage.kpi.ossValue', { count: ossSummary.actionImages })
                : DASH
            }
            icon={<Package {...iconProps('sm')} />}
            token="--color-warning"
            color="var(--color-warning)"
            hint={
              ossSummary
                ? t('securityDashboardPage.kpi.ossHint', {
                    exposed: ossSummary.gradeAImages ?? DASH,
                    notScanned: ossSummary.notScannedStacks ?? DASH,
                  })
                : undefined
            }
          />
        )}
      </div>

      <Tabs
        className="mb-3"
        value={activeTab}
        onChange={setTab}
        items={[
          {
            id: 'apps' as const,
            label: t('securityDashboardPage.tabs.apps'),
            badge: summary?.apps !== undefined ? <TabCount value={summary.apps} /> : undefined,
          },
          ...(canSeeOss
            ? [
                {
                  id: 'oss' as const,
                  label: t('securityDashboardPage.tabs.oss'),
                  badge: ossSummary?.stacks !== undefined ? <TabCount value={ossSummary.stacks} /> : undefined,
                },
              ]
            : []),
        ]}
      />

      {activeTab === 'apps' ? (
        <AppsTab
          report={report}
          isLoading={appsQuery.isLoading}
          isError={appsQuery.isError}
          hasScopeFilter={hasScopeFilter}
          grouped={grouped}
          problemOnly={problemOnly}
          hidden={hidden}
          onShowAll={() => setProblemOnly(false)}
          selectedApp={selectedApp}
          onSelect={(app) => setSelectedAppId(app.pipelineId)}
          groupOpen={groupOpen}
          onToggleGroup={(key, open) => setGroupOpen((prev) => ({ ...prev, [key]: open }))}
          groupNameOf={groupNameOf}
          locale={locale}
        />
      ) : (
        <OssTab
          report={stacksReport}
          isLoading={stacksQuery.isLoading && stackId !== NO_STACK}
          isError={stacksQuery.isError}
          locale={locale}
        />
      )}
    </div>
  )
}

function TabCount({ value }: { value: number }) {
  return (
    <span className="rounded-[var(--radius-full)] bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-1.5 text-[11px] tabular-nums text-[var(--color-text-secondary)]">
      {value}
    </span>
  )
}

/** 빈 상태 — 공용 컴포넌트가 아직 없어 이 화면에 둔다(설계 §6). "0건 통과" 처럼 보이면 안 된다. */
function EmptyState({ title, hint, action }: { title: string; hint?: string; action?: ReactNode }) {
  return (
    <div
      data-testid="security-empty"
      className="flex flex-col items-center justify-center gap-2 rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] px-6 py-12 text-center"
    >
      <ShieldCheck {...iconProps('lg')} className="text-[var(--color-text-muted)] opacity-40" />
      <p className="m-0 text-sm font-medium text-[var(--color-text-primary)]">{title}</p>
      {hint && <p className="m-0 max-w-[560px] text-[13px] text-[var(--color-text-secondary)]">{hint}</p>}
      {action}
    </div>
  )
}

function AppsTab({
  report,
  isLoading,
  isError,
  hasScopeFilter,
  grouped,
  problemOnly,
  hidden,
  onShowAll,
  selectedApp,
  onSelect,
  groupOpen,
  onToggleGroup,
  groupNameOf,
  locale,
}: {
  report: SecurityAppsReport | undefined
  isLoading: boolean
  isError: boolean
  hasScopeFilter: boolean
  grouped: boolean
  problemOnly: boolean
  hidden: number | undefined
  onShowAll: () => void
  selectedApp: SecurityApp | null
  onSelect: (app: SecurityApp) => void
  groupOpen: Record<string, boolean>
  onToggleGroup: (key: string, open: boolean) => void
  groupNameOf: (app: SecurityApp) => string
  locale: string
}) {
  const { t } = useTranslation()

  if (!report) {
    if (isError) return <EmptyState title={t('securityDashboardPage.loadError')} />
    if (isLoading) return <SkeletonTable rows={6} columns={6} />
    return null
  }

  const total = report.summary.apps
  if (total === 0) {
    return hasScopeFilter ? (
      <EmptyState title={t('securityDashboardPage.empty.noMatch')} />
    ) : (
      <EmptyState
        title={t('securityDashboardPage.empty.noApps')}
        hint={t('securityDashboardPage.empty.noAppsHint')}
        action={
          <Link to="/cicd/list" className="text-[13px] font-semibold text-[var(--color-primary)] hover:underline">
            {t('securityDashboardPage.empty.openPipelines')}
          </Link>
        }
      />
    )
  }
  if (report.summary.withSecurityStage === 0) {
    return (
      <EmptyState
        title={t('securityDashboardPage.empty.noStages')}
        hint={t('securityDashboardPage.empty.noStagesHint', { count: total ?? 0 })}
      />
    )
  }

  return (
    <>
      {report.truncated && (
        <p
          role="status"
          className="mb-3 mt-0 flex items-center gap-1.5 rounded-md border border-[color-mix(in_srgb,_var(--color-warning)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-warning)_10%,_transparent)] px-3 py-2 text-[13px] text-[var(--color-text-primary)]"
        >
          <StatusIcon tone="warning" size="xs" className="shrink-0" />
          {t('securityDashboardPage.truncated', { count: MAX_SECURITY_APPS })}
        </p>
      )}
      {/* 상세는 표 아래가 아니라 오른쪽에 붙는다 — 알림 이력과 같은 구성(DESIGN.md §Layout). */}
      <ListDetailPanel
        detailWidth={380}
        emptyDetailMessage={t('securityDashboardPage.detail.select')}
        detailContent={
          selectedApp && <SecurityAppDetail app={selectedApp} stackName={groupNameOf(selectedApp)} locale={locale} />
        }
        listContent={
          <>
            <SecurityAppsTable
              report={report}
              grouped={grouped}
              problemOnly={problemOnly}
              selectedAppId={selectedApp?.pipelineId ?? null}
              onSelect={onSelect}
              groupOpen={groupOpen}
              onToggleGroup={onToggleGroup}
              locale={locale}
              emptyMessage={
                problemOnly
                  ? t('securityDashboardPage.empty.noProblems')
                  : t('securityDashboardPage.empty.noMatch')
              }
            />
            {problemOnly && hidden !== undefined && hidden > 0 && (
              <div className="flex items-center justify-end gap-2 border-t border-[var(--color-border-default)] px-[var(--table-cell-px)] py-2 text-[12px] text-[var(--color-text-secondary)]">
                {t('securityDashboardPage.hiddenFooter', { count: hidden })}
                <button
                  type="button"
                  onClick={onShowAll}
                  className="cursor-pointer border-none bg-transparent p-0 text-[12px] font-semibold text-[var(--color-primary)] hover:underline"
                >
                  {t('securityDashboardPage.showAll')}
                </button>
              </div>
            )}
          </>
        }
      />
    </>
  )
}

function OssTab({
  report,
  isLoading,
  isError,
  locale,
}: {
  report: ReturnType<typeof useSecurityStacks>['data']
  isLoading: boolean
  isError: boolean
  locale: string
}) {
  const { t } = useTranslation()
  if (!report) {
    if (isError) return <EmptyState title={t('securityDashboardPage.loadError')} />
    if (isLoading) return <SkeletonTable rows={4} columns={6} />
    return <EmptyState title={t('securityDashboardPage.oss.empty.noStacks')} />
  }
  if (report.stacks.length === 0) {
    return <EmptyState title={t('securityDashboardPage.oss.empty.noStacks')} />
  }
  // 모든 스택이 스캔하지 않으면 표의 숫자 칸이 전부 비어 "0" 처럼 읽힐 수 있다. 이유부터 말한다.
  if (report.stacks.every((row) => row.state === 'not_scanned')) {
    return (
      <EmptyState
        title={t('securityDashboardPage.oss.empty.noneScanned')}
        hint={t('securityDashboardPage.oss.empty.noneScannedHint')}
      />
    )
  }
  return (
    <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)]">
      <SecurityOssTable report={report} locale={locale} />
    </div>
  )
}
