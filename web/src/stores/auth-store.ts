import { create } from 'zustand'
import type { Role, User } from '../types'

const SESSION_TOKEN_KEY = 'nullus-token'

function getStoredToken(): string | null {
  return sessionStorage.getItem(SESSION_TOKEN_KEY)
}

interface AuthState {
  role: Role
  user: User | null
  token: string | null
  isAuthenticated: boolean
  setRole: (role: Role) => void
  login: (user: User, token?: string) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()((set) => ({
  token: getStoredToken(),
  role: 'developer',
  user: null,
  isAuthenticated: getStoredToken() !== null,
  setRole: (role) => set({ role }),
  login: (user, token) => {
    const authToken = token ?? `mock-token-${user.id}`
    sessionStorage.setItem(SESSION_TOKEN_KEY, authToken)
    set({ user, role: user.role, token: authToken, isAuthenticated: authToken !== null })
  },
  logout: () => {
    sessionStorage.removeItem(SESSION_TOKEN_KEY)
    set({ user: null, token: null, isAuthenticated: false, role: 'developer' })
  },
}))
