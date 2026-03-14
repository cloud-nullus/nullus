import { type ReactNode } from 'react'

interface PageHeaderProps {
  title: string
  children?: ReactNode
}

export function PageHeader({ title, children }: PageHeaderProps) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        marginBottom: '24px',
      }}
    >
      <h1
        style={{
          margin: 0,
          fontSize: '24px',
          fontWeight: 700,
          color: 'var(--color-text-primary)',
        }}
      >
        {title}
      </h1>
      {children && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          {children}
        </div>
      )}
    </div>
  )
}
