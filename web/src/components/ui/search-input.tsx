// 목록 화면의 검색 상자. 유일한 규격이다.
//
// 개편 전에는 화면 12곳이 같은 조합(relative div + 절대배치 돋보기 + 왼쪽 여백을
// 비운 input)을 손으로 다시 만들고 있었다. 클래스 문자열이 길어서 복사가 반복됐고,
// 그 과정에서 폭(w-[220px] / w-full)과 높이(py-[7px] / py-[9px])가 갈렸다.
// 스케일 밖 값(7px, 9px, 30px)이 들어간 것도 전부 여기서 나왔다 —
// DESIGN.md §Layout 은 spacing 스케일 안에서만 고르라고 못박고 있다.
//
// 표면 스타일은 TextInput 과 같은 값을 쓴다(textInputClass). 아이콘을 겹쳐 놓느라
// 컴포넌트를 감싸지 못할 뿐, 검색 상자가 다른 입력과 다르게 생길 이유는 없다.

import { forwardRef, type InputHTMLAttributes } from 'react'
import { Search } from 'lucide-react'
import { cn } from '../../lib/utils'
import { textInputClass } from './text-input'

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
        className={textInputClass(cn('w-full pl-7', className))}
        {...props}
      />
    </div>
  ),
)

SearchInput.displayName = 'SearchInput'
