// 목록 화면의 검색 상자. 유일한 규격이다.
//
// 개편 전에는 화면 12곳이 같은 조합(relative div + 절대배치 돋보기 + 왼쪽 여백을
// 비운 input)을 손으로 다시 만들고 있었다. 클래스 문자열이 길어서 복사가 반복됐고,
// 그 과정에서 폭(w-[220px] / w-full)과 높이(py-[7px] / py-[9px])가 갈렸다.
// 스케일 밖 값(7px, 9px, 30px)이 들어간 것도 전부 여기서 나왔다 —
// DESIGN.md §Layout 은 spacing 스케일 안에서만 고르라고 못박고 있다.
//
// 높이는 다른 컨트롤과 같은 --control-height 를 쓴다. 툴바에서 셀렉트·버튼과
// 나란히 서는 자리라 높이가 1px 만 어긋나도 줄이 흐트러져 보인다.

import { forwardRef, type InputHTMLAttributes } from 'react'
import { Search } from 'lucide-react'
import { cn } from '../../lib/utils'

interface SearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'size'> {
  /** 감싸는 자리의 클래스 (폭 지정 등). input 자체가 아니라 바깥 div 에 붙는다. */
  wrapperClassName?: string
}

export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  ({ className, wrapperClassName, ...props }, ref) => (
    <div className={cn('relative', wrapperClassName)}>
      <Search
        size={13}
        className="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
      />
      <input
        ref={ref}
        type="search"
        className={cn(
          'h-[var(--control-height)] w-full rounded-[var(--radius-sm)] border border-[var(--color-border-default)]',
          'bg-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)] pl-7 pr-2 text-[13px]',
          'text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]',
          className,
        )}
        {...props}
      />
    </div>
  ),
)

SearchInput.displayName = 'SearchInput'
