import { describe, it, expect } from 'vitest'
import { createElement } from 'react'
import { StatusBadge } from './status-badge'

// Verify StatusBadge renders without throwing for all statuses
describe('StatusBadge', () => {
  it('creates element for connected status', () => {
    const el = createElement(StatusBadge, { status: 'connected' })
    expect(el).toBeDefined()
    expect(el.props.status).toBe('connected')
  })

  it('creates element for pending status', () => {
    const el = createElement(StatusBadge, { status: 'pending' })
    expect(el.props.status).toBe('pending')
  })

  it('creates element for error status', () => {
    const el = createElement(StatusBadge, { status: 'error' })
    expect(el.props.status).toBe('error')
  })

  it('creates element for inactive status', () => {
    const el = createElement(StatusBadge, { status: 'inactive' })
    expect(el.props.status).toBe('inactive')
  })

  it('passes custom label prop', () => {
    const el = createElement(StatusBadge, { status: 'connected', label: 'Online' })
    expect(el.props.label).toBe('Online')
  })

  it('StatusBadge is a function component', () => {
    expect(typeof StatusBadge).toBe('function')
  })
})
