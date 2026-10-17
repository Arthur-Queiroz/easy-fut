import { defineStore } from 'pinia'

const STORAGE_KEY = 'fut-theme'

function initialTheme(): 'light' | 'dark' {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'light' || saved === 'dark') return saved
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export const useThemeStore = defineStore('theme', {
  state: () => ({ theme: initialTheme() }),
  actions: {
    apply() {
      document.documentElement.classList.toggle('dark', this.theme === 'dark')
    },
    toggle() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark'
      localStorage.setItem(STORAGE_KEY, this.theme)
      this.apply()
    },
  },
})
