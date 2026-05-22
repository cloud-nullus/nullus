import { useMemo, useState, useCallback, useId, useEffect, useRef } from 'react'
import { useQueries } from '@tanstack/react-query'
import {
  AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import {
  Cpu, MemoryStick, Box, CheckCircle, AlertCircle, XCircle,
  Server, GitBranch, BarChart3, Settings2, Plus, Trash2, Save,
  GripVertical, ChevronDown, ChevronUp, Check, Lock,
  Activity, Clock, Package, Layers,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { type ToolHealthStatus } from '../api/observability-api'
import { useDeployments, usePipelines } from '../../cicd/api/cicd-api'
import { useAuthStore } from '../../../stores/auth-store'
import { cn } from '../../../lib/utils'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'
import { StackMonitoringOverview } from '../components/stack-monitoring-overview'
import { api } from '../../../lib/api'
import type { StackMonitoringSnapshot } from '../../stack/api/stack-api'
import { useClusterMonitoringSummary } from '../../admin/api/admin-api'


// ─── Types ────────────────────────────────────────────────────────────────────
type ViewType = 'cluster' | 'stack' | 'cicd'
type TimeRange = '1h' | '6h' | '24h' | '7d'

// ─── Shared chart style helpers ───────────────────────────────────────────────
const CHART_STYLE = {
  bg: '#0b1220',
  grid: 'rgba(148,163,184,0.15)',
  tick: { fill: '#94a3b8', fontSize: 11 },
  tooltip: { background: '#111827', border: '1px solid #374151', color: '#e5e7eb' },
}

const TOOL_STATUS: Record<ToolHealthStatus, { icon: React.ReactNode; cls: string; label: string }> = {
  running: { icon: <CheckCircle size={13} />, cls: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Running' },
  warning: { icon: <AlertCircle size={13} />, cls: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Warning' },
  error: { icon: <XCircle size={13} />, cls: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', label: 'Error' },
}

function timeAgo(dateStr: string | null): string {
  if (!dateStr) return '—'
  const ms = Date.now() - new Date(dateStr).getTime()
  if (ms < 60_000) return 'just now'
  if (ms < 3_600_000) return `${Math.floor(ms / 60_000)}m ago`
  if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)}h ago`
  return `${Math.floor(ms / 86_400_000)}d ago`
}

function selectSeries<T extends { ts: number }>(samples: T[], range: TimeRange): T[] {
  if (samples.length === 0) return []
  const now = Date.now()
  const windowMs: Record<TimeRange, number> = {
    '1h': 60 * 60 * 1000,
    '6h': 6 * 60 * 60 * 1000,
    '24h': 24 * 60 * 60 * 1000,
    '7d': 7 * 24 * 60 * 60 * 1000,
  }
  const cutoff = now - windowMs[range]
  const ranged = samples.filter((s) => s.ts >= cutoff)
  if (ranged.length <= 120) return ranged
  const stride = Math.ceil(ranged.length / 120)
  return ranged.filter((_, idx) => idx % stride === 0)
}

function formatRangeLabel(ts: number, range: TimeRange): string {
  const date = new Date(ts)
  if (range === '7d') {
    return date.toLocaleDateString('en', { weekday: 'short' })
  }
  return date.toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit', hour12: false })
}

function formatDuration(startedAt: string | null, completedAt: string | null): string {
  if (!startedAt) return '—'
  const start = new Date(startedAt).getTime()
  const end = completedAt ? new Date(completedAt).getTime() : Date.now()
  const ms = Math.max(0, end - start)
  const totalSec = Math.floor(ms / 1000)
  const minutes = Math.floor(totalSec / 60)
  const seconds = totalSec % 60
  if (minutes === 0) return `${seconds}s`
  return `${minutes}m ${seconds.toString().padStart(2, '0')}s`
}

// ─── Shared chart panel wrapper ───────────────────────────────────────────────
function ChartPanel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-[10px] border border-[var(--color-border-default)] p-3" style={{ background: CHART_STYLE.bg }}>
      <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">{title}</div>
      {children}
    </div>
  )
}

// ─── KPI card ────────────────────────────────────────────────────────────────
function KpiCard({ label, value, icon, color, iconCls, bar }: { label: string; value: string; icon: React.ReactNode; color: string; iconCls: string; bar: number }) {
  return (
    <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
      <div className="mb-2.5 flex items-center gap-2.5">
        <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', iconCls)}>{icon}</div>
        <span className="text-xs font-medium text-[var(--color-text-secondary)]">{label}</span>
      </div>
      <div className="text-[28px] font-extrabold leading-none text-[var(--color-text-primary)]">{value}</div>
      <div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[rgba(255,255,255,0.08)]">
        <svg className="h-full w-full" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
          <rect width={Math.max(0, Math.min(100, bar))} height="6" rx="3" fill={color} />
        </svg>
      </div>
    </div>
  )
}

// ─── DashboardTabLayout — shared tab system for all 3 views ──────────────────
const STORAGE_KEY = (v: string) => `nullus_tabs_${v}_v1`

function loadTabs(viewId: string, seedTabs?: EmbedTab[]): EmbedTab[] {
  try {
    const r = localStorage.getItem(STORAGE_KEY(viewId))
    if (r) return JSON.parse(r) as EmbedTab[]
    if (seedTabs?.length) {
      localStorage.setItem(STORAGE_KEY(viewId), JSON.stringify(seedTabs))
      return seedTabs
    }
  } catch { /* */ }
  return []
}
function persistTabs(viewId: string, tabs: EmbedTab[]) {
  try { localStorage.setItem(STORAGE_KEY(viewId), JSON.stringify(tabs)) } catch { /* */ }
}

interface TabLayoutProps {
  viewId: ViewType
  isAdmin: boolean
  defaultContent: React.ReactNode
  /** Pre-seeded tabs written to localStorage on first load (when no saved tabs exist) */
  seedTabs?: EmbedTab[]
  /** Rendered above default content when no custom tabs exist yet (admin only) */
  firstTimePanel?: (
    onConnect: (tabs: Pick<EmbedTab, 'label' | 'url'>[]) => void,
    onSkip: () => void,
  ) => React.ReactNode
}

const SKIP_KEY = (v: string) => `nullus_skip_connect_${v}`

const normalizeEmbedUrl = (rawUrl: string): string => {
  const trimmed = rawUrl.trim()
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

const isValidEmbedUrl = (rawUrl: string): boolean => {
  try {
    const parsed = new URL(rawUrl)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

const isKnownNonEmbeddableHost = (rawUrl: string): boolean => {
  try {
    const parsed = new URL(rawUrl)
    return parsed.hostname.toLowerCase() === 'play.grafana.org'
  } catch {
    return false
  }
}

function DashboardTabLayout({ viewId, isAdmin, defaultContent, seedTabs, firstTimePanel }: TabLayoutProps) {
  const { t } = useTranslation()
  const uid = useId()
  const [activeId, setActiveId] = useState('default')
  const [tabs, setTabs] = useState<EmbedTab[]>(() => loadTabs(viewId, seedTabs))
  const [isManaging, setIsManaging] = useState(false)
  const [drafts, setDrafts] = useState<EmbedTab[]>([])
  const [saved, setSaved] = useState(false)
  const [embedError, setEmbedError] = useState(false)
  const [skipConnect, setSkipConnect] = useState(() => {
    try { return localStorage.getItem(SKIP_KEY(viewId)) === 'true' } catch { return false }
  })

  const allTabs = [{ id: 'default', label: t('monitoringPage.customTabs.defaultTab', 'Default') }, ...tabs]
  const activeCustom = tabs.find((t) => t.id === activeId)
  const activeEmbedUrl = activeCustom ? normalizeEmbedUrl(activeCustom.url) : ''
  const activeEmbedUrlValid = activeEmbedUrl ? isValidEmbedUrl(activeEmbedUrl) : false
  const activeEmbedBlockedByHost = activeEmbedUrlValid && isKnownNonEmbeddableHost(activeEmbedUrl)

  function openManage() { setDrafts(tabs.map((t) => ({ ...t }))); setIsManaging(true); setSaved(false) }
  function cancelManage() { setIsManaging(false) }
  function saveManage() {
    const ordered = drafts.map((d, i) => ({ ...d, url: normalizeEmbedUrl(d.url), order: i }))
    setTabs(ordered); persistTabs(viewId, ordered); setIsManaging(false); setSaved(true)
    setTimeout(() => setSaved(false), 2500)
  }

  const addDraft = useCallback(() => {
    setDrafts((p) => [...p, { id: `tab-${uid}-${Date.now()}`, label: t('monitoringPage.customTabs.newTab', 'New Tab'), url: '', order: p.length }])
  }, [t, uid])

  function removeDraft(id: string) { setDrafts((p) => p.filter((d) => d.id !== id)) }
  function patchDraft(id: string, patch: Partial<EmbedTab>) { setDrafts((p) => p.map((d) => d.id === id ? { ...d, ...patch } : d)) }
  function moveDraft(id: string, dir: -1 | 1) {
    setDrafts((p) => {
      const i = p.findIndex((d) => d.id === id); if (i < 0) return p
      const n = [...p]; const j = i + dir
      if (j < 0 || j >= n.length) return p
        ;[n[i], n[j]] = [n[j], n[i]]; return n
    })
  }

  function addTabsBatch(newTabs: Pick<EmbedTab, 'label' | 'url'>[]) {
    const toAdd: EmbedTab[] = newTabs.map((t, i) => ({
      id: `tab-${uid}-${Date.now()}-${i}`,
      label: t.label,
      url: normalizeEmbedUrl(t.url),
      order: tabs.length + i,
    }))
    const updated = [...tabs, ...toAdd]
    setTabs(updated)
    persistTabs(viewId, updated)
    setSkipConnect(true)
    if (toAdd[0]) setActiveId(toAdd[0].id)
  }

  function handleSkipConnect() {
    try { localStorage.setItem(SKIP_KEY(viewId), 'true') } catch { /* */ }
    setSkipConnect(true)
  }

  const showFirstTime = isAdmin && tabs.length === 0 && !skipConnect && !!firstTimePanel

  return (
    <div className="w-full">
      {/* Tab bar */}
      <div className="flex items-end overflow-x-auto border-b border-[var(--color-border-default)]">
        {allTabs.map((t) => (
          <button key={t.id} type="button" onClick={() => { setActiveId(t.id); setEmbedError(false) }}
            className={cn('flex shrink-0 items-center gap-1.5 border-b-2 px-4 py-2.5 text-sm font-medium transition-colors',
              activeId === t.id
                ? 'border-b-[var(--color-primary)] text-[var(--color-text-primary)]'
                : 'border-b-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]')}>
            {t.label}
          </button>
        ))}
        {isAdmin && (
          <button type="button" onClick={isManaging ? cancelManage : openManage}
            className={cn('ml-auto flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2.5 text-xs font-medium transition-colors',
              isManaging ? 'border-b-amber-400 text-amber-400' : 'border-b-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]')}>
            <Settings2 size={13} />{isManaging ? t('common.cancel', 'Cancel') : t('monitoringPage.customTabs.manageTabs', 'Manage Tabs')}
          </button>
        )}
      </div>

      {/* Admin manage panel */}
      {isManaging && (
        <div className="w-full border-b border-[var(--color-border-default)] bg-amber-500/5 px-1 py-4 sm:px-2">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <span className="text-sm font-semibold text-[var(--color-text-primary)]">{t('monitoringPage.customTabs.manageCustomTabs', 'Manage Custom Tabs')}</span>
            <div className="flex flex-wrap gap-2">
              <button type="button" onClick={addDraft}
                className="flex items-center gap-1.5 rounded-lg border border-[var(--color-border-default)] px-3 py-1.5 text-xs text-[var(--color-text-secondary)] hover:border-[var(--color-primary)] hover:text-[var(--color-primary)]">
                <Plus size={12} />{t('monitoringPage.customTabs.addTab', 'Add Tab')}
              </button>
              <button type="button" onClick={saveManage}
                className="flex items-center gap-1.5 rounded-lg bg-[var(--color-primary)] px-4 py-1.5 text-xs font-medium text-white hover:opacity-90">
                <Save size={12} />{t('monitoringPage.customTabs.saveChanges', 'Save Changes')}
              </button>
            </div>
          </div>

          {drafts.length === 0 ? (
            <p className="text-xs text-[var(--color-text-secondary)]">{t('monitoringPage.customTabs.empty', 'No custom tabs yet. Click "Add Tab" to create one.')}</p>
          ) : (
            <div className="flex flex-col gap-2">
              {drafts.map((d, idx) => (
                <div key={d.id} className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-3 py-2.5">
                  {/* Row 1: order controls + tab name + delete */}
                  <div className="flex items-center gap-2">
                    <div className="flex shrink-0 flex-col">
                      <button type="button" onClick={() => moveDraft(d.id, -1)} disabled={idx === 0}
                        className="rounded p-0.5 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] disabled:opacity-30">
                        <ChevronUp size={11} />
                      </button>
                      <button type="button" onClick={() => moveDraft(d.id, 1)} disabled={idx === drafts.length - 1}
                        className="rounded p-0.5 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] disabled:opacity-30">
                        <ChevronDown size={11} />
                      </button>
                    </div>
                    <GripVertical size={14} className="shrink-0 text-[var(--color-text-secondary)]" />
                    <input
                      value={d.label}
                      onChange={(e) => patchDraft(d.id, { label: e.target.value })}
                      placeholder={t('monitoringPage.customTabs.tabNamePlaceholder', 'Tab name')}
                      className="min-w-0 flex-1 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[7px] text-xs text-[var(--color-text-primary)] outline-none focus:border-[#6366f1]"
                    />
                    <button type="button" onClick={() => removeDraft(d.id)}
                      className="shrink-0 rounded p-1 text-[var(--color-text-secondary)] hover:bg-red-400/10 hover:text-red-400">
                      <Trash2 size={13} />
                    </button>
                  </div>
                  {/* Row 2: URL input full width */}
                  <div className="mt-1.5 pl-[46px]">
                    <input
                      value={d.url}
                      onChange={(e) => patchDraft(d.id, { url: e.target.value })}
                      placeholder={t('monitoringPage.customTabs.embedUrlPlaceholder', 'Embed URL')}
                      className="w-full rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[7px] text-xs text-[var(--color-text-primary)] placeholder:text-[var(--color-text-secondary)] outline-none focus:border-[#6366f1]"
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
          <p className="mt-2 text-[10px] text-[var(--color-text-secondary)]">
            {t('monitoringPage.customTabs.description', 'Changes apply to all users after saving. Developer role has view-only access.')}
          </p>
        </div>
      )}

      {saved && (
        <div className="flex items-center gap-2 border-b border-emerald-500/20 bg-emerald-500/5 px-1 py-2 text-xs text-emerald-400 sm:px-2">
          <Check size={12} />{t('monitoringPage.customTabs.saved', 'Tab configuration saved.')}
        </div>
      )}

      {/* Tab content */}
      {activeId === 'default' && (
        <div className="pt-4">
          {showFirstTime && firstTimePanel!(addTabsBatch, handleSkipConnect)}
          {defaultContent}
        </div>
      )}

      {activeId !== 'default' && activeCustom && (
        <div className="flex flex-col">
          <div className="flex items-center gap-2 border-b border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-1 py-2 sm:px-2">
            {activeEmbedUrl
              ? <span className="truncate font-mono text-[11px] text-[var(--color-text-secondary)]">{activeEmbedUrl}</span>
              : <span className="italic text-[11px] text-[var(--color-text-secondary)]">No URL configured{isAdmin ? ' — set URL in Manage Tabs' : ''}</span>}
            {activeEmbedUrlValid && (
              <a
                href={activeEmbedUrl}
                target="_blank"
                rel="noreferrer"
                className="ml-auto shrink-0 rounded border border-[var(--color-border-default)] px-2 py-0.5 text-[10px] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
              >
                Open in new tab
              </a>
            )}
            {!isAdmin && <span className="ml-auto flex items-center gap-1 text-[11px] text-[var(--color-text-secondary)]"><Lock size={10} />View only</span>}
          </div>
          {activeEmbedUrl ? (
            !activeEmbedUrlValid ? (
              <div className="flex h-64 flex-col items-center justify-center gap-2 text-center text-[var(--color-text-secondary)]">
                <p className="text-sm font-medium text-[var(--color-text-primary)]">Invalid embed URL</p>
                <p className="max-w-[560px] text-xs">Use a full HTTP(S) URL like `https://grafana.example.com` in Manage Tabs.</p>
              </div>
            ) : activeEmbedBlockedByHost ? (
              <div className="flex h-64 flex-col items-center justify-center gap-2 text-center text-[var(--color-text-secondary)]">
                <p className="text-sm font-medium text-[var(--color-text-primary)]">Embedding blocked by target site</p>
                <p className="max-w-[620px] text-xs">
                  `play.grafana.org` sends `X-Frame-Options: DENY`, so browsers block iframe embedding. Use "Open in new tab".
                </p>
              </div>
            ) : (
              <div className="overflow-x-hidden">
                <iframe
                  src={activeEmbedUrl}
                  title={activeCustom.label}
                  className="border-0"
                  style={{
                    width: 'calc(100% + 2 * var(--page-padding))',
                    marginLeft: 'calc(-1 * var(--page-padding))',
                    height: '72vh',
                    minHeight: 520,
                  }}
                  onError={() => setEmbedError(true)}
                  allowFullScreen
                />
                {embedError && (
                  <div className="border-t border-amber-400/25 bg-amber-400/10 px-3 py-2 text-xs text-amber-300">
                    Embedded page could not be loaded. This URL may block iframe embedding. Use the "Open in new tab" button.
                  </div>
                )}
              </div>
            )
          ) : (
            <div className="flex h-72 flex-col items-center justify-center gap-2 text-[var(--color-text-secondary)]">
              <Server size={28} className="opacity-20" />
              <p className="text-sm font-medium text-[var(--color-text-primary)]">No embed URL configured</p>
              <p className="text-xs">{isAdmin ? 'Click "Manage Tabs" above to set the URL.' : 'An admin can configure this tab.'}</p>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ─── Default content: Cluster view ───────────────────────────────────────────
function ClusterDefault({
  clusterId,
  clusterName,
  stackIds,
}: {
  clusterId: string
  clusterName: string
  stackIds: string[]
}) {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: pipelinesData } = usePipelines()
  const { data: deploymentsData } = useDeployments()
  const { data: clusterSummary } = useClusterMonitoringSummary(clusterId)
  const [samples, setSamples] = useState<Array<{ ts: number; cpu: number; memory: number }>>([])

  const monitoringQueries = useQueries({
    queries: stackIds.map((stackId) => ({
      queryKey: ['stacks', 'monitoring', stackId],
      queryFn: () => api.get<StackMonitoringSnapshot>(`/stacks/${stackId}/monitoring`).then((r) => r.data),
      enabled: !!stackId,
      refetchInterval: 5000,
      staleTime: 0,
    })),
  })

  const snapshots = useMemo(
    () =>
      monitoringQueries
        .map((query) => query.data)
        .filter((snapshot): snapshot is StackMonitoringSnapshot => !!snapshot),
    [monitoringQueries],
  )

  const aggregatedFromStacks = useMemo(() => {
    const totals = snapshots.reduce(
      (acc, snapshot) => {
        acc.totalPods += snapshot.summary.total_pods ?? 0
        acc.readyPods += snapshot.summary.ready_pods ?? 0
        acc.cpuRequest += snapshot.summary.cpu_request_millicores ?? 0
        acc.cpuUsage += snapshot.summary.cpu_usage_millicores ?? 0
        acc.memoryRequest += snapshot.summary.memory_request_mib ?? 0
        acc.memoryUsage += snapshot.summary.memory_usage_mib ?? 0
        return acc
      },
      {
        totalPods: 0,
        readyPods: 0,
        cpuRequest: 0,
        cpuUsage: 0,
        memoryRequest: 0,
        memoryUsage: 0,
      },
    )

    const cpuPercent = totals.cpuRequest > 0 ? Math.max(0, Math.round((totals.cpuUsage / totals.cpuRequest) * 100)) : 0
    const memoryPercent = totals.memoryRequest > 0 ? Math.max(0, Math.round((totals.memoryUsage / totals.memoryRequest) * 100)) : 0

    return {
      podCount: totals.totalPods,
      podRunning: totals.readyPods,
      cpuUsage: cpuPercent,
      memoryUsage: memoryPercent,
    }
  }, [snapshots])

  const aggregated = useMemo(() => {
    if (aggregatedFromStacks.podCount > 0) {
      return aggregatedFromStacks
    }

    return {
      podCount: clusterSummary?.total_pods ?? 0,
      podRunning: clusterSummary?.ready_pods ?? 0,
      cpuUsage: 0,
      memoryUsage: 0,
    }
  }, [aggregatedFromStacks, clusterSummary])

  useEffect(() => {
    if (!clusterId) return
    setSamples((prev) => [
      ...prev,
      {
        ts: Date.now(),
        cpu: aggregated.cpuUsage,
        memory: aggregated.memoryUsage,
      },
    ].slice(-4000))
  }, [clusterId, aggregated.cpuUsage, aggregated.memoryUsage])

  const selected = useMemo(() => selectSeries(samples, range), [samples, range])
  const series = useMemo(
    () => selected.map((s) => ({ time: formatRangeLabel(s.ts, range), cpu: s.cpu, memory: s.memory })),
    [selected, range],
  )

  const weekBars = useMemo(() => {
    const deployments = deploymentsData?.items ?? []
    const pipelineIds = new Set(
      (pipelinesData?.items ?? [])
        .filter((pipeline) => pipeline.clusterId === clusterId)
        .map((pipeline) => pipeline.id),
    )

    const deploymentsForCluster = deployments.filter((deployment) => pipelineIds.has(deployment.pipelineId))
    const now = new Date()
    const dayKeys = Array.from({ length: 7 }, (_, idx) => {
      const d = new Date(now)
      d.setDate(now.getDate() - (6 - idx))
      return d.toLocaleDateString('en-CA')
    })

    const byDay = dayKeys.reduce<Record<string, { day: string; success: number; failed: number }>>((acc, key) => {
      const dayDate = new Date(key)
      acc[key] = { day: dayDate.toLocaleDateString('en', { weekday: 'short' }), success: 0, failed: 0 }
      return acc
    }, {})

    deploymentsForCluster.forEach((deployment) => {
      const key = deployment.startedAt ? new Date(deployment.startedAt).toLocaleDateString('en-CA') : ''
      const bucket = byDay[key]
      if (!bucket) return
      if (deployment.status === 'success') bucket.success += 1
      if (deployment.status === 'failed') bucket.failed += 1
    })

    return dayKeys.map((key) => byDay[key])
  }, [deploymentsData?.items, pipelinesData?.items, clusterId])

  const podCount = aggregated.podCount
  const podRunning = aggregated.podRunning
  const cpuUsage = aggregated.cpuUsage
  const memoryUsage = aggregated.memoryUsage

  const pods = [
    { name: 'Running', value: podRunning, color: '#22c55e' },
    { name: 'Other', value: Math.max(0, podCount - podRunning), color: '#f59e0b' },
  ]

  const runningPodsPercent = podCount > 0 ? Math.round((podRunning / podCount) * 100) : 0

  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <div className={cn('flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium',
          'border-emerald-500/20 bg-emerald-500/5 text-emerald-400')}>
          <span className="h-2 w-2 rounded-full bg-emerald-400" />{clusterName || clusterId}
        </div>
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-[var(--color-text-secondary)]">Range:</span>
          {(['1h', '6h', '24h', '7d'] as TimeRange[]).map((r) => (
            <button key={r} type="button" onClick={() => setRange(r)}
              className={cn('rounded-[7px] border px-2.5 py-[5px] text-xs font-bold',
                range === r ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]')}>
              {r}
            </button>
          ))}
        </div>
      </div>
      <div className="mb-5 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
        <KpiCard label="Running Pods" value={`${podRunning}/${podCount}`} icon={<Server size={18} />} color="#60a5fa" iconCls="bg-[rgba(59,130,246,0.15)] text-[#60a5fa]" bar={runningPodsPercent} />
        <KpiCard label="Pods" value={String(podCount)} icon={<Box size={18} />} color="#22c55e" iconCls="bg-[rgba(34,197,94,0.15)] text-[#22c55e]" bar={runningPodsPercent} />
        <KpiCard label="CPU" value={`${Math.round(cpuUsage)}%`} icon={<Cpu size={18} />} color="#f59e0b" iconCls="bg-[rgba(245,158,11,0.15)] text-[#f59e0b]" bar={cpuUsage} />
        <KpiCard label="Memory" value={`${Math.round(memoryUsage)}%`} icon={<MemoryStick size={18} />} color="#a78bfa" iconCls="bg-[rgba(139,92,246,0.15)] text-[#a78bfa]" bar={memoryUsage} />
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPanel title="CPU Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="ccpu" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.5} /><stop offset="95%" stopColor="#f59e0b" stopOpacity={.05} /></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Area type="monotone" dataKey="cpu" name="CPU %" stroke="#f59e0b" strokeWidth={2} fill="url(#ccpu)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Memory Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="cmem" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={.5} /><stop offset="95%" stopColor="#3b82f6" stopOpacity={.05} /></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Area type="monotone" dataKey="memory" name="Memory %" stroke="#3b82f6" strokeWidth={2} fill="url(#cmem)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pod Status">
          <ResponsiveContainer width="100%" height={220}>
            <PieChart>
              <Pie data={pods} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>
                {pods.map((e) => <Cell key={e.name} fill={e.color} />)}
              </Pie>
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb' }} />
            </PieChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pipeline Success (this week)">
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={weekBars}>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="day" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb' }} />
              <Bar dataKey="success" fill="#22c55e" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>
    </div>
  )
}

// ─── Default content: Stack view ─────────────────────────────────────────────
function StackDefault({ stackId }: { stackId: string }) {
  return <StackMonitoringOverview stackId={stackId} />
}

// ─── Default content: CI/CD view ─────────────────────────────────────────────
// ─── CI/CD Application monitoring data ───────────────────────────────────────
type AppStatus = 'healthy' | 'degraded' | 'down'

interface DeployedAppRow {
  name: string
  version: string
  pipeline: string
  status: AppStatus
  pods: [number | null, number | null]
  cluster: string
  namespace: string
  duration: string
  lastDeploy: string
}

const APP_STATUS_CFG: Record<AppStatus, { label: string; cls: string; dot: string }> = {
  healthy: { label: 'Healthy', cls: 'bg-emerald-500/15 text-emerald-400', dot: 'bg-emerald-400' },
  degraded: { label: 'Degraded', cls: 'bg-amber-500/15 text-amber-400', dot: 'bg-amber-400' },
  down: { label: 'Down', cls: 'bg-red-500/15 text-red-400', dot: 'bg-red-400' },
}

/** Sample Grafana tab pre-seeded into CI/CD localStorage */
export const CICD_DEFAULT_TABS: EmbedTab[] = [
  {
    id: 'cicd-seed-grafana',
    label: 'Grafana',
    url: 'https://play.grafana.org/d/000000012/grafana-play-home?orgId=1&theme=dark&kiosk',
    order: 0,
  },
]

function CicdDefault({ selectedClusterId }: { selectedClusterId: string }) {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: pipelinesData } = usePipelines()
  const { data: deploymentsData } = useDeployments()

  const pipelines = useMemo(
    () => (pipelinesData?.items ?? []).filter((pipeline) => !selectedClusterId || pipeline.clusterId === selectedClusterId),
    [pipelinesData?.items, selectedClusterId],
  )

  const deployments = useMemo(() => {
    const allDeployments = deploymentsData?.items ?? []
    const pipelineIds = new Set(pipelines.map((pipeline) => pipeline.id))
    return allDeployments.filter((deployment) => pipelineIds.has(deployment.pipelineId))
  }, [deploymentsData?.items, pipelines])

  const latestByPipeline = useMemo(() => {
    const map = new Map<string, (typeof deployments)[number]>()
    deployments.forEach((deployment) => {
      const prev = map.get(deployment.pipelineId)
      if (!prev || new Date(deployment.startedAt).getTime() > new Date(prev.startedAt).getTime()) {
        map.set(deployment.pipelineId, deployment)
      }
    })
    return map
  }, [deployments])

  const rows = useMemo<DeployedAppRow[]>(() => pipelines.map((pipeline) => {
    const latest = latestByPipeline.get(pipeline.id)
    const status: AppStatus = latest?.status === 'failed' ? 'down' : latest?.status === 'running' ? 'degraded' : 'healthy'

    return {
      name: pipeline.name,
      version: latest?.version || '—',
      pipeline: pipeline.appType,
      status,
      pods: [null, null],
      cluster: pipeline.clusterName || '—',
      namespace: pipeline.namespace || 'default',
      duration: formatDuration(latest?.startedAt ?? null, latest?.completedAt ?? null),
      lastDeploy: timeAgo(latest?.startedAt ?? null),
    }
  }), [pipelines, latestByPipeline])

  const latestDeployments = useMemo(
    () => [...deployments].sort((a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime()).slice(0, 8),
    [deployments],
  )

  const timeline = useMemo(() => {
    const now = new Date()
    const isDaily = range === '7d'
    const windowMs: Record<TimeRange, number> = {
      '1h': 60 * 60 * 1000,
      '6h': 6 * 60 * 60 * 1000,
      '24h': 24 * 60 * 60 * 1000,
      '7d': 7 * 24 * 60 * 60 * 1000,
    }
    const cutoff = now.getTime() - windowMs[range]

    const keys: string[] = []
    if (isDaily) {
      for (let i = 6; i >= 0; i -= 1) {
        const day = new Date(now)
        day.setDate(now.getDate() - i)
        keys.push(day.toLocaleDateString('en-CA'))
      }
    } else {
      const start = new Date(cutoff)
      start.setMinutes(0, 0, 0)
      const cur = new Date(start)
      while (cur.getTime() <= now.getTime()) {
        const key = `${cur.toLocaleDateString('en-CA')} ${cur.getHours().toString().padStart(2, '0')}:00`
        keys.push(key)
        cur.setHours(cur.getHours() + 1)
      }
    }

    const byKey = keys.reduce<Record<string, { time: string; success: number; failed: number }>>((acc, key) => {
      const label = isDaily
        ? new Date(key).toLocaleDateString('en', { weekday: 'short' })
        : key.slice(-5)
      acc[key] = { time: label, success: 0, failed: 0 }
      return acc
    }, {})

    deployments.forEach((deployment) => {
      const started = new Date(deployment.startedAt).getTime()
      if (Number.isNaN(started) || started < cutoff) return
      const date = new Date(started)
      const key = isDaily
        ? date.toLocaleDateString('en-CA')
        : `${date.toLocaleDateString('en-CA')} ${date.getHours().toString().padStart(2, '0')}:00`
      const bucket = byKey[key]
      if (!bucket) return
      if (deployment.status === 'success') bucket.success += 1
      if (deployment.status === 'failed') bucket.failed += 1
    })

    return keys.map((k) => byKey[k])
  }, [deployments, range])

  const successPipelines = pipelines.reduce((count, pipeline) => {
    const status = latestByPipeline.get(pipeline.id)?.status
    return status === 'success' ? count + 1 : count
  }, 0)
  const failedPipelines = pipelines.reduce((count, pipeline) => {
    const status = latestByPipeline.get(pipeline.id)?.status
    return status === 'failed' ? count + 1 : count
  }, 0)
  const runningDeployments = deployments.filter((d) => ['running', 'pending', 'validating', 'installing', 'configuring', 'health_check', 'rolling_back'].includes(d.status)).length

  const appKpis = [
    { label: 'Total Pipelines', value: String(pipelines.length), icon: <Layers size={18} />, color: '#6366f1', iconCls: 'bg-[rgba(99,102,241,0.15)] text-[#6366f1]', bar: 100 },
    { label: 'Pipeline Success / Failed', value: `${successPipelines} / ${failedPipelines}`, icon: <CheckCircle size={18} />, color: '#22c55e', iconCls: 'bg-emerald-500/15 text-emerald-400', bar: pipelines.length ? Math.round((successPipelines / pipelines.length) * 100) : 0 },
    { label: 'Total Deployments', value: String(deployments.length), icon: <GitBranch size={18} />, color: '#f59e0b', iconCls: 'bg-amber-500/15 text-amber-400', bar: 100 },
    { label: 'Running Deployments', value: String(runningDeployments), icon: <Activity size={18} />, color: '#10b981', iconCls: 'bg-[rgba(16,185,129,0.15)] text-[#10b981]', bar: deployments.length ? Math.round((runningDeployments / deployments.length) * 100) : 0 },
  ]

  return (
    <div>
      {/* Toolbar */}
      <div className="mb-5 flex flex-wrap items-center gap-3">
        <div className="ml-auto flex items-center gap-2">
          {(['1h', '6h', '24h', '7d'] as TimeRange[]).map((r) => (
            <button key={r} type="button" onClick={() => setRange(r)}
              className={cn('rounded-[7px] border px-2.5 py-[5px] text-xs font-bold',
                range === r
                  ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]')}>
              {r}
            </button>
          ))}
        </div>
      </div>

      {/* KPI cards */}
      <div className="mb-5 grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-4">
        {appKpis.map((c) => <KpiCard key={c.label} {...c} />)}
      </div>

      {/* Charts */}
      <div className="mb-5 grid grid-cols-1 gap-3.5">
        <ChartPanel title="Deployment Timeline">
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={timeline}>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb', fontSize: 11 }} />
              <Bar dataKey="success" fill="#22c55e" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>

      {/* Application table */}
      <div className="mb-5 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-4 py-3">
          <h2 className="flex items-center gap-2 text-[14px] font-bold text-[var(--color-text-primary)]">
            <Package size={15} className="text-[#a5b4fc]" />
            Deployed Applications
          </h2>
          <span className="text-xs text-[var(--color-text-secondary)]">{rows.length} apps</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[var(--color-border-default)] text-[11px] text-[var(--color-text-secondary)]">
                {['Application', 'Version', 'Pipeline', 'Status', 'Pods', 'Cluster', 'Namespace', 'Duration', 'Last Deploy'].map((h) => (
                  <th key={h} className="px-4 py-2.5 text-left font-semibold tracking-[0.03em]">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((app, i) => {
                const sc = APP_STATUS_CFG[app.status]
                const isLast = i === rows.length - 1
                return (
                  <tr key={app.name}
                    className={cn('transition-colors hover:bg-[rgba(255,255,255,0.02)]', !isLast && 'border-b border-[var(--color-border-default)]')}>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={cn('h-2 w-2 shrink-0 rounded-full', sc.dot)} />
                        <span className="font-semibold text-[var(--color-text-primary)]">{app.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">{app.version}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <GitBranch size={11} />{app.pipeline}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', sc.cls)}>{sc.label}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn(
                        'font-mono',
                        typeof app.pods[0] === 'number' && typeof app.pods[1] === 'number' && app.pods[0] < app.pods[1]
                          ? 'text-amber-400'
                          : 'text-[var(--color-text-primary)]',
                      )}>
                        {typeof app.pods[0] === 'number' && typeof app.pods[1] === 'number' ? `${app.pods[0]}/${app.pods[1]}` : '—'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-[var(--color-text-secondary)]">{app.cluster}</td>
                    <td className="px-4 py-3 text-[var(--color-text-secondary)]">{app.namespace}</td>
                    <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">{app.duration}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <Clock size={11} />{app.lastDeploy}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* Recent deployments */}
      <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="border-b border-[var(--color-border-default)] px-4 py-3">
          <h2 className="flex items-center gap-2 text-[14px] font-bold text-[var(--color-text-primary)]">
            <GitBranch size={15} className="text-[#a5b4fc]" />
            Recent Deployments
          </h2>
        </div>
        <div className="divide-y divide-[var(--color-border-default)]">
          {latestDeployments.map((d) => (
            <div key={`${d.pipelineName}-${d.startedAt}`} className="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-3">
              <div className="flex items-center gap-2">
                {d.status === 'success'
                  ? <CheckCircle size={13} className="text-emerald-400" />
                  : d.status === 'failed'
                    ? <XCircle size={13} className="text-red-400" />
                    : <AlertCircle size={13} className="text-amber-400" />}
                <span className="font-semibold text-[var(--color-text-primary)]">{d.pipelineName}</span>
              </div>
              <span className="font-mono text-[11px] text-[var(--color-text-secondary)]">{d.version}</span>
              <div className="flex items-center gap-1 text-[11px] text-[var(--color-text-secondary)]">
                <Clock size={10} />{formatDuration(d.startedAt, d.completedAt)}
              </div>
              <span className="ml-auto text-[11px] text-[var(--color-text-secondary)]">{timeAgo(d.startedAt)}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

// ─── Stack Component Connection UI ───────────────────────────────────────────
interface StackComponent {
  name: string
  description: string
  status: ToolHealthStatus
  version: string
}

const DETECTABLE_COMPONENTS: StackComponent[] = [
  { name: 'Grafana', description: 'Metrics & dashboards', status: 'warning', version: '10.3' },
  { name: 'Prometheus', description: 'Metrics & alerting', status: 'running', version: '2.48.1' },
  { name: 'ArgoCD', description: 'GitOps CD', status: 'running', version: '2.9.3' },
  { name: 'Harbor', description: 'Container registry', status: 'running', version: '2.8.2' },
  { name: 'GitLab', description: 'CI/CD pipelines', status: 'running', version: '16.7' },
  { name: 'Kibana', description: 'Log analysis', status: 'running', version: '8.11.0' },
]

function StackConnectPanel({
  stackName,
  onConnect,
  onSkip,
}: {
  stackName: string
  onConnect: (tabs: Pick<EmbedTab, 'label' | 'url'>[]) => void
  onSkip: () => void
}) {
  const { t } = useTranslation()
  const [urls, setUrls] = useState<Record<string, string>>({})
  const [open, setOpen] = useState<Record<string, boolean>>({})
  const [confirmed, setConfirmed] = useState<Record<string, boolean>>({})

  function toggleOpen(name: string) {
    setOpen((p) => ({ ...p, [name]: !p[name] }))
  }
  function setUrl(name: string, val: string) {
    setUrls((p) => ({ ...p, [name]: val }))
    setConfirmed((p) => ({ ...p, [name]: false }))
  }
  function confirmUrl(name: string) {
    if (urls[name]?.trim()) setConfirmed((p) => ({ ...p, [name]: true }))
  }

  const readyItems = DETECTABLE_COMPONENTS.filter((c) => confirmed[c.name] && urls[c.name]?.trim())

  return (
    <div className="mb-6 rounded-[var(--card-radius)] border border-[rgba(99,102,241,0.35)] bg-[rgba(99,102,241,0.04)] p-5">
      {/* Header */}
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="mb-1 flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-[rgba(99,102,241,0.2)] text-[#a5b4fc]">
              <Settings2 size={14} />
            </div>
            <h3 className="text-[15px] font-bold text-[var(--color-text-primary)]">
              {t('observability.connectPanel.title', 'Connect Stack Components')}
            </h3>
          </div>
          <p className="text-xs text-[var(--color-text-secondary)]">
            {t('observability.connectPanel.descriptionPrefix', 'Tools detected in')}{' '}
            <span className="font-semibold text-[var(--color-text-primary)]">{stackName}</span>.
            {' '}
            {t('observability.connectPanel.descriptionSuffix', 'Enter their dashboard URLs to add monitoring tabs.')}
          </p>
        </div>
        <button
          type="button"
          onClick={onSkip}
          className="text-xs text-[var(--color-text-secondary)] underline underline-offset-2 hover:text-[var(--color-text-primary)]"
        >
          Skip for now
        </button>
      </div>

      {/* Component grid */}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
        {DETECTABLE_COMPONENTS.map((comp) => {
          const cfg = TOOL_STATUS[comp.status]
          const isOpen = open[comp.name]
          const isDone = confirmed[comp.name] && !!urls[comp.name]

          return (
            <div
              key={comp.name}
              className={cn(
                'rounded-[10px] border bg-[rgba(255,255,255,0.02)] p-3.5 transition-colors',
                isDone
                  ? 'border-emerald-500/40 bg-emerald-500/5'
                  : isOpen
                    ? 'border-[#6366f1]/40'
                    : 'border-[var(--color-border-default)]',
              )}
            >
              {/* Tool header */}
              <div className="mb-2 flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5">
                    <span className="text-sm font-bold text-[var(--color-text-primary)]">{comp.name}</span>
                    <span className={cn('inline-flex items-center gap-0.5 rounded-[4px] px-1.5 py-0.5 text-[10px] font-semibold', cfg.cls)}>
                      {cfg.icon}{cfg.label}
                    </span>
                  </div>
                  <div className="mt-0.5 text-[11px] text-[var(--color-text-secondary)]">
                    {comp.description} · v{comp.version}
                  </div>
                </div>
                {isDone ? (
                  <span className="flex shrink-0 items-center gap-1 rounded-[5px] bg-emerald-500/15 px-2 py-1 text-[11px] font-semibold text-emerald-400">
                    <Check size={10} />Added
                  </span>
                ) : (
                  <button
                    type="button"
                    onClick={() => toggleOpen(comp.name)}
                    className={cn(
                      'shrink-0 rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors',
                      isOpen
                        ? 'border-[rgba(239,68,68,0.4)] text-[#f87171] hover:bg-red-400/10'
                        : 'border-[rgba(99,102,241,0.4)] text-[#a5b4fc] hover:bg-[rgba(99,102,241,0.1)]',
                    )}
                  >
                    {isOpen ? 'Cancel' : '+ Add URL'}
                  </button>
                )}
              </div>

              {/* URL input (when expanded) */}
              {isOpen && !isDone && (
                <div className="mt-2 flex min-w-0 gap-1.5">
                  <input
                    type="url"
                    value={urls[comp.name] ?? ''}
                    onChange={(e) => setUrl(comp.name, e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && confirmUrl(comp.name)}
                    placeholder={`https://${comp.name.toLowerCase()}.example.com/`}
                    className="min-w-0 flex-1 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.06)] px-2.5 py-[7px] text-xs text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] outline-none focus:border-[#6366f1]"
                  />
                  <button
                    type="button"
                    onClick={() => confirmUrl(comp.name)}
                    disabled={!urls[comp.name]?.trim()}
                    className="shrink-0 rounded-lg bg-[#6366f1] px-3 py-[7px] text-xs font-medium text-white hover:opacity-90 disabled:opacity-40"
                  >
                    Confirm
                  </button>
                </div>
              )}

              {/* Confirmed URL preview */}
              {isDone && (
                <div className="mt-1 flex items-center gap-1.5">
                  <span className="min-w-0 truncate font-mono text-[10px] text-emerald-400/80">{urls[comp.name]}</span>
                  <button
                    type="button"
                    onClick={() => { setConfirmed((p) => ({ ...p, [comp.name]: false })); setOpen((p) => ({ ...p, [comp.name]: true })) }}
                    className="shrink-0 text-[10px] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                  >
                    Edit
                  </button>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Footer actions */}
      <div className="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-[rgba(99,102,241,0.2)] pt-4">
        <p className="text-[11px] text-[var(--color-text-secondary)]">
          {readyItems.length > 0
            ? `${readyItems.length} component${readyItems.length > 1 ? 's' : ''} ready to connect`
            : t('observability.connectPanel.enterDashboardUrls', 'Enter dashboard URLs for the components you want to monitor')}
        </p>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onSkip}
            className="rounded-lg border border-[var(--color-border-default)] px-4 py-1.5 text-xs text-[var(--color-text-secondary)] hover:bg-[rgba(255,255,255,0.04)]"
          >
            Skip
          </button>
          <button
            type="button"
            disabled={readyItems.length === 0}
            onClick={() => onConnect(readyItems.map((c) => ({ label: c.name, url: urls[c.name] })))}
            className="flex items-center gap-1.5 rounded-lg bg-[#6366f1] px-4 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-40"
          >
            <Plus size={12} />Connect {readyItems.length > 0 ? `${readyItems.length} Component${readyItems.length > 1 ? 's' : ''}` : 'Components'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────
export function MonitoringPage() {
  const { t } = useTranslation()
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === 'admin'

  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId] = useState('')
  const [activeView, setActiveView] = useState<ViewType | null>(null)

  const { clusters, stacks, filteredStacks, selectedCluster, selectedStack, hasContext } =
    useClusterStackFilterState(selectedClusterId, selectedStackId)

  const didAutoSelectRef = useRef(false)

  useEffect(() => {
    if (didAutoSelectRef.current) return
    if (clusters.length === 0) return

    const firstCluster = clusters[0]
    if (!firstCluster) return

    setSelectedClusterId(firstCluster.id)

    const firstStackForCluster = stacks.find((stack) => stack.clusterId === firstCluster.id)
    if (firstStackForCluster) {
      setSelectedStackId(firstStackForCluster.id)
      setActiveView('stack')
    } else {
      setActiveView('cluster')
    }

    didAutoSelectRef.current = true
  }, [clusters, stacks])

  // Auto-select initial view
  function handleClusterChange(id: string) {
    const clusterChanged = id !== selectedClusterId
    setSelectedClusterId(id)
    if (clusterChanged) {
      setSelectedStackId('')
      if (activeView === 'stack') setActiveView('cluster')
    }
    if (id && !activeView) setActiveView('cluster')
  }
  function handleStackChange(id: string) {
    setSelectedStackId(id)
    if (id && !activeView) setActiveView('stack')
  }

  const supportsCicd = useMemo(() => {
    if (!selectedCluster) return false
    const types = Array.isArray(selectedCluster.types) && selectedCluster.types.length > 0
      ? selectedCluster.types
      : (selectedCluster.type ? [selectedCluster.type] : [])
    const normalizedTypes = Array.from(new Set(types))
    return normalizedTypes.includes('target')
  }, [selectedCluster])

  useEffect(() => {
    if (activeView === 'cicd' && !supportsCicd) {
      setActiveView(selectedClusterId ? 'cluster' : null)
    }
  }, [activeView, supportsCicd, selectedClusterId])

  const views: { id: ViewType; label: string; icon: React.ReactNode; disabled?: boolean }[] = [
    { id: 'cluster', label: 'Cluster', icon: <Server size={15} />, disabled: !selectedClusterId },
    { id: 'stack', label: 'Stack', icon: <BarChart3 size={15} />, disabled: !selectedStackId },
    { id: 'cicd', label: 'CI/CD', icon: <GitBranch size={15} />, disabled: !selectedClusterId || !supportsCicd },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: t('observability.monitoring', 'Monitoring Dashboard') }]} />

      {/* Page header */}
      <div className="mb-6 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(59,130,246,0.15)] text-[#60a5fa]">
          <BarChart3 size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">{t('observability.monitoring', 'Monitoring Dashboard')}</h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
            {t('observability.monitoringDesc', 'Select a Cluster or Stack to start monitoring')}
          </p>
        </div>
      </div>

      <ClusterStackFilter
        selectedClusterId={selectedClusterId}
        selectedStackId={selectedStackId}
        onClusterChange={handleClusterChange}
        onStackChange={handleStackChange}
        onClear={() => { setSelectedClusterId(''); setSelectedStackId(''); setActiveView(null) }}
        clusters={clusters}
        filteredStacks={filteredStacks}
        selectedCluster={selectedCluster}
        selectedStack={selectedStack}
      />

      {/* ── Empty state ── */}
      {
        !hasContext && (
          <div className="flex h-44 flex-col items-center justify-center gap-2 rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] text-[var(--color-text-secondary)]">
            <BarChart3 size={28} className="opacity-20" />
            <p className="text-sm font-medium text-[var(--color-text-primary)]">Select a Cluster or Stack above to begin</p>
            <p className="text-xs">You can select either one or both.</p>
          </div>
        )
      }

      {/* ── View switcher + content ── */}
      {
        hasContext && (
          <>
            {/* View tabs */}
            <div className="mb-0 flex items-end border-b border-[var(--color-border-default)]">
              {views.map((v) => (
                <button key={v.id} type="button"
                  onClick={() => !v.disabled && setActiveView(v.id)}
                  disabled={v.disabled}
                  className={cn(
                    'flex items-center gap-2 border-b-2 px-5 py-3 text-sm font-semibold transition-colors',
                    activeView === v.id
                      ? 'border-b-[var(--color-primary)] text-[var(--color-text-primary)]'
                      : v.disabled
                        ? 'cursor-not-allowed border-b-transparent text-[var(--color-text-secondary)] opacity-35'
                        : 'border-b-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
                  )}>
                  {v.icon}{v.label}
                </button>
              ))}
            </div>

            {/* View content */}
            <div className="pt-5">
              {activeView === 'cluster' && (
                <DashboardTabLayout
                  viewId="cluster"
                  isAdmin={isAdmin}
                  defaultContent={
                    <ClusterDefault
                      clusterId={selectedClusterId}
                      clusterName={selectedCluster?.name ?? ''}
                      stackIds={stacks.filter((stack) => stack.clusterId === selectedClusterId).map((stack) => stack.id)}
                    />
                  }
                />
              )}
              {activeView === 'stack' && (
                <DashboardTabLayout
                  viewId="stack"
                  isAdmin={isAdmin}
                  defaultContent={<StackDefault stackId={selectedStack?.id ?? selectedStackId} />}
                  firstTimePanel={(onConnect, onSkip) => (
                    <StackConnectPanel
                      stackName={selectedStack?.name ?? selectedStackId}
                      onConnect={onConnect}
                      onSkip={onSkip}
                    />
                  )}
                />
              )}
              {activeView === 'cicd' && (
                <DashboardTabLayout
                  viewId="cicd"
                  isAdmin={isAdmin}
                  defaultContent={<CicdDefault selectedClusterId={selectedClusterId} />}
                  seedTabs={CICD_DEFAULT_TABS}
                />
              )}
            </div>
          </>
        )
      }
    </div >
  )
}
