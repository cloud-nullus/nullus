import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Check, ExternalLink, Plus, Settings2 } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { cn } from "../../../lib/utils"
import { useStackMonitoring } from "../../stack/api/stack-api"
import type { OSSMonitoringStatus } from "../../stack/api/stack-api-types"
import type { EmbedTab } from "../utils/monitoring-utils"
import { isValidEmbedUrl, normalizeEmbedUrl } from "../utils/monitoring-utils"
import { TOOL_STATUS } from "./monitoring-chart-widgets"
import { TextInput } from '../../../components/ui/text-input'

// ─── Stack Component Connection UI ───────────────────────────────────────────

/**
 * 스택이 설치한 OSS 의 접속 주소를 확인시키는 첫 화면.
 *
 * 목록도 주소도 지어내지 않는다. 무엇이 깔렸는지는 스택 모니터링 응답
 * (domain.InstalledToolWorkloads 가 단일 관문)에서, 주소는 설치할 때 받은 접속
 * 도메인에서 서버가 만들어 내려준 값(oss_statuses[].url)에서 온다. 예전에는
 * 도구 6개가 화면에 하드코딩돼 있어 깔리지도 않은 도구를 "detected" 로 보여주고,
 * 이미 아는 주소를 사용자에게 다시 받아 적게 했다.
 *
 * 기본 동작은 새 창으로 여는 것이다. Grafana·Argo CD·Harbor 처럼 대부분의 OSS 는
 * X-Frame-Options 로 iframe 을 막으므로, 임베드 탭은 되는 것만 골라서 켠다.
 */
