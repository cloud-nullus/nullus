import { type ChangeEvent, type ReactNode, useState } from 'react'
import { Search } from 'lucide-react'

interface PageHeaderProps {
  title: string
  subtitle?: string
  searchPlaceholder?: string
  onSearch?: (query: string) => void
  actions?: ReactNode
  children?: ReactNode
}

export function PageHeader({
  title,
  subtitle,
  searchPlaceholder = 'Search...',
  onSearch,
  actions,
  children,
}: PageHeaderProps) {
  const [query, setQuery] = useState('')

  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextQuery = event.target.value
    setQuery(nextQuery)
    onSearch?.(nextQuery)
  }

  return (
    <div className="mb-6 flex items-center justify-between gap-4">
      <div className="min-w-0">
        <h1 className="m-0 text-2xl font-bold text-[var(--color-text-primary)]">
          {title}
        </h1>
        {subtitle && (
          <p className="mb-0 mt-1 text-sm text-[var(--color-text-secondary)]">
            {subtitle}
          </p>
        )}
      </div>

      <div className="ml-auto flex items-center gap-2.5">
        {onSearch && (
          <div className="flex h-9 min-w-[220px] items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-2.5">
            <Search size={14} color="var(--color-text-secondary)" />
            <input
              type="search"
              value={query}
              onChange={handleSearchChange}
              placeholder={searchPlaceholder}
              className="w-full border-none bg-transparent text-[13px] text-[var(--color-text-primary)] outline-none"
            />
          </div>
        )}
        {actions && <div className="flex items-center gap-2">{actions}</div>}
        {children && <div className="flex items-center gap-2">{children}</div>}
      </div>
    </div>
  )
}
