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
  it.each([240, 280, 320])('applies the requested list width (%ipx)', (width) => {
    render(
      <ListDetailPanel
        listWidth={width}
        listContent={<div>Stack C</div>}
        detailContent={<div>Detail C</div>}
      />
    )

    expect(screen.getByTestId('list-detail-list').style.width).toBe(`${width}px`)
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
