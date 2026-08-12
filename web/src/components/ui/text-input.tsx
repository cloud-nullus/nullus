// 맨 입력 상자의 규격.
//
// `Input`(MUI TextField)과 역할이 다르다. Input 은 라벨·헬퍼텍스트·에러 문구를
// 세트로 갖는 폼 필드고, FormControl 래퍼가 딸려 온다. 이 앱에는 그 세트가
// 필요 없는 자리가 많다 — 모달의 "DELETE 를 입력하세요" 확인 칸, 표 셀 안의
// 버전 상자, 탭 이름 편집 줄 같은 곳이다. 거기에 TextField 를 끼우면 래퍼가
// 레이아웃을 밀어낸다. 그래서 두 규격을 나란히 둔다.
//
// 개편 전 이 자리들은 각자 긴 클래스 문자열을 복사하고 있었고 값이 갈렸다:
//   - 높이: py-2 / py-2.5 / py-[7px] / py-[9px] 네 가지 (셋은 spacing 스케일 밖)
//   - 글자: text-xs 와 text-sm
//   - 배경: 본문색 4% / 6% / surface-base 세 가지
//   - 셋(stack-install 의 프로필 이름, stack-template 의 helm·app 버전)은 클래스가
//     아예 없어 브라우저 기본 입력으로 떴다 — 다크 테마에서 흰 상자로 튀었다

import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

/**
 * 입력 상자의 표면 스타일. 컴포넌트를 못 쓰는 자리(아이콘을 겹쳐 놓는 검색 상자
 * 등)가 같은 값을 공유하도록 클래스만 따로 내보낸다.
 */
export function textInputClass(extra?: string): string {
  return cn(
    'h-[var(--control-height)] rounded-[var(--radius-sm)] border border-[var(--color-border-default)]',
    'bg-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)] px-2 text-[13px]',
    'text-[var(--color-text-primary)] outline-none placeholder:text-[var(--color-text-muted)]',
    'transition-colors duration-150 hover:border-[var(--color-border-hover)]',
    extra,
  )
}

interface TextInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  /** 검증 실패 상태. 테두리를 error 색으로 바꾼다. */
  invalid?: boolean
}

export const TextInput = forwardRef<HTMLInputElement, TextInputProps>(
  ({ invalid = false, className, ...props }, ref) => (
    <input
      ref={ref}
      aria-invalid={invalid || undefined}
      className={textInputClass(
        cn(invalid && 'border-[var(--color-error)] hover:border-[var(--color-error)]', className),
      )}
      {...props}
    />
  ),
)

TextInput.displayName = 'TextInput'
