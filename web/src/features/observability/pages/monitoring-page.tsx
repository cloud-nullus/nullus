import { useMemo, useState, useCallback, useId } from 'react'
import {
  AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import {
  Cpu, HardDrive, MemoryStick, Box, CheckCircle, AlertCircle, XCircle,
  Server, GitBranch, BarChart3, Settings2, Plus, Trash2, Save,
  GripVertical, ChevronDown, ChevronUp, Check, RefreshCw, Lock,
  Activity, Clock, Package, TrendingUp, TrendingDown, Layers,
} from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { NativeSelect } from '../../../components/ui/native-select'
import { useDashboard } from '../api/observability-api'
import type { ToolHealthStatus } from '../api/observability-api'
import { useStacks } from '../../stack/api/stack-api'
import { useClusters } from '../../admin/api/admin-api'
import { useAuthStore } from '../../../stores/auth-store'
import { cn } from '../../../lib/utils'

// ─── Types ────────────────────────────────────────────────────────────────────
type ViewType = 'cluster' | 'stack' | 'cicd'
type TimeRange = '1h' | '6h' | '24h' | '7d'

interface EmbedTab {
  id: string
  label: string
  url: string
  order: number
}

// ─── Shared chart style helpers ───────────────────────────────────────────────
const CHART_STYLE = {
  bg: '#0b1220',
  grid: 'rgba(148,163,184,0.15)',
  tick: { fill: '#94a3b8', fontSize: 11 },
  tooltip: { background: '#111827', border: '1px solid #374151', color: '#e5e7eb' },
}

const TOOL_STATUS: Record<ToolHealthStatus, { icon: React.ReactNode; cls: string; label: string }> = {
  running: { icon: <CheckCircle size={13} />, cls: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',  label: 'Running' },
  warning: { icon: <AlertCircle size={13} />, cls: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Warning' },
  error:   { icon: <XCircle size={13} />,    cls: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]',   label: 'Error'   },
}

const statusDot = (s: string) => (
  <span className={cn('h-2 w-2 rounded-full shrink-0 inline-block',
    s === 'running' || s === 'connected' || s === 'completed' || s === 'success' ? 'bg-emerald-400' :
    s === 'warning' || s === 'pending'  ? 'bg-amber-400' : 'bg-red-400')} />
)

// ─── Time series generator ────────────────────────────────────────────────────
function makeSeries(range: TimeRange) {
  const cfg: Record<TimeRange, [number, number]> = {
    '1h': [12, 5], '6h': [12, 30], '24h': [24, 60], '7d': [14, 1440],
  }
  const [pts, stepMin] = cfg[range]
  const now = Date.now()
  return Array.from({ length: pts }, (_, i) => {
    const t = new Date(now - (pts - 1 - i) * stepMin * 60_000)
    const label = range === '7d'
      ? t.toLocaleDateString('en', { weekday: 'short' })
      : t.toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit', hour12: false })
    return {
      time: label,
      cpu:    Math.max(12, Math.min(96, Math.round(56 + Math.sin(i / 2.5) * 16 + (i % 3) * 2))),
      memory: Math.max(24, Math.min(97, Math.round(63 + Math.cos(i / 3.2) * 10 + (i % 4) * 2))),
      success: Math.round(89 + Math.random() * 10),
    }
  })
}

const WEEK_BARS = [
  { day: 'Mon', success: 16, failed: 2 }, { day: 'Tue', success: 19, failed: 3 },
  { day: 'Wed', success: 15, failed: 4 }, { day: 'Thu', success: 21, failed: 2 },
  { day: 'Fri', success: 24, failed: 3 }, { day: 'Sat', success: 11, failed: 2 },
  { day: 'Sun', success: 9,  failed: 1 },
]

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
  const uid = useId()
  const [activeId, setActiveId]     = useState('default')
  const [tabs, setTabs]             = useState<EmbedTab[]>(() => loadTabs(viewId, seedTabs))
  const [isManaging, setIsManaging] = useState(false)
  const [drafts, setDrafts]         = useState<EmbedTab[]>([])
  const [saved, setSaved]           = useState(false)
  const [embedError, setEmbedError] = useState(false)
  const [skipConnect, setSkipConnect] = useState(() => {
    try { return localStorage.getItem(SKIP_KEY(viewId)) === 'true' } catch { return false }
  })

  const allTabs = [{ id: 'default', label: 'Default' }, ...tabs]
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
    setDrafts((p) => [...p, { id: `tab-${uid}-${Date.now()}`, label: 'New Tab', url: '', order: p.length }])
  }, [uid])

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
            <Settings2 size={13} />{isManaging ? 'Cancel' : 'Manage Tabs'}
          </button>
        )}
      </div>

      {/* Admin manage panel */}
      {isManaging && (
        <div className="w-full border-b border-[var(--color-border-default)] bg-amber-500/5 px-1 py-4 sm:px-2">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <span className="text-sm font-semibold text-[var(--color-text-primary)]">Manage Custom Tabs</span>
            <div className="flex flex-wrap gap-2">
              <button type="button" onClick={addDraft}
                className="flex items-center gap-1.5 rounded-lg border border-[var(--color-border-default)] px-3 py-1.5 text-xs text-[var(--color-text-secondary)] hover:border-[var(--color-primary)] hover:text-[var(--color-primary)]">
                <Plus size={12} />Add Tab
              </button>
              <button type="button" onClick={saveManage}
                className="flex items-center gap-1.5 rounded-lg bg-[var(--color-primary)] px-4 py-1.5 text-xs font-medium text-white hover:opacity-90">
                <Save size={12} />Save Changes
              </button>
            </div>
          </div>

          {drafts.length === 0 ? (
            <p className="text-xs text-[var(--color-text-secondary)]">No custom tabs yet. Click "Add Tab" to create one.</p>
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
                      placeholder="Tab name"
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
                      placeholder="Embed URL"
                      className="w-full rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[7px] text-xs text-[var(--color-text-primary)] placeholder:text-[var(--color-text-secondary)] outline-none focus:border-[#6366f1]"
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
          <p className="mt-2 text-[10px] text-[var(--color-text-secondary)]">
            Changes apply to all users after saving. Developer role has view-only access.
          </p>
        </div>
      )}

      {saved && (
        <div className="flex items-center gap-2 border-b border-emerald-500/20 bg-emerald-500/5 px-1 py-2 text-xs text-emerald-400 sm:px-2">
          <Check size={12} />Tab configuration saved.
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
function ClusterDefault({ clusterId }: { clusterId: string }) {
  const [range, setRange] = useState<TimeRange>('24h')
  const series = useMemo(() => makeSeries(range), [range])
  const pods = [
    { name: 'Running', value: 22, color: '#22c55e' },
    { name: 'Pending', value: 1,  color: '#f59e0b' },
    { name: 'Failed',  value: 1,  color: '#ef4444' },
  ]
  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <div className={cn('flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium',
          'border-emerald-500/20 bg-emerald-500/5 text-emerald-400')}>
          <span className="h-2 w-2 rounded-full bg-emerald-400" />{clusterId}
        </div>
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-[var(--color-text-secondary)]">Range:</span>
          {(['1h','6h','24h','7d'] as TimeRange[]).map((r) => (
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
        <KpiCard label="Nodes"   value="3/4"   icon={<Server size={18}/>}      color="#60a5fa" iconCls="bg-[rgba(59,130,246,0.15)] text-[#60a5fa]"  bar={75}/>
        <KpiCard label="Pods"    value="22/24"  icon={<Box size={18}/>}         color="#22c55e" iconCls="bg-[rgba(34,197,94,0.15)] text-[#22c55e]"   bar={92}/>
        <KpiCard label="CPU"     value="62%"   icon={<Cpu size={18}/>}          color="#f59e0b" iconCls="bg-[rgba(245,158,11,0.15)] text-[#f59e0b]"  bar={62}/>
        <KpiCard label="Memory"  value="71%"   icon={<MemoryStick size={18}/>}  color="#a78bfa" iconCls="bg-[rgba(139,92,246,0.15)] text-[#a78bfa]"  bar={71}/>
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPanel title="CPU Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="ccpu" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.5}/><stop offset="95%" stopColor="#f59e0b" stopOpacity={.05}/></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <YAxis domain={[0,100]} stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Area type="monotone" dataKey="cpu" name="CPU %" stroke="#f59e0b" strokeWidth={2} fill="url(#ccpu)"/>
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Memory Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="cmem" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={.5}/><stop offset="95%" stopColor="#3b82f6" stopOpacity={.05}/></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <YAxis domain={[0,100]} stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Area type="monotone" dataKey="memory" name="Memory %" stroke="#3b82f6" strokeWidth={2} fill="url(#cmem)"/>
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pod Status">
          <ResponsiveContainer width="100%" height={220}>
            <PieChart>
              <Pie data={pods} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>
                {pods.map((e) => <Cell key={e.name} fill={e.color}/>)}
              </Pie>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Legend wrapperStyle={{ color: '#e5e7eb' }}/>
            </PieChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pipeline Success (this week)">
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={WEEK_BARS}>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="day" stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Legend wrapperStyle={{ color: '#e5e7eb' }}/>
              <Bar dataKey="success" fill="#22c55e" radius={[4,4,0,0]}/>
              <Bar dataKey="failed"  fill="#ef4444" radius={[4,4,0,0]}/>
            </BarChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>
    </div>
  )
}

// ─── Default content: Stack view ─────────────────────────────────────────────
function StackDefault({ stackName }: { stackName: string }) {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: apiData, isLoading, refetch } = useDashboard(5000)
  const series = useMemo(() => makeSeries(range), [range])

  const fallback = { kpi: { cpuUsage: 68, memoryUsage: 42, storageUsage: 31, podCount: 27, podRunning: 24 }, pipeline: { successRate: 97.3, totalRuns: 145, avgBuildSeconds: 154 }, tools: [{ name: 'GitLab', version: '16.7', status: 'running' as const }, { name: 'ArgoCD', version: '2.9.3', status: 'running' as const }, { name: 'Prometheus', version: '2.48.1', status: 'running' as const }, { name: 'Grafana', version: '10.3', status: 'warning' as const }, { name: 'Harbor', version: '2.8.2', status: 'running' as const }] }
  const dash = (!isLoading && apiData && 'kpi' in apiData) ? apiData : fallback
  const kpi = dash.kpi

  const kpis = [
    { label: 'CPU Usage',    value: `${kpi.cpuUsage}%`,                             icon: <Cpu size={18}/>,        color: '#60a5fa', iconCls: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',  bar: kpi.cpuUsage },
    { label: 'Memory',       value: `${kpi.memoryUsage}%`,                           icon: <MemoryStick size={18}/>, color: '#a78bfa', iconCls: 'bg-[rgba(139,92,246,0.15)] text-[#a78bfa]',  bar: kpi.memoryUsage },
    { label: 'Storage',      value: `${kpi.storageUsage}%`,                          icon: <HardDrive size={18}/>,  color: '#34d399', iconCls: 'bg-[rgba(16,185,129,0.15)] text-[#34d399]',  bar: kpi.storageUsage },
    { label: 'Running Pods', value: `${kpi.podRunning}/${kpi.podCount}`,             icon: <Box size={18}/>,        color: '#fbbf24', iconCls: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]',  bar: kpi.podCount ? Math.round(kpi.podRunning / kpi.podCount * 100) : 0 },
  ]

  const podData = [
    { name: 'Running', value: kpi.podRunning, color: '#22c55e' },
    { name: 'Pending', value: Math.max(1, kpi.podCount - kpi.podRunning - 1), color: '#f59e0b' },
    { name: 'Failed',  value: 1, color: '#ef4444' },
  ]

  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-1.5 text-xs text-[var(--color-text-secondary)]">
          Stack: <span className="font-semibold text-[var(--color-text-primary)]">{stackName}</span>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <button type="button" onClick={() => void refetch()}
            className="flex items-center gap-1 rounded-lg border border-[var(--color-border-default)] px-2.5 py-[5px] text-xs text-[var(--color-text-secondary)] hover:bg-[rgba(255,255,255,0.06)]">
            <RefreshCw size={11} className={cn(isLoading && 'animate-spin')} />Refresh
          </button>
          {(['1h','6h','24h','7d'] as TimeRange[]).map((r) => (
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
        {kpis.map((c) => <KpiCard key={c.label} {...c} />)}
      </div>

      <div className="mb-5 grid grid-cols-2 gap-3.5">
        <ChartPanel title="CPU Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="scpu" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.5}/><stop offset="95%" stopColor="#f59e0b" stopOpacity={.05}/></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick}/><YAxis domain={[0,100]} stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Area type="monotone" dataKey="cpu" name="CPU %" stroke="#f59e0b" strokeWidth={2} fill="url(#scpu)"/>
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Memory Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="smem" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={.5}/><stop offset="95%" stopColor="#3b82f6" stopOpacity={.05}/></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick}/><YAxis domain={[0,100]} stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Area type="monotone" dataKey="memory" name="Memory %" stroke="#3b82f6" strokeWidth={2} fill="url(#smem)"/>
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pipeline Success Rate">
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={WEEK_BARS}>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="day" stroke="#94a3b8" tick={CHART_STYLE.tick}/><YAxis stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/><Legend wrapperStyle={{ color: '#e5e7eb' }}/>
              <Bar dataKey="success" fill="#22c55e" radius={[4,4,0,0]}/><Bar dataKey="failed" fill="#ef4444" radius={[4,4,0,0]}/>
            </BarChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pod Status">
          <ResponsiveContainer width="100%" height={220}>
            <PieChart>
              <Pie data={podData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>
                {podData.map((e) => <Cell key={e.name} fill={e.color}/>)}
              </Pie>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/><Legend wrapperStyle={{ color: '#e5e7eb' }}/>
            </PieChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>

      <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
        <h2 className="mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">Tool Health</h2>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
          {dash.tools.map((t) => {
            const cfg = TOOL_STATUS[t.status]
            return (
              <div key={t.name} className="rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3.5">
                <div className="mb-1.5 flex items-center justify-between">
                  <span className="text-sm font-bold text-[var(--color-text-primary)]">{t.name}</span>
                  <span className={cn('inline-flex items-center gap-1 rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', cfg.cls)}>
                    {cfg.icon}{cfg.label}
                  </span>
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">v{t.version}</div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// ─── Default content: CI/CD view ─────────────────────────────────────────────
// ─── CI/CD Application monitoring data ───────────────────────────────────────
type AppStatus = 'healthy' | 'degraded' | 'down'
type AppEnv    = 'prod' | 'staging' | 'dev'

interface DeployedApp {
  name: string; version: string; pipeline: string; env: AppEnv
  status: AppStatus; pods: [number, number]; respMs: number
  errRate: number; lastDeploy: string; reqRate: number
}

const MOCK_APPS: DeployedApp[] = [
  { name: 'app-frontend',     version: 'v2.3.1', pipeline: 'GitLab CI', env: 'prod',    status: 'healthy',  pods: [3,3], respMs: 125, errRate: 0.1, lastDeploy: '2m ago',  reqRate: 1240 },
  { name: 'app-backend',      version: 'v1.8.0', pipeline: 'GitLab CI', env: 'prod',    status: 'healthy',  pods: [5,5], respMs: 89,  errRate: 0.0, lastDeploy: '15m ago', reqRate: 3870 },
  { name: 'auth-service',     version: 'v0.9.2', pipeline: 'GitLab CI', env: 'prod',    status: 'degraded', pods: [1,3], respMs: 450, errRate: 3.2, lastDeploy: '1h ago',  reqRate: 580  },
  { name: 'payment-api',      version: 'v3.1.0', pipeline: 'ArgoCD',    env: 'prod',    status: 'healthy',  pods: [2,2], respMs: 201, errRate: 0.2, lastDeploy: '3h ago',  reqRate: 920  },
  { name: 'notification-svc', version: 'v0.4.1', pipeline: 'ArgoCD',    env: 'prod',    status: 'healthy',  pods: [2,2], respMs: 67,  errRate: 0.0, lastDeploy: '5h ago',  reqRate: 430  },
  { name: 'worker-jobs',      version: 'v1.2.3', pipeline: 'GitLab CI', env: 'prod',    status: 'healthy',  pods: [3,3], respMs: 0,   errRate: 0.0, lastDeploy: '1d ago',  reqRate: 0    },
  { name: 'data-pipeline',    version: 'v2.0.0', pipeline: 'ArgoCD',    env: 'staging', status: 'healthy',  pods: [1,1], respMs: 312, errRate: 0.5, lastDeploy: '2d ago',  reqRate: 140  },
  { name: 'admin-ui',         version: 'v1.5.0', pipeline: 'GitLab CI', env: 'staging', status: 'down',     pods: [0,2], respMs: 0,   errRate: 100, lastDeploy: '2d ago',  reqRate: 0    },
]

const RECENT_DEPLOYS = [
  { app: 'app-frontend',  version: 'v2.3.1', env: 'prod',    pipeline: 'GitLab CI', status: 'success', time: '2m ago',  duration: '1m 42s' },
  { app: 'app-backend',   version: 'v1.8.0', env: 'prod',    pipeline: 'GitLab CI', status: 'success', time: '15m ago', duration: '2m 11s' },
  { app: 'auth-service',  version: 'v0.9.2', env: 'prod',    pipeline: 'GitLab CI', status: 'failed',  time: '1h ago',  duration: '3m 05s' },
  { app: 'payment-api',   version: 'v3.1.0', env: 'prod',    pipeline: 'ArgoCD',    status: 'success', time: '3h ago',  duration: '58s'    },
  { app: 'admin-ui',      version: 'v1.5.0', env: 'staging', pipeline: 'GitLab CI', status: 'failed',  time: '2d ago',  duration: '4m 22s' },
]

const REQ_SERIES = [
  { time: '00:00', frontend: 980,  backend: 3200, auth: 620  },
  { time: '04:00', frontend: 420,  backend: 1800, auth: 310  },
  { time: '08:00', frontend: 1100, backend: 3900, auth: 740  },
  { time: '12:00', frontend: 1560, backend: 4800, auth: 890  },
  { time: '16:00', frontend: 1380, backend: 4200, auth: 710  },
  { time: '20:00', frontend: 1240, backend: 3870, auth: 580  },
  { time: 'now',   frontend: 1240, backend: 3870, auth: 580  },
]

const ERR_SERIES = [
  { time: '00:00', frontend: 0.0, backend: 0.0, auth: 0.8 },
  { time: '04:00', frontend: 0.1, backend: 0.0, auth: 1.2 },
  { time: '08:00', frontend: 0.0, backend: 0.1, auth: 2.0 },
  { time: '12:00', frontend: 0.2, backend: 0.0, auth: 2.8 },
  { time: '16:00', frontend: 0.1, backend: 0.1, auth: 3.0 },
  { time: '20:00', frontend: 0.1, backend: 0.0, auth: 3.2 },
  { time: 'now',   frontend: 0.1, backend: 0.0, auth: 3.2 },
]

const APP_STATUS_CFG: Record<AppStatus, { label: string; cls: string; dot: string }> = {
  healthy:  { label: 'Healthy',  cls: 'bg-emerald-500/15 text-emerald-400', dot: 'bg-emerald-400' },
  degraded: { label: 'Degraded', cls: 'bg-amber-500/15 text-amber-400',    dot: 'bg-amber-400'   },
  down:     { label: 'Down',     cls: 'bg-red-500/15 text-red-400',        dot: 'bg-red-400'     },
}
const ENV_CFG: Record<AppEnv, { cls: string }> = {
  prod:    { cls: 'bg-[rgba(99,102,241,0.12)] text-[#a5b4fc]'  },
  staging: { cls: 'bg-[rgba(245,158,11,0.12)] text-[#fbbf24]'  },
  dev:     { cls: 'bg-[rgba(148,163,184,0.12)] text-[#94a3b8]' },
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

function CicdDefault() {
  const [envFilter, setEnvFilter] = useState<AppEnv | 'all'>('all')
  const [range, setRange]         = useState<TimeRange>('24h')

  const filtered = envFilter === 'all' ? MOCK_APPS : MOCK_APPS.filter((a) => a.env === envFilter)

  const healthy  = filtered.filter((a) => a.status === 'healthy').length
  const degraded = filtered.filter((a) => a.status === 'degraded').length
  const down     = filtered.filter((a) => a.status === 'down').length
  const totalPods = filtered.reduce((s, a) => s + a.pods[0], 0)

  const appKpis = [
    { label: 'Total Apps',      value: String(filtered.length), icon: <Layers size={18}/>,      color: '#6366f1', iconCls: 'bg-[rgba(99,102,241,0.15)] text-[#6366f1]',  bar: 100 },
    { label: 'Healthy',         value: String(healthy),          icon: <CheckCircle size={18}/>, color: '#22c55e', iconCls: 'bg-emerald-500/15 text-emerald-400',         bar: Math.round(healthy / filtered.length * 100) || 0 },
    { label: 'Degraded / Down', value: `${degraded} / ${down}`, icon: <AlertCircle size={18}/>, color: '#f59e0b', iconCls: 'bg-amber-500/15 text-amber-400',             bar: Math.round((degraded + down) / filtered.length * 100) || 0 },
    { label: 'Running Pods',    value: String(totalPods),        icon: <Box size={18}/>,         color: '#10b981', iconCls: 'bg-[rgba(16,185,129,0.15)] text-[#10b981]',  bar: 80 },
  ]

  return (
    <div>
      {/* Toolbar */}
      <div className="mb-5 flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="text-xs text-[var(--color-text-secondary)]">Environment</span>
          {(['all', 'prod', 'staging', 'dev'] as const).map((e) => (
            <button key={e} type="button" onClick={() => setEnvFilter(e)}
              className={cn('rounded-[7px] border px-2.5 py-[5px] text-xs font-bold capitalize',
                envFilter === e
                  ? 'border-[rgba(99,102,241,0.6)] bg-[rgba(99,102,241,0.15)] text-[#a5b4fc]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]')}>
              {e === 'all' ? 'All' : e}
            </button>
          ))}
        </div>
        <div className="ml-auto flex items-center gap-2">
          {(['1h','6h','24h','7d'] as TimeRange[]).map((r) => (
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
      <div className="mb-5 grid grid-cols-2 gap-3.5">
        <ChartPanel title="Request Rate (req/min)">
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={REQ_SERIES}>
              <defs>
                <linearGradient id="rr-fe" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#6366f1" stopOpacity={.4}/><stop offset="95%" stopColor="#6366f1" stopOpacity={.02}/></linearGradient>
                <linearGradient id="rr-be" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#22c55e" stopOpacity={.4}/><stop offset="95%" stopColor="#22c55e" stopOpacity={.02}/></linearGradient>
                <linearGradient id="rr-au" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.4}/><stop offset="95%" stopColor="#f59e0b" stopOpacity={.02}/></linearGradient>
              </defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Legend wrapperStyle={{ color: '#e5e7eb', fontSize: 11 }}/>
              <Area type="monotone" dataKey="frontend" name="app-frontend" stroke="#6366f1" strokeWidth={2} fill="url(#rr-fe)"/>
              <Area type="monotone" dataKey="backend"  name="app-backend"  stroke="#22c55e" strokeWidth={2} fill="url(#rr-be)"/>
              <Area type="monotone" dataKey="auth"     name="auth-service" stroke="#f59e0b" strokeWidth={2} fill="url(#rr-au)"/>
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Error Rate (%)">
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={ERR_SERIES}>
              <defs>
                <linearGradient id="er-fe" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#6366f1" stopOpacity={.3}/><stop offset="95%" stopColor="#6366f1" stopOpacity={.02}/></linearGradient>
                <linearGradient id="er-au" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#ef4444" stopOpacity={.4}/><stop offset="95%" stopColor="#ef4444" stopOpacity={.02}/></linearGradient>
              </defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3"/>
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick}/>
              <Tooltip contentStyle={CHART_STYLE.tooltip}/>
              <Legend wrapperStyle={{ color: '#e5e7eb', fontSize: 11 }}/>
              <Area type="monotone" dataKey="frontend" name="app-frontend" stroke="#6366f1" strokeWidth={2} fill="url(#er-fe)"/>
              <Area type="monotone" dataKey="auth"     name="auth-service" stroke="#ef4444" strokeWidth={2} fill="url(#er-au)"/>
            </AreaChart>
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
          <span className="text-xs text-[var(--color-text-secondary)]">{filtered.length} apps</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[var(--color-border-default)] text-[11px] text-[var(--color-text-secondary)]">
                {['Application', 'Version', 'Pipeline', 'Env', 'Status', 'Pods', 'Resp. Time', 'Error Rate', 'Req/min', 'Last Deploy'].map((h) => (
                  <th key={h} className="px-4 py-2.5 text-left font-semibold tracking-[0.03em]">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((app, i) => {
                const sc = APP_STATUS_CFG[app.status]
                const ec = ENV_CFG[app.env]
                const isLast = i === filtered.length - 1
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
                      <span className={cn('rounded-[4px] px-1.5 py-0.5 text-[10px] font-bold uppercase', ec.cls)}>{app.env}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', sc.cls)}>{sc.label}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn('font-mono', app.pods[0] < app.pods[1] ? 'text-amber-400' : 'text-[var(--color-text-primary)]')}>
                        {app.pods[0]}/{app.pods[1]}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn('font-mono', app.respMs > 300 ? 'text-amber-400' : app.respMs === 0 ? 'text-[var(--color-text-secondary)]' : 'text-[var(--color-text-primary)]')}>
                        {app.respMs ? `${app.respMs}ms` : '—'}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 font-mono">
                        {app.errRate > 1
                          ? <><TrendingUp size={11} className="text-red-400" /><span className="text-red-400">{app.errRate}%</span></>
                          : app.errRate > 0
                          ? <><TrendingUp size={11} className="text-amber-400" /><span className="text-amber-400">{app.errRate}%</span></>
                          : <><TrendingDown size={11} className="text-emerald-400" /><span className="text-emerald-400">0%</span></>}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <Activity size={11} />
                        <span className="font-mono">{app.reqRate > 0 ? app.reqRate.toLocaleString() : '—'}</span>
                      </div>
                    </td>
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
          {RECENT_DEPLOYS.map((d) => (
            <div key={`${d.app}-${d.time}`} className="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-3">
              <div className="flex items-center gap-2">
                {d.status === 'success'
                  ? <CheckCircle size={13} className="text-emerald-400" />
                  : <XCircle size={13} className="text-red-400" />}
                <span className="font-semibold text-[var(--color-text-primary)]">{d.app}</span>
              </div>
              <span className="font-mono text-[11px] text-[var(--color-text-secondary)]">{d.version}</span>
              <span className={cn('rounded-[4px] px-1.5 py-0.5 text-[10px] font-bold uppercase', ENV_CFG[d.env as AppEnv].cls)}>{d.env}</span>
              <div className="flex items-center gap-1 text-[11px] text-[var(--color-text-secondary)]">
                <GitBranch size={10} />{d.pipeline}
              </div>
              <div className="flex items-center gap-1 text-[11px] text-[var(--color-text-secondary)]">
                <Clock size={10} />{d.duration}
              </div>
              <span className="ml-auto text-[11px] text-[var(--color-text-secondary)]">{d.time}</span>
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
  { name: 'Grafana',    description: 'Metrics & dashboards',  status: 'warning', version: '10.3'   },
  { name: 'Prometheus', description: 'Metrics & alerting',    status: 'running', version: '2.48.1' },
  { name: 'ArgoCD',     description: 'GitOps CD',             status: 'running', version: '2.9.3'  },
  { name: 'Harbor',     description: 'Container registry',    status: 'running', version: '2.8.2'  },
  { name: 'GitLab',     description: 'CI/CD pipelines',       status: 'running', version: '16.7'   },
  { name: 'Kibana',     description: 'Log analysis',          status: 'running', version: '8.11.0' },
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
  const [urls, setUrls]           = useState<Record<string, string>>({})
  const [open, setOpen]           = useState<Record<string, boolean>>({})
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
            <h3 className="text-[15px] font-bold text-[var(--color-text-primary)]">Connect Stack Components</h3>
          </div>
          <p className="text-xs text-[var(--color-text-secondary)]">
            Tools detected in <span className="font-semibold text-[var(--color-text-primary)]">{stackName}</span>.
            Enter their dashboard URLs to add monitoring tabs.
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
            : 'Enter dashboard URLs for the components you want to monitor'}
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
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === 'admin'

  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId]     = useState('')
  const [activeView, setActiveView]               = useState<ViewType | null>(null)

  const { data: clustersData } = useClusters()
  const { data: stacksData }   = useStacks()
  const clusters = clustersData?.items ?? []
  const stacks   = stacksData?.items   ?? []

  const hasContext = selectedClusterId !== '' || selectedStackId !== ''

  // Auto-select initial view
  function handleClusterChange(id: string) {
    setSelectedClusterId(id)
    if (id && !activeView) setActiveView('cluster')
  }
  function handleStackChange(id: string) {
    setSelectedStackId(id)
    if (id && !activeView) setActiveView('stack')
  }

  const selectedCluster = clusters.find((c) => c.id === selectedClusterId)
  const selectedStack   = stacks.find((s) => s.id === selectedStackId)

  const views: { id: ViewType; label: string; icon: React.ReactNode; disabled?: boolean }[] = [
    { id: 'cluster', label: 'Cluster',  icon: <Server size={15} />,    disabled: !selectedClusterId },
    { id: 'stack',   label: 'Stack',    icon: <BarChart3 size={15} />, disabled: !selectedStackId },
    { id: 'cicd',    label: 'CI/CD',    icon: <GitBranch size={15} /> },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: 'Monitoring Dashboard' }]} />

      {/* Page header */}
      <div className="mb-6 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(59,130,246,0.15)] text-[#60a5fa]">
          <BarChart3 size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">Monitoring Dashboard</h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">Select a Cluster or Stack to start monitoring</p>
        </div>
      </div>

      {/* ── Context selector ── */}
      <div className="mb-5 flex flex-wrap items-end gap-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
        <div className="flex items-end gap-3">
          <NativeSelect
            label="Cluster"
            value={selectedClusterId}
            onChange={(e) => handleClusterChange(e.target.value)}
            className="min-w-[200px]"
          >
            <option value="">— Select Cluster —</option>
            {clusters.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </NativeSelect>
          {selectedCluster && (
            <div className="mb-[9px] flex items-center gap-1.5 text-xs">
              {statusDot(selectedCluster.status)}
              <span className="capitalize text-[var(--color-text-secondary)]">{selectedCluster.status}</span>
            </div>
          )}
        </div>

        <div className="flex items-end gap-3">
          <NativeSelect
            label="Stack"
            value={selectedStackId}
            onChange={(e) => handleStackChange(e.target.value)}
            className="min-w-[200px]"
          >
            <option value="">— Select Stack —</option>
            {stacks.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </NativeSelect>
          {selectedStack && (
            <div className="mb-[9px] flex items-center gap-1.5 text-xs">
              {statusDot(selectedStack.status)}
              <span className="capitalize text-[var(--color-text-secondary)]">{selectedStack.status}</span>
            </div>
          )}
        </div>

        {hasContext && (
          <button type="button"
            onClick={() => { setSelectedClusterId(''); setSelectedStackId(''); setActiveView(null) }}
            className="mb-[9px] text-xs text-[var(--color-text-secondary)] hover:text-red-400">
            Clear
          </button>
        )}
      </div>

      {/* ── Empty state ── */}
      {!hasContext && (
        <div className="flex h-44 flex-col items-center justify-center gap-2 rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] text-[var(--color-text-secondary)]">
          <BarChart3 size={28} className="opacity-20" />
          <p className="text-sm font-medium text-[var(--color-text-primary)]">Select a Cluster or Stack above to begin</p>
          <p className="text-xs">You can select either one or both.</p>
        </div>
      )}

      {/* ── View switcher + content ── */}
      {hasContext && (
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
                defaultContent={<ClusterDefault clusterId={selectedCluster?.name ?? selectedClusterId} />}
              />
            )}
            {activeView === 'stack' && (
              <DashboardTabLayout
                viewId="stack"
                isAdmin={isAdmin}
                defaultContent={<StackDefault stackName={selectedStack?.name ?? selectedStackId} />}
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
                defaultContent={<CicdDefault />}
                seedTabs={CICD_DEFAULT_TABS}
              />
            )}
          </div>
        </>
      )}
    </div>
  )
}
