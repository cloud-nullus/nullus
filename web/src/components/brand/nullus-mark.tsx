import { useId } from 'react'
import { useThemeStore } from '../../stores/theme-store'
import {
  MARK_GAP,
  MARK_OVERS,
  MARK_SEGMENTS,
  MARK_STROKE,
  MARK_WHOLE,
} from './mark-geometry.generated'

type Props = {
  /** 한 변의 픽셀. 도형은 32 단위 좌표계라 어느 크기로든 늘어난다. */
  size?: number
  /**
   * brand — 매듭을 따라 흐르는 3색(쿠버네티스 파랑 · 인프라 보라 · 애플리케이션 청록)
   * current — 글자색을 그대로 물려받는 단색. 색이 정해진 바탕 위에 얹을 때.
   */
  tone?: 'brand' | 'current'
  /** 바로 옆에 "Nullus" 글자가 있을 때. 읽는 도구에 두 번 불리지 않게 숨긴다. */
  decorative?: boolean
  className?: string
}

/**
 * Nullus 마크 — 세잎 매듭.
 *
 * 아래로 지나가는 가닥은 마스크로 끊는다. 끊는 자리는 위 가닥과 같은 경로를
 * 굵게 그은 것이라 간격이 언제나 위 가닥과 나란하다.
 */
export function NullusMark({ size = 20, tone = 'brand', decorative = false, className }: Props) {
  // 한 화면에 두 개가 놓이면 mask id 가 겹쳐 한쪽이 통째로 사라진다.
  const maskId = `${useId()}-nullus-cut`
  const stroke = tone === 'current' ? 'currentColor' : undefined
  // 어두운 바탕에서는 파랑·보라 쪽 절반이 배경으로 가라앉는다. 색을 바꾸는 게
  // 아니라 같은 색상의 밝은 벌로 갈아 끼우는 것이다 — 생성기가 함께 구워 둔다.
  const isDark = useThemeStore((state) => state.theme) === 'dark'
  const paint = (light: string, dark: string) => stroke ?? (isDark ? dark : light)

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      width={size}
      height={size}
      className={className}
      {...(decorative
        ? { 'aria-hidden': true }
        : { role: 'img', 'aria-label': 'Nullus' })}
    >
      <mask id={maskId}>
        <rect width="32" height="32" fill="#fff" />
        <g fill="none" stroke="#000" strokeWidth={MARK_STROKE + MARK_GAP * 2} strokeLinecap="butt">
          {MARK_OVERS.map(([d]) => (
            <path key={d} d={d} />
          ))}
        </g>
      </mask>

      <g fill="none" strokeWidth={MARK_STROKE} strokeLinecap="round">
        <g mask={`url(#${maskId})`}>
          {tone === 'current' ? (
            <path d={MARK_WHOLE} stroke={stroke} />
          ) : (
            MARK_SEGMENTS.map(([d, light, dark]) => (
              <path key={d} d={d} stroke={paint(light, dark)} />
            ))
          )}
        </g>
        {MARK_OVERS.map(([d, light, dark]) => (
          <path key={d} d={d} stroke={paint(light, dark)} />
        ))}
      </g>
    </svg>
  )
}
