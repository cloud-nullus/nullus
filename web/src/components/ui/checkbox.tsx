// 체크박스의 유일한 규격.
//
// 감싸는 구조는 일부러 건드리지 않는다. 이 앱의 체크박스는 자리마다 정당하게
// 다른 모양을 하고 있다 — 테두리를 두른 카드형 선택지(cluster-page, organization-page,
// cicd-template-page)도 있고, 설명이 두 줄 붙는 평범한 줄(delete-pipeline-dialog)도
// 있다. 그 바깥 label 까지 하나로 묶으면 오히려 자리마다 예외 prop 이 늘어난다.
//
// 실제로 어긋나 있던 건 입력 자체였다:
//   - 크기: 지정 없음(브라우저 기본) / h-4 w-4 / h-[15px] w-[15px] 세 가지
//   - 색: accent-[var(--color-primary)] 가 붙은 곳과 안 붙은 곳 — 안 붙은 곳은
//         브랜드 블루가 아니라 브라우저 기본 파랑으로 떴다. 다크 테마에서 특히 튄다
//   - 정렬: mt-1 을 붙인 곳과 안 붙인 곳
//
// MUI Checkbox 를 쓰지 않는 이유: 여기 호출부 중 하나(cluster-page)가
// react-hook-form 의 register() 를 그대로 펼쳐 넣는다. 네이티브 input 을 유지하면
// 그 배선과 테스트의 getByRole('checkbox') 가 손대지 않아도 계속 동작한다.

import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'size'> {
  /** 라벨이 두 줄 이상이면 'start' 로 위쪽에 맞춘다. */
  align?: 'center' | 'start'
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ align = 'center', className, ...props }, ref) => (
    <input
      ref={ref}
      type="checkbox"
      className={cn(
        'h-4 w-4 shrink-0 cursor-pointer accent-[var(--color-primary)]',
        align === 'start' && 'mt-0.5',
        className,
      )}
      {...props}
    />
  ),
)

Checkbox.displayName = 'Checkbox'
