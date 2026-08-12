// 목록 + 상세의 유일한 규격. 좌우 분할이다.
//
// 왜 아래로 펼치지 않는가: 목록 아래에 상세를 펼치면 행을 고를 때마다 목록이
// 밀려 내려가 방금 고른 행이 화면 밖으로 나간다. 다음 행을 보려면 스크롤을
// 되감아야 하고, 두 행을 비교하는 일이 사실상 불가능해진다. 좌우로 나누면
// 왼쪽은 고정된 채 오른쪽만 바뀐다 — IDE·메일 클라이언트가 전부 이 모양인 이유다.
//
// 높이는 부모가 준다(`h-full`). 부모가 높이를 안 주면 minHeight 로 버틴다.

import { type ReactNode } from 'react'

interface ListDetailPanelProps {
  /** 왼쪽 목록 폭(px). 데이터 밀집 화면이라 240~320 사이를 쓴다. */
  listWidth?: number
  /**
   * 오른쪽 상세 폭(px). 주면 좌우가 뒤집힌 배치가 된다 — 목록이 늘어나고
   * 상세가 고정폭 사이드 레일이 된다. 컬럼이 6개쯤 되는 표를 좁은 목록으로
   * 접으면 컬럼을 잃으므로, 표가 주인공인 화면은 이쪽을 쓴다.
   */
  detailWidth?: number
  listContent: ReactNode
  detailContent: ReactNode | null
  emptyDetailMessage?: string
  /** 목록 위에 고정으로 붙는 줄 (검색·필터·건수). 스크롤을 따라가지 않는다. */
  listHeader?: ReactNode
  /** 상세 위에 고정으로 붙는 줄 (제목·액션). */
  detailHeader?: ReactNode
}

export function ListDetailPanel({
  listWidth = 280,
  detailWidth,
  listContent,
  detailContent,
  emptyDetailMessage = 'Select an item to view details',
  listHeader,
  detailHeader,
}: ListDetailPanelProps) {
  // 어느 쪽이 고정폭인가만 뒤집는다. 두 배치가 같은 컴포넌트를 쓰는 게 핵심이다.
  const detailIsRail = detailWidth !== undefined

  // 폭은 CSS 변수로 넘긴다.
  //
  // 인라인 style 로 박으면 미디어 쿼리로 되돌릴 수 없어 좁은 화면에서도 그 폭이
  // 그대로 남는다. Tailwind 임의값(w-[280px])은 빌드 시점에 클래스를 굽기 때문에
  // 런타임 값으로 만들 수 없지만, w-[var(--…)] 는 클래스가 고정이라 구워진다.
  const paneWidth = { '--list-detail-pane': `${detailIsRail ? detailWidth : listWidth}px` } as React.CSSProperties

  return (
    // 좁은 창에서는 위아래로 쌓는다. 좌우 고정으로 두면 상세 레일이 폭을 그대로
    // 먹어 목록이 짓눌린다 — 960px 창에서 목록 290px / 상세 380px 로 뒤집히고
    // 7개 컬럼짜리 표가 290px 안에서 가로 스크롤됐다.
    <div
      className="flex h-full min-h-[240px] flex-col overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] xl:flex-row"
      style={paneWidth}
    >
      <div
        data-testid="list-detail-list"
        className={
          detailIsRail
            ? 'flex min-w-0 flex-1 flex-col border-b border-[var(--color-border-default)] xl:border-b-0 xl:border-r'
            : 'flex w-full shrink-0 flex-col border-b border-[var(--color-border-default)] xl:w-[var(--list-detail-pane)] xl:border-b-0 xl:border-r'
        }
      >
        {listHeader && (
          <div className="shrink-0 border-b border-[var(--color-border-default)]">{listHeader}</div>
        )}
        <div className="min-h-0 flex-1 overflow-y-auto">{listContent}</div>
      </div>

      <div
        data-testid="list-detail-detail"
        className={
          detailIsRail
            ? 'flex w-full shrink-0 flex-col xl:w-[var(--list-detail-pane)]'
            : 'flex min-w-0 flex-1 flex-col'
        }
      >
        {detailHeader && (
          <div className="shrink-0 border-b border-[var(--color-border-default)]">{detailHeader}</div>
        )}
        <div className="min-h-0 flex-1 overflow-y-auto">
          {detailContent ?? (
            <div className="flex h-full min-h-[200px] items-center justify-center p-6 text-center text-[13px] text-[var(--color-text-secondary)]">
              {emptyDetailMessage}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
