import { useState } from 'react'
import { Copy, Check } from 'lucide-react'

type Language = 'yaml' | 'json' | 'bash'

interface CodePreviewProps {
  code: string
  language?: Language
  title?: string
  maxHeight?: string
}

export function CodePreview({ code, language = 'yaml', title, maxHeight = '400px' }: CodePreviewProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    void navigator.clipboard.writeText(code).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const lines = code.split('\n')

  return (
    <div
      style={{
        background: '#0d1117',
        border: '1px solid var(--color-border-default)',
        borderRadius: 'var(--card-radius)',
        overflow: 'hidden',
        fontFamily: "'Fira Code', 'Cascadia Code', 'JetBrains Mono', monospace",
      }}
    >
      {/* Header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 14px',
          background: 'rgba(255,255,255,0.04)',
          borderBottom: '1px solid var(--color-border-default)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
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
            {language}
          </span>
          {title && (
            <>
              <span style={{ color: 'var(--color-border-default)' }}>·</span>
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{title}</span>
            </>
          )}
        </div>
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
            transition: 'all var(--transition-fast)',
            fontFamily: 'inherit',
          }}
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>

      {/* Code area */}
      <div style={{ overflowY: 'auto', maxHeight }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
          <colgroup>
            <col style={{ width: '48px' }} />
            <col />
          </colgroup>
          <tbody>
            {lines.map((line, i) => (
              <tr key={i}>
                <td
                  style={{
                    padding: '0 12px',
                    fontSize: '13px',
                    lineHeight: '22px',
                    color: '#4a5568',
                    textAlign: 'right',
                    userSelect: 'none',
                    borderRight: '1px solid rgba(255,255,255,0.06)',
                    verticalAlign: 'top',
                    fontFamily: 'inherit',
                  }}
                >
                  {i + 1}
                </td>
                <td
                  style={{
                    padding: '0 16px',
                    fontSize: '13px',
                    lineHeight: '22px',
                    color: '#e2e8f0',
                    whiteSpace: 'pre',
                    verticalAlign: 'top',
                    fontFamily: 'inherit',
                  }}
                >
                  {line || ' '}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
