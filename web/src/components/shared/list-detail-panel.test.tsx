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

  it('applies narrow list width class when listWidth is 240', () => {
    render(
      <ListDetailPanel
        listWidth={240}
        listContent={<div>Stack C</div>}
        detailContent={<div>Detail C</div>}
      />
    )

    const listContentNode = screen.getByText('Stack C')
    expect(listContentNode.parentElement?.className).toContain('w-[240px]')
  })
})
