import { useMemo, useState } from 'react'
import { Copy, Check } from 'lucide-react'
import { cn } from '../../lib/utils'

type Language = 'yaml' | 'json' | 'bash' | 'typescript'

interface CodePreviewProps {
  code: string
  language?: Language
  title?: string
  maxHeight?: string
}

export function CodePreview({ code, language = 'yaml', title, maxHeight = '400px' }: CodePreviewProps) {
  const [copied, setCopied] = useState(false)
  const maxHeightClass =
    maxHeight === '520px'
      ? 'max-h-[520px]'
      : maxHeight === '500px'
        ? 'max-h-[500px]'
        : maxHeight === '600px'
          ? 'max-h-[600px]'
          : 'max-h-[400px]'

  const highlightedLines = useMemo(() => {
    const lineCounts = new Map<string, number>()
    const result: Array<{ id: string; line: string; lineNumber: number }> = []
    let lineNumber = 0

    for (const line of code.split('\n')) {
      lineNumber += 1
      const seen = (lineCounts.get(line) ?? 0) + 1
      lineCounts.set(line, seen)
      result.push({ id: `${line}-${seen}`, line, lineNumber })
    }

    return result
  }, [code])

  const handleCopy = () => {
    void navigator.clipboard.writeText(code).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[#0d1117] font-mono">
      <div className="flex items-center justify-between border-b border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-[14px] py-2">
        <div className="flex items-center gap-2">
          <span className="text-[11px] font-semibold tracking-[0.06em] text-[var(--color-text-secondary)] uppercase">
            {language}
          </span>
          {title && (
            <>
              <span className="text-[var(--color-border-default)]">·</span>
              <span className="text-xs text-[var(--color-text-secondary)]">{title}</span>
            </>
          )}
        </div>
        <button
          type="button"
          onClick={handleCopy}
          className={cn(
            'flex cursor-pointer items-center gap-[5px] rounded-md border border-[var(--color-border-default)] bg-none px-2.5 py-1 text-xs transition-all duration-150 ease-in-out',
            copied ? 'text-[#22c55e]' : 'text-[var(--color-text-secondary)]'
          )}
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>

      <div className={cn('overflow-y-auto', maxHeightClass)}>
        <table className="w-full table-fixed border-collapse">
          <colgroup>
            <col className="w-12" />
            <col />
          </colgroup>
          <tbody>
            {highlightedLines.map((lineItem) => (
              <tr key={lineItem.id}>
                <td className="select-none border-r border-r-[rgba(255,255,255,0.06)] px-3 text-right align-top text-[13px] leading-[22px] text-[#4a5568]">
                  {lineItem.lineNumber}
                </td>
                <td
                  className="px-4 align-top whitespace-pre text-[13px] leading-[22px] text-[#e2e8f0]"
                >
                  {lineItem.line || ' '}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
