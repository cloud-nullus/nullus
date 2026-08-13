import { icon } from '../../theme/tokens.generated'

export type IconSize = keyof typeof icon

/**
 * 아이콘에 넘길 크기와 선 굵기.
 *
 * 둘을 따로 쓰지 않는 이유가 있다. lucide 는 24 격자에 stroke 2 로 그려지므로
 * `size` 만 줄이면 실제 렌더 굵기가 `2 x (size / 24)` 로 함께 줄어든다 —
 * 12px 에서 1.0px, 28px 에서 2.33px. 크기를 고르는 순간 굵기가 딸려 변하는
 * 셈이라, 같은 화면에서 작은 아이콘은 흐리고 큰 아이콘은 뭉툭해진다.
 * 개편 전 코드가 정확히 그 상태였다: 크기 12가지, strokeWidth 지정은 0곳.
 *
 * 그래서 항상 한 쌍으로 받는다.
 *
 *   <Rocket {...iconProps('sm')} />
 *
 * 값은 DESIGN.md 의 icon 블록이 단일 출처이고 generate-theme.mjs 가 굽는다.
 *
 * 아이콘은 기본적으로 **읽는 도구에 숨긴다**.
 *
 * 전수조사에서 아이콘 314곳 중 aria-label 을 가진 것이 0곳이었다. lucide 는
 * 아무 것도 붙이지 않은 <svg> 를 내보내므로, 그대로 두면 이름 없는 그래픽이
 * 300개 넘게 읽힌다 — 화면을 소리로 듣는 사람에게는 잡음이다.
 *
 * 거의 모든 자리에서 아이콘은 옆의 글자를 거드는 장식이고, 아이콘만 있는
 * 버튼은 버튼 쪽에 aria-label 이 붙는다. 아이콘이 홀로 뜻을 지는 경우에만
 * StatusIcon 의 label 처럼 이름을 준다.
 */
export const iconProps = (size: IconSize = 'sm') => ({ ...icon[size], 'aria-hidden': true }) as const

/** CSS 로 크기를 줘야 할 때(컨테이너 계산 등). 굵기는 CSS 로 못 넘긴다. */
export const iconVar = (size: IconSize) => `var(--icon-${size})`
