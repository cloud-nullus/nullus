import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { YamlEditor } from './yaml-editor'

describe('YamlEditor', () => {
  beforeEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
      configurable: true,
    })
  })

  it('renders without crashing with editable textarea', () => {
    const { container } = render(<YamlEditor value={'apiVersion: v1\nkind: ConfigMap'} onChange={vi.fn()} />)

    expect(container).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Format' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Copy' })).not.toBeNull()
    expect(screen.getByRole('textbox')).not.toBeNull()
    expect(screen.getAllByText('1').length).toBeGreaterThan(0)
    expect(screen.getAllByText('2').length).toBeGreaterThan(0)
  })

  it('calls onChange and shows parse error when tab indentation exists', () => {
    const onChange = vi.fn()

    render(<YamlEditor value="kind: ConfigMap" onChange={onChange} />)

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'kind:\n\tname: sample' },
    })

    expect(onChange).toHaveBeenCalledWith('kind:\n\tname: sample')
    expect(screen.getByText('Line 2: YAML does not allow tab indentation')).not.toBeNull()
  })

  it('formats tabs to spaces and updates value via onChange', () => {
    const onChange = vi.fn()

    render(<YamlEditor value={'kind:\n\tname: sample'} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Format' }))

    expect(onChange).toHaveBeenCalledWith('kind:\n  name: sample')
  })

  it('copies current YAML and shows copied state', async () => {
    render(<YamlEditor value={'kind: ConfigMap'} onChange={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('kind: ConfigMap')
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Copied!' })).not.toBeNull()
    })
  })

  it('renders read-only mode without textarea or format button', () => {
    render(<YamlEditor value={'apiVersion: v1'} readOnly />)

    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Format' })).toBeNull()
    expect(screen.getByText('apiVersion: v1')).not.toBeNull()
  })
})
