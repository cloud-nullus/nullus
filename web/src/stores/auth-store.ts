import { create } from 'zustand'
import type { Role, User } from '../types'

const SESSION_TOKEN_KEY = 'nullus-token'
const SESSION_USER_KEY = 'nullus-user'

function getStoredToken(): string | null {
  return sessionStorage.getItem(SESSION_TOKEN_KEY)
}

function getStoredUser(): User | null {
  const raw = sessionStorage.getItem(SESSION_USER_KEY)
  if (!raw) return null
  try { return JSON.parse(raw) as User } catch { return null }
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

const storedUser = getStoredUser()

export const useAuthStore = create<AuthState>()((set) => ({
  token: getStoredToken(),
  role: storedUser?.role ?? 'developer',
  user: storedUser,
  isAuthenticated: getStoredToken() !== null,
  setRole: (role) => set({ role }),
  login: (user, token) => {
    const authToken = token ?? `mock-token-${user.id}`
    sessionStorage.setItem(SESSION_TOKEN_KEY, authToken)
    sessionStorage.setItem(SESSION_USER_KEY, JSON.stringify(user))
    set({ user, role: user.role, token: authToken, isAuthenticated: true })
  },
  logout: () => {
    sessionStorage.removeItem(SESSION_TOKEN_KEY)
    sessionStorage.removeItem(SESSION_USER_KEY)
    set({ user: null, token: null, isAuthenticated: false, role: 'developer' })
  },
}))
