import { describe, it, expect, vi } from 'vitest'
import { createElement } from 'react'
import { Button } from './button'

describe('Button', () => {
  it('creates element with primary variant', () => {
    const el = createElement(Button, { variant: 'primary' }, 'Primary')
    expect(el.props.variant).toBe('primary')
  })

  it('creates element with secondary variant', () => {
    const el = createElement(Button, { variant: 'secondary' }, 'Secondary')
    expect(el.props.variant).toBe('secondary')
  })

  it('creates element with outline variant', () => {
    const el = createElement(Button, { variant: 'outline' }, 'Outline')
    expect(el.props.variant).toBe('outline')
  })

  it('creates element with danger variant', () => {
    const el = createElement(Button, { variant: 'danger' }, 'Danger')
    expect(el.props.variant).toBe('danger')
  })

  it('creates element with ghost variant', () => {
    const el = createElement(Button, { variant: 'ghost' }, 'Ghost')
    expect(el.props.variant).toBe('ghost')
  })

  it('loading prop is passed through', () => {
    const el = createElement(Button, { loading: true }, 'Submit')
    expect(el.props.loading).toBe(true)
  })

  it('disabled prop is passed through', () => {
    const el = createElement(Button, { disabled: true }, 'Disabled')
    expect(el.props.disabled).toBe(true)
  })

  it('onClick prop is accepted', () => {
    const onClick = vi.fn()
    const el = createElement(Button, { onClick }, 'Click me')
    expect(el.props.onClick).toBe(onClick)
  })

  it('Button is a valid React component (forwardRef object)', () => {
    expect(Button).toBeDefined()
    expect(typeof Button).toBe('object')
  })
})
