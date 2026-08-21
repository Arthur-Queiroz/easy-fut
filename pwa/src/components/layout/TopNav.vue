<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useThemeStore } from '../../stores/theme'
import Avatar from '../ui/Avatar.vue'
import NavIcon from './NavIcon.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()

const items = [
  { name: 'home', label: 'Início', to: '/', icon: 'home' },
  { name: 'match', label: 'Partida', to: '/partida', icon: 'ball' },
  { name: 'payments', label: 'Pagamentos', to: '/pagamentos', icon: 'pix' },
  { name: 'history', label: 'Histórico', to: '/historico', icon: 'clock' },
]

function isActive(name: string): boolean {
  if (name === 'match') return route.name === 'match' || route.name === 'teams'
  if (name === 'history') return route.name === 'history' || route.name === 'match-detail'
  return route.name === name
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <header
    class="hidden h-16 shrink-0 items-center justify-between px-8 lg:flex"
    style="background-image: linear-gradient(135deg, #0c100f 0%, #13251f 100%)"
  >
    <div class="flex items-center gap-8">
      <router-link to="/" class="flex items-center gap-2.5">
        <span
          class="flex h-[30px] w-[30px] items-center justify-center rounded-full border-[1.5px] border-lime bg-lime/10 text-lime"
        >
          <NavIcon name="ball" :size="16" :stroke-width="1.7" />
        </span>
        <span class="font-condensed text-[17px] font-extrabold leading-[1.1] tracking-[.14em] text-lime">
          EASY FUT
        </span>
      </router-link>

      <nav class="flex gap-1">
        <router-link
          v-for="item in items"
          :key="item.name"
          :to="item.to"
          class="rounded-[9px] px-3.5 py-[7px] text-[13.5px] font-medium transition-colors"
          :class="isActive(item.name) ? 'bg-lime/15 font-semibold text-lime' : 'text-white/70 hover:text-white'"
        >
          {{ item.label }}
        </router-link>
      </nav>
    </div>

    <div class="flex items-center gap-3">
      <button
        type="button"
        class="flex h-10 w-10 items-center justify-center rounded-[11px] text-white/70 transition-colors hover:bg-white/10 hover:text-white"
        :title="theme.theme === 'dark' ? 'Tema claro' : 'Tema escuro'"
        @click="theme.toggle()"
      >
        <svg v-if="theme.theme === 'dark'" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <circle cx="12" cy="12" r="4.5" />
          <path d="M12 2.5v2.5M12 19v2.5M2.5 12H5M19 12h2.5M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M19.1 4.9l-1.8 1.8M6.7 17.3l-1.8 1.8" />
        </svg>
        <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <path d="M20 14.5A8.5 8.5 0 019.5 4a8.5 8.5 0 1010.5 10.5z" />
        </svg>
      </button>

      <router-link to="/perfil" class="flex items-center gap-2.5 rounded-[11px] px-2 py-1 transition-colors hover:bg-white/10">
        <span class="text-[13px] font-medium text-white/70">{{ auth.firstName }}</span>
        <Avatar :name="auth.user?.name ?? ''" color="#2EE07C" size="sm" />
      </router-link>

      <button
        type="button"
        class="ml-1 flex h-9 items-center justify-center rounded-[9px] px-3 text-[12.5px] font-semibold text-white/70 hover:bg-white/10 hover:text-white"
        title="Sair"
        @click="logout"
      >
        Sair
      </button>
    </div>
  </header>
</template>
