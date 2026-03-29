import { useMemo, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '../../../lib/utils'
import type { StackVersionDiff } from '../api/stack-api'

interface DiffLine {
  type: 'add' | 'remove' | 'change' | 'unchanged' | 'header'
  oldLineNum?: number
  newLineNum?: number
  content: string
  key: string
}

interface CollapsedRegion {
  startIndex: number
  count: number
}

interface VersionDiffProps {
  versionA: number
  versionB: number
  configA: Record<string, unknown>
  configB: Record<string, unknown>
  diff: StackVersionDiff
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function flattenObject(obj: Record<string, unknown>, prefix = ''): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (isPlainObject(value)) {
      Object.assign(out, flattenObject(value, path))
    } else {
      out[path] = value
    }
  }
  return out
}

function formatValue(value: unknown): string {
  if (value === null) return 'null'
  if (value === undefined) return '~'
  if (typeof value === 'string') return `"${value}"`
  if (typeof value === 'boolean' || typeof value === 'number') return String(value)
  if (Array.isArray(value)) return JSON.stringify(value)
  return JSON.stringify(value)
}

function topLevelSection(key: string): string {
  const dot = key.indexOf('.')
  return dot === -1 ? key : key.slice(0, dot)
}

function computeUnifiedDiff(
  oldObj: Record<string, unknown>,
  newObj: Record<string, unknown>,
  diff: StackVersionDiff,
): DiffLine[] {
  const flatOld = flattenObject(oldObj)
  const flatNew = flattenObject(newObj)

  const allKeys = Array.from(new Set([...Object.keys(flatOld), ...Object.keys(flatNew)])).sort()

  const lines: DiffLine[] = []
  let oldLine = 1
  let newLine = 1
  let lastSection = ''

  for (const key of allKeys) {
    const section = topLevelSection(key)
    if (section !== lastSection) {
      lines.push({ type: 'header', content: `@@ ${section} @@`, key: `header-${section}` })
      lastSection = section
    }

    const inOld = Object.hasOwn(flatOld, key)
    const inNew = Object.hasOwn(flatNew, key)

    if (diff.changed[key]) {
      const [oldVal, newVal] = diff.changed[key]
      lines.push({ type: 'remove', oldLineNum: oldLine, content: `- ${key}: ${formatValue(oldVal)}`, key: `rm-${key}` })
      oldLine++
      lines.push({ type: 'add', newLineNum: newLine, content: `+ ${key}: ${formatValue(newVal)}`, key: `add-${key}` })
      newLine++
    } else if (!inOld && inNew) {
      lines.push({ type: 'add', newLineNum: newLine, content: `+ ${key}: ${formatValue(flatNew[key])}`, key: `add-${key}` })
      newLine++
    } else if (inOld && !inNew) {
      lines.push({ type: 'remove', oldLineNum: oldLine, content: `- ${key}: ${formatValue(flatOld[key])}`, key: `rm-${key}` })
      oldLine++
    } else {
      lines.push({
        type: 'unchanged',
        oldLineNum: oldLine,
        newLineNum: newLine,
        content: `  ${key}: ${formatValue(flatOld[key])}`,
        key: `eq-${key}`,
      })
      oldLine++
      newLine++
    }
  }

  return lines
}

function findCollapsedRegions(lines: DiffLine[], threshold = 3): CollapsedRegion[] {
  const regions: CollapsedRegion[] = []
  let runStart = -1
  let runLength = 0

  for (let i = 0; i < lines.length; i++) {
    if (lines[i].type === 'unchanged') {
      if (runStart === -1) runStart = i
      runLength++
    } else {
      if (runLength > threshold) {
        regions.push({ startIndex: runStart, count: runLength })
      }
      runStart = -1
      runLength = 0
    }
  }
  if (runLength > threshold) {
    regions.push({ startIndex: runStart, count: runLength })
  }

  return regions
}

