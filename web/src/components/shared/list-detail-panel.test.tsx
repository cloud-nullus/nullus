import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ListDetailPanel } from './list-detail-panel'

describe('ListDetailPanel', () => {
  it('renders list and detail content without crashing', () => {
    const { container } = render(
      <ListDetailPanel
        listContent={<div>Stack A</div>}
        detailContent={<div>Detail A</div>}
      />
    )

    expect(container).toBeTruthy()
    expect(screen.getAllByText('Stack A').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Detail A').length).toBeGreaterThan(0)
  })

  it('shows empty detail message when detail content is null', () => {
    render(
      <ListDetailPanel
        listContent={<div>Stack B</div>}
        detailContent={null}
        emptyDetailMessage="Choose a stack"
      />
    )

    expect(screen.getAllByText('Choose a stack').length).toBeGreaterThan(0)
  })

  // 개편 전 구현은 240/280 만 Tailwind 클래스로 매핑하고 그 밖의 값을 조용히
  // 280 으로 접었다. 임의 폭이 실제로 먹는지까지 고정한다.
  //
  // 폭은 CSS 변수로 전달한다 — 인라인 style 로 박으면 좁은 화면에서 위아래로
  // 쌓을 때 그 폭이 그대로 남아 되돌릴 수 없다.
  it.each([240, 280, 320])('applies the requested list width (%ipx)', (width) => {
    render(
      <ListDetailPanel
        listWidth={width}
        listContent={<div>Stack C</div>}
        detailContent={<div>Detail C</div>}
      />
    )

    const list = screen.getByTestId('list-detail-list')
    expect(list.parentElement!.style.getPropertyValue('--list-detail-pane')).toBe(`${width}px`)
    expect(list.className).toContain('xl:w-[var(--list-detail-pane)]')
  })

  // 좁은 창에서는 위아래로 쌓는다.
  //
  // 좌우 고정으로 두면 상세 레일이 폭을 그대로 먹어 목록이 짓눌린다. 실제로
  // CI/CD 이력에서 960px 창일 때 목록 290px / 상세 380px 로 상세가 목록보다
  // 넓어졌고, 7개 컬럼짜리 표가 290px 안에서 가로 스크롤됐다.
  //
  // 폭은 인라인 style 이 아니라 CSS 변수로 준다. style 로 박으면 미디어 쿼리로
  // 되돌릴 수 없어 좁은 화면에서도 그 폭이 그대로 남는다.
  it('좁은 창에서는 위아래로 쌓는다', () => {
    render(
      <ListDetailPanel
        detailWidth={380}
        listContent={<div>Stack E</div>}
        detailContent={<div>Detail E</div>}
      />
    )

    const list = screen.getByTestId('list-detail-list')
    const root = list.parentElement!
    expect(root.className).toContain('flex-col')
    expect(root.className).toContain('xl:flex-row')

    const detail = screen.getByTestId('list-detail-detail')
    expect(detail.style.width).toBe('')
    expect(detail.className).toContain('xl:w-[var(--list-detail-pane)]')
  })

  it('renders the list and detail headers above their panes', () => {
    render(
      <ListDetailPanel
        listContent={<div>Stack D</div>}
        detailContent={<div>Detail D</div>}
        listHeader={<div>List toolbar</div>}
        detailHeader={<div>Detail toolbar</div>}
      />
    )

    expect(screen.getByText('List toolbar')).toBeTruthy()
    expect(screen.getByText('Detail toolbar')).toBeTruthy()
  })
})
