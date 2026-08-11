// 화면 최상단 규격. 28화면이 공유한다.
//
// 개편 전에는 이 파일이 존재하는데 쓰는 화면이 0곳이었고, 28화면이 각자
// `<div className="mb-6 flex items-center gap-2.5">` + 아이콘 + `<h1 className="text-[22px]
// font-extrabold">` 를 손으로 다시 만들고 있었다. 구조는 거의 같은데 mb-6/mb-7,
// items-start/items-center, justify-between 유무가 화면마다 달라서 상단 룩이
// 미묘하게 어긋났다 — "흐트러진 느낌" 의 절반이 여기서 나왔다.
//
// 구획은 카드가 아니라 선으로 나눈다(DESIGN.md §Layout — 화면 골격은 VS Code 를 따른다).
// 그래서 헤더는 아래를 1px 선으로 닫고, 그 아래 본문이 바로 붙는다.

import { type ChangeEvent, type ReactNode, useState } from 'react'
import { Search } from 'lucide-react'
import { Breadcrumb, type BreadcrumbItem } from '../shared/breadcrumb'

/** 아이콘 컨테이너 색. DESIGN.md §Shapes — 해당 기능 색의 15% 알파. */
export type PageHeaderTone = 'primary' | 'info' | 'success' | 'warning' | 'error' | 'accent'

const TONE_VAR: Record<PageHeaderTone, string> = {
  primary: '--color-primary',
  info: '--color-info',
  success: '--color-success',
  warning: '--color-warning',
  error: '--color-error',
  accent: '--color-accent-alt',
}

interface PageHeaderProps {
  title: string
  /** 문자열이 기본이지만, 상태·메타를 섞어 쓰는 화면이 있어 노드도 받는다. */
  subtitle?: ReactNode
  /** lucide 아이콘. 크기는 16이 규격이다 (컨테이너 --icon-size 28px). */
  icon?: ReactNode
  tone?: PageHeaderTone
  breadcrumb?: BreadcrumbItem[]
  searchPlaceholder?: string
  onSearch?: (query: string) => void
  actions?: ReactNode
  children?: ReactNode
}

export function PageHeader({
  title,
  subtitle,
  icon,
  tone = 'primary',
  breadcrumb,
  searchPlaceholder = 'Search...',
  onSearch,
  actions,
  children,
}: PageHeaderProps) {
  const [query, setQuery] = useState('')
  const toneVar = TONE_VAR[tone]

  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextQuery = event.target.value
    setQuery(nextQuery)
    onSearch?.(nextQuery)
  }

  return (
    <div className="mb-4 border-b border-[var(--color-border-default)] pb-3">
      {breadcrumb && breadcrumb.length > 0 && <Breadcrumb items={breadcrumb} />}

      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 items-center gap-2.5">
          {icon && (
            <span
              aria-hidden="true"
              className="flex h-[var(--icon-size)] w-[var(--icon-size)] shrink-0 items-center justify-center rounded-[var(--icon-radius)]"
              style={{
                backgroundColor: `color-mix(in srgb, var(${toneVar}) 15%, transparent)`,
                color: `var(${toneVar})`,
              }}
            >
              {icon}
            </span>
          )}
          <div className="min-w-0">
            <h1 className="m-0 text-[1.375rem] font-bold leading-tight tracking-[-0.01em] text-[var(--color-text-primary)]">
              {title}
            </h1>
            {/* subtitle 이 노드일 수 있어 p 가 아니라 div 로 감싼다 (p 안의 p 는 무효 HTML). */}
            {subtitle && (
              <div className="mt-0.5 text-[13px] leading-snug text-[var(--color-text-secondary)] [&>p]:m-0">
                {subtitle}
              </div>
            )}
          </div>
        </div>

        {(onSearch || actions || children) && (
          <div className="flex min-w-0 shrink-0 items-center gap-2">
            {onSearch && (
              <div className="flex h-[var(--control-height)] min-w-[200px] items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-2">
                <Search size={14} className="shrink-0 text-[var(--color-text-secondary)]" />
                <input
                  type="search"
                  value={query}
                  onChange={handleSearchChange}
                  placeholder={searchPlaceholder}
                  className="w-full border-none bg-transparent text-[13px] text-[var(--color-text-primary)] outline-none"
                />
              </div>
            )}
            {actions}
            {children}
          </div>
        )}
      </div>
    </div>
  )
}