export function VersionDiff({ versionA, versionB, configA, configB, diff }: VersionDiffProps) {
  const { t } = useTranslation()
  const lines = useMemo(() => computeUnifiedDiff(configA, configB, diff), [configA, configB, diff])
  const collapsedRegions = useMemo(() => findCollapsedRegions(lines), [lines])
  const [expandedRegions, setExpandedRegions] = useState<Set<number>>(new Set())

  const toggleRegion = (startIndex: number) => {
    setExpandedRegions((prev) => {
      const next = new Set(prev)
      if (next.has(startIndex)) {
        next.delete(startIndex)
      } else {
        next.add(startIndex)
      }
      return next
    })
  }

  const collapsedIndices = useMemo(() => {
    const set = new Set<number>()
    for (const region of collapsedRegions) {
      if (!expandedRegions.has(region.startIndex)) {
        for (let i = region.startIndex; i < region.startIndex + region.count; i++) {
          set.add(i)
        }
      }
    }
    return set
  }, [collapsedRegions, expandedRegions])

  const adds = lines.filter((l) => l.type === 'add').length
  const removes = lines.filter((l) => l.type === 'remove').length

  return (
    <div className="overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[rgba(15,23,42,0.45)]">
      <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-4 py-2.5">
        <p className="m-0 text-sm font-semibold text-[var(--color-text-primary)]">
          v{versionA} → v{versionB}
        </p>
        <div className="flex items-center gap-3 text-xs font-mono">
          {adds > 0 && <span className="text-[#22c55e]">+{adds}</span>}
          {removes > 0 && <span className="text-[#ef4444]">-{removes}</span>}
        </div>
      </div>

      <div className="max-h-[480px] overflow-auto">
        {lines.length === 0 && (
          <p className="px-4 py-6 text-center text-sm text-[var(--color-text-muted)]">
            {t('versionDiff.empty', 'No configuration changes found.')}
          </p>
        )}

        {lines.map((line, idx) => {
          if (collapsedIndices.has(idx)) {
            const region = collapsedRegions.find((r) => r.startIndex === idx)
            if (region) {
              return (
                <button
                  key={`collapse-${region.startIndex}`}
                  type="button"
                  onClick={() => toggleRegion(region.startIndex)}
                  className="flex w-full items-center gap-1.5 border-y border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-1.5 text-xs text-[var(--color-text-muted)] hover:bg-[rgba(255,255,255,0.05)] transition-colors cursor-pointer"
                >
                  <ChevronRight size={12} />
                  <span>... {region.count} unchanged lines ...</span>
                </button>
              )
            }
            return null
          }

          const expandedRegion = collapsedRegions.find(
            (r) => r.startIndex === idx && expandedRegions.has(r.startIndex),
          )

          return (
            <div key={line.key}>
              {expandedRegion && (
                <button
                  type="button"
                  onClick={() => toggleRegion(expandedRegion.startIndex)}
                  className="flex w-full items-center gap-1.5 border-y border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-1.5 text-xs text-[var(--color-text-muted)] hover:bg-[rgba(255,255,255,0.05)] transition-colors cursor-pointer"
                >
                  <ChevronDown size={12} />
                  <span>... {expandedRegion.count} unchanged lines (click to collapse) ...</span>
                </button>
              )}
              <DiffRow line={line} />
            </div>
          )
        })}
      </div>
    </div>
  )
}

function DiffRow({ line }: { line: DiffLine }) {
  if (line.type === 'header') {
    return (
      <div className="bg-[rgba(99,102,241,0.08)] px-4 py-1.5 font-mono text-xs text-[var(--color-text-muted)]">
        {line.content}
      </div>
    )
  }

  return (
    <div
      className={cn(
        'flex items-stretch font-mono text-xs leading-6',
        line.type === 'add' && 'bg-[rgba(34,197,94,0.15)]',
        line.type === 'remove' && 'bg-[rgba(239,68,68,0.15)]',
        line.type === 'change' && 'bg-[rgba(245,158,11,0.15)]',
        line.type === 'unchanged' && 'text-[var(--color-text-muted)]',
      )}
    >
      <span className="w-10 shrink-0 select-none text-right pr-1 text-[var(--color-text-muted)] opacity-50">
        {line.oldLineNum ?? ''}
      </span>
      <span className="w-10 shrink-0 select-none text-right pr-2 text-[var(--color-text-muted)] opacity-50 border-r border-[var(--color-border-default)]">
        {line.newLineNum ?? ''}
      </span>
      <span className="flex-1 px-3 whitespace-pre">{line.content}</span>
    </div>
  )
}
