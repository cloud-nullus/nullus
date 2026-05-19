import {
  AlertCircle,
  Check,
  CheckCircle,
  ChevronDown,
  ChevronUp,
  GripVertical,
  Lock,
  Plus,
  Save,
  Server,
  Settings2,
  Trash2,
  XCircle,
} from 'lucide-react'
import { useState, useCallback, useId } from 'react'
import type { ToolHealthStatus } from '../api/observability-api'
import { cn } from '../../../lib/utils'

export interface EmbedTab {
  id: string
  label: string
  url: string
  order: number
}

export const TOOL_STATUS: Record<ToolHealthStatus, { icon: React.ReactNode; cls: string; label: string }> = {
  running: { icon: <CheckCircle size={13} />, cls: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Running' },
  warning: { icon: <AlertCircle size={13} />, cls: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Warning' },
  error: { icon: <XCircle size={13} />, cls: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', label: 'Error' },
}

const STORAGE_KEY = (v: string) => `nullus_tabs_${v}_v1`
const SKIP_KEY = (v: string) => `nullus_skip_connect_${v}`

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
  viewId: string
  isAdmin: boolean
  defaultContent: React.ReactNode
  seedTabs?: EmbedTab[]
  firstTimePanel?: (
    onConnect: (tabs: Pick<EmbedTab, 'label' | 'url'>[]) => void,
    onSkip: () => void,
  ) => React.ReactNode
}

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

export function DashboardTabLayout({ viewId, isAdmin, defaultContent, seedTabs, firstTimePanel }: TabLayoutProps) {
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

export function StackConnectPanel({
  stackName,
  onConnect,
  onSkip,
}: {
  stackName: string
  onConnect: (tabs: Pick<EmbedTab, 'label' | 'url'>[]) => void
  onSkip: () => void
}) {
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
