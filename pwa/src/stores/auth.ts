import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { User, UserStats } from '../api/types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    stats: null as UserStats | null,
    loaded: false,
  }),
  getters: {
    isLoggedIn: (s) => s.user !== null,
    isAdmin: (s) => s.user?.role === 'admin',
    isActive: (s) => s.user?.status === 'active',
    firstName: (s) => s.user?.name.split(' ')[0] ?? '',
  },
  actions: {
    async fetchMe() {
      try {
        const { data } = await api.get<{ user: User; stats: UserStats }>('/me')
        this.user = data.user
        this.stats = data.stats
      } catch {
        this.user = null
        this.stats = null
      } finally {
        this.loaded = true
      }
    },
    async login(email: string, password: string) {
      const { data } = await api.post<User>('/login', { email, password })
      this.user = data
      this.loaded = true
    },
    async logout() {
      await api.post('/logout')
      this.user = null
      this.stats = null
    },
  },
})
