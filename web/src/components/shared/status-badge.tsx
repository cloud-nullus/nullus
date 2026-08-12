// 상태 배지 — 목록·상세 화면의 모든 상태 표시를 여기 하나로 모은다.
//
// 개편 전 문제: 이 컴포넌트가 만들어져 있는데 **아무도 쓰지 않았다**(사용처 0곳).
// 화면들은 각자 <span className="rounded-md px-2 py-1 ..."> 로 배지를 다시 만들고,
// 색은 utils/*.ts 의 팔레트에서 인라인 style 로 받아 썼다. 그래서 같은 뜻의 상태가
// 화면마다 모양이 달랐다.
//
// 원인은 API 였다. status prop 이 ClusterStatus 로 좁혀져 있어서 파이프라인·스택·
// 배포 상태에는 쓸 수 없었다. 이제 의미(tone)를 받아 어떤 도메인 상태든 담는다.
//
// DESIGN.md §Components: 상태는 색 + 아이콘 + 텍스트를 항상 함께 쓴다.
// 색만으로 표시하지 않는다 — 색맹 사용자와 흑백 인쇄를 위해서다.

import Chip from '@mui/material/Chip'
import { StatusIcon, STATUS_TOKEN, type StatusTone } from '../ui/status-icon'
import type { ClusterStatus } from '../../types'

// 상태의 뜻·색·아이콘은 components/ui/status-icon.tsx 가 단일 출처다.
// 여기서 다시 고르지 않는다 — 배지가 자기 아이콘을 들고 있으면 배지 밖에서
// 같은 상태를 그릴 때 또 고르게 되고, 그게 개편 전 상태였다.
export { toneForStatus } from '../ui/status-icon'
export type { StatusTone }

/** 기존 호출부(ClusterStatus)를 위한 매핑. */
const CLUSTER_STATUS_TONE: Record<ClusterStatus, StatusTone> = {
  connected: 'success',
  pending: 'warning',
  error: 'error',
  unreachable: 'warning',
  auth_failed: 'error',
  inactive: 'neutral',
}

const CLUSTER_STATUS_LABEL: Record<ClusterStatus, string> = {
  connected: 'Connected',
  pending: 'Pending',
  error: 'Error',
  unreachable: 'Unreachable',
  auth_failed: 'Auth Failed',
  inactive: 'Inactive',
}

interface StatusBadgeProps {
  /** 클러스터 상태를 그대로 넘기는 기존 방식. tone 을 주면 무시된다. */
  status?: ClusterStatus
  /** 상태의 의미. 파이프라인·스택·배포 등 임의 도메인 상태를 담을 때 쓴다. */
  tone?: StatusTone
  /** 표시 문자열. 도메인 라벨은 호출부가 i18n 으로 만들어 넘긴다. */
  label?: string
  /** 아이콘을 숨긴다. 밀집한 표에서 폭이 문제될 때만 쓴다 — 기본은 표시다. */
  hideIcon?: boolean
  className?: string
}

export function StatusBadge({ status, tone, label, hideIcon = false, className }: StatusBadgeProps) {
  const resolvedTone: StatusTone = tone ?? (status ? CLUSTER_STATUS_TONE[status] : 'neutral')
  const resolvedLabel = label ?? (status ? CLUSTER_STATUS_LABEL[status] : '')
  const cssVar = STATUS_TOKEN[resolvedTone]

  return (
    <Chip
      className={className}
      size="small"
      icon={hideIcon ? undefined : <StatusIcon tone={resolvedTone} size="xs" inheritColor />}
      label={resolvedLabel}
      sx={{
        // 색은 토큰만 참조한다. 테마 전환에 따라 두 테마 모두 AA 를 넘는 값이 들어온다.
        backgroundColor: `color-mix(in srgb, var(${cssVar}) 15%, transparent)`,
        color: `var(${cssVar})`,
        fontWeight: 600,
        // 아이콘도 텍스트와 같은 색으로. MUI 기본값은 회색이다.
        '& .MuiChip-icon': { color: `var(${cssVar})` },
      }}
    />
  )
}
