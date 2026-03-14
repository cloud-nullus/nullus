import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './auth-store'
import type { User } from '../types'

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
})

describe('auth-store', () => {
  it('initial state has developer role and not authenticated', () => {
    const state = useAuthStore.getState()
    expect(state.role).toBe('developer')
    expect(state.isAuthenticated).toBe(false)
    expect(state.user).toBeNull()
  })

  it('setRole changes role', () => {
    useAuthStore.getState().setRole('admin')
    expect(useAuthStore.getState().role).toBe('admin')
  })

  it('setRole to devops', () => {
    useAuthStore.getState().setRole('devops')
    expect(useAuthStore.getState().role).toBe('devops')
  })

  it('login sets user, role, and isAuthenticated', () => {
    const user: User = { id: '1', name: 'Alice', email: 'alice@example.com', role: 'admin' }
    useAuthStore.getState().login(user)
    const state = useAuthStore.getState()
    expect(state.user).toEqual(user)
    expect(state.role).toBe('admin')
    expect(state.isAuthenticated).toBe(true)
  })

  it('logout clears user and authentication', () => {
    const user: User = { id: '1', name: 'Alice', email: 'alice@example.com', role: 'admin' }
    useAuthStore.getState().login(user)
    useAuthStore.getState().logout()
    const state = useAuthStore.getState()
    expect(state.user).toBeNull()
    expect(state.isAuthenticated).toBe(false)
    expect(state.role).toBe('developer')
  })
})
