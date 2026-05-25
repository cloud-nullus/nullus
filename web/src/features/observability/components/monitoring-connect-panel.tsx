import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Check, Plus, Settings2 } from "lucide-react"
import { cn } from "../../../lib/utils"
import type { ToolHealthStatus } from "../api/observability-api"
import type { EmbedTab } from "../utils/monitoring-utils"
import { TOOL_STATUS } from "./monitoring-chart-widgets"

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

export function StackConnectPanel({
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
