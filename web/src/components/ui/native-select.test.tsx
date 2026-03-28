import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { NativeSelect } from './native-select'

describe('NativeSelect', () => {
  it('renders label and options without crashing', () => {
    const { container } = render(
      <NativeSelect label="Environment" defaultValue="dev">
        <option value="dev">Development</option>
        <option value="prod">Production</option>
      </NativeSelect>
    )

    expect(container).toBeTruthy()
    expect(screen.getByText('Environment')).not.toBeNull()
    expect(screen.getByRole('combobox')).not.toBeNull()
    expect(screen.getAllByText('Development').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Production').length).toBeGreaterThan(0)
  })

  it('calls onChange when selected option changes', () => {
    const onChange = vi.fn()

    render(
      <NativeSelect label="Cluster" defaultValue="cluster-a" onChange={onChange}>
        <option value="cluster-a">Cluster A</option>
        <option value="cluster-b">Cluster B</option>
      </NativeSelect>
    )

    fireEvent.change(screen.getByRole('combobox'), {
      target: { value: 'cluster-b' },
    })

    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('renders error message when error prop is provided', () => {
    render(
      <NativeSelect label="Namespace" error="Namespace is required">
        <option value="">Select namespace</option>
      </NativeSelect>
    )

    expect(screen.getAllByText('Namespace is required').length).toBeGreaterThan(0)
  })
})
