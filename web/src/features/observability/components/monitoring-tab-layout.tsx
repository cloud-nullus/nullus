import React, { useState, useCallback, useId } from "react"
import { useTranslation } from "react-i18next"
import { Settings2, Plus, Save, ChevronUp, ChevronDown, GripVertical, Trash2, Check, Lock, Server } from "lucide-react"
import { Tabs } from "../../../components/ui/tabs"
import { Button } from "../../../components/ui/button"
import type { EmbedTab } from "../utils/monitoring-utils"
import {
  SKIP_KEY,
  loadTabs,
  persistTabs,
  normalizeEmbedUrl,
  isValidEmbedUrl,
  isKnownNonEmbeddableHost,
} from "../utils/monitoring-utils"
import { TextInput } from "../../../components/ui/text-input"

export type ViewType = 'cluster' | 'stack' | 'cicd'
export type TimeRange = '1h' | '6h' | '24h' | '7d'

// ─── DashboardTabLayout — shared tab system for all 3 views ──────────────────
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

export function DashboardTabLayout({ viewId, isAdmin, defaultContent, seedTabs, firstTimePanel }: TabLayoutProps) {
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
      <Tabs
        value={activeId}
        onChange={(id) => { setActiveId(id); setEmbedError(false) }}
        items={allTabs.map((t) => ({ id: t.id, label: t.label }))}
        trailing={
          isAdmin && (
            <Button
              variant="ghost"
              size="sm"
              type="button"
              onClick={isManaging ? cancelManage : openManage}
              // 관리 모드는 "지금 편집 중" 상태 표시라 경고 색을 쓴다.
              // 개편 전에는 토큰이 아닌 amber-400 이 직접 박혀 있어 테마를 안 따랐다.
              className={isManaging ? 'text-[var(--color-warning)]' : undefined}
            >
              <Settings2 size={13} />
              {isManaging ? t('common.cancel', 'Cancel') : t('monitoringPage.customTabs.manageTabs', 'Manage Tabs')}
            </Button>
          )
        }
      />

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
                    <TextInput
                      value={d.label}
                      onChange={(e) => patchDraft(d.id, { label: e.target.value })}
                      placeholder={t('monitoringPage.customTabs.tabNamePlaceholder', 'Tab name')}
                      className="min-w-0 flex-1 focus:border-[var(--color-primary)]"
                    />
                    <button type="button" onClick={() => removeDraft(d.id)}
                      className="shrink-0 rounded p-1 text-[var(--color-text-secondary)] hover:bg-red-400/10 hover:text-red-400">
                      <Trash2 size={13} />
                    </button>
                  </div>
                  {/* Row 2: URL input full width */}
                  <div className="mt-1.5 pl-[46px]">
                    <TextInput
                      value={d.url}
                      onChange={(e) => patchDraft(d.id, { url: e.target.value })}
                      placeholder={t('monitoringPage.customTabs.embedUrlPlaceholder', 'Embed URL')}
                      className="w-full placeholder:text-[var(--color-text-secondary)] focus:border-[var(--color-primary)]"
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
          <div className="flex items-center gap-2 border-b border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-1 py-2 sm:px-2">
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
