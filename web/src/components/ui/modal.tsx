// MUI Dialog 어댑터.
//
// 공개 API(open / onClose / title / children / wide / footer)를 유지한다.
// 이 컴포넌트를 쓰는 11곳은 고치지 않는다.
//
// 손으로 만든 포커스 트랩 ~70줄을 지웠다. MUI Modal 이 같은 일을 한다:
//   - 열릴 때 내부 첫 요소로 포커스 이동, Tab/Shift+Tab 순환
//   - 닫힐 때 이전 포커스 복원
//   - Esc 로 닫기, 배경 스크롤 락
//
// 배경 클릭 동작도 계약이 같다. MUI Dialog 는 mouseDown 시점의 타겟을 기록해
// "내용에서 드래그를 시작해 배경에서 놓은 경우"에는 닫지 않는다 — 이전 구현이
// pointerDown/pointerUp 으로 직접 구현했던 가드와 동일한 의미다.

import type { ReactNode } from 'react'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import IconButton from '@mui/material/IconButton'
import { X } from 'lucide-react'
import { iconProps } from './icon'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  wide?: boolean
  /**
   * wide(800) 보다 한 단계 넓은 자리. 탭 안에 선택 카드가 세로로 깔리는
   * 편집 모달용이다 — 그 폭에서는 카드 안의 이름과 설명이 매 줄 접힌다.
   * 기존 `wide` 를 키우지 않는 이유는 그 값을 쓰는 자리가 열 곳이 넘고
   * 대부분은 폼 두 칸짜리라 더 넓어지면 오히려 휑해지기 때문이다.
   */
  size?: 'default' | 'wide' | 'xl'
  footer?: ReactNode
}

const MAX_WIDTH_BY_SIZE = { default: 480, wide: 800, xl: 1080 } as const

export function Modal({ open, onClose, title, children, wide = false, size, footer }: ModalProps) {
  // 닫히면 즉시 언마운트한다. 이전 구현이 `if (!open) return null` 이라 퇴장
  // 애니메이션이 아예 없었고, 소비자 테스트들이 그 동기적 제거에 의존한다
  // (예: Cancel 클릭 직후 queryByText(...).not.toBeInTheDocument()).
  // MUI Dialog 에 맡기면 전환 시간만큼 DOM 에 남아 그 계약이 깨진다.
  // 트랩·Esc·스크롤 락은 그대로 얻으면서 타이밍 계약만 이전과 같게 유지한다.
  if (!open) return null

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth={false}
      slotProps={{
        // role="dialog" 는 Paper 에 붙는다. 이전 구현의 aria-label={title} 계약을 유지한다.
        paper: {
          'aria-label': title,
          sx: {
            width: '100%',
            maxWidth: MAX_WIDTH_BY_SIZE[size ?? (wide ? 'wide' : 'default')],
            maxHeight: '90vh',
            backgroundColor: 'var(--color-surface-raised)',
            border: '1px solid var(--color-border-default)',
            borderRadius: 'var(--card-radius)',
          },
        },
        // 배경 클릭 테스트가 잡을 지점. MUI 는 이 컨테이너에서 mouseDown/click 을 듣는다.
        // container 슬롯 타입에 data-* 가 없어 캐스팅한다 — 런타임에는 그대로 전달된다.
        container: { 'data-testid': 'modal-overlay' } as React.HTMLAttributes<HTMLDivElement>,
      }}
    >
      {title && (
        <DialogTitle
          component="h2"
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 2,
            borderBottom: '1px solid var(--color-border-default)',
            color: 'var(--color-text-primary)',
            fontSize: '1rem',
            fontWeight: 700,
            wordBreak: 'keep-all',
          }}
        >
          {title}
          <IconButton onClick={onClose} aria-label="Close modal" sx={{ color: 'var(--color-text-secondary)' }}>
            <X {...iconProps('md')} />
          </IconButton>
        </DialogTitle>
      )}

      {/* 위쪽 여백을 되돌려 준다. MUI 는 DialogTitle 바로 뒤에 오는
          DialogContent 의 padding-top 을 0 으로 만드는데, 이 본문은
          overflow-y: auto 라 그 경계에서 잘린다. 첫 줄이 외곽선 입력(Input)이면
          떠 있는 라벨이 상자 위로 나가 있으므로 **라벨의 위쪽 절반이 사라진다**
          — 템플릿 편집 모달의 "Template ID" 가 그렇게 잘려 있었다. */}
      <DialogContent sx={{ color: 'var(--color-text-primary)', pt: 1.5 }}>{children}</DialogContent>

      {footer && (
        <DialogActions sx={{ borderTop: '1px solid var(--color-border-default)', gap: 1.25, px: 2.5, py: 2 }}>
          {footer}
        </DialogActions>
      )}
    </Dialog>
  )
}
