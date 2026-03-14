import { create } from 'zustand'
import type { Role, User } from '../types'

interface AuthState {
  role: Role
  user: User | null
  isAuthenticated: boolean
  setRole: (role: Role) => void
  login: (user: User) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()((set) => ({
  role: 'developer',
  user: null,
  isAuthenticated: false,
  setRole: (role) => set({ role }),
  login: (user) => set({ user, role: user.role, isAuthenticated: true }),
  logout: () => set({ user: null, isAuthenticated: false, role: 'developer' }),
}))
