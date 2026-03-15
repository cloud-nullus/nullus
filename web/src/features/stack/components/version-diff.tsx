import { type ReactNode, useMemo, useState } from 'react'
import Prism from 'prismjs'
import 'prismjs/components/prism-json'
import 'prismjs/themes/prism-tomorrow.css'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { cn } from '../../../lib/utils'
import type { StackVersionDiff } from '../api/stack-api'

type DiffPane = 'left' | 'right'

interface VersionDiffProps {
  versionA: number
  versionB: number
  configA: Record<string, unknown>
  configB: Record<string, unknown>
  diff: StackVersionDiff
}

export function VersionDiff({ versionA, versionB, configA, configB, diff }: VersionDiffProps) {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <DiffPaneCard title={`Version A (v${versionA})`} pane="left" data={configA} diff={diff} />
      <DiffPaneCard title={`Version B (v${versionB})`} pane="right" data={configB} diff={diff} />
    </div>
  )
}

function DiffPaneCard({
  title,
  pane,
  data,
  diff,
}: {
  title: string
  pane: DiffPane
  data: Record<string, unknown>
  diff: StackVersionDiff
}) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  const keys = useMemo(() => Object.keys(data).sort(), [data])

  return (
    <div className="overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[rgba(15,23,42,0.45)]">
      <div className="border-b border-[var(--color-border-default)] px-4 py-3">
        <p className="m-0 text-sm font-semibold text-[var(--color-text-primary)]">{title}</p>
      </div>
      <div className="max-h-[480px] overflow-auto p-3 font-mono text-[12px] leading-6">
        {keys.map((key) => (
          <JsonNode
            key={key}
            keyName={key}
            path={key}
            value={data[key]}
            depth={0}
            pane={pane}
            diff={diff}
            expanded={expanded}
            onToggle={(target) => setExpanded((prev) => ({ ...prev, [target]: !prev[target] }))}
          />
        ))}
      </div>
    </div>
  )
}

function JsonNode({
  keyName,
  path,
  value,
  depth,
  pane,
  diff,
  expanded,
  onToggle,
}: {
  keyName: string
  path: string
  value: unknown
  depth: number
  pane: DiffPane
  diff: StackVersionDiff
  expanded: Record<string, boolean>
  onToggle: (path: string) => void
}) {
  const indent = depth * 16
  const pathState = stateForPath(path, pane, diff)
  const isObject = isPlainObject(value)
  const open = expanded[path] ?? true

  if (isObject) {
    const entries = Object.entries(value).sort(([a], [b]) => a.localeCompare(b))
    return (
      <div>
        <button
          type="button"
          onClick={() => onToggle(path)}
          className={cn('flex w-full items-center gap-1 rounded px-1 py-0.5 text-left', stateClass(pathState))}
          style={{ paddingLeft: indent + 4 }}
        >
          {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          <span className="text-[#93c5fd]">"{keyName}"</span>
          <span className="text-[#94a3b8]">{`{ ${entries.length} }`}</span>
        </button>
        {open &&
          entries.map(([childKey, childValue]) => (
            <JsonNode
              key={childKey}
              keyName={childKey}
              path={`${path}.${childKey}`}
              value={childValue}
              depth={depth + 1}
              pane={pane}
              diff={diff}
              expanded={expanded}
              onToggle={onToggle}
            />
          ))}
      </div>
    )
  }

  const tokens = Prism.tokenize(JSON.stringify(value), Prism.languages.json)
  return (
    <div className={cn('rounded px-1 py-0.5', stateClass(pathState))} style={{ marginLeft: indent }}>
      <span className="text-[#93c5fd]">"{keyName}"</span>
      <span className="text-[#94a3b8]">: </span>
      <span>{renderPrismTokens(tokens, `${path}-${keyName}`)}</span>
    </div>
  )
}

function stateForPath(path: string, pane: DiffPane, diff: StackVersionDiff): 'none' | 'added' | 'removed' | 'changed' {
  if (diff.changed[path]) {
    return 'changed'
  }
  if (pane === 'left' && Object.hasOwn(diff.removed, path)) {
    return 'removed'
  }
  if (pane === 'right' && Object.hasOwn(diff.added, path)) {
    return 'added'
  }
  return 'none'
}

function stateClass(state: 'none' | 'added' | 'removed' | 'changed') {
  if (state === 'added') {
    return 'bg-[rgba(34,197,94,0.18)]'
  }
  if (state === 'removed') {
    return 'bg-[rgba(239,68,68,0.2)]'
  }
  if (state === 'changed') {
    return 'bg-[rgba(250,204,21,0.18)]'
  }
  return ''
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function renderPrismTokens(tokens: Array<string | Prism.Token>, keyPrefix: string): ReactNode {
  return tokens.map((token, index) => {
    const key = `${keyPrefix}-${index}`
    if (typeof token === 'string') {
      return <span key={key}>{token}</span>
    }

    const className = Array.isArray(token.type) ? token.type.join(' ') : token.type
    const content = renderPrismTokenContent(token.content, key)

    return (
      <span key={key} className={`token ${className}`}>
        {content}
      </span>
    )
  })
}

function renderPrismTokenContent(content: Prism.Token['content'], keyPrefix: string): ReactNode {
  if (Array.isArray(content)) {
    return renderPrismTokens(content as Array<string | Prism.Token>, keyPrefix)
  }
  if (typeof content === 'string') {
    return content
  }

  const className = Array.isArray(content.type) ? content.type.join(' ') : content.type
  return (
    <span className={`token ${className}`}>
      {renderPrismTokenContent(content.content, `${keyPrefix}-nested`)}
    </span>
  )
}
