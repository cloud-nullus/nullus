import { Link } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'

export interface BreadcrumbItem {
  label: string
  path?: string
}

interface BreadcrumbProps {
  items: BreadcrumbItem[]
}

export function Breadcrumb({ items }: BreadcrumbProps) {
  if (items.length === 0) return null

  return (
    <nav
      aria-label="Breadcrumb"
      className="mb-3 flex items-center gap-0.5 text-[12px]"
    >
      {items.map((item, index) => {
        const isLast = index === items.length - 1
        return (
          <div key={item.path ?? item.label} className="flex items-center gap-0.5">
            {index > 0 && (
              <ChevronRight
                size={12}
                className="mx-0.5 shrink-0 text-[var(--color-text-muted)]"
              />
            )}
            {isLast ? (
              <span className="font-medium text-[var(--color-text-primary)]">
                {item.label}
              </span>
            ) : (
              <Link
                to={item.path ?? '#'}
                className="text-[var(--color-text-secondary)] transition-colors duration-100 hover:text-[var(--color-text-primary)] no-underline"
              >
                {item.label}
              </Link>
            )}
          </div>
        )
      })}
    </nav>
  )
}
