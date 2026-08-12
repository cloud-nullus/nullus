// F8-UIUX-EmptyLoading — MUI Skeleton 어댑터.
//
// 공개 API(className)와 data-testid="skeleton" 을 유지한다. 형태는 호출자가
// className 으로 잡던 방식 그대로다.
//
// variant="rectangular" + animation="pulse" 로 이전의 단순 펄스 룩을 유지한다.
// MUI 기본 "text" variant 는 자체 여백과 스케일을 넣어 호출자의 크기 지정을 흐린다.

import MuiSkeleton from '@mui/material/Skeleton'

export function Skeleton({ className }: { className?: string }) {
  return (
    <MuiSkeleton
      className={className}
      variant="rectangular"
      animation="pulse"
      data-testid="skeleton"
      sx={{
        borderRadius: 'var(--radius-sm)',
        backgroundColor: 'color-mix(in srgb, var(--color-text-secondary) 12%, transparent)',
      }}
    />
  )
}
