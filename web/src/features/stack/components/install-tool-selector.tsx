import { useTranslation } from 'react-i18next'
import { Check } from 'lucide-react'
import { cn } from '../../../lib/utils'
import type { ToolSelection } from '../stores/stack-config-store'
import type { ToolOption } from '../utils/install-constants'

export interface ToolSelectorProps {
  label: string
  options: ToolOption[]
  value: ToolSelection
  onChange: (v: ToolSelection) => void
}

export function ToolSelector({ label, options, value, onChange }: ToolSelectorProps) {
  const { t } = useTranslation()
  const optionsWithNone = [
    {
      id: '',
      label: t('stackInstall.common.unselected', 'Not selected'),
      description: t('stackInstall.common.notInstalled', 'This item will not be installed.'),
    },
    ...options,
  ]

  return (
    <div className="mb-5">
      <div className="mb-2.5 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
        {label}
      </div>
      <div className="flex flex-col gap-2">
        {optionsWithNone.map((opt) => {
          const selected = value.tool === opt.id
          const displayLabel = opt.id ? t(`stackAddTools.tools.${opt.id}.label`, opt.label) : opt.label
          const displayDescription = opt.id ? t(`stackAddTools.tools.${opt.id}.description`, opt.description) : opt.description
          return (
            <button
              key={opt.id}
              type="button"
              onClick={() => onChange({ tool: opt.id, version: opt.id ? 'latest' : '' })}
              className={cn(
                'flex w-full cursor-pointer items-center gap-3 rounded-lg border px-[14px] py-3 text-left transition-all duration-150',
                selected
                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
              )}
            >
              <div
                className={cn(
                  'flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2',
                  selected
                    ? 'border-[#6366f1] bg-[#6366f1]'
                    : 'border-[var(--color-border-hover)] bg-transparent'
                )}
              >
                {selected && <div className="h-1.5 w-1.5 rounded-full bg-white" />}
              </div>
              <div>
                <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                  {displayLabel}
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">{displayDescription}</div>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}

export interface MultiToolSelectorProps {
  label: string
  options: ToolOption[]
  values: ToolSelection[]
  onChange: (values: ToolSelection[]) => void
}

export function MultiToolSelector({ label, options, values, onChange }: MultiToolSelectorProps) {
  const { t } = useTranslation()
  const selectedIds = new Set(values.map((item) => item.tool).filter(Boolean))

  const toggleSelection = (toolId: string) => {
    if (!toolId) {
      onChange([])
      return
    }
    const next = selectedIds.has(toolId)
      ? values.filter((item) => item.tool !== toolId)
      : [...values, { tool: toolId, version: 'latest' }]
    onChange(next)
  }

  return (
    <div className="mb-5">
      <div className="mb-2.5 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
        {label}
      </div>
      <div className="flex flex-col gap-2">
        <button
          type="button"
          onClick={() => onChange([])}
          className={cn(
            'flex w-full cursor-pointer items-center gap-3 rounded-lg border px-[14px] py-3 text-left transition-all duration-150',
            values.length === 0
              ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
              : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
          )}
        >
          <div
            className={cn(
              'flex h-4 w-4 shrink-0 items-center justify-center rounded border-2',
              values.length === 0
                ? 'border-[#6366f1] bg-[#6366f1]'
                : 'border-[var(--color-border-hover)] bg-transparent'
            )}
          >
            {values.length === 0 && <Check size={11} className="text-white" />}
          </div>
          <div>
            <div className={cn('text-sm font-semibold', values.length === 0 ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
              {t('stackInstall.common.unselected', 'Not selected')}
            </div>
            <div className="text-xs text-[var(--color-text-secondary)]">
              {t('stackInstall.common.notInstalled', 'This item will not be installed.')}
            </div>
          </div>
        </button>
        {options.map((opt) => {
          const selected = selectedIds.has(opt.id)
          const displayLabel = t(`stackAddTools.tools.${opt.id}.label`, opt.label)
          const displayDescription = t(`stackAddTools.tools.${opt.id}.description`, opt.description)
          return (
            <button
              key={opt.id}
              type="button"
              onClick={() => toggleSelection(opt.id)}
              className={cn(
                'flex w-full cursor-pointer items-center gap-3 rounded-lg border px-[14px] py-3 text-left transition-all duration-150',
                selected
                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
              )}
            >
              <div
                className={cn(
                  'flex h-4 w-4 shrink-0 items-center justify-center rounded border-2',
                  selected
                    ? 'border-[#6366f1] bg-[#6366f1]'
                    : 'border-[var(--color-border-hover)] bg-transparent'
                )}
              >
                {selected && <Check size={11} className="text-white" />}
              </div>
              <div>
                <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                  {displayLabel}
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">{displayDescription}</div>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
