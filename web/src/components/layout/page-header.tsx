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
//
// 스크롤해도 제자리에 남는다. 참조 대상이 IDE 인 도구에서 "지금 어느 화면인가" 가
// 스크롤 몇 줄에 사라지면 안 된다 — 사이드바·상단바를 뷰포트에 못박은 것과 같은
// 이유다. 스크롤 컨테이너는 AppLayout 의 <main> 이고, 헤더는 그 안에서 sticky 다.

import { type ChangeEvent, type ReactNode, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Breadcrumb, type BreadcrumbItem } from '../shared/breadcrumb'
import { SearchInput } from '../ui/search-input'
import { resolveNavGroupLabel } from './nav-model'

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
  const { t } = useTranslation()
  const { pathname } = useLocation()

  // 브레드크럼 맨 앞은 사이드바의 상위 메뉴다. 화면이 직접 적지 않는다 —
  // 개편 전에는 25화면이 각자 적어서 "Stack List" 처럼 상위가 통째로 빠지거나
  // ('데브섹옵스 스택 >' 이 없다) 'Admin' 만 영어로 남는 식으로 갈렸다.
  const groupKey = resolveNavGroupLabel(pathname)
  const groupLabel = groupKey ? t(groupKey) : null
  const pageTrail =
    breadcrumb && breadcrumb.length > 0 ? breadcrumb : [{ label: title }]
  const trail =
    groupLabel && pageTrail[0]?.label !== groupLabel
      ? [{ label: groupLabel }, ...pageTrail]
      : pageTrail

  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextQuery = event.target.value
    setQuery(nextQuery)
    onSearch?.(nextQuery)
  }

  return (
    // main 의 안쪽 여백(px/py)을 음수 마진으로 되돌렸다가 그대로 되돌려 준다.
    // 그러지 않으면 top-0 이 여백 안쪽에 붙어, 헤더 위·옆 여백 틈으로 본문이
    // 스쳐 지나간다. 배경은 본문 면과 같은 색이어야 글자가 겹쳐 보이지 않는다.
    <div
      className={[
        'sticky top-0 z-[var(--z-page-header)]',
        '-mx-[var(--page-padding)] px-[var(--page-padding)]',
        // 위쪽은 main 의 여백(20px)만큼 끌어올린 뒤 12px 만 되돌린다.
        // 상단 바가 바로 위에 붙어 있어 20px 를 그대로 두면 제목이 떠 보인다.
        '-mt-[var(--page-padding-y)] pt-[var(--space-md)]',
        'mb-4 border-b border-[var(--color-border-default)] pb-3',
        'bg-[var(--color-surface-base)]',
      ].join(' ')}
    >
      <Breadcrumb items={trail} />

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
              <SearchInput
                wrapperClassName="min-w-[200px]"
                value={query}
                onChange={handleSearchChange}
                placeholder={searchPlaceholder}
              />
            )}
            {actions}
            {children}
          </div>
        )}
      </div>
    </div>
  )
}
