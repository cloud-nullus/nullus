// 탭 스트립의 유일한 규격.
//
// 개편 전에는 화면 8곳이 같은 모양을 손으로 다시 만들고 있었다. 구조는 같은데
// 값이 조금씩 달랐다 — 좌우 여백이 px-3.5 / px-4 / px-5 / px-[16px] / px-[18px]
// 다섯 가지, 글자 크기가 text-sm 과 text-[13px] 두 가지, 활성 표시는 "primary 글자"
// 와 "primary 밑줄 + 기본 글자" 두 계열로 갈려 있었다. 한 화면 안에서 탭을 두 번
// 쓰는 곳(cicd-list, user-management)은 그 둘이 서로 달랐다.
//
// 접근성: 활성 상태를 aria-pressed 로 알린다. 손으로 만든 11곳은 활성 표시가
// 순전히 색과 밑줄뿐이라 스크린리더에 아무 것도 전달되지 않았다.
//
// role="tab" / role="tablist" 은 일부러 쓰지 않는다. ARIA 탭 패턴은 각 패널의
// role="tabpanel" + aria-labelledby 연결과 좌우 화살표 이동까지 갖춰야 성립한다.
// 역할 이름만 붙이면 스크린리더는 "탭"이라 읽고 짝이 되는 패널을 못 찾으며,
// 키보드 사용자는 있지도 않은 화살표 이동을 기대하게 된다 — 절반만 구현한 역할은
// 네이티브 버튼보다 나쁘다. 패널 배선까지 갈 준비가 되면 그때 올린다.

import { type ReactNode } from 'react'
import { cn } from '../../lib/utils'

export interface TabItem<T extends string | number = string> {
  id: T
  label: ReactNode
  /** lucide 아이콘. 크기는 13~14가 규격이다. */
  icon?: ReactNode
  /** 라벨 뒤에 붙는 개수 배지 등. */
  badge?: ReactNode
  disabled?: boolean
}

interface TabsProps<T extends string | number> {
  items: TabItem<T>[]
  value: T
  onChange: (id: T) => void
  /** 오른쪽 끝에 붙는 슬롯 (관리 버튼 등). 탭이 아니므로 tablist 밖에 둔다. */
  trailing?: ReactNode
  className?: string
}

export function Tabs<T extends string | number>({
  items,
  value,
  onChange,
  trailing,
  className,
}: TabsProps<T>) {
  return (
    <div
      className={cn(
        'flex items-end overflow-x-auto border-b border-[var(--color-border-default)]',
        className,
      )}
    >
      <div className="flex min-w-0 items-end">
        {items.map((item) => {
          const active = item.id === value
          return (
            <button
              key={String(item.id)}
              type="button"
              aria-pressed={active}
              disabled={item.disabled}
              onClick={() => !item.disabled && onChange(item.id)}
              className={cn(
                '-mb-px flex shrink-0 cursor-pointer items-center gap-1.5 border-b-2 bg-none px-3 py-2 text-[13px] transition-colors duration-150',
                active
                  ? 'border-b-[var(--color-primary)] font-semibold text-[var(--color-primary)]'
                  : 'border-b-transparent font-normal text-[var(--color-text-secondary)]',
                item.disabled
                  ? 'cursor-not-allowed opacity-40'
                  : !active && 'hover:text-[var(--color-text-primary)]',
              )}
            >
              {item.icon}
              {item.label}
              {item.badge}
            </button>
          )
        })}
      </div>
      {trailing && <div className="ml-auto flex shrink-0 items-center pb-1">{trailing}</div>}
    </div>
  )
}
