import type { ReactNode } from 'react'
import { iconTile, rounded } from '../../theme/tokens.generated'

type Props = {
  /** sm 28px — 화면 제목 옆 · md 36px — KPI 카드, 기능 카드 */
  size?: keyof typeof iconTile
  /** 기능 색 토큰 이름. 예: '--color-success' */
  token: string
  children: ReactNode
  className?: string
}

/**
 * 아이콘에 바탕을 까는 타일 — DESIGN.md §아이콘 타일.
 *
 * 개편 전에는 같은 타일이 세 벌이었다: KPI 는 `h-9 w-9 rounded-lg`, 기능 카드는
 * `h-9 w-9 rounded-[10px]`, 제목 옆은 `--icon-size` + `--icon-radius`. 배경 알파도
 * 4·8·10·15·20% 가 섞여 있어서, 같은 뜻의 타일이 화면마다 다른 크기·다른 농도였다.
 *
 * 색을 클래스가 아니라 인라인 style 로 넣는 이유: Tailwind 임의값은 소스 문자열을
 * 스캔해 만들어지므로 `bg-[color-mix(...var(${token})...)]` 처럼 토큰 이름을
 * 조립하면 그 클래스가 생성되지 않는다. 조용히 색이 빠진다.
 */
export function IconTile({ size = 'md', token, children, className }: Props) {
  return (
    <div
      className={`flex shrink-0 items-center justify-center ${className ?? ''}`.trim()}
      style={{
        width: iconTile[size],
        height: iconTile[size],
        borderRadius: size === 'sm' ? rounded.sm : rounded.md,
        backgroundColor: `color-mix(in srgb, var(${token}) 15%, transparent)`,
        color: `var(${token})`,
      }}
    >
      {children}
    </div>
  )
}
