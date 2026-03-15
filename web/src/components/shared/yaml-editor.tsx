import { useRef, useState } from 'react'
import { Copy, Check, AlignLeft } from 'lucide-react'
import { cn } from '../../lib/utils'

interface YamlEditorProps {
  value: string
  onChange?: (value: string) => void
  readOnly?: boolean
  height?: string
}

function parseYaml(text: string): string | null {
  const lines = text.split('\n')
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i]
    if (/^\t/.test(line)) {
      return `Line ${i + 1}: YAML does not allow tab indentation`
    }
  }

  return null
}

function formatYaml(text: string): string {
  const lines = text.split('\n')
  const result: string[] = []
  for (const line of lines) {
    const withoutTabs = line.replace(/^\t+/, (tabs) => '  '.repeat(tabs.length))
    result.push(withoutTabs)
  }
  return result.join('\n')
}

export function YamlEditor({ value, onChange, readOnly = false, height = '400px' }: YamlEditorProps) {
  const lineNumbersRef = useRef<HTMLDivElement>(null)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const lineCount = value.split('\n').length
  const heightClass = height === '360px' ? 'h-[360px]' : 'h-[400px]'

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newValue = e.target.value
    const parseError = parseYaml(newValue)
    setError(parseError)
    onChange?.(newValue)
  }

  const handleCopy = () => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const handleFormat = () => {
    const formatted = formatYaml(value)
    const parseError = parseYaml(formatted)
    setError(parseError)
    onChange?.(formatted)
  }

  const handleScroll = (e: React.UIEvent<HTMLTextAreaElement | HTMLDivElement>) => {
    if (lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = e.currentTarget.scrollTop
    }
  }

  return (
    <div
      className={cn(
        'flex flex-col overflow-hidden rounded-[var(--card-radius)] border bg-[#0d1117] font-mono',
        error ? 'border-[rgba(239,68,68,0.5)]' : 'border-[var(--color-border-default)]'
      )}
    >
      <div className="flex shrink-0 items-center justify-between border-b border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-[14px] py-2">
        <span className="text-[11px] font-semibold tracking-[0.06em] text-[var(--color-text-secondary)] uppercase">
          yaml
        </span>
        <div className="flex gap-1.5">
          {!readOnly && (
            <button
              type="button"
              onClick={handleFormat}
              className="flex cursor-pointer items-center gap-[5px] rounded-md border border-[var(--color-border-default)] bg-none px-2.5 py-1 text-xs text-[var(--color-text-secondary)]"
            >
              <AlignLeft size={12} />
              Format
            </button>
          )}
          <button
            type="button"
            onClick={handleCopy}
            className={cn(
              'flex cursor-pointer items-center gap-[5px] rounded-md border border-[var(--color-border-default)] bg-none px-2.5 py-1 text-xs',
              copied ? 'text-[#22c55e]' : 'text-[var(--color-text-secondary)]'
            )}
          >
            {copied ? <Check size={12} /> : <Copy size={12} />}
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>
      </div>

      <div className={cn('flex flex-1 overflow-hidden', heightClass)}>
        <div
          ref={lineNumbersRef}
          className="w-12 shrink-0 overflow-y-hidden border-r border-r-[rgba(255,255,255,0.06)] pt-2.5 select-none"
        >
          {Array.from({ length: lineCount }, (_, i) => i + 1).map((lineNo) => (
            <div
              key={`yaml-line-${lineNo}`}
              className="h-[22px] pr-2.5 text-right text-[13px] leading-[22px] text-[#4a5568]"
            >
              {lineNo}
            </div>
          ))}
        </div>

        {readOnly ? (
          <div
            onScroll={handleScroll}
            className="flex-1 overflow-auto px-4 py-2.5 text-[13px] leading-[22px] whitespace-pre text-[#e2e8f0]"
          >
            {value || ' '}
          </div>
        ) : (
          <textarea
            value={value}
            onChange={handleChange}
            onScroll={handleScroll}
            readOnly={readOnly}
            spellCheck={false}
            className="h-full w-full flex-1 resize-none overflow-auto border-none bg-transparent px-4 py-2.5 font-mono text-[13px] leading-[22px] text-[#e2e8f0] outline-none"
          />
        )}
      </div>

      {error && (
        <div className="shrink-0 border-t border-t-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.1)] px-[14px] py-1.5 text-xs text-[#f87171]">
          {error}
        </div>
      )}
    </div>
  )
}
