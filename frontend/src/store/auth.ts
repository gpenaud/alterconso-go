import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { User } from '../api/auth'

interface AuthState {
  token: string | null
  user: User | null
  currentGroupId: number | null
  login: (token: string, user: User) => void
  setGroup: (groupId: number) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      currentGroupId: null,
      login: (token, user) => set({ token, user, currentGroupId: null }),
      setGroup: (groupId) => set({ currentGroupId: groupId }),
      logout: () => set({ token: null, user: null, currentGroupId: null }),
    }),
    { name: 'alterconso-auth' }
  )
)
