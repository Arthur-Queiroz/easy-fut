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
  <aside
    class="sticky top-0 hidden h-dvh w-[216px] shrink-0 flex-col px-3 py-[18px] lg:flex"
    style="background-image: linear-gradient(180deg, #0c100f, #13251f)"
  >
    <router-link to="/" class="flex items-center gap-2.5 px-2 pb-[18px]" aria-label="Easy Fut — início">
      <span class="flex h-[34px] w-[34px] items-center justify-center rounded-full border-[1.5px] border-lime bg-lime/10 text-lime">
        <NavIcon name="ball" :size="18" :stroke-width="1.7" />
      </span>
      <span class="font-condensed text-[15px] font-bold leading-[1.1] tracking-[.1em] text-lime">EASY FUT</span>
    </router-link>

    <nav aria-label="Navegação principal" class="flex flex-col gap-0.5">
      <router-link
        v-for="item in items"
        :key="item.name"
        :to="item.to"
        :aria-current="isActive(item.name) ? 'page' : undefined"
        class="flex items-center gap-2.5 rounded-[10px] px-3 py-2 text-[13.5px] transition-colors"
        :class="isActive(item.name) ? 'bg-lime/15 font-semibold text-lime' : 'font-medium text-white/70 hover:bg-white/5 hover:text-white'"
      >
        <NavIcon :name="item.icon" :size="17" />
        {{ item.label }}
      </router-link>
    </nav>

    <div class="flex-1" />

    <router-link
      v-if="auth.isAdmin"
      to="/admin"
      class="mb-2 flex items-center gap-2.5 rounded-[10px] bg-lime/10 px-3 py-2 text-[13.5px] font-semibold text-lime hover:bg-lime/15"
    >
      <NavIcon name="grid" :size="17" />
      Painel admin
    </router-link>

    <button
      type="button"
      class="flex items-center gap-2.5 rounded-[10px] px-3 py-2 text-left text-[13.5px] font-medium text-white/70 hover:bg-white/5 hover:text-white"
      @click="theme.toggle()"
    >
      <svg v-if="theme.theme === 'dark'" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
        <circle cx="12" cy="12" r="4.5" />
        <path d="M12 2.5v2.5M12 19v2.5M2.5 12H5M19 12h2.5M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M19.1 4.9l-1.8 1.8M6.7 17.3l-1.8 1.8" />
      </svg>
      <svg v-else width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
        <path d="M20 14.5A8.5 8.5 0 019.5 4a8.5 8.5 0 1010.5 10.5z" />
      </svg>
      {{ theme.theme === 'dark' ? 'Tema claro' : 'Tema escuro' }}
    </button>

    <div class="mt-2 flex items-center gap-2.5 border-t border-white/10 px-2 py-3">
      <router-link
        to="/perfil"
        :aria-current="route.name === 'profile' ? 'page' : undefined"
        class="flex min-w-0 flex-1 items-center gap-2.5 rounded-lg py-1 hover:bg-white/5"
        :class="route.name === 'profile' ? 'bg-lime/15' : ''"
      >
        <Avatar :name="auth.user?.name ?? ''" :url="auth.user?.avatar_url" color="#2EE07C" size="sm" />
        <span class="min-w-0">
          <span class="block truncate text-[13px] font-semibold text-white">{{ auth.firstName }}</span>
          <span class="block text-[10.5px] font-medium text-white/50">Meu perfil</span>
        </span>
      </router-link>
      <button type="button" class="rounded-lg p-2 text-white/50 hover:bg-white/5 hover:text-white" aria-label="Sair" title="Sair" @click="logout">
        <NavIcon name="x" :size="14" :stroke-width="2" />
      </button>
    </div>
  </aside>
</template>
