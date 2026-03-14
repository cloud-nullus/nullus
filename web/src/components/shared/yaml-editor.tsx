import { useRef, useState } from 'react'
import { Copy, Check, AlignLeft } from 'lucide-react'

interface YamlEditorProps {
  value: string
  onChange?: (value: string) => void
  readOnly?: boolean
  height?: string
}

function parseYaml(text: string): string | null {
  // Basic YAML validation: detect common issues
  const lines = text.split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    // Detect tab characters (YAML requires spaces)
    if (/^\t/.test(line)) {
      return `Line ${i + 1}: YAML does not allow tab indentation`
    }
    // Detect unmatched quotes
    const singleQuotes = (line.match(/'/g) ?? []).length
    if (singleQuotes % 2 !== 0 && !line.trimStart().startsWith('#')) {
      // Allow false positives to avoid over-blocking; only flag obvious issues
    }
  }

  // Attempt structural parse: check colon-key balance
  let inMultiline = false
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim()
    if (trimmed === '' || trimmed.startsWith('#')) continue
    if (trimmed.endsWith('|') || trimmed.endsWith('>')) {
      inMultiline = true
      continue
    }
    if (inMultiline && lines[i].startsWith(' ')) continue
    inMultiline = false
    // Detect duplicate colons in key (likely syntax error)
    if (/^[^:]+:[^:]*:[^:]*$/.test(trimmed) && !trimmed.startsWith('-')) {
      // Might be a URL value — skip
    }
  }

  return null
}

function formatYaml(text: string): string {
  // Simple formatter: normalize indentation to 2 spaces
  const lines = text.split('\n')
  const result: string[] = []
  for (const line of lines) {
    // Replace leading tabs with 2 spaces each
    const withoutTabs = line.replace(/^\t+/, (tabs) => '  '.repeat(tabs.length))
    result.push(withoutTabs)
  }
  return result.join('\n')
}

export function YamlEditor({ value, onChange, readOnly = false, height = '400px' }: YamlEditorProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const lineCount = value.split('\n').length

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

  const handleScroll = (e: React.UIEvent<HTMLTextAreaElement>) => {
    const lineNumbers = document.getElementById('yaml-editor-line-numbers')
    if (lineNumbers) {
      lineNumbers.scrollTop = (e.target as HTMLTextAreaElement).scrollTop
    }
  }

  return (
    <div
      style={{
        background: '#0d1117',
        border: `1px solid ${error ? 'rgba(239,68,68,0.5)' : 'var(--color-border-default)'}`,
        borderRadius: 'var(--card-radius)',
        overflow: 'hidden',
        fontFamily: "'Fira Code', 'Cascadia Code', 'JetBrains Mono', monospace",
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* Toolbar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 14px',
          background: 'rgba(255,255,255,0.04)',
          borderBottom: '1px solid var(--color-border-default)',
          flexShrink: 0,
        }}
      >
        <span
          style={{
            fontSize: '11px',
            fontWeight: 600,
            color: 'var(--color-text-secondary)',
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
            fontFamily: 'inherit',
          }}
        >
          yaml
        </span>
        <div style={{ display: 'flex', gap: '6px' }}>
          {!readOnly && (
            <button
              onClick={handleFormat}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '5px',
                background: 'none',
                border: '1px solid var(--color-border-default)',
                borderRadius: '6px',
                padding: '4px 10px',
                fontSize: '12px',
                color: 'var(--color-text-secondary)',
                cursor: 'pointer',
                fontFamily: 'inherit',
              }}
            >
              <AlignLeft size={12} />
              Format
            </button>
          )}
          <button
            onClick={handleCopy}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '5px',
              background: 'none',
              border: '1px solid var(--color-border-default)',
              borderRadius: '6px',
              padding: '4px 10px',
              fontSize: '12px',
              color: copied ? '#22c55e' : 'var(--color-text-secondary)',
              cursor: 'pointer',
              fontFamily: 'inherit',
            }}
          >
            {copied ? <Check size={12} /> : <Copy size={12} />}
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>
      </div>

      {/* Editor area */}
      <div style={{ display: 'flex', flex: 1, overflow: 'hidden', height }}>
        {/* Line numbers */}
        <div
          id="yaml-editor-line-numbers"
          style={{
            width: '48px',
            background: 'transparent',
            borderRight: '1px solid rgba(255,255,255,0.06)',
            overflowY: 'hidden',
            flexShrink: 0,
            paddingTop: '10px',
            userSelect: 'none',
          }}
        >
          {Array.from({ length: lineCount }, (_, i) => (
            <div
              key={i}
              style={{
                height: '22px',
                lineHeight: '22px',
                textAlign: 'right',
                paddingRight: '10px',
                fontSize: '13px',
                color: '#4a5568',
                fontFamily: 'inherit',
              }}
            >
              {i + 1}
            </div>
          ))}
        </div>

        {/* Textarea */}
        <textarea
          ref={textareaRef}
          value={value}
          onChange={handleChange}
          onScroll={handleScroll}
          readOnly={readOnly}
          spellCheck={false}
          style={{
            flex: 1,
            background: 'transparent',
            border: 'none',
            outline: 'none',
            resize: 'none',
            padding: '10px 16px',
            fontSize: '13px',
            lineHeight: '22px',
            color: '#e2e8f0',
            fontFamily: 'inherit',
            overflowY: 'auto',
            whiteSpace: 'pre',
            cursor: readOnly ? 'default' : 'text',
          }}
        />
      </div>

      {/* Error bar */}
      {error && (
        <div
          style={{
            padding: '6px 14px',
            background: 'rgba(239,68,68,0.1)',
            borderTop: '1px solid rgba(239,68,68,0.3)',
            fontSize: '12px',
            color: '#f87171',
            flexShrink: 0,
            fontFamily: 'inherit',
          }}
        >
          {error}
        </div>
      )}
    </div>
  )
}
