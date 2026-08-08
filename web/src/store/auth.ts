import { create } from 'zustand'
import { queryClient } from '@/lib/query'
import type { User } from '@/lib/types'

export type { User }

interface AuthState {
  user: User | null
  token: string | null
  isAdmin: boolean
  setUser: (user: User | null) => void
  setToken: (token: string | null) => void
  signIn: (email: string, password: string) => Promise<void>
  signUp: (name: string, email: string, password: string) => Promise<void>
  signOut: () => Promise<void>
}

// Authula responds with { user, session } — not wrapped in ApiResponse envelope.
interface AuthulaResponse {
  user: User
  session: { token: string }
}

async function authFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })
  if (!res.ok) {
    const error = await res.json().catch(() => ({ message: 'Request failed' }))
    throw new Error(error.message || 'Request failed')
  }
  return res.json()
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  token: null,
  isAdmin: false,

  setUser: (user) => set({ user, isAdmin: user?.is_admin === true }),

  setToken: (token) => set({ token }),

  signIn: async (email, password) => {
    const data = await authFetch<AuthulaResponse>('/auth/sign-in', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
    // Authula sets httpOnly cookie automatically.
    // Keep token in memory for fallback if cookie is unavailable.
    // Derive isAdmin from the user (like setUser) so admin redirects
    // work before the /api/me refetch completes.
    set({ user: data.user, token: data.session.token, isAdmin: data.user.is_admin === true })
    // Force AuthLoader to refetch /api/me with the new session cookie
    // so organizations are populated before OrgGuard redirects.
    await queryClient.invalidateQueries({ queryKey: ['auth', 'me'] })
  },

  signUp: async (name, email, password) => {
    const data = await authFetch<AuthulaResponse>('/auth/sign-up', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    })
    set({ user: data.user, token: data.session.token, isAdmin: data.user.is_admin === true })
    await queryClient.invalidateQueries({ queryKey: ['auth', 'me'] })
  },

  signOut: async () => {
    // Try Bearer first (for API clients), then fall back to cookie
    const { token } = useAuthStore.getState()
    await fetch('/api/sign-out', {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }).catch(() => {})
    set({ user: null, token: null })
  },
}))