export function StackConnectPanel({
  stackId,
  stackName,
  onConnect,
  onSkip,
}: {
  stackId: string
  stackName: string
  onConnect: (tabs: Pick<EmbedTab, 'label' | 'url'>[]) => void
  onSkip: () => void
}) {
  const { t } = useTranslation()
  const { data: monitoring } = useStackMonitoring(stackId)
  const [edited, setEdited] = useState<Record<string, string>>({})
  const [embedded, setEmbedded] = useState<Record<string, boolean>>({})

  const tools = useMemo(
    () => (monitoring?.oss_statuses ?? []).filter((tool) => tool.enabled),
    [monitoring],
  )

  // 편집한 값이 있으면 그것이, 없으면 서버가 준 주소가 진실이다.
  const urlOf = (tool: OSSMonitoringStatus) => edited[tool.key] ?? tool.url ?? ''
  const openableURL = (tool: OSSMonitoringStatus) => {
    const url = normalizeEmbedUrl(urlOf(tool))
    return url && isValidEmbedUrl(url) ? url : ''
  }

  const selected = tools.filter((tool) => embedded[tool.key] && urlOf(tool).trim())

  return (
    <div className="mb-6 rounded-[var(--card-radius)] border border-[color-mix(in_srgb,_var(--color-primary)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_4%,_transparent)] p-5">
      {/* Header */}
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="mb-1 flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-[color-mix(in_srgb,_var(--color-primary)_20%,_transparent)] text-[var(--color-primary)]">
              <Settings2 {...iconProps('sm')} />
            </div>
            <h3 className="text-[15px] font-bold text-[var(--color-text-primary)]">
              {t('observability.connectPanel.title', 'Connect Stack Components')}
            </h3>
          </div>
          <p className="text-xs text-[var(--color-text-secondary)]">
            {t('observability.connectPanel.descriptionPrefix', 'Tools installed by')}{' '}
            <span className="font-semibold text-[var(--color-text-primary)]">{stackName}</span>.
            {' '}
            {t('observability.connectPanel.descriptionSuffix', 'Addresses come from the access domain you set at install time — edit any that differ.')}
          </p>
        </div>
        <button
          type="button"
          onClick={onSkip}
          className="text-xs text-[var(--color-text-secondary)] underline underline-offset-2 hover:text-[var(--color-text-primary)]"
        >
          {t('observability.connectPanel.skipForNow', 'Skip for now')}
        </button>
      </div>

      {tools.length === 0 ? (
        <p
          data-testid="connect-empty"
          className="rounded-[10px] border border-dashed border-[var(--color-border-default)] px-4 py-6 text-center text-xs text-[var(--color-text-secondary)]"
        >
          {t('observability.connectPanel.noTools', 'No installed components could be read for this stack yet.')}
        </p>
      ) : (
        <>
          {/* Component grid */}
          <div className="grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3">
            {tools.map((tool) => {
              const cfg = TOOL_STATUS[tool.status]
              const launchURL = openableURL(tool)
              const isEmbedded = !!embedded[tool.key] && !!urlOf(tool).trim()

              return (
                <div
                  key={tool.key}
                  data-testid={`connect-row-${tool.key}`}
                  className={cn(
                    'rounded-[10px] border bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3.5 transition-colors',
                    isEmbedded
                      ? 'border-[color-mix(in_srgb,_var(--color-success)_40%,_transparent)] bg-[color-mix(in_srgb,_var(--color-success)_5%,_transparent)]'
                      : 'border-[var(--color-border-default)]',
                  )}
                >
                  {/* Tool header */}
                  <div className="mb-2 flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span className="text-sm font-bold text-[var(--color-text-primary)]">{tool.name}</span>
                        <span className="inline-flex items-center gap-0.5 rounded-[4px] px-1.5 py-0.5 text-[10px] font-semibold" style={cfg?.style}>
                          {cfg?.icon}{cfg?.label}
                        </span>
                      </div>
                      <div className="mt-0.5 text-[11px] text-[var(--color-text-secondary)]">
                        {t(`observability.connectPanel.role.${tool.key}`, tool.key)}
                        {tool.version && tool.version !== '-' ? ` · ${tool.version}` : ''}
                      </div>
                    </div>
                    {launchURL && (
                      <a
                        data-testid={`connect-open-${tool.key}`}
                        href={launchURL}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex shrink-0 items-center gap-1 rounded-lg border border-[color-mix(in_srgb,_var(--color-primary)_40%,_transparent)] px-2.5 py-1 text-xs font-medium text-[var(--color-primary)] hover:bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)]"
                      >
                        <ExternalLink {...iconProps('xs')} />
                        {t('observability.connectPanel.open', 'Open')}
                      </a>
                    )}
                  </div>

                  {/* Address — prefilled, editable */}
                  <TextInput
                    type="url"
                    data-testid={`connect-url-${tool.key}`}
                    value={urlOf(tool)}
                    onChange={(e) => setEdited((p) => ({ ...p, [tool.key]: e.target.value }))}
                    placeholder={t('observability.connectPanel.urlPlaceholder', 'https://tool.your-domain')}
                    className="w-full min-w-0 focus:border-[var(--color-primary)]"
                  />

                  {/* Embed opt-in */}
                  <label className="mt-2 flex items-center gap-1.5 text-[11px] text-[var(--color-text-secondary)]">
                    <input
                      type="checkbox"
                      data-testid={`connect-embed-${tool.key}`}
                      checked={!!embedded[tool.key]}
                      disabled={!urlOf(tool).trim()}
                      onChange={(e) => setEmbedded((p) => ({ ...p, [tool.key]: e.target.checked }))}
                      className="h-3.5 w-3.5 accent-[var(--color-primary)]"
                    />
                    {t('observability.connectPanel.embedAsTab', 'Add as embedded tab')}
                  </label>
                </div>
              )
            })}
          </div>

          {/* Footer actions */}
          <div className="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-[color-mix(in_srgb,_var(--color-primary)_20%,_transparent)] pt-4">
            <p className="text-[11px] text-[var(--color-text-secondary)]">
              {selected.length > 0 ? (
                <span className="inline-flex items-center gap-1 text-[var(--color-success)]">
                  <Check {...iconProps('xs')} />
                  {t('observability.connectPanel.selectedCount', '{{count}} selected', { count: selected.length })}
                </span>
              ) : (
                t('observability.connectPanel.embedHint', 'Most tools block iframe embedding — open them in a new tab, and embed only the ones that allow it.')
              )}
            </p>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={onSkip}
                className="rounded-lg border border-[var(--color-border-default)] px-4 py-1.5 text-xs text-[var(--color-text-secondary)] hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)]"
              >
                {t('observability.connectPanel.skip', 'Skip')}
              </button>
              <button
                type="button"
                data-testid="connect-submit"
                disabled={selected.length === 0}
                onClick={() => onConnect(selected.map((tool) => ({ label: tool.name, url: urlOf(tool) })))}
                className="flex items-center gap-1.5 rounded-lg bg-[var(--color-primary)] px-4 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-40"
              >
                <Plus {...iconProps('xs')} />
                {t('observability.connectPanel.addTabs', 'Add tabs')}
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
